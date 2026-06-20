#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `repairpager`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never reads or runs the agent's own `test_*.py`. It grades the produced
`repairpager` package against the BRIEF'S CONTRACT (the `repairpager` /
`repairpager.public` `paginate` API and return shape), NOT against the visible
test file and NOT against any particular internal file layout.

The visible suite is a SUBSET; this oracle is a SUPERSET that also pins the
edge cases the visible tests leave implicit: a trailing partial page, an exact
multiple of page_size, a single full page, an empty input, and an out-of-range
page request. Expected values are computed here independently — the grader never
runs `test_repairpager.py`.

Planted bugs the fixed package must repair (all three are exercised below):
  1. total_pages floored instead of ceil'd -> a trailing partial page is dropped.
  2. off-by-one slice start -> every page returns the wrong items.
  3. has_next uses `<=` instead of `<` -> the last page wrongly reports a next page.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs).
"""
import importlib
import json
import os
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --- import the produced package (contract: repairpager.public, fallback repairpager) ---
import_ok = True
import_detail = ""
paginate = None
try:
    try:
        mod = importlib.import_module("repairpager.public")
    except Exception:  # noqa: BLE001 - tolerate a flattened package without .public
        mod = importlib.import_module("repairpager")
    paginate = getattr(mod, "paginate")
    if not callable(paginate):
        raise TypeError("paginate is not callable")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def get(result, key):
    """Pull `key` from a paginate result, tolerating non-dict returns."""
    if not isinstance(result, dict):
        raise TypeError(f"result is {type(result).__name__}, expected dict")
    if key not in result:
        raise KeyError(f"missing key {key!r} (got keys {sorted(result)})")
    return result[key]


if import_ok:
    SEQ5 = [10, 20, 30, 40, 50]   # 5 items -> with size 2: pages [10,20][30,40][50]
    SEQ6 = [1, 2, 3, 4, 5, 6]     # 6 items -> exact multiple of size 3: 2 pages

    # 1. page 1 returns the FIRST page_size items, in order (catches the slice off-by-one).
    def c_first_page():
        r = paginate(SEQ5, 2, 1)
        return get(r, "items") == [10, 20], f"items={get(r, 'items')!r}"

    check("first_page_items", "page 1 returns the first page_size items in order", c_first_page)

    # 2. an interior page returns the correct middle chunk.
    def c_middle_page():
        r = paginate(SEQ5, 2, 2)
        return get(r, "items") == [30, 40], f"items={get(r, 'items')!r}"

    check("middle_page_items", "an interior page returns the correct chunk", c_middle_page)

    # 3. the trailing PARTIAL page holds exactly the leftover items.
    def c_last_partial_items():
        r = paginate(SEQ5, 2, 3)
        return get(r, "items") == [50], f"items={get(r, 'items')!r}"

    check("last_partial_page_items", "the trailing partial page holds the leftover items", c_last_partial_items)

    # 4. total_pages uses CEIL: 5 items / size 2 -> 3 pages (catches the floor bug).
    def c_total_pages_ceil():
        r = paginate(SEQ5, 2, 1)
        return get(r, "total_pages") == 3, f"total_pages={get(r, 'total_pages')!r}"

    check("total_pages_ceil_partial", "total_pages rounds up for a trailing partial page", c_total_pages_ceil)

    # 5. total_pages for an EXACT multiple is exactly that many pages (no spurious extra page).
    def c_total_pages_exact():
        r = paginate(SEQ6, 3, 1)
        return get(r, "total_pages") == 2, f"total_pages={get(r, 'total_pages')!r}"

    check("total_pages_exact_multiple", "an exact multiple yields exactly that many pages", c_total_pages_exact)

    # 6. total_items always equals the input length.
    def c_total_items():
        r = paginate(SEQ5, 2, 1)
        return get(r, "total_items") == 5, f"total_items={get(r, 'total_items')!r}"

    check("total_items_count", "total_items equals the number of input items", c_total_items)

    # 7. has_next/has_prev on the FIRST page of a multi-page result.
    def c_flags_first():
        r = paginate(SEQ5, 2, 1)
        hn, hp = get(r, "has_next"), get(r, "has_prev")
        return (hn is True and hp is False), f"has_next={hn!r} has_prev={hp!r}"

    check("flags_first_page", "first page: has_next True, has_prev False", c_flags_first)

    # 8. has_next is FALSE on the LAST page (catches the `<=` boundary bug).
    def c_has_next_last():
        r = paginate(SEQ5, 2, 3)
        hn, hp = get(r, "has_next"), get(r, "has_prev")
        return (hn is False and hp is True), f"has_next={hn!r} has_prev={hp!r}"

    check("has_next_false_last_page", "last page: has_next False, has_prev True", c_has_next_last)

    # 9. an interior page reports BOTH neighbours present.
    def c_flags_middle():
        r = paginate(SEQ5, 2, 2)
        hn, hp = get(r, "has_next"), get(r, "has_prev")
        return (hn is True and hp is True), f"has_next={hn!r} has_prev={hp!r}"

    check("flags_middle_page", "interior page: has_next and has_prev both True", c_flags_middle)

    # 10. a SINGLE full page: everything fits, no next/prev.
    def c_single_page():
        r = paginate([1, 2, 3], 3, 1)
        return (
            get(r, "items") == [1, 2, 3]
            and get(r, "total_pages") == 1
            and get(r, "has_next") is False
            and get(r, "has_prev") is False
        ), (
            f"items={get(r, 'items')!r} total_pages={get(r, 'total_pages')!r} "
            f"has_next={get(r, 'has_next')!r} has_prev={get(r, 'has_prev')!r}"
        )

    check("single_full_page", "a single full page has no next or prev", c_single_page)

    # 11. an EMPTY input is one empty page with no navigation.
    def c_empty():
        r = paginate([], 4, 1)
        return (
            get(r, "items") == []
            and get(r, "total_items") == 0
            and get(r, "total_pages") == 1
            and get(r, "has_next") is False
            and get(r, "has_prev") is False
        ), (
            f"items={get(r, 'items')!r} total_items={get(r, 'total_items')!r} "
            f"total_pages={get(r, 'total_pages')!r} has_next={get(r, 'has_next')!r} "
            f"has_prev={get(r, 'has_prev')!r}"
        )

    check("empty_input", "empty input is a single empty page with no navigation", c_empty)

    # 12. an OUT-OF-RANGE page is clamped to the last page (not an empty/garbage page).
    def c_out_of_range():
        r = paginate(SEQ5, 2, 99)  # only 3 pages exist
        return (
            get(r, "page") == 3
            and get(r, "items") == [50]
            and get(r, "has_next") is False
        ), f"page={get(r, 'page')!r} items={get(r, 'items')!r} has_next={get(r, 'has_next')!r}"

    check("out_of_range_page_clamped", "a too-large page clamps to the last real page", c_out_of_range)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 12

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "repairpager",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
