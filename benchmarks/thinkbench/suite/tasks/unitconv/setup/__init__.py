"""unitconv — a tiny unit converter (stdlib only).

Public API lives in :mod:`unitconv.public`. ``convert(value, from_unit,
to_unit)`` converts a numeric value between units of the same dimension (length,
time, or compound speed) and returns a ``float``; mismatched dimensions raise
``UnitError``.
"""

from .public import convert, UnitError

__all__ = ["convert", "UnitError"]
