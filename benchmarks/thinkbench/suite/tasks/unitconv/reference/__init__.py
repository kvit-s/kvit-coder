"""unitconv — a tiny unit converter (stdlib only).

Public API lives in :mod:`unitconv.public`. ``convert(value, from_unit,
to_unit)`` converts a numeric value between units of the same dimension (length,
time, or compound speed) and returns a ``float``; mismatched dimensions raise
``UnitError``.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import convert, UnitError

__all__ = ["convert", "UnitError"]
