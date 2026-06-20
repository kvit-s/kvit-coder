"""serialhook — a JSON-like serializer with custom-type hooks.

Public API is re-exported here for convenience; the implementation lives in
``serialhook.public``.

    >>> from serialhook import dumps, loads, register
    >>> loads(dumps([1, "two", True, None]))
    [1, 'two', True, None]
"""

from .public import (
    CircularReferenceError,
    SerializationError,
    UnknownTagError,
    dumps,
    loads,
    register,
)

__all__ = [
    "dumps",
    "loads",
    "register",
    "SerializationError",
    "CircularReferenceError",
    "UnknownTagError",
]
