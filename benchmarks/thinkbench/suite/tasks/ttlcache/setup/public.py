"""In-memory TTL cache with an injectable clock.

The cache stores ``key -> value`` pairs, each tagged with an absolute expiry
time computed from the ``ttl`` (time-to-live, in clock units) supplied at
``set`` time. Once an entry's TTL has elapsed it must be treated as a miss.

The clock is injectable so behaviour is fully deterministic in tests: pass any
zero-argument callable returning a monotonically non-decreasing number. When no
clock is given, the wall clock (``time.monotonic``) is used.

Example
-------
    >>> ticks = [0]
    >>> c = Cache(clock=lambda: ticks[0])
    >>> c.set("a", 1, ttl=10)
    >>> ticks[0] = 5
    >>> c.get("a")
    1
    >>> ticks[0] = 10        # TTL has elapsed
    >>> c.get("a") is None
    True
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Any, Callable, Hashable, Optional


@dataclass
class CacheStats:
    """Cumulative counters for cache activity."""

    hits: int = 0
    misses: int = 0
    expirations: int = 0

    def as_dict(self) -> dict:
        return {
            "hits": self.hits,
            "misses": self.misses,
            "expirations": self.expirations,
        }


class _Entry:
    __slots__ = ("value", "expires_at")

    def __init__(self, value: Any, expires_at: float) -> None:
        self.value = value
        self.expires_at = expires_at


class Cache:
    """A simple TTL cache.

    Parameters
    ----------
    clock:
        Zero-argument callable returning the current time as a number. Defaults
        to :func:`time.monotonic`.
    """

    def __init__(self, clock: Optional[Callable[[], float]] = None) -> None:
        self._clock: Callable[[], float] = clock if clock is not None else time.monotonic
        self._store: dict[Hashable, _Entry] = {}
        self._stats = CacheStats()

    # -- core operations ---------------------------------------------------

    def set(self, key: Hashable, value: Any, ttl: float) -> None:
        """Store ``value`` under ``key``, expiring ``ttl`` units from now.

        A ``ttl`` of zero (or negative) stores an entry that is already expired.
        """
        now = self._clock()
        self._store[key] = _Entry(value, now + ttl)

    def _is_expired(self, entry: _Entry, now: float) -> bool:
        # An entry is fresh up to and including its expiry instant; it becomes a
        # miss strictly after that. (See get/contains.)
        return now > entry.expires_at

    def get(self, key: Hashable, default: Any = None) -> Any:
        """Return the value for ``key``, or ``default`` if missing/expired."""
        entry = self._store.get(key)
        if entry is None:
            self._stats.misses += 1
            return default
        now = self._clock()
        if self._is_expired(entry, now):
            # Lazily evict expired entries on access.
            del self._store[key]
            self._stats.misses += 1
            self._stats.expirations += 1
            return default
        self._stats.hits += 1
        return entry.value

    def contains(self, key: Hashable) -> bool:
        """Return True iff ``key`` is present and not expired (no stats effect)."""
        entry = self._store.get(key)
        if entry is None:
            return False
        if self._is_expired(entry, self._clock()):
            return False
        return True

    def __contains__(self, key: Hashable) -> bool:
        return self.contains(key)

    def delete(self, key: Hashable) -> bool:
        """Remove ``key``; return True if it was present (expired or not)."""
        if key in self._store:
            del self._store[key]
            return True
        return False

    def ttl_remaining(self, key: Hashable) -> Optional[float]:
        """Return remaining TTL for ``key``, or ``None`` if missing/expired."""
        entry = self._store.get(key)
        if entry is None:
            return None
        now = self._clock()
        if self._is_expired(entry, now):
            return None
        return entry.expires_at - now

    # -- maintenance -------------------------------------------------------

    def purge(self) -> int:
        """Evict all currently-expired entries; return the number removed."""
        now = self._clock()
        expired = [k for k, e in self._store.items() if self._is_expired(e, now)]
        for k in expired:
            del self._store[k]
        self._stats.expirations += len(expired)
        return len(expired)

    def clear(self) -> None:
        """Drop all entries (stats are preserved)."""
        self._store.clear()

    def __len__(self) -> int:
        """Number of live (non-expired) entries."""
        now = self._clock()
        return sum(1 for e in self._store.values() if not self._is_expired(e, now))

    @property
    def stats(self) -> CacheStats:
        return self._stats
