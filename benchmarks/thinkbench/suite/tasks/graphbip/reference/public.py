"""graphbip.public — bipartite 2-coloring of an undirected graph (stdlib only).

``two_color(graph)`` 2-colors an undirected graph:

* ``graph`` is an adjacency map ``{node: neighbors}`` where ``neighbors`` is an
  iterable (set or list) of the nodes adjacent to ``node``. The graph is
  UNDIRECTED and may be DISCONNECTED, with isolated nodes allowed (empty
  neighbor iterables).
* The return value is a ``{node: color}`` dict assigning every node either ``0``
  or ``1`` such that no edge connects two same-colored nodes, OR ``None`` if the
  graph cannot be 2-colored (it is not bipartite).

A graph is bipartite iff it has no odd-length cycle. The two structural ways a
graph fails to be bipartite, both handled here:

* a self-loop (``node in graph[node]``) — a node adjacent to itself is the
  shortest odd cycle and is never 2-colorable;
* any other odd cycle — surfaced as a neighbor already colored the SAME as the
  current node during the breadth-first sweep.

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

    # Visit EVERY node so that every connected component gets colored, not just
    # the component of the first node. Already-colored nodes are skipped, so each
    # component is swept exactly once.
    for source in graph:
        if source in color:
            continue
        color[source] = 0
        queue = deque([source])
        while queue:
            node = queue.popleft()
            for neighbor in graph.get(node, ()):
                # A self-loop makes a node adjacent to itself: an odd cycle of
                # length 1 that can never be 2-colored.
                if neighbor == node:
                    return None
                if neighbor not in color:
                    color[neighbor] = 1 - color[node]
                    queue.append(neighbor)
                elif color[neighbor] == color[node]:
                    # Neighbor already colored the SAME -> odd cycle -> not
                    # bipartite.
                    return None

    return color
