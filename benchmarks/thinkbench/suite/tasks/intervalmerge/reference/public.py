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
    # Drop zero-width (and any reversed) intervals up front -- they cover no
    # points -- then sort by start so a single left-to-right sweep suffices.
    cleaned = [(s, e) for (s, e) in intervals if s < e]
    cleaned.sort(key=lambda iv: iv[0])

    out = []
    for s, e in cleaned:
        if out and s <= out[-1][1]:
            # Overlap (s < prev_end) OR touch (s == prev_end): extend the run.
            # Half-open adjacency means s == prev_end is contiguous, so the
            # comparison is ``<=``, not ``<``.
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
    # Canonicalise both sides first so the sweep below works on disjoint, sorted
    # runs and so unsorted / overlapping inputs are handled uniformly.
    holes = merge(b)
    out = []
    for s, e in merge(a):
        # Carve every hole that overlaps this piece out of it, left to right.
        cur = s
        for hs, he in holes:
            if he <= cur:
                continue  # hole entirely left of where we are
            if hs >= e:
                break     # hole (and all later holes) entirely right of piece
            if hs > cur:
                # Emit the left remainder before the hole. Guarded by hs > cur,
                # so a hole flush with the left edge emits nothing (no zero-width).
                out.append((cur, hs))
            # Advance past the hole; the right remainder continues from he.
            if he > cur:
                cur = he
            if cur >= e:
                break     # the rest of this piece is fully covered
        if cur < e:
            # Right remainder after the last hole (or the whole piece if no hole
            # touched it). The cur < e guard drops a zero-width tail.
            out.append((cur, e))
    return out
