"""graphpath -- Dijkstra shortest path on a weighted directed graph.

Public API is re-exported here for convenience; the implementation lives in
``graphpath.public``.

    >>> from graphpath import shortest
    >>> g = {"a": {"b": 1}, "b": {}}
    >>> shortest(g, "a", "b")
    (1, ['a', 'b'])
"""

from .public import shortest

__all__ = ["shortest"]
