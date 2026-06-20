"""A tiny in-process request router WITH onion-model middleware.

``Router`` maps registered string paths to handler callables and dispatches a
path to its handler. On top of the plain router it supports MIDDLEWARE: small
functions that wrap around the handler, forming concentric layers (the "onion").

A middleware is a callable ``fn(request, next)`` where:

* ``request`` is the request object (here the path string), and
* ``next`` is a zero-argument callable that runs the REST of the chain (the
  inner layers, ending in the handler) and returns that inner response.

A middleware returns the response for its layer. The classic shape is::

    def logging(request, next):
        # ... before logic ...
        response = next()        # run inner layers + handler
        # ... after logic, may transform `response` ...
        return response

Calling ``next()`` descends one layer inward; returning WITHOUT calling
``next()`` SHORT-CIRCUITS — the inner layers and the handler never run, but every
OUTER middleware still gets to run its after-logic on the way back out (because
each is still inside its own ``next()`` call). That is the onion model: before-
logic runs in registration order (outer -> inner), after-logic unwinds in
reverse (inner -> outer).

Example
-------
    >>> r = Router()
    >>> r.add("/hi", lambda req: "hello")
    >>> def shout(request, next):
    ...     return next().upper()
    >>> r.use(shout)
    >>> r.dispatch("/hi")
    'HELLO'
"""

from __future__ import annotations

from typing import Any, Callable

Handler = Callable[[Any], Any]
Middleware = Callable[[Any, Callable[[], Any]], Any]


class NotFound(KeyError):
    """Raised by ``dispatch`` when no handler is registered for a path."""


class Router:
    """A path-to-handler router with onion-model before/after middleware."""

    def __init__(self) -> None:
        self._routes: dict[str, Handler] = {}
        self._middleware: list[Middleware] = []

    def add(self, path: str, handler: Handler) -> None:
        """Register ``handler`` for ``path`` (last registration wins)."""
        self._routes[path] = handler

    def use(self, fn: Middleware) -> None:
        """Register a middleware. Middleware run in registration order on the
        way in (before the handler) and in reverse on the way out (after)."""
        self._middleware.append(fn)

    def dispatch(self, path: str) -> Any:
        """Run the middleware chain around the handler for ``path``.

        The chain is built so that ``self._middleware[0]`` is the OUTERMOST
        layer (its before-logic runs first, its after-logic runs last) and the
        handler sits at the centre. A middleware that returns without calling
        ``next()`` short-circuits the inner layers and the handler, but the
        already-entered outer layers still run their after-logic.

        Raises ``NotFound`` if no handler is registered for ``path`` AND the
        innermost layer is actually reached (a short-circuiting middleware can
        prevent the lookup from ever happening).
        """

        # The core: look the handler up lazily, so a short-circuit before the
        # centre never triggers NotFound for an unregistered path.
        def core() -> Any:
            if path not in self._routes:
                raise NotFound(f"no handler for {path!r}")
            return self._routes[path](path)

        # Fold the middleware list into nested closures, innermost-last. We walk
        # registration order in REVERSE so that index 0 ends up outermost.
        chain: Callable[[], Any] = core
        for fn in reversed(self._middleware):
            chain = self._wrap(fn, path, chain)
        return chain()

    @staticmethod
    def _wrap(fn: Middleware, request: Any, inner: Callable[[], Any]) -> Callable[[], Any]:
        """Bind one middleware ``fn`` so that calling the result runs ``fn`` with
        ``next`` set to ``inner`` (the rest of the chain)."""

        def layer() -> Any:
            return fn(request, inner)

        return layer
