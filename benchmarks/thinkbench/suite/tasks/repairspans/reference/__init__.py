"""repairspans — a tiny closed-interval library.

Public API lives in :mod:`repairspans.public`: :func:`merge` collapses a list of
``[start, end]`` intervals into the minimal set of non-overlapping intervals, and
:func:`overlaps` reports whether two intervals share any point.

Intervals are CLOSED: ``[start, end]`` includes both endpoints, so two intervals
that merely *touch* at an endpoint (e.g. ``[1, 2]`` and ``[2, 3]``) share the
point ``2`` and are therefore considered to overlap / adjacent.
"""

from .public import merge, overlaps

__all__ = ["merge", "overlaps"]
