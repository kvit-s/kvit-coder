"""In-memory cache keyed by ``key``, with an explicitly-injected clock.

Every read and write takes ``now`` (a number — the caller's current time) as an
explicit argument rather than reading a real clock. This keeps the cache fully
deterministic and trivially testable: the caller owns time. The core operations
``get`` / ``set`` work today; there is NO expiry and NO tagging yet.

The task (see ``brief.txt``) is to ADD per-entry TTL expiry and tag-based
invalidation on top of this base, without breaking plain ``get`` / ``set``.

Example
-------
    >>> c = Cache()
    >>> c.set("a", 1, now=0)
    >>> c.get("a", now=5)
    1
    >>> c.get("missing", now=5) is None
    True
"""

from __future__ import annotations

from typing import Any, Hashable, Optional


class Cache:
    """A simple in-memory cache. Values never expire; there is no tagging."""

    def __init__(self) -> None:
        # key -> value. (No expiry metadata, no tags — that's the feature to add.)
        self._data: dict[Hashable, Any] = {}

    def set(self, key: Hashable, value: Any, now: float) -> None:
        """Store ``value`` under ``key``.

        ``now`` is the caller's current time. The base cache does not use it
        (entries never expire), but it is part of the signature so that adding
        TTL support does not change how callers invoke ``set``.
        """
        self._data[key] = value

    def get(self, key: Hashable, now: float, default: Any = None) -> Any:
        """Return the value stored under ``key``, or ``default`` if absent.

        ``now`` is the caller's current time (unused by the base cache).
        """
        if key in self._data:
            return self._data[key]
        return default

    def delete(self, key: Hashable) -> bool:
        """Remove ``key``; return True if it was present."""
        if key in self._data:
            del self._data[key]
            return True
        return False

    def __len__(self) -> int:
        """Number of stored entries."""
        return len(self._data)
