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
    # BUG: strict comparisons treat a shared endpoint as NON-overlapping, so
    # touching closed intervals like [1,2] and [2,3] wrongly report False.
    return a_start < b_end and b_start < a_end


def merge(intervals: Sequence[Sequence[int]]) -> List[List[int]]:
    """Collapse overlapping or touching closed intervals into a minimal set.

    Returns a new list of ``[start, end]`` intervals sorted by start, with no two
    results overlapping or touching. The input is not mutated.
    """
    if not intervals:
        return []

    # BUG: the input is assumed to be pre-sorted by start. Unsorted input (e.g.
    # [[3, 4], [1, 2]]) is walked in its original order, so disjoint pairs get
    # left un-merged or merged incorrectly.
    merged: List[List[int]] = [list(intervals[0])]
    for interval in intervals[1:]:
        start, end = interval[0], interval[1]
        last = merged[-1]
        # BUG: off-by-one adjacency. Using strict ``<`` means touching intervals
        # ([1,2] then [2,3], where start == last[1]) are NOT merged; closed
        # intervals that share an endpoint should collapse into one.
        if start < last[1]:
            if end > last[1]:
                last[1] = end
        else:
            merged.append([start, end])
    return merged
