"""Half-open interval algebra over ``[start, end)`` intervals.

An interval is a pair ``(start, end)`` denoting the half-open range
``start <= x < end``. The two public operations are:

* :func:`merge` -- collapse a collection of intervals into the minimal sorted
  list of disjoint intervals covering exactly the same points. Overlapping AND
  *touching* (adjacent) intervals are merged: because the ranges are half-open,
  ``[1, 2)`` and ``[2, 3)`` are contiguous and merge into ``[1, 3)``.
* :func:`subtract` -- remove from ``a`` every point covered by ``b``, splitting
  intervals where ``b`` punches a hole in the middle.

Both functions normalise their output: the result is always sorted by start,
disjoint, and free of zero-width intervals (``start == end`` covers no points
and is dropped). Inputs may be unsorted, overlapping, or contain zero-width
intervals; the functions cope.

Example
-------
    >>> merge([(2, 4), (1, 3)])         # unsorted + overlapping
    [(1, 4)]
    >>> merge([(1, 2), (2, 3)])         # touching half-open -> contiguous
    [(1, 3)]
    >>> subtract([(0, 10)], [(3, 5)])   # punch a hole -> split in two
    [(0, 3), (5, 10)]
"""

from __future__ import annotations


def merge(intervals):
    """Return the minimal sorted list of disjoint ``(start, end)`` intervals.

    Overlapping and touching half-open intervals are merged. Zero-width
    intervals (``start == end``) cover no points and are dropped. The input may
    be in any order; the output is sorted by start and never mutates the input.
    """
    out = []
    for s, e in intervals:
        if out and s < out[-1][1]:
            # Extend the current run when the next interval starts inside it.
            prev_s, prev_e = out[-1]
            out[-1] = (prev_s, e if e > prev_e else prev_e)
        else:
            out.append((s, e))
    return out


def subtract(a, b):
    """Remove every point covered by ``b`` from the intervals ``a``.

    Returns the merged-canonical (sorted, disjoint, zero-width-free) list of
    intervals covering exactly the points in ``a`` but not in ``b``. Where a
    ``b`` interval lies strictly inside an ``a`` interval the ``a`` interval is
    split into the left and right remainders.
    """
    holes = sorted(b)
    out = []
    for s, e in merge(a):
        cur = s
        for hs, he in holes:
            if he <= cur:
                continue  # hole entirely left of where we are
            if hs >= e:
                break     # hole entirely right of this piece
            # Emit the part of the piece that lies to the left of the hole, then
            # move past the hole.
            out.append((cur, hs))
            cur = he
        if cur < e:
            out.append((cur, e))
    return out
