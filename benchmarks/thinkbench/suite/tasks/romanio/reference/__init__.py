"""romanio — a tiny Roman-numeral converter (stdlib only).

Public API lives in :mod:`romanio.public`. ``to_roman(n)`` renders an integer
in the range 1..3999 as a Roman numeral, and ``from_roman(s)`` parses a Roman
numeral back into an integer. The two are inverses of each other.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import to_roman, from_roman, RomanError

__all__ = ["to_roman", "from_roman", "RomanError"]
