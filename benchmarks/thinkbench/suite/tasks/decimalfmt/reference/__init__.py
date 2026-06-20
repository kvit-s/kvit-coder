"""decimalfmt — integer-cents money formatting (stdlib only).

Public API lives in :mod:`decimalfmt.public`. ``format_amount(cents, places,
sep)`` renders an integer number of cents as a grouped decimal string, and
``parse_amount(s, places, sep)`` is its inverse.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import MoneyError, format_amount, parse_amount

__all__ = ["format_amount", "parse_amount", "MoneyError"]
