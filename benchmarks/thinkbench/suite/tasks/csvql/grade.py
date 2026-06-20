#!/usr/bin/env python3
"""Held-out behavior-level oracle for the greenfield csvql task.

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `csvql.public`
API: ``query_csv(path, query) -> list[dict]``, and the `python -m csvql query`
CLI), NOT against the model's own tests and NOT against any particular internal
file layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

Fixed denominator: the full check list is fixed up front. If importing
`csvql.public` fails, EVERY behavior check is recorded as FAILED (never skipped),
so a broken import scores ~0 and can never masquerade as a passing run.

Tolerance: the brief under-specifies result shapes (numeric-vs-string inference,
aggregate key naming). This oracle accepts any contract-conformant representation
and checks BEHAVIOR, not incidental key names. Spots where it assumes a convention
the brief does not pin are marked `# ASSUMES`.
"""
import importlib
import json
import os
import subprocess
import sys
import tempfile

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


def fail_all(cid, desc):
    """Record a check as failed without running it (used on import failure)."""
    checks.append({"id": cid, "desc": desc, "passed": False, "detail": "import failed"})


# --- numeric / shape tolerance helpers ---------------------------------------

def _num_eq(a, b):
    """Tolerate int-vs-float for the same numeric value."""
    try:
        return abs(float(a) - float(b)) < 1e-9
    except (TypeError, ValueError):
        return False


def _as_number(v):
    """Coerce a cell to a number if it looks like one (tolerates str-typed inference)."""
    if isinstance(v, bool):
        return None
    if isinstance(v, (int, float)):
        return v
    if isinstance(v, str):
        s = v.strip()
        try:
            return int(s)
        except ValueError:
            try:
                return float(s)
            except ValueError:
                return None
    return None


def _val_eq(a, b):
    """Equality tolerant of numeric-vs-string inference: 18 == 18.0 == '18'."""
    if _num_eq(a, b):
        return True
    na, nb = _as_number(a), _as_number(b)
    if na is not None and nb is not None:
        return _num_eq(na, nb)
    return str(a) == str(b)


def _norm_key(k):
    """Normalize an aggregate key: uppercase, strip all whitespace."""
    return "".join(str(k).split()).upper()


def _agg_value(row, canonical):
    """Pull an aggregate value from a row tolerantly.

    Accepts the value under the canonical key (e.g. "COUNT(*)") or under any key
    whose normalized text (uppercased, whitespace removed) equals the canonical
    key's normalized text. Returns (found, value).
    """
    target = _norm_key(canonical)
    if canonical in row:
        return True, row[canonical]
    for k, v in row.items():
        if _norm_key(k) == target:
            return True, v
    return False, None


def _get_col(row, name):
    """Pull a projected column tolerantly (exact, else case-insensitive)."""
    if name in row:
        return True, row[name]
    for k, v in row.items():
        if str(k).lower() == name.lower():
            return True, v
    return False, None


# --- temp CSV fixtures the grader owns (always cleaned up) --------------------

_TMP_PATHS = []


def write_csv(text):
    """Write a CSV fixture to a grader-owned temp file; return its path."""
    fd, path = tempfile.mkstemp(suffix=".csv", dir=ROOT, prefix="_grade_csvql_")
    with os.fdopen(fd, "w", newline="", encoding="utf-8") as f:
        f.write(text)
    _TMP_PATHS.append(path)
    return path


def cleanup():
    for p in _TMP_PATHS:
        try:
            os.remove(p)
        except OSError:
            pass


PEOPLE = (
    "name,age,city\n"
    "Alice,30,NYC\n"
    "Bob,17,LA\n"
    "Carol,25,NYC\n"
    "Dave,40,SF\n"
    "Eve,17,LA\n"
)

EMPLOYEES = (
    "name,department,salary\n"
    "Ann,eng,100\n"
    "Ben,eng,120\n"
    "Cy,sales,90\n"
    "Di,sales,80\n"
    "Ed,sales,70\n"
)

# CSV with quoting: a comma and an escaped quote inside quoted fields.
QUOTED = (
    'id,note\n'
    '1,"hello, world"\n'
    '2,"she said ""hi"""\n'
    '3,plain\n'
)


# --- import the produced package (contract: csvql.public) --------------------

import_ok = True
import_detail = ""
qcsv = None
try:
    pub = importlib.import_module("csvql.public")
    qcsv = getattr(pub, "query_csv")
    if not callable(qcsv):
        raise TypeError("csvql.public.query_csv is not callable")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# The fixed behavior-check roster. Built once so the denominator is constant
# whether or not the import succeeded.
BEHAVIOR_CHECKS = [
    ("select_columns", "SELECT named columns returns list of row dicts with those keys"),
    ("select_star", "SELECT * projects every source column"),
    ("where_numeric", "WHERE numeric comparison (age >= 18) filters numerically, not lexically"),
    ("where_string", "WHERE string equality (city = 'NYC') filters string columns"),
    ("where_neq", "WHERE != excludes matching rows"),
    ("where_and", "WHERE ... AND ... requires both conditions"),
    ("where_or", "WHERE ... OR ... admits either condition"),
    ("order_by_desc", "ORDER BY col DESC sorts descending"),
    ("order_by_asc", "ORDER BY col (default ASC) sorts ascending"),
    ("limit", "LIMIT n truncates the result to n rows"),
    ("count_star", "COUNT(*) over whole table returns the row count under a COUNT(*) key"),
    ("sum_agg", "SUM(col) totals a numeric column"),
    ("avg_agg", "AVG(col) averages a numeric column"),
    ("group_by_count", "GROUP BY col with COUNT(*) yields one row per group with group key + count"),
    ("group_by_order", "GROUP BY with ORDER BY COUNT(*) DESC orders groups by aggregate"),
    ("numeric_inference", "numeric fields are inferred as numbers (age compares/sorts numerically)"),
    ("csv_quoting", "quoted CSV fields with embedded commas/quotes parse correctly"),
    ("malformed_raises", "a malformed query raises an exception rather than returning silently"),
    ("cli_query_json", "`python -m csvql query` emits machine-readable JSON rows"),
]


if not import_ok:
    for cid, desc in BEHAVIOR_CHECKS:
        fail_all(cid, desc)
else:
    people = write_csv(PEOPLE)
    employees = write_csv(EMPLOYEES)
    quoted = write_csv(QUOTED)

    # 1. SELECT named columns -> list[dict] with exactly those output keys present
    def c_select_columns():
        rows = qcsv(people, "SELECT name, age FROM people")
        if not isinstance(rows, list) or len(rows) != 5:
            return False, f"type/len: {type(rows).__name__} len={len(rows) if isinstance(rows, list) else 'NA'}"
        for r in rows:
            if not isinstance(r, dict):
                return False, f"row not dict: {r!r}"
            okn, _ = _get_col(r, "name")
            oka, _ = _get_col(r, "age")
            if not (okn and oka):
                return False, f"missing projected keys in {r!r}"
        names = {_get_col(r, "name")[1] for r in rows}
        return names == {"Alice", "Bob", "Carol", "Dave", "Eve"}, f"names={names!r}"

    check("select_columns", "SELECT named columns returns list of row dicts with those keys", c_select_columns)

    # 2. SELECT * -> every column present
    def c_select_star():
        rows = qcsv(people, "SELECT * FROM people")
        if not isinstance(rows, list) or len(rows) != 5:
            return False, f"len={len(rows) if isinstance(rows, list) else 'NA'}"
        r0 = rows[0]
        for col in ("name", "age", "city"):
            ok, _ = _get_col(r0, col)
            if not ok:
                return False, f"missing {col} in {r0!r}"
        return True, f"cols={list(r0.keys())!r}"

    check("select_star", "SELECT * projects every source column", c_select_star)

    # 3. WHERE numeric comparison — age >= 18 must compare as numbers (excludes the 17s)
    def c_where_numeric():
        rows = qcsv(people, "SELECT name FROM people WHERE age >= 18")
        names = {_get_col(r, "name")[1] for r in rows}
        # Adults: Alice(30), Carol(25), Dave(40). Bob(17), Eve(17) excluded.
        return names == {"Alice", "Carol", "Dave"}, f"names={names!r}"

    check("where_numeric", "WHERE numeric comparison (age >= 18) filters numerically, not lexically", c_where_numeric)

    # 4. WHERE string equality
    def c_where_string():
        rows = qcsv(people, "SELECT name FROM people WHERE city = 'NYC'")
        names = {_get_col(r, "name")[1] for r in rows}
        return names == {"Alice", "Carol"}, f"names={names!r}"

    check("where_string", "WHERE string equality (city = 'NYC') filters string columns", c_where_string)

    # 5. WHERE !=
    def c_where_neq():
        rows = qcsv(people, "SELECT name FROM people WHERE city != 'NYC'")
        names = {_get_col(r, "name")[1] for r in rows}
        return names == {"Bob", "Dave", "Eve"}, f"names={names!r}"

    check("where_neq", "WHERE != excludes matching rows", c_where_neq)

    # 6. WHERE AND
    def c_where_and():
        rows = qcsv(people, "SELECT name FROM people WHERE city = 'LA' AND age >= 18")
        names = {_get_col(r, "name")[1] for r in rows}
        # LA people are Bob(17) and Eve(17); none is >= 18.
        return names == set(), f"names={names!r}"

    check("where_and", "WHERE ... AND ... requires both conditions", c_where_and)

    # 7. WHERE OR
    def c_where_or():
        rows = qcsv(people, "SELECT name FROM people WHERE city = 'SF' OR age <= 17")
        names = {_get_col(r, "name")[1] for r in rows}
        # SF: Dave; age<=17: Bob, Eve.
        return names == {"Dave", "Bob", "Eve"}, f"names={names!r}"

    check("where_or", "WHERE ... OR ... admits either condition", c_where_or)

    # 8. ORDER BY DESC
    def c_order_desc():
        rows = qcsv(people, "SELECT name, age FROM people ORDER BY age DESC")
        ages = [_as_number(_get_col(r, "age")[1]) for r in rows]
        return ages == sorted(ages, reverse=True) and ages[0] == 40, f"ages={ages!r}"

    check("order_by_desc", "ORDER BY col DESC sorts descending", c_order_desc)

    # 9. ORDER BY default ASC
    def c_order_asc():
        rows = qcsv(people, "SELECT name, age FROM people ORDER BY age")
        ages = [_as_number(_get_col(r, "age")[1]) for r in rows]
        return ages == sorted(ages) and ages[0] == 17, f"ages={ages!r}"

    check("order_by_asc", "ORDER BY col (default ASC) sorts ascending", c_order_asc)

    # 10. LIMIT
    def c_limit():
        rows = qcsv(people, "SELECT name, age FROM people ORDER BY age DESC LIMIT 2")
        if len(rows) != 2:
            return False, f"len={len(rows)}"
        ages = [_as_number(_get_col(r, "age")[1]) for r in rows]
        return ages == [40, 30], f"ages={ages!r}"

    check("limit", "LIMIT n truncates the result to n rows", c_limit)

    # 11. COUNT(*) whole table -> single row, value under tolerant COUNT(*) key
    def c_count_star():
        rows = qcsv(people, "SELECT COUNT(*) FROM people")
        if not isinstance(rows, list) or len(rows) != 1:
            return False, f"rows={rows!r}"
        found, val = _agg_value(rows[0], "COUNT(*)")
        if not found:
            return False, f"no COUNT(*) key in {rows[0]!r}"
        return _val_eq(val, 5), f"count={val!r}"

    check("count_star", "COUNT(*) over whole table returns the row count under a COUNT(*) key", c_count_star)

    # 12. SUM(col)
    def c_sum():
        rows = qcsv(employees, "SELECT SUM(salary) FROM employees")
        found, val = _agg_value(rows[0], "SUM(salary)")
        if not found:
            return False, f"no SUM(salary) key in {rows[0]!r}"
        return _val_eq(val, 460), f"sum={val!r}"  # 100+120+90+80+70

    check("sum_agg", "SUM(col) totals a numeric column", c_sum)

    # 13. AVG(col)
    def c_avg():
        rows = qcsv(employees, "SELECT AVG(salary) FROM employees")
        found, val = _agg_value(rows[0], "AVG(salary)")
        if not found:
            return False, f"no AVG(salary) key in {rows[0]!r}"
        return _val_eq(val, 92), f"avg={val!r}"  # 460/5

    check("avg_agg", "AVG(col) averages a numeric column", c_avg)

    # 14. GROUP BY col + COUNT(*) -> one row per group, group key + count present
    def c_group_count():
        rows = qcsv(employees, "SELECT department, COUNT(*) FROM employees GROUP BY department")
        if not isinstance(rows, list) or len(rows) != 2:
            return False, f"len={len(rows) if isinstance(rows, list) else 'NA'} rows={rows!r}"
        got = {}
        for r in rows:
            okd, dept = _get_col(r, "department")
            found, cnt = _agg_value(r, "COUNT(*)")
            if not (okd and found):
                return False, f"missing department/COUNT(*) in {r!r}"
            got[dept] = _as_number(cnt)
        return got == {"eng": 2, "sales": 3}, f"groups={got!r}"

    check("group_by_count", "GROUP BY col with COUNT(*) yields one row per group with group key + count", c_group_count)

    # 15. GROUP BY + ORDER BY COUNT(*) DESC -> groups ordered by aggregate
    def c_group_order():
        rows = qcsv(
            employees,
            "SELECT department, COUNT(*) FROM employees GROUP BY department ORDER BY COUNT(*) DESC",
        )
        seq = []
        for r in rows:
            _, dept = _get_col(r, "department")
            seq.append(dept)
        # sales(3) before eng(2)
        return seq == ["sales", "eng"], f"order={seq!r}"

    check("group_by_order", "GROUP BY with ORDER BY COUNT(*) DESC orders groups by aggregate", c_group_order)

    # 16. numeric inference — sorting by age must be numeric (40 > 25 > 17, not "40" < "9")
    def c_numeric_inference():
        # Add a row whose age (9) would sort AFTER 40 under lexicographic string order.
        csv_text = "name,age\nZoe,9\nAmy,40\nBea,17\n"
        path = write_csv(csv_text)
        rows = qcsv(path, "SELECT name, age FROM people ORDER BY age DESC")
        names = [_get_col(r, "name")[1] for r in rows]
        # Numeric: 40(Amy) > 17(Bea) > 9(Zoe). Lexical "9">"40">"17" would give Zoe first.
        return names == ["Amy", "Bea", "Zoe"], f"order={names!r}"

    check("numeric_inference", "numeric fields are inferred as numbers (age compares/sorts numerically)", c_numeric_inference)

    # 17. CSV quoting — embedded comma and escaped quote must parse as single fields
    def c_quoting():
        rows = qcsv(quoted, "SELECT id, note FROM q ORDER BY id")
        notes = {}
        for r in rows:
            _, rid = _get_col(r, "id")
            _, note = _get_col(r, "note")
            notes[_as_number(rid)] = note
        return (
            notes.get(1) == "hello, world"
            and notes.get(2) == 'she said "hi"'
            and notes.get(3) == "plain"
        ), f"notes={notes!r}"

    check("csv_quoting", "quoted CSV fields with embedded commas/quotes parse correctly", c_quoting)

    # 18. malformed query raises (does not silently return)
    def c_malformed():
        raised = False
        try:
            qcsv(people, "SELEKT nonsense FROM FROM WHERE")
        except Exception:  # noqa: BLE001 - any exception satisfies the contract
            raised = True
        return raised, "raised" if raised else "no exception on malformed input"

    check("malformed_raises", "a malformed query raises an exception rather than returning silently", c_malformed)

    # 19. CLI — `python -m csvql query <csv> "<SQL>"` emits JSON rows
    def c_cli():
        proc = subprocess.run(
            [sys.executable, "-m", "csvql", "query", people, "SELECT name FROM people WHERE age >= 18"],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        out = proc.stdout.strip()
        if not out:
            return False, f"empty stdout (rc={proc.returncode}, stderr={proc.stderr[:200]!r})"
        # Accept a JSON array OR JSON-Lines (one object per line). # ASSUMES one of these two.
        parsed = None
        try:
            parsed = json.loads(out)
            rows = parsed if isinstance(parsed, list) else [parsed]
        except json.JSONDecodeError:
            rows = []
            for line in out.splitlines():
                line = line.strip()
                if line:
                    rows.append(json.loads(line))
        names = set()
        for r in rows:
            if isinstance(r, dict):
                ok, v = _get_col(r, "name")
                if ok:
                    names.add(v)
        return names == {"Alice", "Carol", "Dave"}, f"cli names={names!r} rc={proc.returncode}"

    check("cli_query_json", "`python -m csvql query` emits machine-readable JSON rows", c_cli)


cleanup()

passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "csvql",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": round(passed / total, 4) if total else 0.0,
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
