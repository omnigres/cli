import logging
import os
import sys
from collections import namedtuple
from typing import List, Optional

import rich
import rich_click as click
from InquirerPy import inquirer
from psycopg import sql
from psycopg.rows import namedtuple_row

from omnigres.instance import Database, Instance
from omnigres.workspace import Workspace


class GlobalOptions:
    _instance = None

    verbose: bool = False
    instance: Optional[Instance] = None
    database: Optional[Database] = None
    workspace: Optional[Workspace] = None

    def __init__(self, dict):
        # FIXME: gets called multiple times
        self.verbose = dict.get('verbose', False)
        if dict.get('workspace', None) is not None:
            self.workspace = Workspace(dict['workspace'])
        else:
            self.workspace = Workspace(os.getcwd())
        if dict.get('instance', None) is not None:
            self.instance = Instance[dict['instance']]
        else:
            self.instance = self.workspace.attached_instance()
        if dict.get('db', None) is not None:
            self.database = self.instance.databases()[dict['db']]


logger = logging.getLogger("omnigres")


class DatabaseCommandGroup(click.RichGroup):

    def __init__(self, **kwargs):
        super().__init__(**kwargs)

    def list_commands(self, ctx) -> List[str]:
        options = GlobalOptions(ctx.params)
        dynamic_commands = []
        if options.database:
            with options.database.connection() as conn:
                with conn.cursor(row_factory=namedtuple_row) as cur:
                    cur.execute("select * from omni_cli.cli_command")
                    for cmd in cur:
                        dynamic_commands.append(cmd.name)
        return super().list_commands(ctx) + dynamic_commands

    def get_command(self, ctx, name: str) -> Optional[click.Command]:
        options = GlobalOptions(ctx.params)
        if name not in self.list_commands(ctx):
            return None
        subcommand = super().get_command(ctx, name)
        if subcommand:
            return subcommand
        if options.database:
            with options.database.connection() as conn:
                with conn.cursor(row_factory=namedtuple_row) as cur:
                    cur.execute("select * from omni_cli.cli_command where name = %s and parent is null", (name,))
                    res = cur.fetchone()
                    attrs = {}
                    cmd = self.command()(
                        lambda **kwargs: self.run_remote_command(ctx, __name__=name, __rel__=res, __attrs__=attrs,
                                                                 **kwargs))
                    with conn.cursor(row_factory=namedtuple_row) as cur:
                        cur.execute("select * from omni_cli.cli_command_argument where command_name = %s", (name,))
                        Arg = namedtuple("Arg", ["name", "column_name"])
                        processed_options = set(cmd.params)
                        for arg in cur:
                            click.option(*arg.names, help=arg.description, is_flag=arg.type == 'boolean')(cmd)
                            new_option = next(iter(list(set(cmd.params) - processed_options)))
                            attrs[new_option.name] = Arg(name, arg.column_name)
                            processed_options.add(new_option)

                        return cmd

        # Create command dynamically

    def run_remote_command(self, ctx, __name__, __rel__, __attrs__, **kwargs):
        cctx = click.get_current_context()
        options = GlobalOptions(ctx.params)
        if options.database:
            with options.database.connection() as conn:
                with conn.cursor(row_factory=namedtuple_row) as cur:
                    pass
                    value_holders = ([sql.Placeholder(key) for key in cctx.params.keys()])
                    columns = [sql.Identifier(__attrs__[name].column_name) for name in cctx.params.keys()]
                    query = sql.SQL("insert into {schema_name}.{relation_name} ({columns}) values ({values})").format(
                        schema_name=sql.Identifier(__rel__.schema_name),
                        relation_name=sql.Identifier(__rel__.relation_name),
                        columns=sql.SQL(', ').join(columns),
                        values=sql.SQL(', ').join(value_holders))
                    cur.execute(query, cctx.params)


@click.group(cls=DatabaseCommandGroup)
@click.option("--verbose", "-v", default=False, help="Enable verbose output", is_flag=True)
@click.option("--instance", "-i", default=None, help="Postgres instance", type=str)
@click.option("--db", "-d", default=None, help="Postgres database name", type=str)
@click.option("--workspace", "-w", default=None, help="Project workspace",
              type=click.Path(exists=True, file_okay=False, resolve_path=True))
@click.pass_context
def main(
        ctx: click.Context,
        verbose: bool,
        instance: str,
        db: str,
        workspace: str = None):
    options = GlobalOptions(ctx.params)

    if options.verbose:
        logger.setLevel(logging.DEBUG)

    if options.workspace:
        logger.debug("Workspace selected: %s", options.workspace)

    if options.instance:
        logger.debug("Instance selected: %s", options.instance)

    if options.database:
        logger.debug("Database selected: %s", options.database)

    ctx.meta['options'] = options


import logging
from rich.logging import RichHandler

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s", handlers=[RichHandler()]
)


@click.command()
@click.option("--template", "-t", help="Template to use for creating new project", type=str)
@click.argument("path", type=click.Path(dir_okay=True, file_okay=False))
def init(template: Optional[str], path):
    if os.path.exists(path):
        rich.print(f"Directory already exists, can't initialize it")
        sys.exit(1)
    name = os.path.basename(os.path.abspath(path))
    from cookiecutter.main import cookiecutter
    template_path = ".omnigres/templates"
    if template:
        template_path = os.path.join(template_path, template)
    if os.path.exists(template_path) and os.path.isdir(template_path):
        cookiecutter(template_path, output_dir=path, extra_context={"name": name})
        return
    rich.print(f"Template not found")
    sys.exit(1)


@click.command()
@click.pass_context
def instances(ctx: click.Context):
    for instance in Instance.list():
        print(instance.name())
        rich.inspect([db for db in instance.databases()])


@click.command()
@click.pass_context
def psql(ctx: click.Context):
    options = ctx.meta['options']
    instance = options.instance
    if options.database:
        dbname = options.database.name
    else:
        dbname = inquirer.select("Select a database", choices=[db.name for db in instance.databases()]).execute()
    instance.psql(dbname)


@click.group()
@click.pass_context
def instance(ctx: click.Context):
    pass


@click.command(name="attach")
@click.pass_context
@click.argument("instance", type=str, required=False)
def attach_instance(ctx: click.Context, instance: Optional[str]):
    if instance is None:
        instance = inquirer.select("Choose an instance to attach to",
                                   choices=[instance.name() for instance in Instance.list()]).execute()

    options = ctx.meta['options']
    workspace = options.workspace
    Instance[instance].attach(workspace.db)


instance.add_command(attach_instance)

main.add_command(init)
main.add_command(instance)
main.add_command(instances)
main.add_command(psql)

import atexit


@atexit.register
def close_psycopg_pools():
    # A workaround for a known issue
    # https://github.com/psycopg/psycopg/issues/930
    # https://github.com/psycopg/psycopg/issues/954
    import gc
    import psycopg_pool
    [obj.close() for obj in gc.get_objects() if isinstance(obj, psycopg_pool.ConnectionPool) if not obj.closed]


if __name__ == "__main__":
    main()
