#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `querygroup`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `querygroup`
package against the BRIEF'S CONTRACT (a `group_by(keys, aggregates)` that
collapses rows into one output row per distinct key-tuple, composing after a
`where`, in first-appearance order, with None-aware aggregates), plus the
unchanged core API (`where` / `order_by` / `rows`), NOT against any particular
internal file layout.

The defining behaviors under test are the ones a SUPERFICIAL implementation gets
wrong:

  * GROUP ORDER is first-appearance, not sorted — an impl that emits sorted keys
    (or relies on an unordered set) reorders groups;
  * `avg` divides by the count of NON-None values, not the row count — a naive
    `sum(vals)/len(members)` is wrong whenever a field has a None;
  * `avg` / `min` / `max` of an all-None (or empty) field are None, and `sum` is
    0 — a naive impl divides by zero (crash) or returns 0/garbage;
  * grouping must run on the FILTERED rows when chained after `where`;
  * `count` counts rows, including rows whose aggregate field is None.

A naive `group_by` (sorted keys, sum/len over the whole group, no None-guarding)
passes the simple single-group / sum / count checks but fails ordering, the
None-aware avg, the empty-field, and possibly the filtered-then-grouped checks —
so it lands well under 1.0, while a careful implementation lands at 1.0.

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
    ("single_group_count_sum", "one group: count counts rows, sum sums the field"),
    ("multi_group_basic", "two groups, each with correct count/sum"),
    ("first_appearance_order", "groups emitted in first-appearance order, not sorted"),
    ("avg_divides_by_nonnull", "avg divides by count of non-None values, not row count"),
    ("count_includes_null_rows", "count counts a row even when its aggregate field is None"),
    ("min_max_skip_null", "min/max ignore None values in the field"),
    ("all_null_field_aggs", "all-None field: avg/min/max are None, sum is 0"),
    ("empty_group_via_filter", "empty Query (after a filter) groups to no rows"),
    ("group_after_where", "group_by composes on the filtered rows after where()"),
    ("multi_key_tuple", "grouping on two key columns keys by the tuple"),
    ("default_alias_names", "default aggregate column name is '<func>_<field>'"),
    ("custom_alias_used", "an explicit alias names the output column"),
    ("result_is_chainable", "group_by returns a Query that can be order_by'd / rows()'d"),
    ("avg_exact_not_rounded", "avg is the exact mean, not rounded/truncated"),
    ("regression_where", "where() still filters rows with no grouping"),
    ("regression_order_by", "order_by() still sorts rows (stable, reverse) with no grouping"),
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


# --- import the produced package (contract: querygroup.public, fallback pkg) ---
import_ok = True
import_detail = ""
Query = None
try:
    try:
        mod = importlib.import_module("querygroup.public")
    except Exception:
        mod = importlib.import_module("querygroup")
    Query = getattr(mod, "Query")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def _row_by_key(out_rows, key_col, key_val):
    """Find the single output row whose ``key_col`` equals ``key_val``."""
    for r in out_rows:
        if r.get(key_col) == key_val:
            return r
    return None


if import_ok:
    # 1. single group: count counts rows; sum sums the field.
    def c_single_group_count_sum():
        q = Query([
            {"d": "a", "n": 2},
            {"d": "a", "n": 3},
            {"d": "a", "n": 5},
        ])
        out = q.group_by("d", [("count", "n", None), ("sum", "n", None)]).rows()
        ok = (len(out) == 1
              and out[0].get("d") == "a"
              and out[0].get("count_n") == 3
              and out[0].get("sum_n") == 10)
        return ok, f"out={out!r} (expected count_n=3, sum_n=10)"

    check("single_group_count_sum", c_single_group_count_sum)

    # 2. two groups, each with correct count/sum.
    def c_multi_group_basic():
        q = Query([
            {"d": "a", "n": 1},
            {"d": "b", "n": 10},
            {"d": "a", "n": 4},
            {"d": "b", "n": 20},
        ])
        out = q.group_by("d", [("count", "n", None), ("sum", "n", None)]).rows()
        a = _row_by_key(out, "d", "a")
        b = _row_by_key(out, "d", "b")
        ok = (len(out) == 2
              and a is not None and a.get("count_n") == 2 and a.get("sum_n") == 5
              and b is not None and b.get("count_n") == 2 and b.get("sum_n") == 30)
        return ok, f"out={out!r} (expected a:2/5, b:2/30)"

    check("multi_group_basic", c_multi_group_basic)

    # 3. THE ordering check: groups in first-appearance order, NOT sorted.
    def c_first_appearance_order():
        # First-appearance order is c, a, b; sorted would be a, b, c.
        q = Query([
            {"d": "c", "n": 1},
            {"d": "a", "n": 1},
            {"d": "b", "n": 1},
            {"d": "c", "n": 1},
            {"d": "a", "n": 1},
        ])
        out = q.group_by("d", [("count", "n", None)]).rows()
        keys = [r.get("d") for r in out]
        return keys == ["c", "a", "b"], f"group order={keys!r} (expected ['c','a','b'], not sorted)"

    check("first_appearance_order", c_first_appearance_order)

    # 4. avg divides by the count of NON-None values, not the row count.
    def c_avg_divides_by_nonnull():
        # 3 rows, one None -> non-None [10, 20]; mean is 15.0, NOT 30/3 == 10.0.
        q = Query([
            {"d": "a", "n": 10},
            {"d": "a", "n": None},
            {"d": "a", "n": 20},
        ])
        out = q.group_by("d", [("avg", "n", None)]).rows()
        v = out[0].get("avg_n") if out else None
        return v == 15.0, f"avg_n={v!r} (expected 15.0, not 10.0 from /3)"

    check("avg_divides_by_nonnull", c_avg_divides_by_nonnull)

    # 5. count counts a row even when its aggregate field is None.
    def c_count_includes_null_rows():
        q = Query([
            {"d": "a", "n": 1},
            {"d": "a", "n": None},
            {"d": "a", "n": 3},
        ])
        out = q.group_by("d", [("count", "n", None), ("sum", "n", None)]).rows()
        c = out[0].get("count_n") if out else None
        s = out[0].get("sum_n") if out else None
        return (c == 3 and s == 4), f"count_n={c!r} sum_n={s!r} (expected 3 and 4)"

    check("count_includes_null_rows", c_count_includes_null_rows)

    # 6. min/max ignore None values in the field.
    def c_min_max_skip_null():
        q = Query([
            {"d": "a", "n": None},
            {"d": "a", "n": 7},
            {"d": "a", "n": 2},
            {"d": "a", "n": None},
        ])
        out = q.group_by("d", [("min", "n", None), ("max", "n", None)]).rows()
        mn = out[0].get("min_n") if out else "X"
        mx = out[0].get("max_n") if out else "X"
        return (mn == 2 and mx == 7), f"min_n={mn!r} max_n={mx!r} (expected 2 and 7)"

    check("min_max_skip_null", c_min_max_skip_null)

    # 7. all-None field: avg/min/max are None, sum is 0 (no crash).
    def c_all_null_field_aggs():
        q = Query([
            {"d": "a", "n": None},
            {"d": "a", "n": None},
        ])
        out = q.group_by("d", [
            ("avg", "n", None), ("min", "n", None),
            ("max", "n", None), ("sum", "n", None), ("count", "n", None),
        ]).rows()
        r = out[0] if out else {}
        ok = (r.get("avg_n") is None
              and r.get("min_n") is None
              and r.get("max_n") is None
              and r.get("sum_n") == 0
              and r.get("count_n") == 2)
        return ok, f"row={r!r} (expected avg/min/max=None, sum=0, count=2)"

    check("all_null_field_aggs", c_all_null_field_aggs)

    # 8. grouping an empty Query (here produced by a filter) yields no rows.
    def c_empty_group_via_filter():
        q = Query([{"d": "a", "n": 1}, {"d": "b", "n": 2}])
        out = q.where(lambda r: r["n"] > 100).group_by("d", [("count", "n", None)]).rows()
        return out == [], f"out={out!r} (expected [])"

    check("empty_group_via_filter", c_empty_group_via_filter)

    # 9. group_by composes on the FILTERED rows after where().
    def c_group_after_where():
        q = Query([
            {"d": "a", "n": 1, "keep": True},
            {"d": "a", "n": 100, "keep": False},
            {"d": "a", "n": 3, "keep": True},
            {"d": "b", "n": 5, "keep": False},
        ])
        out = (q.where(lambda r: r["keep"])
                .group_by("d", [("count", "n", None), ("sum", "n", None)])
                .rows())
        # Only the two kept 'a' rows survive; 'b' is filtered out entirely.
        ok = (len(out) == 1
              and out[0].get("d") == "a"
              and out[0].get("count_n") == 2
              and out[0].get("sum_n") == 4)
        return ok, f"out={out!r} (expected single a: count 2, sum 4)"

    check("group_after_where", c_group_after_where)

    # 10. grouping on two key columns keys by the tuple of both.
    def c_multi_key_tuple():
        q = Query([
            {"r": "us", "t": "x", "n": 1},
            {"r": "us", "t": "y", "n": 2},
            {"r": "us", "t": "x", "n": 4},
            {"r": "eu", "t": "x", "n": 8},
        ])
        out = q.group_by(["r", "t"], [("sum", "n", None)]).rows()
        # Distinct (r,t): (us,x), (us,y), (eu,x) in first-appearance order.
        usx = next((r for r in out if r.get("r") == "us" and r.get("t") == "x"), None)
        usy = next((r for r in out if r.get("r") == "us" and r.get("t") == "y"), None)
        eux = next((r for r in out if r.get("r") == "eu" and r.get("t") == "x"), None)
        ok = (len(out) == 3
              and usx is not None and usx.get("sum_n") == 5
              and usy is not None and usy.get("sum_n") == 2
              and eux is not None and eux.get("sum_n") == 8)
        return ok, f"out={out!r} (expected (us,x)=5, (us,y)=2, (eu,x)=8)"

    check("multi_key_tuple", c_multi_key_tuple)

    # 11. default aggregate column name is '<func>_<field>'.
    def c_default_alias_names():
        q = Query([{"d": "a", "pay": 5}, {"d": "a", "pay": 7}])
        out = q.group_by("d", [("sum", "pay", None), ("avg", "pay", None)]).rows()
        r = out[0] if out else {}
        ok = ("sum_pay" in r and r.get("sum_pay") == 12
               and "avg_pay" in r and r.get("avg_pay") == 6.0)
        return ok, f"row={r!r} (expected keys sum_pay=12, avg_pay=6.0)"

    check("default_alias_names", c_default_alias_names)

    # 12. an explicit alias names the output column.
    def c_custom_alias_used():
        q = Query([{"d": "a", "pay": 5}, {"d": "a", "pay": 7}])
        out = q.group_by("d", [("sum", "pay", "total")]).rows()
        r = out[0] if out else {}
        ok = (r.get("total") == 12 and "sum_pay" not in r)
        return ok, f"row={r!r} (expected total=12 and no sum_pay key)"

    check("custom_alias_used", c_custom_alias_used)

    # 13. group_by returns a Query: result can be order_by'd and rows()'d.
    def c_result_is_chainable():
        q = Query([
            {"d": "a", "n": 5},
            {"d": "b", "n": 1},
            {"d": "a", "n": 5},
        ])
        grouped = q.group_by("d", [("sum", "n", None)])
        # Order the GROUPED result ascending by its aggregate.
        ordered = grouped.order_by("sum_n").rows()
        keys = [r.get("d") for r in ordered]
        # b (sum 1) before a (sum 10).
        return keys == ["b", "a"], f"ordered group keys={keys!r} (expected ['b','a'])"

    check("result_is_chainable", c_result_is_chainable)

    # 14. avg is the exact mean, not rounded or integer-truncated.
    def c_avg_exact_not_rounded():
        q = Query([
            {"d": "a", "n": 1},
            {"d": "a", "n": 2},
        ])
        out = q.group_by("d", [("avg", "n", None)]).rows()
        v = out[0].get("avg_n") if out else None
        return v == 1.5, f"avg_n={v!r} (expected 1.5, not 1 or 2)"

    check("avg_exact_not_rounded", c_avg_exact_not_rounded)

    # 15. REGRESSION: where() still filters with no grouping involved.
    def c_regression_where():
        q = Query([{"n": 1}, {"n": 2}, {"n": 3}, {"n": 4}])
        out = q.where(lambda r: r["n"] % 2 == 0).rows()
        vals = [r["n"] for r in out]
        return vals == [2, 4], f"where out={vals!r} (expected [2, 4])"

    check("regression_where", c_regression_where)

    # 16. REGRESSION: order_by() still sorts (stable, and reverse) with no group.
    def c_regression_order_by():
        q = Query([
            {"n": 2, "i": 0},
            {"n": 1, "i": 1},
            {"n": 2, "i": 2},
            {"n": 1, "i": 3},
        ])
        asc = [r["i"] for r in q.order_by("n").rows()]
        rev = [r["n"] for r in q.order_by("n", reverse=True).rows()]
        # Stable ascending by n keeps original order within equal keys:
        # n=1 -> i 1,3 ; n=2 -> i 0,2  => [1, 3, 0, 2].
        ok = (asc == [1, 3, 0, 2] and rev == [2, 2, 1, 1])
        return ok, f"asc_i={asc!r} rev_n={rev!r} (expected [1,3,0,2] and [2,2,1,1])"

    check("regression_order_by", c_regression_order_by)


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
    "task": "querygroup",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
