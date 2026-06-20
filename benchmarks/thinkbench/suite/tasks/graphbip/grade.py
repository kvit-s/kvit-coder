#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `graphbip`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``graphbip`` package against the BRIEF'S CONTRACT (the ``graphbip.public``
``two_color`` API), NOT against any particular internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE component / odd-cycle / self-loop edge cases, all decided HERE (never read
from the agent's tests). Because the colors themselves are not fixed (any valid
2-coloring is accepted), the success checks VALIDATE the returned coloring (every
node assigned, no monochromatic edge) rather than compare against one canonical
map; the failure checks require ``None``. The FIXED reference passes every check;
the planted-bug starter fails a discriminating subset, so a partial fix lands
strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``graphbip.public``:
  1. components — the sweep starts from a single node and only colors that
     node's connected component, so a disconnected graph comes back missing the
     nodes of every other component;
  2. odd cycle — a neighbor already colored the SAME as the current node (an
     odd-cycle conflict) is not detected, so non-bipartite graphs are wrongly
     reported 2-colorable instead of returning ``None``;
  3. self-loop — a node adjacent to itself (``node in graph[node]``) is silently
     skipped instead of making the graph non-bipartite (``None``).

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs).
"""
import importlib
import json
import sys

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --- import the produced package ---------------------------------------------
# Contract path is ``graphbip.public``; fall back to the package root
# ``graphbip`` so a submission that re-exports ``two_color`` from ``__init__``
# (but moved it off ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
two_color = None
GraphError = None
try:
    try:
        mod = importlib.import_module("graphbip.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("graphbip")
    two_color = getattr(mod, "two_color")
    GraphError = getattr(mod, "GraphError", None)
    if not (isinstance(GraphError, type) and issubclass(GraphError, BaseException)):
        GraphError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
def _is_bipartite(graph):
    """Independent ground truth: does ``graph`` admit a 2-coloring? Sweeps every
    component, treats a self-loop and a same-color neighbor as non-bipartite."""
    color = {}
    for source in graph:
        if source in color:
            continue
        color[source] = 0
        stack = [source]
        while stack:
            node = stack.pop()
            for nb in graph.get(node, ()):
                if nb == node:
                    return False
                if nb not in color:
                    color[nb] = 1 - color[node]
                    stack.append(nb)
                elif color[nb] == color[node]:
                    return False
    return True


def _valid_coloring(graph, coloring):
    """True iff ``coloring`` assigns a 0/1 to EVERY node and no edge joins two
    same-colored nodes. Independent of which color landed where."""
    if not isinstance(coloring, dict):
        return False, f"expected a dict coloring, got {type(coloring).__name__}"
    if set(coloring) != set(graph):
        missing = set(graph) - set(coloring)
        extra = set(coloring) - set(graph)
        return False, f"node set mismatch: missing={sorted(map(str, missing))}, extra={sorted(map(str, extra))}"
    if any(c not in (0, 1) for c in coloring.values()):
        return False, f"colors must be 0/1, got {sorted(set(map(str, coloring.values())))}"
    for node, neighbors in graph.items():
        for nb in neighbors:
            if coloring[node] == coloring[nb]:
                return False, f"edge ({node!r},{nb!r}) is monochromatic (both {coloring[node]})"
    return True, "valid 2-coloring"


def expect_colorable(label, graph):
    """Check that ``two_color(graph)`` returns a VALID complete 2-coloring."""
    def _fn():
        got = two_color(dict(graph))
        if got is None:
            return False, f"{label}: got None, expected a valid 2-coloring of {graph!r}"
        ok, why = _valid_coloring(graph, got)
        return ok, f"{label}: {why} (got {got!r})"

    return _fn


def expect_none(label, graph):
    """Check that ``two_color(graph)`` returns ``None`` (graph not bipartite)."""
    def _fn():
        got = two_color(dict(graph))
        return (got is None), f"{label}: got {got!r}, expected None (not bipartite)"

    return _fn


if import_ok:
    # --- baseline: single connected bipartite graphs (pass even buggy) -------
    check("single_edge", "a single edge a-b is 2-colorable",
          expect_colorable("single_edge", {"a": {"b"}, "b": {"a"}}))
    check("even_path", "a-b-c-d (even path) is 2-colorable",
          expect_colorable("even_path",
                           {"a": {"b"}, "b": {"a", "c"}, "c": {"b", "d"}, "d": {"c"}}))
    check("even_cycle_4", "a 4-cycle is 2-colorable",
          expect_colorable("even_cycle_4",
                           {"a": {"b", "d"}, "b": {"a", "c"},
                            "c": {"b", "d"}, "d": {"c", "a"}}))
    check("list_adjacency", "neighbor iterables may be lists, not just sets",
          expect_colorable("list_adjacency",
                           {1: [2, 3], 2: [1], 3: [1]}))

    def c_empty():
        got = two_color({})
        return (got == {}), f"got {got!r}, expected {{}}"

    check("empty_graph", "the empty graph returns an empty coloring", c_empty)

    # --- BUG 1: every component must be colored ------------------------------
    check("two_components", "two separate edges -> all four nodes colored",
          expect_colorable("two_components",
                           {"a": {"b"}, "b": {"a"}, "c": {"d"}, "d": {"c"}}))
    check("isolated_node", "an isolated node is its own component and must be colored",
          expect_colorable("isolated_node",
                           {"a": {"b"}, "b": {"a"}, "z": set()}))
    check("three_components_mixed", "three components (edge, path, isolated) all colored",
          expect_colorable("three_components_mixed",
                           {"a": {"b"}, "b": {"a"},
                            "c": {"d"}, "d": {"c", "e"}, "e": {"d"},
                            "lone": set()}))
    check("all_isolated", "a graph of only isolated nodes colors every node",
          expect_colorable("all_isolated", {"x": set(), "y": set(), "z": set()}))
    check("two_components_one_isolated", "edge + bigger path + isolated all colored",
          expect_colorable("two_components_one_isolated",
                           {"a": {"b"}, "b": {"a"},
                            "p": {"q"}, "q": {"p", "r"}, "r": {"q", "s"}, "s": {"r"},
                            "solo": set()}))

    # --- BUG 2: odd cycles are not bipartite ---------------------------------
    check("triangle", "a triangle (3-cycle) is not bipartite",
          expect_none("triangle",
                      {"a": {"b", "c"}, "b": {"a", "c"}, "c": {"a", "b"}}))
    check("five_cycle", "a 5-cycle (odd) is not bipartite",
          expect_none("five_cycle",
                      {0: {1, 4}, 1: {0, 2}, 2: {1, 3}, 3: {2, 4}, 4: {3, 0}}))
    check("odd_cycle_with_tail", "an odd cycle with a pendant tail is not bipartite",
          expect_none("odd_cycle_with_tail",
                      {"a": {"b", "c"}, "b": {"a", "c"}, "c": {"a", "b", "d"}, "d": {"c"}}))

    # --- BUG 3: self-loops are not bipartite ---------------------------------
    check("self_loop_with_edge", "a self-loop on a connected node -> not bipartite",
          expect_none("self_loop_with_edge", {"a": {"a", "b"}, "b": {"a"}}))
    check("self_loop_isolated", "a self-loop on an otherwise isolated node -> not bipartite",
          expect_none("self_loop_isolated", {"a": {"a"}}))

    # --- INTERACTIONS: a defect hides behind a clean first component ---------
    # The first component is bipartite; a LATER component is not. Catching these
    # needs BUG 1 (visit later components) AND the matching detection (BUG 2/3).
    check("good_then_triangle", "clean first component, odd-cycle later -> None",
          expect_none("good_then_triangle",
                      {"a": {"b"}, "b": {"a"},
                       "c": {"d", "e"}, "d": {"c", "e"}, "e": {"c", "d"}}))
    check("good_then_self_loop", "clean first component, self-loop later -> None",
          expect_none("good_then_self_loop",
                      {"a": {"b"}, "b": {"a"}, "s": {"s"}}))
    check("triangle_then_good", "odd-cycle FIRST component still -> None",
          expect_none("triangle_then_good",
                      {"a": {"b", "c"}, "b": {"a", "c"}, "c": {"a", "b"},
                       "x": {"y"}, "y": {"x"}}))
    check("two_good_then_self_loop", "two clean components then a self-loop -> None",
          expect_none("two_good_then_self_loop",
                      {"a": {"b"}, "b": {"a"}, "c": {"d"}, "d": {"c"}, "z": {"z"}}))

    # --- validation ----------------------------------------------------------
    def c_non_dict():
        try:
            two_color([("a", "b")])
        except GraphError:
            return True, "raised GraphError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected GraphError"
        return False, "did not raise"

    check("non_dict_raises", "a non-dict graph raises GraphError", c_non_dict)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 20

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "graphbip",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
