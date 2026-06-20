#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_intervalmerge`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced
`intervalmerge` package against the BRIEF'S CONTRACT for half-open ``[start,
end)`` interval algebra:

  * ``merge(intervals)`` -> the minimal sorted list of disjoint intervals
    covering the same points; OVERLAPPING and TOUCHING (adjacent) half-open
    intervals merge (``[1,2)`` + ``[2,3)`` -> ``[1,3)``); input may be unsorted;
    zero-width intervals (``start == end``) are dropped.
  * ``subtract(a, b)`` -> the merged-canonical list covering the points of ``a``
    but not ``b``, splitting an ``a`` interval into two where ``b`` punches a
    hole in its middle; output is sorted, disjoint, and zero-width-free.

It grades behaviour, NOT any particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- merge uses a STRICT ``<`` overlap test, so half-open TOUCHING
    intervals (``[1,2)`` and ``[2,3)``) are left un-merged instead of joined.
  * BUG 2 -- merge does NOT sort its input, so any unsorted collection produces
    wrong (and extra) runs. subtract inherits this through ``merge(a)``.
  * BUG 3 -- neither function drops ZERO-WIDTH intervals: merge keeps a
    ``[x,x)`` input, and subtract emits a ``[x,x)`` remainder when a hole sits
    flush against an interval edge (and emits ``[x,x)`` for a fully-covered
    interval instead of nothing).
  * BUG 4 -- subtract only ``sorted()``s ``b`` instead of merging it, so
    OVERLAPPING holes are not coalesced and carve out a reversed/invalid
    ``(hi, lo)`` interval, corrupting the result.

Simple sorted, strictly-overlapping, single-mid-hole cases still look correct,
so a superficial fix (e.g. only flipping ``<`` to ``<=``) passes the easy checks
while still failing the edge cases.

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
    ("merge_overlap_sorted", "sorted overlapping intervals merge into one"),
    ("merge_nested", "an interval fully inside another merges to the outer"),
    ("merge_gap_preserved", "a real gap between intervals is preserved (not merged)"),
    ("merge_empty", "merging an empty list returns an empty list"),
    ("merge_touching_halfopen", "touching half-open intervals [1,2)+[2,3) merge to [1,3)"),
    ("merge_chain_touch", "a chain of touching intervals collapses to one span"),
    ("merge_unsorted", "unsorted overlapping input is merged correctly"),
    ("merge_unsorted_touch", "unsorted touching input is merged correctly"),
    ("merge_zero_width_dropped", "a zero-width [x,x) input interval is dropped"),
    ("merge_no_mutate", "merge does not mutate or reorder its input argument"),
    ("subtract_single_mid_split", "a hole in the middle splits an interval into two"),
    ("subtract_two_holes", "two interior holes split an interval into three pieces"),
    ("subtract_flush_left", "a hole flush with the left edge leaves no zero-width piece"),
    ("subtract_flush_right", "a hole flush with the right edge trims cleanly"),
    ("subtract_full_cover", "a fully-covered interval yields nothing (no zero-width)"),
    ("subtract_overlapping_holes", "overlapping holes in b are coalesced before carving"),
    ("subtract_unsorted_a", "unsorted a is normalised before subtracting"),
    ("subtract_empty_b", "subtracting nothing returns a normalised copy of a"),
    ("subtract_adjacent_holes_touch", "touching holes carve a single contiguous gap"),
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


def norm(result):
    """Coerce a returned interval list into a list of plain (int/float) tuples.

    The contract is order-significant (sorted by start), so this does NOT sort:
    a result that is correct as a SET but mis-ordered must still fail.
    """
    return [tuple(iv) for iv in result]


# --- import the produced package (contract: intervalmerge.public, fallback pkg) -
import_ok = True
import_detail = ""
merge = None
subtract = None
try:
    try:
        mod = importlib.import_module("intervalmerge.public")
    except Exception:
        mod = importlib.import_module("intervalmerge")
    merge = getattr(mod, "merge")
    subtract = getattr(mod, "subtract")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # ---- merge: baselines that the buggy code already gets right ------------
    def c_merge_overlap_sorted():
        got = norm(merge([(1, 3), (2, 4)]))
        exp = [(1, 4)]
        return got == exp, f"merge([(1,3),(2,4)]) -> {got} (expected {exp})"

    check("merge_overlap_sorted", c_merge_overlap_sorted)

    def c_merge_nested():
        got = norm(merge([(1, 10), (3, 5)]))
        exp = [(1, 10)]
        return got == exp, f"merge([(1,10),(3,5)]) -> {got} (expected {exp})"

    check("merge_nested", c_merge_nested)

    def c_merge_gap_preserved():
        got = norm(merge([(1, 2), (3, 4)]))
        exp = [(1, 2), (3, 4)]
        return got == exp, f"merge([(1,2),(3,4)]) -> {got} (expected {exp})"

    check("merge_gap_preserved", c_merge_gap_preserved)

    def c_merge_empty():
        got = norm(merge([]))
        return got == [], f"merge([]) -> {got} (expected [])"

    check("merge_empty", c_merge_empty)

    # ---- merge: BUG 1 -- touching half-open intervals must merge -------------
    def c_merge_touching_halfopen():
        got = norm(merge([(1, 2), (2, 3)]))
        exp = [(1, 3)]
        return got == exp, f"merge([(1,2),(2,3)]) -> {got} (expected {exp}; half-open adjacency)"

    check("merge_touching_halfopen", c_merge_touching_halfopen)

    def c_merge_chain_touch():
        got = norm(merge([(1, 2), (2, 3), (3, 4)]))
        exp = [(1, 4)]
        return got == exp, f"merge of 3 touching -> {got} (expected {exp})"

    check("merge_chain_touch", c_merge_chain_touch)

    # ---- merge: BUG 2 -- unsorted input must be sorted first -----------------
    def c_merge_unsorted():
        got = norm(merge([(2, 4), (1, 3)]))
        exp = [(1, 4)]
        return got == exp, f"merge([(2,4),(1,3)]) -> {got} (expected {exp})"

    check("merge_unsorted", c_merge_unsorted)

    def c_merge_unsorted_touch():
        # unsorted AND touching: trips both BUG 1 and BUG 2.
        got = norm(merge([(3, 4), (1, 2), (2, 3)]))
        exp = [(1, 4)]
        return got == exp, f"merge([(3,4),(1,2),(2,3)]) -> {got} (expected {exp})"

    check("merge_unsorted_touch", c_merge_unsorted_touch)

    # ---- merge: BUG 3 -- zero-width inputs are dropped ----------------------
    def c_merge_zero_width_dropped():
        got = norm(merge([(1, 1), (2, 4), (5, 5)]))
        exp = [(2, 4)]
        return got == exp, f"merge([(1,1),(2,4),(5,5)]) -> {got} (expected {exp})"

    check("merge_zero_width_dropped", c_merge_zero_width_dropped)

    # ---- merge: must not mutate the caller's input --------------------------
    def c_merge_no_mutate():
        arg = [(2, 4), (1, 3)]
        snapshot = list(arg)
        merge(arg)
        return arg == snapshot, f"input after merge -> {arg} (expected unchanged {snapshot})"

    check("merge_no_mutate", c_merge_no_mutate)

    # ---- subtract: baselines the buggy code already gets right --------------
    def c_subtract_single_mid_split():
        got = norm(subtract([(0, 10)], [(3, 5)]))
        exp = [(0, 3), (5, 10)]
        return got == exp, f"subtract([(0,10)],[(3,5)]) -> {got} (expected {exp})"

    check("subtract_single_mid_split", c_subtract_single_mid_split)

    def c_subtract_two_holes():
        got = norm(subtract([(0, 10)], [(3, 4), (6, 7)]))
        exp = [(0, 3), (4, 6), (7, 10)]
        return got == exp, f"two interior holes -> {got} (expected {exp})"

    check("subtract_two_holes", c_subtract_two_holes)

    # ---- subtract: BUG 3 -- no zero-width remainders ------------------------
    def c_subtract_flush_left():
        got = norm(subtract([(0, 10)], [(0, 3)]))
        exp = [(3, 10)]
        return got == exp, f"subtract([(0,10)],[(0,3)]) -> {got} (expected {exp}; no [0,0))"

    check("subtract_flush_left", c_subtract_flush_left)

    def c_subtract_flush_right():
        got = norm(subtract([(0, 10)], [(7, 10)]))
        exp = [(0, 7)]
        return got == exp, f"subtract([(0,10)],[(7,10)]) -> {got} (expected {exp})"

    check("subtract_flush_right", c_subtract_flush_right)

    def c_subtract_full_cover():
        got = norm(subtract([(0, 10)], [(0, 10)]))
        exp = []
        return got == exp, f"fully covered -> {got} (expected {exp}; nothing, not [(0,0)])"

    check("subtract_full_cover", c_subtract_full_cover)

    # ---- subtract: BUG 4 -- overlapping holes in b must be coalesced --------
    def c_subtract_overlapping_holes():
        got = norm(subtract([(0, 10)], [(3, 6), (5, 8)]))
        exp = [(0, 3), (8, 10)]
        return got == exp, f"overlapping holes -> {got} (expected {exp}; no reversed interval)"

    check("subtract_overlapping_holes", c_subtract_overlapping_holes)

    # ---- subtract: BUG 2 (inherited) -- unsorted a normalised first ---------
    def c_subtract_unsorted_a():
        got = norm(subtract([(5, 10), (0, 3)], [(1, 2)]))
        exp = [(0, 1), (2, 3), (5, 10)]
        return got == exp, f"subtract unsorted a -> {got} (expected {exp})"

    check("subtract_unsorted_a", c_subtract_unsorted_a)

    def c_subtract_empty_b():
        # subtracting nothing returns a's points, normalised (merged/sorted).
        got = norm(subtract([(3, 5), (1, 2)], []))
        exp = [(1, 2), (3, 5)]
        return got == exp, f"subtract([(3,5),(1,2)],[]) -> {got} (expected {exp})"

    check("subtract_empty_b", c_subtract_empty_b)

    # ---- subtract: touching holes carve one contiguous gap (BUG 4 corollary)-
    def c_subtract_adjacent_holes_touch():
        # [3,5) and [5,7) are contiguous half-open holes -> one gap [3,7).
        got = norm(subtract([(0, 10)], [(3, 5), (5, 7)]))
        exp = [(0, 3), (7, 10)]
        return got == exp, f"touching holes -> {got} (expected {exp})"

    check("subtract_adjacent_holes_touch", c_subtract_adjacent_holes_touch)


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
    "task": "fix_intervalmerge",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
