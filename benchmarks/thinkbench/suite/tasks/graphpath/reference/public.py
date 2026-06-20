"""Dijkstra shortest path on a weighted directed graph.

The graph is an adjacency map ``{node: {neighbor: weight}}`` where every weight
is a non-negative number. :func:`shortest` finds the minimum-total-weight path
from ``src`` to ``dst`` and returns the pair ``(distance, path)``, where ``path``
is the list of nodes visited from ``src`` to ``dst`` inclusive. If ``dst`` cannot
be reached from ``src`` the function returns ``None``.

Example
-------
    >>> g = {"a": {"b": 1, "c": 4}, "b": {"c": 2, "d": 5}, "c": {"d": 1}, "d": {}}
    >>> shortest(g, "a", "d")
    (4, ['a', 'b', 'c', 'd'])
    >>> shortest(g, "a", "a")
    (0, ['a'])
    >>> shortest(g, "d", "a")        # unreachable
    None
"""

from __future__ import annotations

import heapq


def shortest(graph, src, dst):
    """Return ``(distance, path)`` for the shortest ``src`` -> ``dst`` route.

    Parameters
    ----------
    graph:
        Adjacency map ``{node: {neighbor: weight}}``. Weights are non-negative.
        A node with no outgoing edges maps to an empty ``{}``.
    src, dst:
        Start and end nodes (any hashable key present in ``graph``).

    Returns
    -------
    ``(distance, path)`` where ``distance`` is the total weight (``0`` when
    ``src == dst``) and ``path`` is the node list from ``src`` to ``dst``
    inclusive, or ``None`` when ``dst`` is unreachable from ``src``.
    """
    # Trivial route: a node always reaches itself at zero cost, even when it has
    # no outgoing edges (or is absent from the adjacency map's keys).
    if src == dst:
        return (0, [src])

    # Best known distance to each settled/seen node, and the predecessor that
    # achieved it (for path reconstruction).
    dist = {src: 0}
    prev = {}
    # Nodes whose shortest distance is finalised; never relax them again.
    visited = set()

    # Lazy-deletion heap of (distance, node). A node may appear several times;
    # only the entry matching its current best distance is acted on.
    heap = [(0, src)]

    while heap:
        d, node = heapq.heappop(heap)
        # Skip stale heap entries: a shorter route to this node was already
        # popped, so this one is obsolete.
        if node in visited:
            continue
        visited.add(node)

        # The first time we pop dst its distance is final -- stop and rebuild.
        if node == dst:
            return (d, _build_path(prev, src, dst))

        for neighbor, weight in graph.get(node, {}).items():
            if neighbor in visited:
                continue
            nd = d + weight
            # Relax only on a strict improvement.
            if neighbor not in dist or nd < dist[neighbor]:
                dist[neighbor] = nd
                prev[neighbor] = node
                heapq.heappush(heap, (nd, neighbor))

    # Exhausted every reachable node without settling dst.
    return None


def _build_path(prev, src, dst):
    """Walk predecessors from ``dst`` back to ``src``, then reverse to forward."""
    path = [dst]
    node = dst
    while node != src:
        node = prev[node]
        path.append(node)
    path.reverse()
    return path
