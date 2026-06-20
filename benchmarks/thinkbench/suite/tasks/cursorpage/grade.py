#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `cursorpage`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `cursorpage`
package against the BRIEF'S CONTRACT (cursor/keyset pagination via `page_after`,
plus the unchanged offset `page`), NOT against any particular internal layout.

The defining behaviors under test are the ones a SUPERFICIAL implementation gets
wrong:

  * walking with `next_cursor` must visit every record EXACTLY ONCE even when
    sort keys TIE — a cursor that encodes only the sort key (not the id) either
    re-emits or skips the tied records;
  * `next_cursor` must be None EXACTLY on the last page — an impl that always
    returns a token (or returns a non-None cursor on an empty trailing page)
    fails the termination checks and can loop forever;
  * a cursor landing in the MIDDLE of a run of tied keys must resume at the very
    next record (strictly-after semantics), neither dropping nor repeating;
  * a None / invalid / malformed cursor must start from the beginning WITHOUT
    raising;
  * offset `page(n, size)` must be untouched (regression).

A naive `page_after` that keys the cursor on the sort value alone, or that omits
the strictly-after tie-break, passes the simple no-tie checks while failing the
tie / termination / mid-tie checks — that's what makes the task discriminate
(naive lands well under 1.0; a careful keyset implementation lands at 1.0).

Output: a single JSON scorecard on stdout. Each check runs in isolation, so the
score is continuous (passed / total), never all-or-nothing. FIXED DENOMINATOR:
the full check roster is declared up front, so an import failure records every
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
CHECK_SPECS = [
    ("first_page_no_cursor", "page_after(None, size) returns the first `size` records in sorted order"),
    ("first_page_next_cursor_present", "a non-final first page yields a non-None next_cursor"),
    ("walk_all_once_no_ties", "walking via next_cursor visits every record exactly once (no ties)"),
    ("last_page_cursor_none", "the final page returns next_cursor=None"),
    ("walk_all_once_with_ties", "walking visits every record exactly once when sort keys tie"),
    ("resume_mid_tie_no_dup_no_gap", "a cursor inside a run of tied keys resumes strictly after it"),
    ("opaque_cursor_round_trips", "feeding back next_cursor resumes at the right place (cursor is opaque)"),
    ("size_one_walk_full", "size=1 walk yields the full sorted order one at a time"),
    ("size_larger_than_data", "size >= len returns all records with next_cursor=None"),
    ("empty_paginator", "page_after(None, size) on empty data returns [] and next_cursor=None"),
    ("none_cursor_from_start", "a None cursor starts from the beginning"),
    ("invalid_cursor_from_start", "a garbage/invalid cursor starts from the beginning without raising"),
    ("size_nonpositive_raises", "size <= 0 raises ValueError in page_after"),
    ("regression_offset_paging", "offset page(n, size) still returns the correct sorted pages"),
    ("regression_offset_out_of_range", "offset page() past the end returns an empty list"),
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


# --- import the produced package (contract: cursorpage.public, fallback pkg) ---
import_ok = True
import_detail = ""
Paginator = None
try:
    try:
        mod = importlib.import_module("cursorpage.public")
    except Exception:
        mod = importlib.import_module("cursorpage")
    Paginator = getattr(mod, "Paginator")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- helpers the grader uses to compute EXPECTED results itself ----------------
def sorted_ids(rows, key):
    """The fully-deterministic order the contract specifies: (key, then id)."""
    return [r["id"] for r in sorted(rows, key=lambda r: (r[key], r["id"]))]


def walk(p, size, start=None):
    """Walk a Paginator with cursor pagination, returning (ids_in_order,
    pages_count). Guards against a non-terminating cursor by capping iterations
    well above any honest walk length."""
    ids = []
    cursor = start
    pages = 0
    cap = 1000
    while True:
        out = p.page_after(cursor, size)
        items = out["items"]
        ids.extend(r["id"] for r in items)
        pages += 1
        cursor = out["next_cursor"]
        if cursor is None:
            break
        if pages > cap:  # a broken cursor that never terminates
            raise AssertionError("page_after did not terminate (next_cursor never None)")
    return ids, pages


if import_ok:
    NO_TIE = [
        {"id": 10, "score": 1},
        {"id": 20, "score": 2},
        {"id": 30, "score": 3},
        {"id": 40, "score": 4},
        {"id": 50, "score": 5},
    ]
    # Heavy ties on `score`: several records share each score, so a cursor must
    # carry the id to separate them.
    TIES = [
        {"id": 3, "score": 5},
        {"id": 1, "score": 5},
        {"id": 5, "score": 5},
        {"id": 2, "score": 9},
        {"id": 8, "score": 9},
        {"id": 4, "score": 1},
        {"id": 7, "score": 5},
        {"id": 6, "score": 9},
    ]

    # 1. first page with no cursor == first `size` of the sorted order.
    def c_first_page_no_cursor():
        p = Paginator(list(NO_TIE), key="score")
        out = p.page_after(None, 2)
        got = [r["id"] for r in out["items"]]
        exp = sorted_ids(NO_TIE, "score")[:2]
        return got == exp, f"first page ids {got!r} (expected {exp!r})"

    check("first_page_no_cursor", c_first_page_no_cursor)

    # 2. a non-final first page must hand back a non-None cursor.
    def c_first_page_next_cursor_present():
        p = Paginator(list(NO_TIE), key="score")
        out = p.page_after(None, 2)  # 5 records, page of 2 -> more remain
        return out["next_cursor"] is not None, \
            f"next_cursor={out['next_cursor']!r} (expected a token, more records remain)"

    check("first_page_next_cursor_present", c_first_page_next_cursor_present)

    # 3. walk the whole thing (no ties): every id once, in sorted order.
    def c_walk_all_once_no_ties():
        p = Paginator(list(NO_TIE), key="score")
        got, _ = walk(p, 2)
        exp = sorted_ids(NO_TIE, "score")
        return got == exp, f"walked {got!r} (expected {exp!r}, each exactly once)"

    check("walk_all_once_no_ties", c_walk_all_once_no_ties)

    # 4. the final page reports next_cursor=None.
    def c_last_page_cursor_none():
        p = Paginator(list(NO_TIE), key="score")
        # 5 records, size 2 -> pages of [.,.][.,.][.] ; last must terminate.
        c = None
        last = None
        for _ in range(10):
            out = p.page_after(c, 2)
            last = out
            c = out["next_cursor"]
            if c is None:
                break
        return last is not None and last["next_cursor"] is None, \
            f"last page next_cursor={None if last is None else last['next_cursor']!r} (expected None)"

    check("last_page_cursor_none", c_last_page_cursor_none)

    # 5. THE tie check: walking with ties visits every record exactly once.
    def c_walk_all_once_with_ties():
        p = Paginator(list(TIES), key="score")
        got, _ = walk(p, 2)
        exp = sorted_ids(TIES, "score")
        # Both length (no dupes/gaps) and order must match.
        same = (got == exp)
        return same, f"walked {got!r} (expected {exp!r}; len got={len(got)} exp={len(exp)})"

    check("walk_all_once_with_ties", c_walk_all_once_with_ties)

    # 6. resume from a cursor INSIDE a run of tied keys: no dup, no gap.
    def c_resume_mid_tie_no_dup_no_gap():
        p = Paginator(list(TIES), key="score")
        exp = sorted_ids(TIES, "score")
        # Grab the first page of 1 (the first score-1... actually first overall),
        # then take a page of 1 repeatedly until we are mid-way through a tie run,
        # i.e. after the FIRST of the score==5 records. Easiest robust approach:
        # walk one at a time and confirm the running prefix always matches exp.
        got = []
        cursor = None
        for _ in range(len(exp) + 2):
            out = p.page_after(cursor, 1)
            got.extend(r["id"] for r in out["items"])
            cursor = out["next_cursor"]
            if cursor is None:
                break
        return got == exp, f"one-at-a-time walk {got!r} (expected {exp!r}; mid-tie resume must not dup/gap)"

    check("resume_mid_tie_no_dup_no_gap", c_resume_mid_tie_no_dup_no_gap)

    # 7. the cursor is opaque but must round-trip: feeding it back resumes
    #    exactly after the last returned record (verified across a tie boundary).
    def c_opaque_cursor_round_trips():
        p = Paginator(list(TIES), key="score")
        exp = sorted_ids(TIES, "score")
        out1 = p.page_after(None, 3)        # first three in sorted order
        first3 = [r["id"] for r in out1["items"]]
        out2 = p.page_after(out1["next_cursor"], 3)
        next3 = [r["id"] for r in out2["items"]]
        got = first3 + next3
        exp6 = exp[:6]
        return got == exp6, f"two pages of 3 -> {got!r} (expected {exp6!r})"

    check("opaque_cursor_round_trips", c_opaque_cursor_round_trips)

    # 8. size=1 walk yields the full sorted order, one per page.
    def c_size_one_walk_full():
        p = Paginator(list(TIES), key="score")
        got, pages = walk(p, 1)
        exp = sorted_ids(TIES, "score")
        ok = (got == exp and pages == len(exp))
        return ok, f"got {got!r} in {pages} pages (expected {exp!r} in {len(exp)} pages)"

    check("size_one_walk_full", c_size_one_walk_full)

    # 9. size >= len returns everything and terminates immediately.
    def c_size_larger_than_data():
        p = Paginator(list(NO_TIE), key="score")
        out = p.page_after(None, 100)
        got = [r["id"] for r in out["items"]]
        exp = sorted_ids(NO_TIE, "score")
        ok = (got == exp and out["next_cursor"] is None)
        return ok, f"ids={got!r} next_cursor={out['next_cursor']!r} (expected {exp!r} / None)"

    check("size_larger_than_data", c_size_larger_than_data)

    # 10. empty paginator: empty page, no cursor.
    def c_empty_paginator():
        p = Paginator([], key="score")
        out = p.page_after(None, 5)
        ok = (out["items"] == [] and out["next_cursor"] is None)
        return ok, f"items={out['items']!r} next_cursor={out['next_cursor']!r} (expected []/None)"

    check("empty_paginator", c_empty_paginator)

    # 11. a None cursor starts from the beginning (same as first page).
    def c_none_cursor_from_start():
        p = Paginator(list(TIES), key="score")
        a = [r["id"] for r in p.page_after(None, 4)["items"]]
        exp = sorted_ids(TIES, "score")[:4]
        return a == exp, f"None-cursor page {a!r} (expected {exp!r})"

    check("none_cursor_from_start", c_none_cursor_from_start)

    # 12. a garbage cursor starts from the beginning and does NOT raise.
    def c_invalid_cursor_from_start():
        p = Paginator(list(TIES), key="score")
        exp = sorted_ids(TIES, "score")[:4]
        outs = []
        for bad in ("not-a-cursor", "!!!!", "", "0", "[]", "eyJ4Ijox"):
            out = p.page_after(bad, 4)
            outs.append([r["id"] for r in out["items"]])
        ok = all(o == exp for o in outs)
        return ok, f"invalid-cursor pages {outs!r} (each expected {exp!r}, no raise)"

    check("invalid_cursor_from_start", c_invalid_cursor_from_start)

    # 13. size <= 0 raises ValueError.
    def c_size_nonpositive_raises():
        p = Paginator(list(NO_TIE), key="score")
        results_local = []
        for bad in (0, -1):
            try:
                p.page_after(None, bad)
                results_local.append(("no-raise", bad))
            except ValueError:
                results_local.append(("ValueError", bad))
            except Exception as e:  # noqa: BLE001
                results_local.append((type(e).__name__, bad))
        ok = all(kind == "ValueError" for kind, _ in results_local)
        return ok, f"{results_local!r} (expected ValueError for each)"

    check("size_nonpositive_raises", c_size_nonpositive_raises)

    # 14. REGRESSION: offset page(n, size) still returns the right sorted pages.
    def c_regression_offset_paging():
        p = Paginator(list(TIES), key="score")
        exp = sorted_ids(TIES, "score")
        p0 = [r["id"] for r in p.page(0, 3)]
        p1 = [r["id"] for r in p.page(1, 3)]
        p2 = [r["id"] for r in p.page(2, 3)]
        got = p0 + p1 + p2
        ok = (p0 == exp[0:3] and p1 == exp[3:6] and p2 == exp[6:9] and got == exp)
        return ok, f"pages 0/1/2 -> {p0!r}/{p1!r}/{p2!r} (expected {exp[0:3]!r}/{exp[3:6]!r}/{exp[6:9]!r})"

    check("regression_offset_paging", c_regression_offset_paging)

    # 15. REGRESSION: offset page past the end returns an empty list.
    def c_regression_offset_out_of_range():
        p = Paginator(list(NO_TIE), key="score")
        got = p.page(99, 10)
        return got == [], f"page(99, 10) -> {got!r} (expected [])"

    check("regression_offset_out_of_range", c_regression_offset_out_of_range)


# --- assemble the scorecard with a FIXED denominator -------------------------
checks_out = []
for cid in CHECK_IDS:
    r = results.get(cid)
    if r is None:
        r = {"passed": False, "detail": "not run (import failed)" if not import_ok else "not run"}
    checks_out.append({"id": cid, "desc": DESC[cid], "passed": r["passed"], "detail": r["detail"]})

passed = sum(1 for c in checks_out if c["passed"])
total = len(checks_out)  # always len(CHECK_SPECS): fixed denominator
card = {
    "task": "cursorpage",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
