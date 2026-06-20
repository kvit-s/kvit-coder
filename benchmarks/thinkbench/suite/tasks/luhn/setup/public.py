"""The Luhn (mod-10) checksum: validation and check-digit generation.

The Luhn algorithm walks a string of digits, doubling every second digit, folding
any doubled value over 9 back down, and summing; the number is valid when that
sum is a multiple of 10.

    >>> is_valid("4539148803436467")
    True
    >>> is_valid("4539148803436466")      # last digit wrong
    False
    >>> check_digit("7992739871")         # appending 3 makes it valid
    3
"""

from __future__ import annotations


def _luhn_sum(digits: str) -> int:
    """Luhn checksum total of an all-digit string.

    Double every second digit and fold doubled values back below ten, then add
    everything up.
    """
    total = 0
    for pos, ch in enumerate(digits):
        d = int(ch)
        # Double every second digit, starting from the first one we see.
        if pos % 2 == 0:
            d *= 2
            if d > 9:
                # Bring the doubled value back under ten.
                d -= 10
        total += d
    return total


def is_valid(number: str) -> bool:
    """Return True iff ``number`` passes the Luhn checksum."""
    return _luhn_sum(number) % 10 == 0


def check_digit(partial: str) -> int:
    """Return the digit that, appended to ``partial``, makes it pass ``is_valid``.

    Compute the checksum of the partial number and return whatever is needed to
    reach the next multiple of ten.
    """
    total = _luhn_sum(partial + "0")
    return 10 - total % 10
