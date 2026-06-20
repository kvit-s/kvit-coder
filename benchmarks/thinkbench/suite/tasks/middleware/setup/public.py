"""A tiny in-process request router.

``Router`` maps registered string paths to handler callables and dispatches a
path to its handler. Nothing here touches the network: it is a plain object you
drive by hand.

A handler is any callable taking a single ``request`` object (here just the path
string) and returning a ``response`` (any object). ``add`` registers a handler
for a path; ``dispatch`` looks the path up and calls its handler.

There is NO middleware support yet: ``dispatch`` simply finds the handler and
calls it. The task is to add before/after middleware (the onion model). See
``brief.txt`` for the contract.

Example
-------
    >>> r = Router()
    >>> r.add("/hi", lambda req: "hello")
    >>> r.dispatch("/hi")
    'hello'
    >>> r.dispatch("/missing")
    Traceback (most recent call last):
        ...
    middleware.public.NotFound: no handler for '/missing'
"""

from __future__ import annotations

from typing import Any, Callable

Handler = Callable[[Any], Any]


class NotFound(KeyError):
    """Raised by ``dispatch`` when no handler is registered for a path."""


class Router:
    """A minimal path-to-handler router (no middleware yet)."""

    def __init__(self) -> None:
        self._routes: dict[str, Handler] = {}

    def add(self, path: str, handler: Handler) -> None:
        """Register ``handler`` for ``path`` (last registration wins)."""
        self._routes[path] = handler

    def dispatch(self, path: str) -> Any:
        """Look up ``path`` and call its handler with the request (the path).

        Raises ``NotFound`` if no handler is registered for ``path``.
        """
        if path not in self._routes:
            raise NotFound(f"no handler for {path!r}")
        handler = self._routes[path]
        return handler(path)
