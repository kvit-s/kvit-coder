"""querygroup — a tiny chainable query engine over a list of dicts.

Public API is re-exported here for convenience; the implementation lives in
``querygroup.public``.

    >>> from querygroup import Query
    >>> q = Query([{"a": 1}, {"a": 2}])
    >>> q.where(lambda r: r["a"] > 1).rows()
    [{'a': 2}]
"""

from .public import Query

__all__ = ["Query"]
