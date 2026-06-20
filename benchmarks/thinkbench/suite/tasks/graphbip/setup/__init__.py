"""graphbip — bipartite 2-coloring of an undirected graph (stdlib only).

Public API lives in :mod:`graphbip.public`. ``two_color(graph)`` attempts to
assign every node one of two colors (``0`` / ``1``) such that no edge joins two
nodes of the same color, returning the ``{node: color}`` map on success or
``None`` when the graph is not bipartite.
"""

from .public import two_color, GraphError

__all__ = ["two_color", "GraphError"]
