"""romanio.public — a small Roman-numeral converter (stdlib only).

``to_roman(n)`` renders an integer ``1 <= n <= 3999`` as a Roman numeral.
``from_roman(s)`` parses a Roman numeral string back into an integer. The two
are inverses: ``from_roman(to_roman(n)) == n`` for every ``n`` in range.

Roman numerals use SUBTRACTIVE notation for the six "one-before-five/ten"
values: 4 is ``IV`` (not ``IIII``), 9 is ``IX``, 40 is ``XL``, 90 is ``XC``,
400 is ``CD`` and 900 is ``CM``. Everything else is additive, with symbols
written from largest to smallest (e.g. 2026 -> ``MMXXVI``).

Standard library only.
"""

from __future__ import annotations


class RomanError(ValueError):
    """Raised when a value is out of range or a numeral string is malformed."""


# (value, symbol) pairs, largest first. Used by both directions.
#
# BUG (to_roman: no subtractive notation): this table only carries the "plain"
# powers-of-ten and their halves (1000, 500, 100, 50, 10, 5, 1). It is MISSING
# the six subtractive pairs (900=CM, 400=CD, 90=XC, 40=XL, 9=IX, 4=IV), so
# greedy emission spells 4 as "IIII", 9 as "VIIII", 40 as "XXXX", 90 as
# "LXXXX", 400 as "CCCC" and 900 as "DCCCC". Purely additive numbers (III, VI,
# VIII, XXX, LX, ...) still come out right.
_VALUES = [
    (1000, "M"),
    (500, "D"),
    (100, "C"),
    (50, "L"),
    (10, "X"),
    (5, "V"),
    (1, "I"),
]

# Single-symbol values, used when parsing.
_SYMBOL = {
    "M": 1000,
    "D": 500,
    "C": 100,
    "L": 50,
    "X": 10,
    "V": 5,
    "I": 1,
}


def to_roman(n: int) -> str:
    """Render integer ``n`` (1..3999) as a Roman numeral."""
    if not isinstance(n, int) or isinstance(n, bool):
        raise RomanError(f"n must be an int, got {type(n).__name__}")
    # BUG (no range check): n < 1 or n > 3999 should raise RomanError, but this
    # falls straight through to the greedy loop. For n <= 0 it returns "" and
    # for n > 3999 it emits a long run of leading Ms instead of refusing.
    out = []
    for value, symbol in _VALUES:
        while n >= value:
            out.append(symbol)
            n -= value
    return "".join(out)


def from_roman(s: str) -> int:
    """Parse a Roman numeral string ``s`` back into an integer."""
    if not isinstance(s, str):
        raise RomanError(f"s must be a str, got {type(s).__name__}")
    text = s.strip().upper()
    if not text:
        raise RomanError("empty numeral")

    total = 0
    for ch in text:
        if ch not in _SYMBOL:
            raise RomanError(f"invalid symbol {ch!r} in {s!r}")
        # BUG (additive-only parse): this just sums every symbol's value. It
        # never notices that a smaller symbol sitting BEFORE a larger one should
        # be SUBTRACTED, so "IV" parses as I + V = 6 instead of 4, "IX" as 11,
        # "XL" as 60, "CM" as 1100, and so on. Purely additive numerals (III,
        # VI, XXX) parse correctly.
        total += _SYMBOL[ch]
    return total
