"""deepget -- nested access by dotted path.

Public API is re-exported here for convenience; the implementation lives in
``deepget.public``.

    >>> from deepget import get, set_
    >>> get({"a": {"b": 1}}, "a.b")
    1
    >>> set_({}, "a.b", 2)
    {'a': {'b': 2}}
"""

from .public import get, set_

__all__ = ["get", "set_"]
