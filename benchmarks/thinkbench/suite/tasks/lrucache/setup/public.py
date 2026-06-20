"""A fixed-capacity least-recently-used (LRU) cache.

An :class:`LRUCache` holds at most ``capacity`` key/value entries. When a new
key is inserted into a full cache, the *least recently used* entry -- the one
whose key has gone the longest without a successful ``get`` or ``put`` -- is
evicted to make room.

"Use" means a *successful* access: both a ``get`` that finds the key and a
``put`` that stores it (whether inserting a new key or overwriting an existing
one) mark that key as the most recently used.

    >>> c = LRUCache(capacity=2)
    >>> c.put("a", 1)
    >>> c.put("b", 2)
    >>> c.get("a")           # "a" is now most-recently-used
    1
    >>> c.put("c", 3)        # full -> evict the LRU, which is "b"
    >>> c.get("b") is c.MISSING
    True
    >>> c.get("a")
    1
"""

from __future__ import annotations

from collections import OrderedDict
from typing import Any, Hashable


class _Missing:
    """Singleton sentinel returned by :meth:`LRUCache.get` on a miss."""

    __slots__ = ()

    def __repr__(self) -> str:  # pragma: no cover - cosmetic
        return "MISSING"


MISSING = _Missing()


class LRUCache:
    """A least-recently-used cache with a fixed capacity.

    Parameters
    ----------
    capacity:
        The maximum number of entries the cache may hold (a non-negative int).
        A capacity of 0 means the cache never stores anything.
    """

    #: Sentinel object returned by :meth:`get` when the key is absent.
    MISSING = MISSING

    def __init__(self, capacity: int) -> None:
        if capacity < 0:
            raise ValueError("capacity must be non-negative")
        self.capacity = capacity
        # An OrderedDict ordered from LRU (front/left) to MRU (back/right).
        self._data: "OrderedDict[Hashable, Any]" = OrderedDict()

    def get(self, key: Hashable) -> Any:
        """Return the value for ``key``, marking it most-recently-used.

        On a hit the value is returned; on a miss the :attr:`MISSING` sentinel
        is returned.
        """
        if key not in self._data:
            return MISSING
        return self._data[key]

    def put(self, key: Hashable, value: Any) -> None:
        """Insert or update ``key`` -> ``value``, marking it most-recently-used.

        Overwriting an existing key updates its value. Inserting a new key into
        a full cache evicts the least-recently-used entry to make room.
        """
        if key in self._data:
            # Existing key: update the stored value.
            self._data[key] = value
            return
        # New key: if we are over capacity, drop an entry to make room, then
        # store the new value.
        if len(self._data) > self.capacity:
            self._data.popitem(last=True)
        self._data[key] = value

    def __len__(self) -> int:
        return len(self._data)

    def __contains__(self, key: Hashable) -> bool:
        # Membership test must NOT change recency.
        return key in self._data
