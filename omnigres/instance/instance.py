import os
import subprocess
from enum import Enum
from typing import ContextManager, Optional, Self

import psycopg
from psycopg.rows import namedtuple_row
from psycopg_pool import ConnectionPool

from .api import Dispatcher
from .database import Database, DatabaseCollection


class InstanceStatus(Enum):  # Supported
    STOPPED = 1
    RUNNING = 2


class Instance:
    _dispatcher: Optional[Dispatcher] = None

    @property
    def dispatcher(self) -> Dispatcher:
        if self._dispatcher is None:
            self._dispatcher = Dispatcher()
        return self._dispatcher

    @classmethod
    def list(cls) -> list[Self]:
        return [instance for subclass in cls.instance_classes() for instance in subclass.list()]

    @classmethod
    def instance_classes(cls):
        return cls.__subclasses__()

    @classmethod
    def init_workspace_database(cls, db: 'omnigres.workerspace.WorkspaceDatabase') -> str:
        for subclass in cls.instance_classes():
            subclass.init_workspace_database(db)

    @classmethod
    def __class_getitem__(cls, key):
        return next((instance for instance in cls.list() if instance.name() == key), None)

    def __init__(self):
        self.pool = {}

    def name(self) -> str:
        raise NotImplementedError("abstract instance type")

    def identifier(self) -> str:
        raise NotImplementedError("abstract instance type")

    def postgres_connection_string(self, dbname: str = None) -> str:
        raise NotImplementedError("abstract instance type")

    def postgres_connection(self, dbname: str = None) -> ContextManager[psycopg.Connection]:
        return self.postgres_connection_pool(dbname=dbname).connection()

    def postgres_connection_pool(self, dbname: str = None) -> ConnectionPool:
        if self.pool.get(dbname) is None:
            self.pool[dbname] = ConnectionPool(self.postgres_connection_string(dbname=dbname),
                                               name=f"{repr(self)}",
                                               open=True,
                                               configure=self._configure_connection)
        return self.pool[dbname]

    def _configure_connection(self, conn: psycopg.Connection):
        conn.add_notice_handler(self._notice_handler)

    def _notice_handler(self, notice: psycopg.errors.Diagnostic):
        self.dispatcher.dispatch(notice.message_primary)

    def databases(self) -> DatabaseCollection:
        with self.postgres_connection(dbname="postgres") as conn:
            with conn.cursor(row_factory=namedtuple_row) as cur:
                cur.execute("select datname from pg_database where not datistemplate")
                return DatabaseCollection(
                    [Database(self.postgres_connection_pool(row.datname), row.datname) for row in cur])

    def psql(self, dbname: str, psql: Optional[str] = None):
        if psql is None:
            psql = os.getenv('PSQL', "psql")
        subprocess.run(f"{psql} '{self.postgres_connection_string(dbname)}'",
                       shell=True, text=True)

    def start(self):
        raise NotImplementedError("abstract instance type")

    def stop(self):
        raise NotImplementedError("abstract instance type")

    def status(self) -> InstanceStatus:
        raise NotImplementedError("abstract instance type")

    def attach(self, workspace_db: 'omnigres.workspace.WorkspaceDatabase'):
        raise NotImplementedError("abstract instance type")

    def attached(self, workspace: 'omnigres.workspace.WorkspaceDatabase') -> bool:
        raise NotImplementedError("abstract instance type")
