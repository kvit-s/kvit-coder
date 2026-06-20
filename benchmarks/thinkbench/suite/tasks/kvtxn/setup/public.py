"""In-memory key-value store.

Stores ``key -> value`` pairs. The core operations ``get`` / ``set`` /
``delete`` work today. There is a FIRST, SINGLE-LEVEL attempt at transactions
(``begin`` / ``commit`` / ``rollback``) that snapshots the whole store on
``begin`` and restores it on ``rollback`` — but it keeps only ONE snapshot, so
it does not support nesting (a second ``begin`` clobbers the first snapshot).

The task is to replace this with proper NESTED transactions (savepoints). See
``brief.txt`` for the contract.

Example
-------
    >>> s = Store()
    >>> s.set("a", 1)
    >>> s.get("a")
    1
    >>> s.begin()
    >>> s.set("a", 2)
    >>> s.rollback()
    >>> s.get("a")
    1
"""

from __future__ import annotations

from typing import Any, Hashable, Optional

_MISSING = object()


class Store:
    """A simple in-memory key-value store with a flat (non-nesting) txn attempt."""

    def __init__(self) -> None:
        self._data: dict[Hashable, Any] = {}
        # A single saved snapshot of the whole store, or None when no txn is open.
        self._snapshot: Optional[dict[Hashable, Any]] = None

    # -- core operations ---------------------------------------------------

    def get(self, key: Hashable, default: Any = None) -> Any:
        """Return the value for ``key``, or ``default`` if absent."""
        return self._data.get(key, default)

    def set(self, key: Hashable, value: Any) -> None:
        """Store ``value`` under ``key``."""
        self._data[key] = value

    def delete(self, key: Hashable) -> bool:
        """Remove ``key``; return True if it was present."""
        if key in self._data:
            del self._data[key]
            return True
        return False

    # -- transactions (flat, single-level — does NOT nest) -----------------

    def begin(self) -> None:
        """Open a transaction by snapshotting the whole store.

        NOTE: only one snapshot is kept, so nesting is broken — a second
        ``begin`` overwrites the first snapshot and loses the outer scope.
        """
        self._snapshot = dict(self._data)

    def commit(self) -> None:
        """Keep current changes; just drop the snapshot."""
        self._snapshot = None

    def rollback(self) -> None:
        """Restore the store to the last snapshot, if any."""
        if self._snapshot is not None:
            self._data = dict(self._snapshot)
            self._snapshot = None
