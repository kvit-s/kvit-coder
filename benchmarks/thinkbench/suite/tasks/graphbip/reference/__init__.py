"""graphbip — bipartite 2-coloring of an undirected graph (stdlib only).

Public API lives in :mod:`graphbip.public`. ``two_color(graph)`` assigns every
node one of two colors (``0`` / ``1``) so that no edge joins two same-colored
nodes, returning the ``{node: color}`` map on success or ``None`` when the graph
is not bipartite.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import two_color, GraphError

__all__ = ["two_color", "GraphError"]
