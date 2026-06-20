"""A base-62 integer codec.

Encodes a non-negative integer as a compact base-62 string and decodes it back,
over the fixed 62-character alphabet::

    0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz

i.e. ``'0'`` has value 0, ``'9'`` is 9, ``'A'`` is 10, ``'Z'`` is 35, ``'a'`` is
36, ``'z'`` is 61. The most-significant digit comes first (big-endian), exactly
like ordinary decimal notation, so the codec round-trips: ``decode(encode(n)) ==
n`` for every ``n >= 0``.

Example
-------
    >>> from base62 import encode, decode
    >>> encode(62)
    '10'
    >>> decode('10')
    62
"""

from __future__ import annotations

ALPHABET = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
BASE = len(ALPHABET)  # 62


def encode(n: int) -> str:
    """Encode a non-negative integer ``n`` as a base-62 string."""
    digits = []
    # Peel off base-62 digits one at a time.
    while n > 0:
        rem = n % BASE
        digits.append(ALPHABET[rem])
        n = n // BASE
    # Glue the digits together.
    return "".join(digits)


def decode(s: str) -> int:
    """Decode a base-62 string ``s`` back to its non-negative integer value."""
    n = 0
    for ch in s:
        # Look the character up in the alphabet to get its value.
        value = ALPHABET.find(ch)
        n = n * BASE + value
    return n
