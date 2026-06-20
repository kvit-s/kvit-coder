"""lrucache -- a fixed-capacity least-recently-used (LRU) cache.

Public API is re-exported here for convenience; the implementation lives in
``lrucache.public``.

    >>> from lrucache import LRUCache
    >>> c = LRUCache(capacity=2)
    >>> c.put("a", 1)
    >>> c.get("a")
    1
"""

from .public import LRUCache

__all__ = ["LRUCache"]
