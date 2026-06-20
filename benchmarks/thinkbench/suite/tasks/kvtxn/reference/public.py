"""In-memory key-value store with NESTED transactions (savepoints).

The store holds ``key -> value`` pairs. On top of ``get`` / ``set`` /
``delete`` it supports nestable transactions:

* ``begin()`` opens a new savepoint, nested inside any currently-open one.
* ``rollback()`` discards every change made since the matching ``begin()``,
  restoring keys to exactly the values they held at that ``begin()`` — and
  ONLY those; changes already committed by an enclosing scope are untouched
  because each savepoint only knows about the keys it itself first modified.
* ``commit()`` folds the current savepoint's changes into the ENCLOSING scope
  (the parent savepoint, or the durable store when at top level). Folding into
  a parent means the parent now "owns" those changes: a later rollback of the
  parent still undoes them.

Implementation: an undo log per open savepoint. Each savepoint records, for
every key it is the FIRST in its scope to touch, the value that key held when
that savepoint began (or a sentinel meaning "absent"). Mutations write straight
through to the live store after stashing the original. Rollback replays the
undo log in any order (each key has exactly one recorded original per scope).
Commit merges the child's undo log into the parent's, but a key already present
in the parent's log keeps the PARENT's (older) original — so the parent can
still rewind past both scopes.
"""

from __future__ import annotations

from typing import Any, Hashable

# Sentinel recorded in an undo log when a key was ABSENT at savepoint start.
_ABSENT = object()


class TransactionError(RuntimeError):
    """Raised when commit/rollback is called with no open transaction."""


class Store:
    """An in-memory key-value store supporting nested transactions."""

    def __init__(self) -> None:
        self._data: dict[Hashable, Any] = {}
        # Stack of undo logs, one per open savepoint. Each maps
        # key -> original value (or _ABSENT) captured the first time this scope
        # touches that key.
        self._txns: list[dict[Hashable, Any]] = []

    # -- core operations ---------------------------------------------------

    def get(self, key: Hashable, default: Any = None) -> Any:
        """Return the value for ``key``, or ``default`` if absent."""
        return self._data.get(key, default)

    def set(self, key: Hashable, value: Any) -> None:
        """Store ``value`` under ``key`` (records undo info if in a txn)."""
        self._record(key)
        self._data[key] = value

    def delete(self, key: Hashable) -> bool:
        """Remove ``key``; return True if it was present."""
        present = key in self._data
        self._record(key)
        if present:
            del self._data[key]
        return present

    # -- transactions ------------------------------------------------------

    def begin(self) -> None:
        """Open a new (possibly nested) savepoint."""
        self._txns.append({})

    def rollback(self) -> None:
        """Undo all changes since the matching ``begin()``."""
        if not self._txns:
            raise TransactionError("rollback() with no active transaction")
        undo = self._txns.pop()
        for key, original in undo.items():
            if original is _ABSENT:
                self._data.pop(key, None)
            else:
                self._data[key] = original

    def commit(self) -> None:
        """Fold the current savepoint into the enclosing scope (or the store)."""
        if not self._txns:
            raise TransactionError("commit() with no active transaction")
        undo = self._txns.pop()
        if self._txns:
            # Fold into the parent: the parent inherits responsibility for
            # undoing these keys. A key the parent already tracks keeps the
            # parent's (older) original, so an outer rollback rewinds past both.
            parent = self._txns[-1]
            for key, original in undo.items():
                if key not in parent:
                    parent[key] = original
        # Top-level commit: changes are already in self._data; nothing to undo.

    # -- internals ---------------------------------------------------------

    @property
    def depth(self) -> int:
        """Number of currently-open (nested) transactions."""
        return len(self._txns)

    def _record(self, key: Hashable) -> None:
        """Stash the pre-mutation value of ``key`` in the active savepoint.

        Only the FIRST touch of a key within a scope is recorded; later touches
        within the same scope leave the original intact so rollback rewinds all
        the way to the savepoint's start.
        """
        if not self._txns:
            return
        top = self._txns[-1]
        if key in top:
            return
        top[key] = self._data[key] if key in self._data else _ABSENT
