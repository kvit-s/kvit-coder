"""Visible test suite for graphbip.

Run it from the directory that CONTAINS the ``graphbip`` package:

    python -m unittest graphbip.test_graphbip

These tests currently FAIL because of bugs in ``graphbip.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).
"""
import unittest

from graphbip.public import two_color, GraphError


def _valid_coloring(graph, coloring):
    """True iff ``coloring`` assigns a 0/1 to EVERY node and no edge joins two
    same-colored nodes."""
    if coloring is None:
        return False
    if set(coloring) != set(graph):
        return False
    if any(c not in (0, 1) for c in coloring.values()):
        return False
    for node, neighbors in graph.items():
        for nb in neighbors:
            if coloring[node] == coloring[nb]:
                return False
    return True


class TestGraphbip(unittest.TestCase):
    # --- single connected bipartite graphs already work (sanity baseline) ----
    def test_single_edge(self):
        graph = {"a": {"b"}, "b": {"a"}}
        self.assertTrue(_valid_coloring(graph, two_color(graph)))

    def test_path_of_four(self):
        # a - b - c - d  is bipartite (an even path)
        graph = {"a": {"b"}, "b": {"a", "c"}, "c": {"b", "d"}, "d": {"c"}}
        self.assertTrue(_valid_coloring(graph, two_color(graph)))

    def test_even_cycle(self):
        # 4-cycle a-b-c-d-a is bipartite
        graph = {"a": {"b", "d"}, "b": {"a", "c"}, "c": {"b", "d"}, "d": {"c", "a"}}
        self.assertTrue(_valid_coloring(graph, two_color(graph)))

    def test_empty_graph(self):
        self.assertEqual(two_color({}), {})

    # --- BUG 1: every component must be colored, not just the first ----------
    def test_disconnected_two_components(self):
        # two separate edges: {a-b} and {c-d}. ALL four nodes must be colored.
        graph = {"a": {"b"}, "b": {"a"}, "c": {"d"}, "d": {"c"}}
        coloring = two_color(graph)
        self.assertIsNotNone(coloring)
        self.assertEqual(set(coloring), {"a", "b", "c", "d"})
        self.assertTrue(_valid_coloring(graph, coloring))

    def test_isolated_node_component(self):
        # an edge a-b plus a lone node z (its own component) -> z must be colored
        graph = {"a": {"b"}, "b": {"a"}, "z": set()}
        coloring = two_color(graph)
        self.assertIsNotNone(coloring)
        self.assertIn("z", coloring)
        self.assertTrue(_valid_coloring(graph, coloring))

    # --- BUG 2: odd cycle is not bipartite ----------------------------------
    def test_triangle_not_bipartite(self):
        # 3-cycle a-b-c-a is an odd cycle -> not 2-colorable
        graph = {"a": {"b", "c"}, "b": {"a", "c"}, "c": {"a", "b"}}
        self.assertIsNone(two_color(graph))

    # --- BUG 3: a self-loop is never bipartite ------------------------------
    def test_self_loop_not_bipartite(self):
        # a node adjacent to itself cannot be 2-colored
        graph = {"a": {"a", "b"}, "b": {"a"}}
        self.assertIsNone(two_color(graph))

    # --- interaction: a bad component hides behind a good first one ---------
    def test_good_component_then_odd_component(self):
        # first component a-b is fine; second component c-d-e is a triangle.
        # The whole graph is NOT bipartite, so the answer must be None even
        # though the FIRST component colors cleanly.
        graph = {
            "a": {"b"}, "b": {"a"},
            "c": {"d", "e"}, "d": {"c", "e"}, "e": {"c", "d"},
        }
        self.assertIsNone(two_color(graph))

    # --- validation ----------------------------------------------------------
    def test_non_dict_raises(self):
        with self.assertRaises(GraphError):
            two_color([("a", "b")])


if __name__ == "__main__":
    unittest.main()
