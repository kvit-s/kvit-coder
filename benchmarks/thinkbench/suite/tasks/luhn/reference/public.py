"""The Luhn (mod-10) checksum: validation and check-digit generation.

The Luhn algorithm walks a string of digits from right to left, doubling every
second digit (the rightmost is not doubled, the next one to its left is, and so
on). Each doubled value greater than 9 has its two decimal digits folded back
together (equivalently, 9 is subtracted). The digits are summed; the number is
valid when that sum is a multiple of 10.

    >>> is_valid("4539148803436467")
    True
    >>> is_valid("4539 1488 0343 6467")   # spaces are ignored
    True
    >>> is_valid("4539148803436466")      # last digit wrong
    False
    >>> check_digit("7992739871")         # appending 3 makes it valid
    3
"""

from __future__ import annotations


def _luhn_sum(digits: str) -> int:
    """Luhn checksum total of an all-digit string, doubling from the right.

    Position 0 is the RIGHTMOST digit (never doubled); odd positions counting
    from the right are doubled, and a doubled value over 9 has its decimal
    digits folded together (the same as subtracting 9).
    """
    total = 0
    for pos, ch in enumerate(reversed(digits)):
        d = ord(ch) - 48  # ch is guaranteed a digit by the callers
        if pos % 2 == 1:
            d *= 2
            if d > 9:
                d -= 9  # fold 10..18 back to 1..9 (sum of the two decimal digits)
        total += d
    return total


def _clean(number: str) -> str | None:
    """Strip spaces; return the digit string, or None if empty / non-digit."""
    stripped = number.replace(" ", "")
    if not stripped or not stripped.isdigit():
        return None
    return stripped


def is_valid(number: str) -> bool:
    """Return True iff ``number`` passes the Luhn checksum.

    Spaces are ignored. Empty input, or input containing any non-digit after
    spaces are removed, is invalid (returns False, never raises).
    """
    digits = _clean(number)
    if digits is None:
        return False
    return _luhn_sum(digits) % 10 == 0


def check_digit(partial: str) -> int:
    """Return the digit (0..9) that, appended to ``partial``, makes it valid.

    The appended digit lands at right-position 0 (undoubled), so ``partial``'s
    own digits all shift one place left. The result is the amount needed to
    round the checksum up to the next multiple of 10 -- and exactly 0 (never 10)
    when the checksum is already a multiple of 10.
    """
    digits = _clean(partial)
    if digits is None:
        digits = ""
    total = _luhn_sum(digits + "0")
    return (10 - total % 10) % 10
