"""A tiny query engine over a list of dicts ("rows").

A :class:`Query` wraps an immutable sequence of row dicts and offers a small,
chainable API. Today it supports:

* ``where(predicate)`` — keep only rows for which ``predicate(row)`` is truthy.
* ``order_by(key)``    — return rows sorted by ``key`` (a column name or a
  function ``row -> sort key``). Stable; ascending by default, ``reverse=True``
  for descending.
* ``rows()``           — materialise the current rows as a list of dicts.

Every operation returns a NEW ``Query`` (the original is never mutated), so
chains compose:

    >>> q = Query([{"dept": "eng", "pay": 10}, {"dept": "eng", "pay": 30}])
    >>> q.where(lambda r: r["pay"] >= 20).rows()
    [{'dept': 'eng', 'pay': 30}]
    >>> q.order_by("pay", reverse=True).rows()[0]["pay"]
    30

There is NO aggregation yet: see ``brief.txt`` for the ``group_by`` task.
"""

from __future__ import annotations

from typing import Any, Callable, Dict, Iterable, List, Union

Row = Dict[str, Any]
KeyArg = Union[str, Callable[[Row], Any]]


class Query:
    """An immutable, chainable view over a list of row dicts."""

    def __init__(self, rows: Iterable[Row]) -> None:
        # Defensive copy of the row list (shallow: row dicts are shared).
        self._rows: List[Row] = list(rows)

    # -- terminal --------------------------------------------------------

    def rows(self) -> List[Row]:
        """Materialise the current rows as a fresh list."""
        return list(self._rows)

    def __len__(self) -> int:
        return len(self._rows)

    def __iter__(self):
        return iter(self._rows)

    # -- transforms ------------------------------------------------------

    def where(self, predicate: Callable[[Row], bool]) -> "Query":
        """Return a new Query keeping only rows where ``predicate`` is truthy."""
        return Query([r for r in self._rows if predicate(r)])

    def order_by(self, key: KeyArg, reverse: bool = False) -> "Query":
        """Return a new Query sorted by ``key``.

        ``key`` is either a column name (string) or a function ``row -> value``.
        The sort is stable, so rows that compare equal keep their relative order.
        """
        keyfn = _as_keyfn(key)
        return Query(sorted(self._rows, key=keyfn, reverse=reverse))


def _as_keyfn(key: KeyArg) -> Callable[[Row], Any]:
    """Normalise a string column name or a callable into a key function."""
    if callable(key):
        return key
    return lambda r: r.get(key)
