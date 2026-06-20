"""In-memory cache with per-entry TTL expiry and tag-based invalidation.

Every read and write takes ``now`` (the caller's current time, a number) as an
explicit argument, so the cache is fully deterministic — the caller owns time.

On top of plain ``get`` / ``set`` this adds two features:

* **Per-entry TTL.** ``set(key, value, now, ttl=...)`` records an expiry of
  ``now + ttl``. A ``get`` at or after that instant is a MISS (the entry is
  treated as gone). ``ttl=None`` (the default) means the entry never expires.
  A ``ttl`` of 0 or less means the entry is already expired the moment it is
  set. The expiry boundary is HALF-OPEN: an entry set with ``ttl=T`` at ``t0``
  is a hit for every ``now`` in ``[t0, t0 + T)`` and a miss at and after
  ``t0 + T``.

* **Tag-based invalidation.** ``set(key, value, now, tags=[...])`` attaches a
  set of tags to the entry. ``invalidate_tag(t)`` drops every *live* entry that
  carries tag ``t``.

Subtle, deliberately-tricky semantics
-------------------------------------
1. An EXPIRED entry must also lose its tag membership. Once an entry's TTL has
   elapsed it is gone in every sense: ``invalidate_tag`` on one of its (former)
   tags is a no-op for it, and it never resurfaces. (A naive tag index that
   keeps pointing at expired keys, or that re-deletes already-expired entries,
   gets this wrong.)
2. Re-``set``ting an existing key REPLACES its tags wholesale: the key loses its
   old tags and carries only the new ones. A subsequent ``invalidate_tag`` with
   an OLD tag must not touch the re-set entry. (A naive union-of-tags index gets
   this wrong.)
3. Plain ``get`` / ``set`` (no ttl, no tags) must keep working exactly as
   before.

Implementation notes
---------------------
Each entry stores its value, an absolute ``expires_at`` (or ``None``), and its
tag set. A reverse index maps ``tag -> set(keys)``. Expiry is resolved lazily on
access: any read that observes an entry past its expiry removes it AND prunes it
from the tag index, so the index never points at a dead key. ``invalidate_tag``
first drops keys whose entries have themselves expired (without counting them),
then deletes the still-live keys carrying the tag.
"""

from __future__ import annotations

from typing import Any, Hashable, Iterable, Optional


class _Entry:
    __slots__ = ("value", "expires_at", "tags")

    def __init__(self, value: Any, expires_at: Optional[float], tags: frozenset) -> None:
        self.value = value
        self.expires_at = expires_at
        self.tags = tags

    def expired(self, now: float) -> bool:
        # Half-open boundary: expired AT and after expires_at.
        return self.expires_at is not None and now >= self.expires_at


class Cache:
    """In-memory cache with per-entry TTL and tag-based invalidation."""

    def __init__(self) -> None:
        self._data: dict[Hashable, _Entry] = {}
        # Reverse index: tag -> set of keys currently carrying that tag.
        self._tag_index: dict[Any, set] = {}

    # -- internals ---------------------------------------------------------

    def _unindex(self, key: Hashable, entry: _Entry) -> None:
        """Remove ``key`` from the tag index for every tag the entry carried."""
        for tag in entry.tags:
            keys = self._tag_index.get(tag)
            if keys is not None:
                keys.discard(key)
                if not keys:
                    del self._tag_index[tag]

    def _index(self, key: Hashable, tags: frozenset) -> None:
        """Add ``key`` to the tag index under each of ``tags``."""
        for tag in tags:
            self._tag_index.setdefault(tag, set()).add(key)

    def _drop(self, key: Hashable) -> None:
        """Remove an entry entirely (data + tag index). Caller ensures it exists."""
        entry = self._data.pop(key, None)
        if entry is not None:
            self._unindex(key, entry)

    def _evict_if_expired(self, key: Hashable, now: float) -> bool:
        """If ``key``'s entry is expired at ``now``, drop it (data + tags).

        Returns True if the entry was present and expired (and is now gone).
        """
        entry = self._data.get(key)
        if entry is not None and entry.expired(now):
            self._drop(key)
            return True
        return False

    # -- core operations ---------------------------------------------------

    def set(
        self,
        key: Hashable,
        value: Any,
        now: float,
        ttl: Optional[float] = None,
        tags: Optional[Iterable] = None,
    ) -> None:
        """Store ``value`` under ``key``.

        ``ttl`` (if given) sets an absolute expiry of ``now + ttl``; ``None``
        means no expiry. ``tags`` (if given) REPLACE any tags the key currently
        has. Re-setting a key always overwrites its value, expiry and tags.
        """
        # Re-setting replaces tags wholesale: scrub the old tag membership first.
        old = self._data.get(key)
        if old is not None:
            self._unindex(key, old)

        expires_at = None if ttl is None else now + ttl
        tag_set = frozenset(tags) if tags else frozenset()
        self._data[key] = _Entry(value, expires_at, tag_set)
        self._index(key, tag_set)

    def get(self, key: Hashable, now: float, default: Any = None) -> Any:
        """Return the value under ``key``, or ``default`` if absent or expired.

        An entry whose TTL has elapsed is treated as a miss and is evicted
        (along with its tag membership) on access.
        """
        entry = self._data.get(key)
        if entry is None:
            return default
        if entry.expired(now):
            self._drop(key)
            return default
        return entry.value

    def delete(self, key: Hashable) -> bool:
        """Remove ``key``; return True if it was present (regardless of expiry)."""
        if key in self._data:
            self._drop(key)
            return True
        return False

    def invalidate_tag(self, tag: Any, now: float) -> int:
        """Drop every LIVE entry carrying ``tag``. Return how many were dropped.

        Entries that have themselves already expired are NOT counted (they are
        gone in every sense); they are simply pruned from the index.
        """
        keys = self._tag_index.get(tag)
        if not keys:
            return 0
        dropped = 0
        # Snapshot: dropping mutates the index set we're iterating.
        for key in list(keys):
            entry = self._data.get(key)
            if entry is None:
                continue
            if entry.expired(now):
                # Expired entry: it has already lost its tag membership in
                # spirit; prune it without counting it as an invalidation.
                self._drop(key)
                continue
            self._drop(key)
            dropped += 1
        return dropped

    def __len__(self) -> int:
        """Number of stored entries (including any not-yet-evicted expired ones)."""
        return len(self._data)
