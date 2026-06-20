"""jsonquery — a JSONPath-lite selector (stdlib only).

Public API lives in :mod:`jsonquery.public`. ``select(obj, path)`` evaluates a
small path expression (``.key``, ``[index]``, ``[*]``, ``..key``) against a
nested dict/list structure and returns the flat list of values it selects.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import select, SelectError

__all__ = ["select", "SelectError"]
