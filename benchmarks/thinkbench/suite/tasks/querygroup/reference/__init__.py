"""querygroup — a chainable query engine over a list of dicts, with grouping.

Public API is re-exported here for convenience; the implementation lives in
``querygroup.public``.

    >>> from querygroup import Query
    >>> q = Query([{"d": "a", "n": 1}, {"d": "a", "n": 3}])
    >>> q.group_by("d", [("avg", "n", None)]).rows()
    [{'d': 'a', 'avg_n': 2.0}]
"""

from .public import Query

__all__ = ["Query"]
