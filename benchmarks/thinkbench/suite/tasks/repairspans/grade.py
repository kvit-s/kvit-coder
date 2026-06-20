#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `repairspans`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never reads or runs the agent's own visible tests
(`repairspans/test_repairspans.py`). It grades the produced `repairspans` package
against the BRIEF'S CONTRACT (the `repairspans` / `repairspans.public` API), NOT
against any particular internal file layout.

This is a SUPERSET of the visible suite: it re-checks the visible behaviors and
adds extra overlap / adjacency / nesting / mutation edge cases. Every expected
value is computed here directly (no dependency on the model's tests). The FIXED
reference package passes every check; the BUGGY shipped package fails several.

The planted bugs (in the shipped `setup/` package):
  1. `merge` uses a strict `<` adjacency test, so touching closed intervals
     ([1,2],[2,3]) are NOT merged (off-by-one on the closed endpoint).
  2. `merge` assumes pre-sorted input and never sorts, so unsorted input is
     merged incorrectly.
  3. `overlaps` uses strict `<` comparisons, so touching closed intervals report
     `False` when they share an endpoint and should report `True`.

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


# --- import the produced package (contract: repairspans.public, fallback pkg) -
import_ok = True
import_detail = ""
pub = None
try:
    try:
        pub = importlib.import_module("repairspans.public")
    except ModuleNotFoundError:
        # Tolerate a layout that exposes merge/overlaps from the package root.
        pub = importlib.import_module("repairspans")
    if not hasattr(pub, "merge") or not hasattr(pub, "overlaps"):
        # Last resort: package root may re-export even if .public did not import.
        root = importlib.import_module("repairspans")
        if hasattr(root, "merge") and hasattr(root, "overlaps"):
            pub = root
    if not hasattr(pub, "merge") or not hasattr(pub, "overlaps"):
        raise ImportError("package does not expose both merge and overlaps")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def _norm(result):
    """Normalize a merge result to a list of [start, end] lists for comparison."""
    return [list(iv) for iv in result]


if import_ok:
    merge = pub.merge
    overlaps = pub.overlaps

    # ----- merge: visible behaviors (re-checked) -----------------------------

    def c_merge_empty():
        r = merge([])
        return _norm(r) == [], f"got={r!r}"

    check("merge_empty", "merge([]) returns []", c_merge_empty)

    def c_merge_single():
        r = merge([[4, 7]])
        return _norm(r) == [[4, 7]], f"got={r!r}"

    check("merge_single", "merge of one interval returns that interval", c_merge_single)

    def c_merge_disjoint():
        r = merge([[1, 2], [5, 6]])
        return _norm(r) == [[1, 2], [5, 6]], f"got={r!r}"

    check("merge_disjoint", "disjoint intervals stay separate", c_merge_disjoint)

    def c_merge_overlapping():
        r = merge([[1, 4], [2, 5]])
        return _norm(r) == [[1, 5]], f"got={r!r}"

    check("merge_overlapping", "overlapping intervals merge into one", c_merge_overlapping)

    def c_merge_touching():
        # BUG 1 target: closed intervals touching at an endpoint must merge.
        r = merge([[1, 2], [2, 3]])
        return _norm(r) == [[1, 3]], f"got={r!r} (touching [1,2],[2,3] must merge to [1,3])"

    check("merge_touching_adjacent", "touching closed intervals merge (off-by-one)", c_merge_touching)

    def c_merge_unsorted():
        # BUG 2 target: input is not guaranteed sorted by start.
        r = merge([[3, 4], [1, 2]])
        return _norm(r) == [[1, 2], [3, 4]], f"got={r!r} (unsorted input must still sort)"

    check("merge_unsorted_input", "unsorted input is handled correctly", c_merge_unsorted)

    # ----- merge: extra held-out edge cases ----------------------------------

    def c_merge_unsorted_overlap():
        # Unsorted AND overlapping: must merge across the (out-of-order) pair.
        r = merge([[2, 5], [1, 3]])
        return _norm(r) == [[1, 5]], f"got={r!r}"

    check("merge_unsorted_overlap", "unsorted overlapping intervals merge", c_merge_unsorted_overlap)

    def c_merge_nested():
        # A fully nested interval is absorbed; the end must not shrink.
        r = merge([[1, 10], [3, 4]])
        return _norm(r) == [[1, 10]], f"got={r!r} (nested interval must not shrink the end)"

    check("merge_nested", "a nested interval is absorbed without shrinking", c_merge_nested)

    def c_merge_nested_unsorted():
        # Nested but the wider interval comes second (also exercises sorting).
        r = merge([[3, 4], [1, 10]])
        return _norm(r) == [[1, 10]], f"got={r!r}"

    check("merge_nested_unsorted", "nested interval absorbed regardless of order", c_merge_nested_unsorted)

    def c_merge_chain():
        # A chain of touching/overlapping intervals collapses to a single span.
        r = merge([[1, 2], [2, 3], [3, 4], [10, 11]])
        return _norm(r) == [[1, 4], [10, 11]], f"got={r!r}"

    check("merge_chain", "a touching chain collapses to one span", c_merge_chain)

    def c_merge_duplicates():
        r = merge([[1, 3], [1, 3], [1, 3]])
        return _norm(r) == [[1, 3]], f"got={r!r}"

    check("merge_duplicates", "identical intervals collapse to one", c_merge_duplicates)

    def c_merge_no_mutate():
        # merge must not mutate the input list or its interval objects.
        src = [[5, 6], [1, 2]]
        snapshot = [list(iv) for iv in src]
        merge(src)
        return src == snapshot, f"input mutated: before={snapshot!r} after={src!r}"

    check("merge_no_mutation", "merge does not mutate its input", c_merge_no_mutate)

    # ----- overlaps: visible + held-out edge cases ---------------------------

    def c_overlaps_true():
        return overlaps([1, 4], [2, 5]) is True, f"got={overlaps([1, 4], [2, 5])!r}"

    check("overlaps_overlap", "overlapping intervals report True", c_overlaps_true)

    def c_overlaps_touch():
        # BUG 3 target: touching closed intervals share an endpoint -> True.
        return overlaps([1, 2], [2, 3]) is True, f"got={overlaps([1, 2], [2, 3])!r} (touch must be True)"

    check("overlaps_touch", "touching closed intervals report True", c_overlaps_touch)

    def c_overlaps_disjoint():
        return overlaps([1, 2], [5, 6]) is False, f"got={overlaps([1, 2], [5, 6])!r}"

    check("overlaps_disjoint", "disjoint intervals report False", c_overlaps_disjoint)

    def c_overlaps_nested():
        return overlaps([1, 10], [3, 4]) is True, f"got={overlaps([1, 10], [3, 4])!r}"

    check("overlaps_nested", "a nested interval overlaps its container", c_overlaps_nested)

    def c_overlaps_identical():
        return overlaps([2, 5], [2, 5]) is True, f"got={overlaps([2, 5], [2, 5])!r}"

    check("overlaps_identical", "identical intervals overlap", c_overlaps_identical)

    def c_overlaps_symmetric():
        # Symmetry across touching, disjoint, and nested cases.
        cases = ([1, 2], [2, 3]), ([1, 2], [5, 6]), ([1, 10], [3, 4])
        for a, b in cases:
            if overlaps(a, b) != overlaps(b, a):
                return False, f"asymmetric for {a!r},{b!r}: {overlaps(a, b)!r} != {overlaps(b, a)!r}"
        return True, "symmetric across touch/disjoint/nested"

    check("overlaps_symmetric", "overlaps is symmetric in its arguments", c_overlaps_symmetric)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 18

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "repairspans",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
