"""middleware — a tiny request router with onion-model middleware.

Public API is re-exported here for convenience; the implementation lives in
``middleware.public``.

    >>> from middleware import Router
    >>> r = Router()
    >>> r.add("/hi", lambda req: "hello")
    >>> r.dispatch("/hi")
    'hello'
"""

from .public import NotFound, Router

__all__ = ["Router", "NotFound"]
