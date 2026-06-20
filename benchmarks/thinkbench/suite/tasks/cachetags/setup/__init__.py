"""cachetags — a tiny in-memory cache with an explicitly-injected clock.

Public API is re-exported here for convenience; the implementation lives in
``cachetags.public``.

    >>> from cachetags import Cache
    >>> c = Cache()
    >>> c.set("k", "v", now=0)
    >>> c.get("k", now=1)
    'v'
"""

from .public import Cache

__all__ = ["Cache"]
