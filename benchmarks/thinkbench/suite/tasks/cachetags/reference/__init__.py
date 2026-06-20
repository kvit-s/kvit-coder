"""cachetags — a tiny in-memory cache with per-entry TTL and tag invalidation.

Public API is re-exported here for convenience; the implementation lives in
``cachetags.public``.

    >>> from cachetags import Cache
    >>> c = Cache()
    >>> c.set("k", "v", now=0, ttl=10)
    >>> c.get("k", now=5)
    'v'
    >>> c.get("k", now=10) is None
    True
"""

from .public import Cache

__all__ = ["Cache"]
