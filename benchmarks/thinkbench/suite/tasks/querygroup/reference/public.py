"""A tiny query engine over a list of dicts ("rows"), with grouped aggregation.

A :class:`Query` wraps an immutable sequence of row dicts and offers a small,
chainable API:

* ``where(predicate)`` — keep only rows for which ``predicate(row)`` is truthy.
* ``order_by(key)``    — return rows sorted by ``key`` (a column name or a
  function ``row -> sort key``). Stable; ``reverse=True`` for descending.
* ``group_by(keys, aggregates=...)`` — collapse rows into one output row per
  distinct key-tuple, with aggregates computed over chosen fields. Returns a new
  :class:`Query` so the result can itself be filtered / ordered.
* ``rows()`` — materialise the current rows as a list of dicts.

Every operation returns a NEW ``Query`` (the original is never mutated).

Grouping semantics (the contract — see ``brief.txt``):

* Groups appear in FIRST-APPEARANCE order: the order in which each distinct
  key-tuple is first seen while scanning the (already filtered/ordered) rows.
* Each output row carries the group key columns plus one column per requested
  aggregate, named ``"<agg>_<field>"`` (e.g. ``"sum_pay"``), or the caller's
  chosen alias.
* ``count`` counts ROWS in the group (it ignores ``None`` only in the sense
  that it does not look at any field at all).
* ``sum`` / ``avg`` / ``min`` / ``max`` consider only NON-``None`` values of the
  field. Missing keys are treated as ``None`` and skipped.
* ``avg`` is the true mean of the non-``None`` values (no rounding). If a group
  has NO non-``None`` values for the field, ``avg`` is ``None`` (not 0, not a
  crash), and likewise ``min`` / ``max`` are ``None`` and ``sum`` is 0.
"""

from __future__ import annotations

from typing import Any, Callable, Dict, Iterable, List, Optional, Sequence, Tuple, Union

Row = Dict[str, Any]
KeyArg = Union[str, Callable[[Row], Any]]

# An aggregate spec is ("func", "field", "alias-or-None").
AggSpec = Tuple[str, str, Optional[str]]


class Query:
    """An immutable, chainable view over a list of row dicts."""

    def __init__(self, rows: Iterable[Row]) -> None:
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
        """Return a new Query sorted by ``key`` (column name or function)."""
        keyfn = _as_keyfn(key)
        return Query(sorted(self._rows, key=keyfn, reverse=reverse))

    def group_by(
        self,
        keys: Union[str, Sequence[str]],
        aggregates: Optional[Sequence[AggSpec]] = None,
    ) -> "Query":
        """Collapse rows into one output row per distinct key-tuple.

        Parameters
        ----------
        keys:
            A single column name, or a sequence of column names. Each output row
            carries these columns with the group's shared values.
        aggregates:
            A sequence of ``(func, field, alias)`` triples. ``func`` is one of
            ``count`` / ``sum`` / ``avg`` / ``min`` / ``max``. ``field`` is the
            column the aggregate runs over (ignored for ``count`` but still
            required for naming; pass any placeholder). ``alias`` is the output
            column name, or ``None`` to use ``"<func>_<field>"``.

        Groups are emitted in FIRST-APPEARANCE order. Operates on the current
        rows, so a preceding ``where`` filters BEFORE grouping.
        """
        key_cols = [keys] if isinstance(keys, str) else list(keys)
        specs: List[AggSpec] = list(aggregates or [])

        # Preserve first-appearance order while collecting member rows per group.
        order: List[Tuple[Any, ...]] = []
        buckets: Dict[Tuple[Any, ...], List[Row]] = {}
        for r in self._rows:
            gk = tuple(r.get(c) for c in key_cols)
            if gk not in buckets:
                buckets[gk] = []
                order.append(gk)
            buckets[gk].append(r)

        out: List[Row] = []
        for gk in order:
            members = buckets[gk]
            row: Row = {col: val for col, val in zip(key_cols, gk)}
            for func, field, alias in specs:
                name = alias if alias is not None else f"{func}_{field}"
                row[name] = _aggregate(func, field, members)
            out.append(row)
        return Query(out)


# -- helpers -------------------------------------------------------------


def _as_keyfn(key: KeyArg) -> Callable[[Row], Any]:
    """Normalise a string column name or a callable into a key function."""
    if callable(key):
        return key
    return lambda r: r.get(key)


def _aggregate(func: str, field: str, members: List[Row]) -> Any:
    """Compute one aggregate over a group's member rows."""
    if func == "count":
        return len(members)

    # All other aggregates run over the NON-None values of ``field``.
    vals = [r.get(field) for r in members]
    vals = [v for v in vals if v is not None]

    if func == "sum":
        # Sum of an empty selection is 0 (the additive identity).
        return sum(vals)
    if func == "avg":
        # Mean of the non-None values; undefined (None) when there are none.
        if not vals:
            return None
        return sum(vals) / len(vals)
    if func == "min":
        return min(vals) if vals else None
    if func == "max":
        return max(vals) if vals else None

    raise ValueError(f"unknown aggregate function: {func!r}")
