"""decimalfmt — integer-cents money formatting (stdlib only).

Public API lives in :mod:`decimalfmt.public`. ``format_amount(cents, places,
sep)`` renders an integer number of cents as a grouped decimal string, and
``parse_amount(s, places, sep)`` is its inverse.
"""

from .public import MoneyError, format_amount, parse_amount

__all__ = ["format_amount", "parse_amount", "MoneyError"]
