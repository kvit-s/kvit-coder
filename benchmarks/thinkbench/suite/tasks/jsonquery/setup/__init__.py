"""jsonquery — a JSONPath-lite selector (stdlib only).

Public API lives in :mod:`jsonquery.public`. ``select(obj, path)`` evaluates a
small path expression (``.key``, ``[index]``, ``[*]``, ``..key``) against a
nested dict/list structure and returns the flat list of values it selects.
"""

from .public import select, SelectError

__all__ = ["select", "SelectError"]
