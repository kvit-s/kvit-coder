"""serialhook — a tiny JSON-like serializer.

Public API is re-exported here for convenience; the implementation lives in
``serialhook.public``.

    >>> from serialhook import dumps, loads
    >>> loads(dumps([1, "two", True, None]))
    [1, 'two', True, None]
"""

from .public import dumps, loads

__all__ = ["dumps", "loads"]
