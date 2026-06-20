#!/usr/bin/env python3
"""Held-out behavior-level oracle for the feature-add task `csvgroupby`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never reads the model's own tests. It grades the produced `csvgroupby`
package against the BRIEF'S CONTRACT (the `csvgroupby.public.query` API), not
against any particular internal layout.

The capability to ADD: `GROUP BY <col>` with `COUNT(*)`. A correct solution adds
grouping while keeping the existing SELECT/WHERE behavior. The reference (with
GROUP BY) passes every check; the working STARTER (no GROUP BY) passes the
EXISTING-behavior checks but fails the GROUP BY checks — so the score discriminates
"added the feature" from "left it as shipped".

Output: ONE JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is 0
whenever grading ran to completion (even score 0.0); a failed import forces score
0.0. This grader computes its OWN expected groups from the input rows; it never
trusts the package to tell it the answer.

Tolerance: the brief under-specifies a few representations. This oracle accepts
any contract-conformant shape and checks BEHAVIOR:
  - int-vs-float: COUNT(*) of 3 matches `3` or `3.0`; group keys compare with the
    same numeric tolerance.
  - COUNT(*) key: matched under a NORMALIZED key (case/space/punctuation folded),
    so `COUNT(*)`, `count(*)`, `count` all read as the aggregate.
"""
import importlib
import json
import os
import re
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# FIXED roster of check ids — the denominator never depends on which checks ran, so
# an import crash or a half-built package scores against the same total as a full one.
CHECK_IDS = [
    # --- NEW: GROUP BY + COUNT(*) -------------------------------------------
    "groupby_basic_counts",
    "groupby_one_row_per_group",
    "groupby_honors_where",
    "groupby_numeric_key",
    "groupby_single_group",
    # --- EXISTING (regression): SELECT + WHERE must still work --------------
    "select_cols_projection",
    "select_star_passthrough",
    "where_numeric_ge",
    "where_not_equal_string",
    "where_lt_numeric",
]
TOTAL = len(CHECK_IDS)

results = {}  # cid -> (passed: bool, detail: str)


def record(cid, passed, detail=""):
    results[cid] = (bool(passed), str(detail or ""))


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


# --- tolerant matching helpers ----------------------------------------------

def _num(v):
    """Return v as a float if it is (or looks like) a number, else None."""
    if isinstance(v, bool):
        return None
    if isinstance(v, (int, float)):
        return float(v)
    if isinstance(v, str):
        s = v.strip()
        if re.fullmatch(r"[+-]?(\d+\.\d*|\.\d+|\d+)([eE][+-]?\d+)?", s):
            try:
                return float(s)
            except ValueError:
                return None
    return None


def _val_eq(a, b):
    """Tolerant scalar equality: numbers compare numerically (int-vs-float),
    everything else compares by string form."""
    na, nb = _num(a), _num(b)
    if na is not None and nb is not None:
        return abs(na - nb) < 1e-9
    return str(a) == str(b)


def _norm_key(k):
    """Fold a key to bare alphanumerics, lowercased: 'COUNT(*)' -> 'count'."""
    return re.sub(r"[^a-z0-9]", "", str(k).lower())


_COUNT_KEYS = {"count", "counts", "countstar", "cnt", "n"}


def _get_count(row):
    """Pull the COUNT(*) aggregate from an output row under a normalized key."""
    if not isinstance(row, dict):
        return None
    for k, v in row.items():
        if _norm_key(k) in _COUNT_KEYS:
            return v
    return None


def _get_field(row, name):
    """Pull a (non-count) field from an output row, tolerant to key casing/spacing."""
    if not isinstance(row, dict):
        return None
    if name in row:
        return row[name]
    target = _norm_key(name)
    for k, v in row.items():
        if _norm_key(k) == target:
            return v
    return None


def _group_map(result, group_col):
    """Build {group_value(as compare-key) -> count} from a GROUP BY result, so the
    grader can compare against its OWN computed expectation regardless of row order.
    The compare-key normalizes numbers so 3 and 3.0 collide."""
    out = {}
    for row in result:
        gv = _get_field(row, group_col)
        cnt = _get_count(row)
        n = _num(gv)
        key = ("num", round(n, 9)) if n is not None else ("str", str(gv))
        out[key] = cnt
    return out


def _expected_groups(rows, group_col, where=None):
    """Compute the grader's OWN expected {group-key -> count}, in first-seen order.
    `where` is an optional predicate over a row dict."""
    counts = {}
    for r in rows:
        if where is not None and not where(r):
            continue
        gv = r.get(group_col)
        n = _num(gv)
        key = ("num", round(n, 9)) if n is not None else ("str", str(gv))
        counts[key] = counts.get(key, 0) + 1
    return counts


# --- import the produced package (contract: csvgroupby.public.query) ---------
import_ok = True
import_detail = ""
query = None
try:
    try:
        pub = importlib.import_module("csvgroupby.public")
    except Exception:  # noqa: BLE001 - fall back to the top-level re-export
        pub = importlib.import_module("csvgroupby")
    query = getattr(pub, "query")
    if not callable(query):
        raise TypeError("csvgroupby.public.query is not callable")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# shared fixtures (string-valued cells, as read from a CSV)
PEOPLE = [
    {"city": "NYC", "age": "30"},
    {"city": "LA", "age": "17"},
    {"city": "NYC", "age": "22"},
    {"city": "LA", "age": "40"},
    {"city": "NYC", "age": "12"},
]
NUMS = [
    {"bucket": "1", "v": "a"},
    {"bucket": "2", "v": "b"},
    {"bucket": "1", "v": "c"},
    {"bucket": "1", "v": "d"},
]


if import_ok:
    # ====================== NEW: GROUP BY + COUNT(*) ======================

    # 1. Basic group counts: one row per city with the right COUNT(*).
    def c_groupby_basic_counts():
        result = query(PEOPLE, "SELECT city, COUNT(*) FROM t GROUP BY city")
        if not isinstance(result, list):
            return False, f"result type={type(result).__name__}"
        got = _group_map(result, "city")
        want = _expected_groups(PEOPLE, "city")
        return (got == want), f"got={got!r} want={want!r}"

    check("groupby_basic_counts", "GROUP BY yields correct COUNT(*) per group", c_groupby_basic_counts)

    # 2. Exactly one output row per distinct group (no dupes, no missing).
    def c_groupby_one_row_per_group():
        result = query(PEOPLE, "SELECT city, COUNT(*) FROM t GROUP BY city")
        if not isinstance(result, list):
            return False, f"result type={type(result).__name__}"
        distinct = {("num", round(_num(r["city"]), 9)) if _num(r["city"]) is not None
                    else ("str", str(r["city"])) for r in PEOPLE}
        return (len(result) == len(distinct)), f"rows={len(result)} distinct={len(distinct)}"

    check("groupby_one_row_per_group", "GROUP BY emits exactly one row per distinct value", c_groupby_one_row_per_group)

    # 3. GROUP BY honors WHERE: filter first, then count survivors per group.
    def c_groupby_honors_where():
        result = query(PEOPLE, "SELECT city, COUNT(*) FROM t WHERE age >= 18 GROUP BY city")
        got = _group_map(result, "city")
        want = _expected_groups(PEOPLE, "city", where=lambda r: int(r["age"]) >= 18)
        return (got == want), f"got={got!r} want={want!r}"

    check("groupby_honors_where", "GROUP BY counts only WHERE-surviving rows", c_groupby_honors_where)

    # 4. Group key may be numeric-looking; counts still correct, keys still distinct.
    def c_groupby_numeric_key():
        result = query(NUMS, "SELECT bucket, COUNT(*) FROM t GROUP BY bucket")
        got = _group_map(result, "bucket")
        want = _expected_groups(NUMS, "bucket")
        return (got == want), f"got={got!r} want={want!r}"

    check("groupby_numeric_key", "GROUP BY over a numeric-looking column counts correctly", c_groupby_numeric_key)

    # 5. A single-group table -> one row whose count is the whole (filtered) table.
    def c_groupby_single_group():
        rows = [{"k": "x", "n": "1"}, {"k": "x", "n": "2"}, {"k": "x", "n": "3"}]
        result = query(rows, "SELECT k, COUNT(*) FROM t GROUP BY k")
        if not isinstance(result, list) or len(result) != 1:
            return False, f"expected 1 row, got {result!r}"
        cnt = _get_count(result[0])
        return _val_eq(cnt, 3), f"count={cnt!r}"

    check("groupby_single_group", "GROUP BY with one distinct value returns a single full-count row", c_groupby_single_group)

    # ================ EXISTING (regression): SELECT + WHERE ================

    # 6. Column projection: SELECT <cols> keeps only those keys, one row per input.
    def c_select_cols_projection():
        result = query(PEOPLE, "SELECT city FROM t")
        if not isinstance(result, list) or len(result) != len(PEOPLE):
            return False, f"result={result!r}"
        ok = all(
            _get_field(r, "city") is not None and _val_eq(_get_field(r, "city"), src["city"])
            for r, src in zip(result, PEOPLE)
        )
        # projection should not surface the un-selected 'age' column
        no_age = all(_get_field(r, "age") is None for r in result)
        return (ok and no_age), f"result={result!r}"

    check("select_cols_projection", "SELECT <cols> projects only the named columns", c_select_cols_projection)

    # 7. SELECT * passes the whole row through (still one row per input row).
    def c_select_star_passthrough():
        result = query(PEOPLE, "SELECT * FROM t")
        if not isinstance(result, list) or len(result) != len(PEOPLE):
            return False, f"len={len(result) if isinstance(result, list) else result!r}"
        ok = all(
            _val_eq(_get_field(r, "city"), src["city"]) and _val_eq(_get_field(r, "age"), src["age"])
            for r, src in zip(result, PEOPLE)
        )
        return ok, f"result={result!r}"

    check("select_star_passthrough", "SELECT * returns every column of every row", c_select_star_passthrough)

    # 8. WHERE >= with numeric inference (string cells compared as numbers).
    def c_where_numeric_ge():
        result = query(PEOPLE, "SELECT city, age FROM t WHERE age >= 18")
        if not isinstance(result, list):
            return False, f"result type={type(result).__name__}"
        ages = sorted(int(str(_get_field(r, "age"))) for r in result)
        want = sorted(int(p["age"]) for p in PEOPLE if int(p["age"]) >= 18)
        return (ages == want), f"ages={ages!r} want={want!r}"

    check("where_numeric_ge", "WHERE col >= n filters by numeric inference", c_where_numeric_ge)

    # 9. WHERE != on a string column.
    def c_where_not_equal_string():
        result = query(PEOPLE, "SELECT city FROM t WHERE city != 'LA'")
        cities = sorted(str(_get_field(r, "city")) for r in result)
        want = sorted(p["city"] for p in PEOPLE if p["city"] != "LA")
        return (cities == want), f"cities={cities!r} want={want!r}"

    check("where_not_equal_string", "WHERE col != 'str' filters out matching rows", c_where_not_equal_string)

    # 10. WHERE < with numeric inference.
    def c_where_lt_numeric():
        result = query(PEOPLE, "SELECT age FROM t WHERE age < 18")
        ages = sorted(int(str(_get_field(r, "age"))) for r in result)
        want = sorted(int(p["age"]) for p in PEOPLE if int(p["age"]) < 18)
        return (ages == want), f"ages={ages!r} want={want!r}"

    check("where_lt_numeric", "WHERE col < n filters by numeric inference", c_where_lt_numeric)


# --- assemble the scorecard over the FIXED denominator ----------------------
checks = []
for cid in CHECK_IDS:
    passed, detail = results.get(cid, (False, "not run (import failed)"))
    checks.append({"id": cid, "passed": passed, "detail": detail})

passed = sum(1 for c in checks if c["passed"])
card = {
    "task": "csvgroupby",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": TOTAL,
    # An unimportable package scores a hard 0.0, regardless of any partial credit.
    "score": 0.0 if not import_ok else round(passed / TOTAL, 4),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
