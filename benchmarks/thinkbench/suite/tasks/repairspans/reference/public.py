"""repairspans.public — closed-interval merge and overlap helpers.

An interval is a two-element sequence ``[start, end]`` with ``start <= end``.
Intervals are CLOSED, so they include both endpoints. Two closed intervals that
share *any* point overlap — including the case where they only touch at a single
endpoint (``[1, 2]`` and ``[2, 3]`` share the point ``2``). Such touching /
adjacent intervals merge into one.

Standard library only.
"""

from __future__ import annotations

from typing import List, Sequence


def overlaps(a: Sequence[int], b: Sequence[int]) -> bool:
    """Return ``True`` if closed intervals ``a`` and ``b`` share any point.

    Because the intervals are closed, touching at an endpoint counts as
    overlapping: ``overlaps([1, 2], [2, 3])`` is ``True``.
    """
    a_start, a_end = a[0], a[1]
    b_start, b_end = b[0], b[1]
    # Closed intervals overlap iff each starts no later than the other ends.
    return a_start <= b_end and b_start <= a_end


def merge(intervals: Sequence[Sequence[int]]) -> List[List[int]]:
    """Collapse overlapping or touching closed intervals into a minimal set.

    Returns a new list of ``[start, end]`` intervals sorted by start, with no two
    results overlapping or touching. The input is not mutated.
    """
    if not intervals:
        return []

    # Sort by start (then end) so a single left-to-right sweep is correct even
    # when the input is unordered.
    ordered = sorted(([iv[0], iv[1]] for iv in intervals), key=lambda iv: (iv[0], iv[1]))

    merged: List[List[int]] = [ordered[0]]
    for start, end in ordered[1:]:
        last = merged[-1]
        # Closed intervals touch/overlap when the next start is at or before the
        # current end (``start <= last[1]``), so they collapse into one.
        if start <= last[1]:
            if end > last[1]:
                last[1] = end
        else:
            merged.append([start, end])
    return merged
