"""ttlcache — a tiny in-memory cache with per-entry time-to-live (TTL).

Public API is re-exported here for convenience; the implementation lives in
``ttlcache.public``.

    >>> from ttlcache import Cache
    >>> c = Cache()
    >>> c.set("k", "v", ttl=10)
    >>> c.get("k")
    'v'
"""

from .public import Cache, CacheStats

__all__ = ["Cache", "CacheStats"]
