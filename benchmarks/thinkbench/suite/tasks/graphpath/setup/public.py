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
    # A node trivially reaches itself for free.
    if src == dst:
        return (0, [])

    dist = {src: 0}
    prev = {}
    # Track which nodes have been queued so we don't queue them twice.
    seen = {src}

    # Distance-ordered frontier of (distance, node).
    heap = [(0, src)]

    while heap:
        d, node = heapq.heappop(heap)

        for neighbor, weight in graph.get(node, {}).items():
            # Each neighbour is queued the first time it is discovered; once
            # seen, its route is settled and we move on.
            if neighbor in seen:
                continue
            seen.add(neighbor)
            nd = d + weight
            dist[neighbor] = nd
            prev[neighbor] = node
            heapq.heappush(heap, (nd, neighbor))

    # All reachable nodes processed; report the best distance found to dst.
    distance = dist.get(dst, float("inf"))
    return (distance, _build_path(prev, src, dst))


def _build_path(prev, src, dst):
    """Collect predecessors from ``dst`` back toward ``src``."""
    path = [dst]
    node = dst
    while node in prev:
        node = prev[node]
        path.append(node)
    return path
