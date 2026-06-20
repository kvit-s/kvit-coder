#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_graphpath`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced
`graphpath` package against the BRIEF'S CONTRACT (Dijkstra shortest path on a
weighted directed graph `{node: {neighbor: weight}}`, whose
`shortest(graph, src, dst) -> (distance, path) | None` returns the minimum total
weight and the node list from src to dst inclusive, 0/[src] for src==dst, and
None when dst is unreachable), NOT against any particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG A -- it finalises a node the first time ANY edge reaches it ("seen" on
    discovery) and never relaxes it again, so a cheaper route found later is
    ignored: the returned distance is inflated and the path is wrong.
  * BUG B -- the src==dst case returns an EMPTY path `[]` instead of `[src]`
    (the distance 0 is right, but the path drops the single node).
  * BUG C -- an unreachable dst returns `(inf, ...)` instead of `None`, and the
    accompanying path is garbage rather than signalling "no route".
  * BUG D -- the path is reconstructed back-to-front (dst -> src) and never
    reversed, so even correct distances come with a reversed node list.

Basic single-edge / already-sorted graphs still look correct, so a superficial
fix can pass the easy checks while still failing the edge cases. The exact-path
checks below use graphs with a UNIQUE shortest path so a half-fix is caught.

Output: a single JSON scorecard on stdout. Each check runs in isolation, so the
score is continuous (passed / total), never all-or-nothing. FIXED DENOMINATOR:
the full check list is registered up front, so an import failure records every
check as failed and forces score 0.0. Exit code is 0 whenever grading ran to
completion (even score 0.0); the process never raises out.
"""
import importlib
import json
import os
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# --- fixed denominator: the full roster of checks, declared before any run ----
# (id, human description). Kept in lockstep with the checks registered below so
# that an import failure can still report a result line for every single check.
CHECK_SPECS = [
    ("single_edge", "a one-hop graph returns (weight, [src, dst])"),
    ("chain_distance", "a simple chain returns the summed distance"),
    ("chain_path", "a simple chain returns the full node list in src->dst order"),
    ("picks_cheaper_branch", "the cheaper of two branches is chosen for distance and path"),
    ("path_is_forward", "the path runs src -> dst, not reversed"),
    ("late_cheap_route_distance", "a cheaper route discovered LATER wins (no settle-on-discovery)"),
    ("late_cheap_route_path", "the late-discovered cheaper route is reflected in the path"),
    ("relax_after_seen", "a node first seen via an expensive edge is later relaxed to the cheap one"),
    ("src_eq_dst_distance", "src == dst has distance 0"),
    ("src_eq_dst_path", "src == dst returns the single-node path [src]"),
    ("src_eq_dst_no_outgoing", "src == dst works for a node with no outgoing edges"),
    ("unreachable_is_none", "an unreachable dst returns None (not inf, not a tuple)"),
    ("unreachable_isolated", "a node with no path to dst returns None"),
    ("unreachable_wrong_direction", "a directed edge does not imply the reverse route exists"),
    ("path_endpoints", "the path begins at src and ends at dst"),
    ("path_is_valid_walk", "consecutive path nodes are joined by real edges summing to the distance"),
    ("multi_hop_unique", "a longer unique-shortest route is found exactly (distance and path)"),
    ("zero_weight_edges", "zero-weight edges are handled without skipping or looping"),
]
CHECK_IDS = [cid for cid, _ in CHECK_SPECS]
DESC = dict(CHECK_SPECS)

results = {}  # cid -> {"passed": bool, "detail": str}


def record(cid, passed, detail=""):
    results[cid] = {"passed": bool(passed), "detail": str(detail or "")}


def check(cid, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


def is_pair(r):
    """A non-None result must be a (distance, path) 2-tuple/list."""
    return isinstance(r, (tuple, list)) and len(r) == 2


def walk_weight(graph, path):
    """Sum the edge weights along `path`; None if any hop is not a real edge."""
    if not isinstance(path, (list, tuple)) or len(path) < 1:
        return None
    total = 0
    for u, v in zip(path, path[1:]):
        edges = graph.get(u, {})
        if v not in edges:
            return None
        total += edges[v]
    return total


# --- import the produced package (contract: graphpath.public, fallback pkg) ---
import_ok = True
import_detail = ""
shortest = None
try:
    try:
        mod = importlib.import_module("graphpath.public")
    except Exception:
        mod = importlib.import_module("graphpath")
    shortest = getattr(mod, "shortest")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. baseline: a single directed edge.
    def c_single_edge():
        g = {"a": {"b": 5}, "b": {}}
        r = shortest(g, "a", "b")
        ok = is_pair(r) and r[0] == 5 and list(r[1]) == ["a", "b"]
        return ok, f"a->b -> {r!r} (expected (5, ['a','b']))"

    check("single_edge", c_single_edge)

    # 2. baseline distance along a plain chain.
    def c_chain_distance():
        g = {"a": {"b": 2}, "b": {"c": 3}, "c": {"d": 4}, "d": {}}
        r = shortest(g, "a", "d")
        ok = is_pair(r) and r[0] == 9
        return ok, f"a->d distance -> {r!r} (expected 9)"

    check("chain_distance", c_chain_distance)

    # 3. baseline path along a plain chain (forward order, all nodes).
    def c_chain_path():
        g = {"a": {"b": 2}, "b": {"c": 3}, "c": {"d": 4}, "d": {}}
        r = shortest(g, "a", "d")
        ok = is_pair(r) and list(r[1]) == ["a", "b", "c", "d"]
        return ok, f"a->d path -> {r!r} (expected ['a','b','c','d'])"

    check("chain_path", c_chain_path)

    # 4. two branches, the cheaper one wins (distance and path).
    def c_picks_cheaper_branch():
        # a->b->d = 1+1 = 2 ; a->c->d = 4+1 = 5. Unique cheapest is via b.
        g = {"a": {"b": 1, "c": 4}, "b": {"d": 1}, "c": {"d": 1}, "d": {}}
        r = shortest(g, "a", "d")
        ok = is_pair(r) and r[0] == 2 and list(r[1]) == ["a", "b", "d"]
        return ok, f"a->d -> {r!r} (expected (2, ['a','b','d']))"

    check("picks_cheaper_branch", c_picks_cheaper_branch)

    # 5. BUG D: the path must read src -> dst, not dst -> src.
    def c_path_is_forward():
        g = {"a": {"b": 1, "c": 4}, "b": {"c": 2}, "c": {"d": 1}, "d": {}}
        r = shortest(g, "a", "d")  # cheapest: a->b->c->d = 1+2+1 = 4
        path = list(r[1]) if is_pair(r) else None
        ok = path is not None and path[0] == "a" and path[-1] == "d"
        return ok, f"a->d path -> {path!r} (must start 'a', end 'd')"

    check("path_is_forward", c_path_is_forward)

    # 6. BUG A: a cheaper route discovered AFTER the expensive one must win.
    #    'd' is first reached directly from a (cost 10); the real shortest is
    #    a->b->c->d = 1+1+1 = 3. A settle-on-discovery bug freezes 'd' at 10.
    def c_late_cheap_route_distance():
        g = {"a": {"d": 10, "b": 1}, "b": {"c": 1}, "c": {"d": 1}, "d": {}}
        r = shortest(g, "a", "d")
        ok = is_pair(r) and r[0] == 3
        return ok, f"a->d distance -> {r!r} (expected 3, buggy ~10)"

    check("late_cheap_route_distance", c_late_cheap_route_distance)

    # 7. BUG A corollary: the path follows the cheap late route, not the direct edge.
    def c_late_cheap_route_path():
        g = {"a": {"d": 10, "b": 1}, "b": {"c": 1}, "c": {"d": 1}, "d": {}}
        r = shortest(g, "a", "d")
        ok = is_pair(r) and list(r[1]) == ["a", "b", "c", "d"]
        return ok, f"a->d path -> {r!r} (expected ['a','b','c','d'])"

    check("late_cheap_route_path", c_late_cheap_route_path)

    # 8. BUG A sharper: a node first SEEN on an expensive edge must still be
    #    relaxed when a cheaper predecessor is settled later.
    def c_relax_after_seen():
        # x is reachable directly a->x = 8, or via a->y->x = 2+2 = 4 (cheaper).
        # then x->t = 1. Shortest a->t = 4+1 = 5 via y.
        g = {"a": {"x": 8, "y": 2}, "y": {"x": 2}, "x": {"t": 1}, "t": {}}
        r = shortest(g, "a", "t")
        ok = is_pair(r) and r[0] == 5 and list(r[1]) == ["a", "y", "x", "t"]
        return ok, f"a->t -> {r!r} (expected (5, ['a','y','x','t']))"

    check("relax_after_seen", c_relax_after_seen)

    # 9. src == dst: distance is 0.
    def c_src_eq_dst_distance():
        g = {"a": {"b": 1}, "b": {}}
        r = shortest(g, "a", "a")
        ok = is_pair(r) and r[0] == 0
        return ok, f"a->a -> {r!r} (expected distance 0)"

    check("src_eq_dst_distance", c_src_eq_dst_distance)

    # 10. BUG B: src == dst returns the single-node path [src], not [].
    def c_src_eq_dst_path():
        g = {"a": {"b": 1}, "b": {}}
        r = shortest(g, "a", "a")
        ok = is_pair(r) and list(r[1]) == ["a"]
        return ok, f"a->a path -> {r!r} (expected ['a'], buggy [])"

    check("src_eq_dst_path", c_src_eq_dst_path)

    # 11. BUG B edge: src == dst for a sink node (no outgoing edges) still [src].
    def c_src_eq_dst_no_outgoing():
        g = {"a": {"b": 1}, "b": {}}
        r = shortest(g, "b", "b")
        ok = is_pair(r) and r[0] == 0 and list(r[1]) == ["b"]
        return ok, f"b->b -> {r!r} (expected (0, ['b']))"

    check("src_eq_dst_no_outgoing", c_src_eq_dst_no_outgoing)

    # 12. BUG C: an unreachable dst returns None (not inf, not a tuple).
    def c_unreachable_is_none():
        g = {"a": {"b": 1}, "b": {}, "c": {}}
        r = shortest(g, "a", "c")  # no edge into c
        return (r is None), f"a->c -> {r!r} (expected None)"

    check("unreachable_is_none", c_unreachable_is_none)

    # 13. BUG C: a node sitting in its own disconnected island returns None.
    def c_unreachable_isolated():
        g = {"a": {"b": 1}, "b": {}, "x": {"y": 1}, "y": {}}
        r = shortest(g, "a", "y")
        return (r is None), f"a->y -> {r!r} (expected None)"

    check("unreachable_isolated", c_unreachable_isolated)

    # 14. BUG C: edges are DIRECTED -- a->b existing does not make b->a reachable.
    def c_unreachable_wrong_direction():
        g = {"a": {"b": 1}, "b": {}}
        r = shortest(g, "b", "a")
        return (r is None), f"b->a -> {r!r} (expected None, edge is a->b only)"

    check("unreachable_wrong_direction", c_unreachable_wrong_direction)

    # 15. the path's endpoints are exactly src and dst.
    def c_path_endpoints():
        g = {"a": {"b": 3}, "b": {"c": 3}, "c": {}}
        r = shortest(g, "a", "c")
        ok = is_pair(r) and list(r[1])[0] == "a" and list(r[1])[-1] == "c"
        return ok, f"a->c path -> {r!r} (endpoints must be a..c)"

    check("path_endpoints", c_path_endpoints)

    # 16. the returned path is a real walk whose edge weights sum to the distance.
    def c_path_is_valid_walk():
        g = {"a": {"b": 1, "c": 4}, "b": {"c": 2, "d": 7}, "c": {"d": 1}, "d": {}}
        r = shortest(g, "a", "d")  # a->b->c->d = 1+2+1 = 4
        if not is_pair(r):
            return False, f"a->d -> {r!r} (expected a pair)"
        w = walk_weight(g, r[1])
        ok = (w is not None) and (w == r[0])
        return ok, f"a->d -> {r!r}; walk weight={w!r} (must equal distance {r[0]!r})"

    check("path_is_valid_walk", c_path_is_valid_walk)

    # 17. a longer graph with a single unique shortest route, checked exactly.
    def c_multi_hop_unique():
        g = {
            "s": {"a": 2, "b": 9},
            "a": {"b": 1, "c": 7},
            "b": {"c": 2},
            "c": {"t": 1},
            "t": {},
        }
        # s->a->b->c->t = 2+1+2+1 = 6 (unique shortest); s->b = 9 is worse.
        r = shortest(g, "s", "t")
        ok = is_pair(r) and r[0] == 6 and list(r[1]) == ["s", "a", "b", "c", "t"]
        return ok, f"s->t -> {r!r} (expected (6, ['s','a','b','c','t']))"

    check("multi_hop_unique", c_multi_hop_unique)

    # 18. zero-weight edges are valid and must not be skipped or loop forever.
    def c_zero_weight_edges():
        g = {"a": {"b": 0}, "b": {"c": 0}, "c": {}}
        r = shortest(g, "a", "c")
        ok = is_pair(r) and r[0] == 0 and list(r[1]) == ["a", "b", "c"]
        return ok, f"a->c -> {r!r} (expected (0, ['a','b','c']))"

    check("zero_weight_edges", c_zero_weight_edges)


# --- assemble the scorecard with a FIXED denominator -------------------------
checks_out = []
for cid in CHECK_IDS:
    r = results.get(cid)
    if r is None:
        # Not run (e.g. import failed): record as a failed check, keep denominator.
        r = {"passed": False, "detail": "not run (import failed)" if not import_ok else "not run"}
    checks_out.append({"id": cid, "desc": DESC[cid], "passed": r["passed"], "detail": r["detail"]})

passed = sum(1 for c in checks_out if c["passed"])
total = len(checks_out)  # always len(CHECK_SPECS): fixed denominator
card = {
    "task": "fix_graphpath",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
