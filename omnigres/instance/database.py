from typing import ContextManager

import psycopg
from psycopg_pool import ConnectionPool


class Database:
    def __init__(self, pool: ConnectionPool, name: str):
        self.pool = pool
        self.name = name

    def __repr__(self):
        return f"Database({self.name}{("*" if self.has_omni_cli else "")})"

    def connection(self) -> ContextManager[psycopg.Connection]:
        return self.pool.connection()

    @property
    def has_omni_cli(self):
        with self.connection() as conn:
            with conn.cursor() as cur:
                cur.execute("select from pg_extension where extname = 'omni_cli'")
                return cur.fetchone() is not None


class DatabaseCollection:

    def __init__(self, databases: list[Database]):
        self.databases = databases

    def __iter__(self):
        return iter(sorted(self.databases, key=lambda db: not db.has_omni_cli))

    def __getitem__(self, index):
        if index is None:
            raise KeyError("None is not a valid index for a database collection")
        if isinstance(index, str):
            return next(iter([db for db in self if db.name == index]))
        else:
            return self.databases[index]
