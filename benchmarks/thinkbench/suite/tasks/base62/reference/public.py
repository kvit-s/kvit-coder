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
    >>> encode(0)
    '0'
    >>> encode(61)
    'z'
    >>> encode(62)
    '10'
    >>> decode('10')
    62
    >>> decode(encode(123456789))
    123456789
"""

from __future__ import annotations

ALPHABET = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
BASE = len(ALPHABET)  # 62
# Reverse lookup: character -> its integer value. Used to validate input too.
_VALUE = {ch: i for i, ch in enumerate(ALPHABET)}


def encode(n: int) -> str:
    """Encode a non-negative integer ``n`` as a base-62 string.

    The most-significant digit comes first. ``encode(0)`` is ``"0"`` (a single
    zero digit), never the empty string.
    """
    if not isinstance(n, int) or isinstance(n, bool):
        raise TypeError(f"encode expects an int, got {type(n).__name__}")
    if n < 0:
        raise ValueError(f"encode expects a non-negative integer, got {n}")
    if n == 0:
        return ALPHABET[0]
    digits = []
    while n > 0:
        n, rem = divmod(n, BASE)
        digits.append(ALPHABET[rem])
    # divmod peels the LEAST-significant digit first, so reverse for big-endian.
    digits.reverse()
    return "".join(digits)


def decode(s: str) -> int:
    """Decode a base-62 string ``s`` back to its non-negative integer value.

    The most-significant digit is leftmost. Any character outside the alphabet
    raises ``ValueError``; the empty string is invalid and also raises.
    """
    if not isinstance(s, str):
        raise TypeError(f"decode expects a str, got {type(s).__name__}")
    if s == "":
        raise ValueError("decode received an empty string")
    n = 0
    for ch in s:
        value = _VALUE.get(ch)
        if value is None:
            raise ValueError(f"invalid base-62 character: {ch!r}")
        n = n * BASE + value
    return n
