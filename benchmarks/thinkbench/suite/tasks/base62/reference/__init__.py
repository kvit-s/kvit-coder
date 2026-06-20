"""base62 -- a base-62 integer codec.

Public API is re-exported here for convenience; the implementation lives in
``base62.public``.

    >>> from base62 import encode, decode
    >>> encode(0)
    '0'
    >>> decode('A')
    10
"""

from .public import decode, encode

__all__ = ["encode", "decode"]
