"""graphbip.public — bipartite 2-coloring of an undirected graph (stdlib only).

``two_color(graph)`` 2-colors an undirected graph:

* ``graph`` is an adjacency map ``{node: neighbors}`` where ``neighbors`` is an
  iterable (set or list) of the nodes adjacent to ``node``. The graph is
  UNDIRECTED: if ``b`` is in ``graph[a]`` then ``a`` is in ``graph[b]``. It may
  be DISCONNECTED (several separate pieces) and may contain isolated nodes
  (empty neighbor iterables).
* The return value is a ``{node: color}`` dict assigning every node either ``0``
  or ``1`` such that no edge connects two same-colored nodes, OR ``None`` if the
  graph cannot be 2-colored (it is not bipartite).

Standard library only.
"""

from __future__ import annotations

from collections import deque
from typing import Dict, Optional


class GraphError(ValueError):
    """Raised when the graph argument is malformed."""


def two_color(graph: dict) -> Optional[Dict[object, int]]:
    """Return a ``{node: 0/1}`` 2-coloring of ``graph``, or ``None`` if the graph
    is not bipartite."""
    if not isinstance(graph, dict):
        raise GraphError(f"graph must be a dict, got {type(graph).__name__}")

    color: Dict[object, int] = {}

    # BUG (components): the BFS is seeded from a SINGLE start node (the first key)
    # and only ever explores that node's connected component. Any node that lives
    # in a different component is never visited and never colored, so a
    # disconnected graph comes back missing those nodes. The fix has to iterate
    # over EVERY node and start a fresh BFS for each one still uncolored.
    start = next(iter(graph), None)
    if start is None:
        return color

    color[start] = 0
    queue = deque([start])
    while queue:
        node = queue.popleft()
        for neighbor in graph.get(node, ()):
            # BUG (self-loop): a self-loop edge (``node in graph[node]``) makes a
            # node adjacent to itself, which can never be 2-colored. This skips
            # the self edge as if it were harmless, so the graph is wrongly
            # reported bipartite. The fix has to make ``neighbor == node`` return
            # None instead of skipping it.
            if neighbor == node:
                continue
            if neighbor not in color:
                color[neighbor] = 1 - color[node]
                queue.append(neighbor)
            # BUG (odd cycle): a neighbor that is ALREADY colored the SAME as the
            # current node is an odd-cycle conflict and must make the whole
            # coloring fail (return None). The starter just falls through here
            # and keeps the (now contradictory) coloring, so an odd cycle is
            # mis-reported as 2-colorable. The fix has to detect the same-color
            # collision and return None.

    return color
