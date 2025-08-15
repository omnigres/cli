import os
import sqlite3
from typing import Optional

import rich
from InquirerPy import inquirer

from .instance.api import ConsoleAPIHandler, FileAPIHandler
from .instance.instance import Instance


class WorkspaceDatabase:
    workspace: Optional['Workspace'] = None
    database: Optional[sqlite3.Connection] = None

    def __init__(self, workspace: 'Workspace', name: str = "default"):
        self.workspace = workspace
        os.makedirs(os.path.join(self.workspace.path, ".omnigres"), exist_ok=True)
        db_file = os.path.join(self.workspace.path, ".omnigres", f"{name}.db")
        new_file = not os.path.exists(db_file)
        self.database = sqlite3.connect(db_file)
        if new_file:
            with self.database as db:
                db.executescript("""
                    create table metadata (
                      key text,
                      value text
                    );
                    insert into metadata values ('version', '0.1.0');
                """)
                Instance.init_workspace_database(self)
        with self.database as db:
            cur = db.execute("select value from metadata where key = 'version'")
            version = cur.fetchone()[0]
            if version != "0.1.0":
                raise ValueError("Workspace database is not compatible with this version of omnigres CLI")
        pass


from pathlib import Path


class ConstrainedPath:
    def __init__(self, root_dir):
        self.root = Path(root_dir).resolve()

    def resolve_path(self, user_path):
        # Treat all paths as relative to root
        if user_path.startswith('/'):
            user_path = user_path[1:]  # Remove leading slash

        # Resolve the path within the constraint
        full_path = (self.root / user_path).resolve()

        # Ensure it's still within the root directory
        if not full_path.is_relative_to(self.root):
            raise ValueError("Path escapes the constrained directory")

        return full_path


class WorkspaceHandler(ConsoleAPIHandler, FileAPIHandler):
    def __init__(self, workspace: 'Workspace'):
        self.workspace = workspace

    def print(self, msg: str):
        rich.print(msg)

    def list_dir(self, path: str):
        constrained_path = ConstrainedPath(self.workspace.path)
        resolved_path = constrained_path.resolve_path(path)
        allow = inquirer.confirm(amark=">",
                                 message=f"Received a request access directory {resolved_path}, allow?").execute()
        if allow:
            rich.print(os.listdir(resolved_path))
            return os.listdir(resolved_path)
        else:
            raise PermissionError("Access denied")


class Workspace:
    path: str = None
    db: Optional[WorkspaceDatabase] = None

    def __init__(self, path: str):
        self.path = path
        if not os.path.exists(path):
            raise ValueError("Workspace path does not exist")
        # if not os.path.exists(os.path.join(path, "omnigres.yaml")):
        #     raise ValueError("Directory does not have omnigres.yaml – not a valid workspace")
        self.db = WorkspaceDatabase(self)
        self.instance = self.attached_instance()
        if self.instance:
            self.instance.dispatcher.set_handlers(WorkspaceHandler(self))

    def __repr__(self):
        return f"Workspace<{self.path}>"

    def attached_instance(self) -> Optional[Instance]:
        return next(iter([instance for instance in Instance.list() if instance.attached(self.db)]), None)
