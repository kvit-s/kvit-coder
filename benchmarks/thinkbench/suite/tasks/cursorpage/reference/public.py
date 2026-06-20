"""Paginator over a list of records sorted by a key, with OFFSET and CURSOR
pagination.

Records are ``dict``s carrying at least an integer ``"id"`` (stable, unique)
and a sort key. They are kept in a fully-deterministic order: by ``key``
ascending, ties broken by ``"id"`` ascending.

Offset pagination (unchanged): ``page(n, size)`` returns the ``n``-th page.

Cursor pagination: ``page_after(cursor, size)`` returns ``{"items", "next_cursor"}``
where the cursor opaquely encodes the (sort-key, id) of the LAST record handed
out so far. Resumption is "strictly after that position" in the sorted order —
which is why ties are walked correctly: two records with the same sort key are
still distinguished by id, so iterating with ``next_cursor`` visits every record
exactly once with no duplicates and no gaps, even across ties. The final page
returns ``next_cursor=None``; an invalid or ``None`` cursor starts from the
beginning.

Implementation: the cursor is an opaque base64-encoded JSON ``[key, id]``. To
serve a page we find the first record whose (key, id) is strictly greater than
the cursor's (key, id) under the same (key, id) ordering, then take ``size``
records from there. We do NOT trust the cursor to point at a record that still
exists; we only use it as an ordering boundary.
"""

from __future__ import annotations

import base64
import json
from typing import Any, Dict, List, Optional

Record = Dict[str, Any]


class Paginator:
    """Paginate records sorted by ``key`` (ties broken by ``id``)."""

    def __init__(self, records: List[Record], key: str = "id") -> None:
        self._key = key
        self._records: List[Record] = sorted(
            records, key=lambda r: (r[key], r["id"])
        )

    # -- offset pagination (unchanged) -------------------------------------

    def page(self, n: int, size: int) -> List[Record]:
        """Return the ``n``-th page (0-based) of ``size`` records."""
        if size <= 0:
            raise ValueError("size must be positive")
        if n < 0:
            raise ValueError("n must be non-negative")
        start = n * size
        return self._records[start:start + size]

    # -- cursor pagination -------------------------------------------------

    def page_after(self, cursor: Optional[str], size: int) -> Dict[str, Any]:
        """Return ``size`` records strictly after ``cursor``.

        ``cursor`` is the opaque token returned as ``next_cursor`` from a prior
        call (or ``None`` / invalid to start from the beginning). The result is
        ``{"items": [...], "next_cursor": <token-or-None>}``. ``next_cursor`` is
        ``None`` exactly when the returned page is the last one.
        """
        if size <= 0:
            raise ValueError("size must be positive")

        bound = self._decode(cursor)  # None -> start from the beginning
        start = self._first_after(bound)
        items = self._records[start:start + size]

        # next_cursor is None iff there is nothing left after this page.
        if start + size < len(self._records):
            last = items[-1]
            next_cursor: Optional[str] = self._encode(last[self._key], last["id"])
        else:
            next_cursor = None

        return {"items": items, "next_cursor": next_cursor}

    # -- introspection -----------------------------------------------------

    def __len__(self) -> int:
        return len(self._records)

    # -- internals ---------------------------------------------------------

    def _first_after(self, bound: Optional[tuple]) -> int:
        """Index of the first record whose (key, id) is strictly > ``bound``.

        ``bound`` is a ``(key_value, id)`` tuple, or ``None`` to mean "before
        everything" (start from index 0).
        """
        if bound is None:
            return 0
        i = 0
        for r in self._records:
            if (r[self._key], r["id"]) > bound:
                return i
            i += 1
        return i

    def _encode(self, key_value: Any, rid: int) -> str:
        raw = json.dumps([key_value, rid], separators=(",", ":")).encode("utf-8")
        return base64.urlsafe_b64encode(raw).decode("ascii")

    def _decode(self, cursor: Optional[str]) -> Optional[tuple]:
        """Decode an opaque cursor to a ``(key_value, id)`` boundary.

        Any malformed / ``None`` cursor decodes to ``None`` (start from the
        beginning) rather than raising — invalid cursors must not crash.
        """
        if not cursor:
            return None
        try:
            raw = base64.urlsafe_b64decode(cursor.encode("ascii"))
            parsed = json.loads(raw.decode("utf-8"))
            if (isinstance(parsed, list) and len(parsed) == 2
                    and isinstance(parsed[1], int)):
                return (parsed[0], parsed[1])
        except Exception:  # noqa: BLE001 - any decode failure means "from the start"
            return None
        return None
