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


# (value, symbol) pairs, largest first. The six subtractive pairs sit BETWEEN
# the plain powers-of-ten so the greedy emission in ``to_roman`` naturally
# produces IV / IX / XL / XC / CD / CM instead of the additive runs.
_VALUES = [
    (1000, "M"),
    (900, "CM"),
    (500, "D"),
    (400, "CD"),
    (100, "C"),
    (90, "XC"),
    (50, "L"),
    (40, "XL"),
    (10, "X"),
    (9, "IX"),
    (5, "V"),
    (4, "IV"),
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
    if n < 1 or n > 3999:
        raise RomanError(f"n must be in 1..3999, got {n}")
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
    prev = 0
    for ch in text:
        if ch not in _SYMBOL:
            raise RomanError(f"invalid symbol {ch!r} in {s!r}")
        value = _SYMBOL[ch]
        # Subtractive notation: a smaller symbol placed before a larger one is
        # subtracted (IV = 5 - 1 = 4). We add it now and, on the next larger
        # symbol, back out twice the value we wrongly added.
        if value > prev:
            total += value - 2 * prev
        else:
            total += value
        prev = value
    return total
