"""Paginator over a list of records sorted by a key.

Each record is a ``dict`` carrying at least an integer ``"id"`` (a stable,
unique identifier) and a sort key. The paginator keeps the records in a stable
sorted order and today supports OFFSET pagination only:

    ``page(n, size)`` -> the ``n``-th page (0-based) of ``size`` records.

The records are ordered by ``key`` ascending, with ties broken by ``"id"``
ascending so the order is fully deterministic even when several records share a
sort-key value.

There is NO cursor-based pagination yet. The task is to add it (see
``brief.txt`` for the contract).

Example
-------
    >>> rows = [{"id": 3, "score": 5}, {"id": 1, "score": 5}, {"id": 2, "score": 9}]
    >>> p = Paginator(rows, key="score")
    >>> [r["id"] for r in p.page(0, 2)]   # sorted by score, ties by id
    [1, 3]
    >>> [r["id"] for r in p.page(1, 2)]
    [2]
"""

from __future__ import annotations

from typing import Any, Dict, List

Record = Dict[str, Any]


class Paginator:
    """Paginate a list of records sorted by ``key`` (ties broken by ``id``)."""

    def __init__(self, records: List[Record], key: str = "id") -> None:
        self._key = key
        # Materialize a stable, fully-deterministic order: by sort key ascending,
        # then by id ascending to break ties.
        self._records: List[Record] = sorted(
            records, key=lambda r: (r[key], r["id"])
        )

    # -- offset pagination -------------------------------------------------

    def page(self, n: int, size: int) -> List[Record]:
        """Return the ``n``-th page (0-based) of ``size`` records.

        Out-of-range pages return an empty list. ``size`` must be positive.
        """
        if size <= 0:
            raise ValueError("size must be positive")
        if n < 0:
            raise ValueError("n must be non-negative")
        start = n * size
        return self._records[start:start + size]

    # -- introspection -----------------------------------------------------

    def __len__(self) -> int:
        return len(self._records)
