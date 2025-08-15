import os
from functools import cached_property
from getpass import getuser
from typing import Optional

import psutil
from psycopg.rows import namedtuple_row

from .instance import Instance, InstanceStatus
from ..workspace import WorkspaceDatabase


class PostgresInstance(Instance):
    @classmethod
    def list(cls):
        c = [PostgresInstance(proc.cmdline()[2])
             for proc in psutil.process_iter(['pid', 'name', 'cmdline', 'username']) if
             proc.username() == getuser() and proc.name() == 'postgres' and proc.cmdline()[
                 1] == '-D' and os.path.exists(os.path.join(proc.cmdline()[2], "postmaster.pid"))]
        return [i for i in c if i._valid()]

    @classmethod
    def init_workspace_database(cls, db: 'omnigres.workerspace.WorkspaceDatabase') -> str:
        with db.database as db:
            db.executescript("""
              create table postgres_instance (
                path text
              );
              """)

    def __init__(self, path: str):
        super().__init__()
        with open(os.path.join(path, "postmaster.pid")) as f:
            self.pid = int(f.readlines()[0])
        self.path = path

    def __repr__(self):
        return f"<PostgresInstance {self.pid} {self.path}>"

    @cached_property
    def _port(self):
        with open(os.path.join(self.path, "postmaster.pid")) as f:
            c = f.read().splitlines()
            return int(c[3])

    def _valid(self) -> bool:
        with self.postgres_connection("postgres") as conn:
            with conn.cursor() as cur:
                cur.execute("select from pg_available_extensions where name = 'omni'")
                if cur.fetchone() is None:
                    return False
        return True

    def name(self) -> str:
        return self.path

    def identifier(self) -> str:
        return f"postgres(pid={self._port})"

    def postgres_connection_string(self, dbname: str = None) -> str:
        return f"dbname={dbname or "postgres"} user={getuser()} host=127.0.0.1 port={self._port}"

    def start(self):
        raise Exception("Cannot start an external instance")

    def stop(self):
        with self.postgres_connection("postgres") as conn:
            # TODO
            pass

    def status(self) -> InstanceStatus:
        with self.postgres_connection("postgres") as conn:
            return InstanceStatus.RUNNING

    def psql(self, dbname: str, psql: Optional[str] = None):
        if psql is None:
            with self.postgres_connection("postgres") as conn:
                with conn.cursor(row_factory=namedtuple_row) as cur:
                    cur.execute("select setting from pg_config where name = 'BINDIR'")
                    result = cur.fetchone()
                    if result:
                        psql = f"{result.setting}/psql"
        super().psql(dbname=dbname, psql=psql)

    def attach(self, workspace_db: WorkspaceDatabase):
        print(f"Attaching {self} to {workspace_db}")
        with workspace_db.database as db:
            cur = db.cursor()
            cur.execute("delete from postgres_instance")
            cur.execute("insert into postgres_instance (path) values (?)", (self.path,))

    def attached(self, workspace: WorkspaceDatabase) -> bool:
        with workspace.database as db:
            cur = db.cursor()
            cur.execute("select * from postgres_instance where path = ?", (self.path,))
            return cur.fetchone() is not None
