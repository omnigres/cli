import inspect
import types
from abc import ABC, abstractmethod
from typing import Callable, Optional

import jsonrpcserver


class APIHandler(ABC):

    def handles_methods(self):
        methods = {}
        for klass in inspect.getmro(self.__class__):
            for name, method in vars(klass).items():
                if callable(method) and hasattr(method, '_handles'):
                    methods[method._handles] = types.MethodType(getattr(self.__class__, method.__name__), self)
        return methods


def handles(method: str):
    def decorate(f):
        f._handles = method
        return f

    return decorate


class ConsoleAPIHandler(APIHandler):

    @abstractmethod
    @handles("console.print")
    def print(self, msg: str):
        raise NotImplementedError("abstract API handler")


class FileAPIHandler(APIHandler):

    @abstractmethod
    @handles("file.list_dir")
    def list_dir(self, path: str):
        raise NotImplementedError("abstract API handler")


def _wrapper(func):
    def f(*args, **kwargs):
        try:
            return jsonrpcserver.Success(func(*args, **kwargs))
        except Exception as e:
            import logging
            logging.exception(e)
            return jsonrpcserver.Error(code=0, message=str(e))

    return f


class Dispatcher:
    handlers: list[APIHandler] = []
    methods: dict[str, Callable] = {}

    def __init__(self, handlers: Optional[list[APIHandler]] = None):
        self.set_handlers(*(handlers or []))

    def set_handlers(self, *handlers):
        if handlers is None:
            handlers = []
        self.handlers = handlers
        for handler in handlers:
            for method, handler_callable in handler.handles_methods().items():
                self.methods[method] = _wrapper(handler_callable)

    def dispatch(self, message):
        jsonrpcserver.dispatch(message, methods=self.methods)
