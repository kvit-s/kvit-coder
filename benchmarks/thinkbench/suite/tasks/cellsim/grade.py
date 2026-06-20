#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield task `cellsim`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it
and it never reads the agent's own tests. Grades the produced package against the
BRIEF'S CONTRACT (the `cellsim.public` API and the `python -m cellsim` CLI), NOT
against any particular internal file layout.

Output: a single JSON scorecard on stdout. Each check is independent (its own
try/except), so the score is continuous (passed / total), never binary. Exit code
is 0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure.

FIXED DENOMINATOR: the full check list is decided up front. If the package fails
to import, EVERY behavior check is recorded as FAILED (never skipped), so a broken
import scores ~0 and never a false 1.0. The CLI checks live behind the same import
guard for the same reason.

Tolerance: the brief under-specifies some return SHAPES. This oracle accepts any
contract-conformant representation (see brief.txt "## Contract") and checks
BEHAVIOR, not incidental key names. Residual assumptions are marked `# ASSUMES`.
"""
import importlib
import json
import math
import os
import subprocess
import sys
import tempfile

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

checks = []


def record(cid, desc, passed, detail=""):
    checks.append({"id": cid, "desc": desc, "passed": bool(passed), "detail": str(detail or "")})


# --- tolerant value helpers --------------------------------------------------

MISSING = object()


def is_number(v):
    return isinstance(v, (int, float)) and not isinstance(v, bool)


def num_eq(v, expected, tol=1e-9):
    """Numeric equality with tolerance. Bools are NOT numbers here."""
    if not is_number(v):
        return False
    return math.isclose(float(v), float(expected), rel_tol=0, abs_tol=tol)


def get_cells(result):
    """Pull the cell map out of an evaluate_sheet result, tolerantly.

    Accepts {"cells": {...}} (pinned) or a bare {name: value} mapping.
    """
    if not isinstance(result, dict):
        return MISSING
    if "cells" in result and isinstance(result["cells"], dict):
        return result["cells"]
    # bare mapping fallback: a dict whose keys look like cell names
    if result and all(isinstance(k, str) for k in result) and "errors" not in result:
        return result
    return MISSING


def cell_of(result, name):
    cells = get_cells(result)
    if cells is MISSING or name not in cells:
        return MISSING
    return cells[name]


def has_circular(blob):
    """Tolerant: does a structure report a circular reference anywhere?"""
    s = json.dumps(blob, default=str).lower()
    return ("circular" in s) or ("cycle" in s)


def errors_of(result):
    if isinstance(result, dict) and isinstance(result.get("errors"), dict):
        return result["errors"]
    return {}


def sheet(cells):
    return {"cells": cells}


# --- import the produced package (contract: cellsim.public) ------------------

import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("cellsim.public")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# Every behavior check is declared here as (id, desc, fn). The list is FIXED.
# fn returns (passed: bool, detail: str). On import failure we DO NOT call fn;
# we record each as FAILED so the denominator is constant and a broken import
# scores ~0 rather than skipping its way to a false high score.

def chk_arith_precedence():
    # =2+3*4 must be 14 (precedence), not 20. =(2+3)*4 must be 20.
    s = sheet({"A1": "=2+3*4", "A2": "=(2+3)*4"})
    r = pub.evaluate_sheet(s)
    a1, a2 = cell_of(r, "A1"), cell_of(r, "A2")
    ok = num_eq(a1, 14) and num_eq(a2, 20)
    return ok, f"A1(=2+3*4)={a1!r} A2(=(2+3)*4)={a2!r}"


def chk_cell_refs_and_decimal():
    # references + decimals: A3 = A1 + A2; division yields a float
    s = sheet({"A1": 10, "A2": 20, "A3": "=A1+A2", "A4": "=10/4"})
    r = pub.evaluate_sheet(s)
    a3, a4 = cell_of(r, "A3"), cell_of(r, "A4")
    ok = num_eq(a3, 30) and num_eq(a4, 2.5)
    return ok, f"A3(=A1+A2)={a3!r} A4(=10/4)={a4!r}"


def chk_subtraction_division():
    s = sheet({"A1": 100, "A2": 30, "A3": "=A1-A2", "A4": "=A1/A2"})
    r = pub.evaluate_sheet(s)
    a3, a4 = cell_of(r, "A3"), cell_of(r, "A4")
    ok = num_eq(a3, 70) and num_eq(a4, 100 / 30)
    return ok, f"A3(=A1-A2)={a3!r} A4(=A1/A2)={a4!r}"


def chk_sum_range():
    s = sheet({"A1": 10, "A2": 20, "A3": 30, "B1": "=SUM(A1:A3)"})
    r = pub.evaluate_sheet(s)
    b1 = cell_of(r, "B1")
    return num_eq(b1, 60), f"B1(=SUM(A1:A3))={b1!r}"


def chk_min_max_avg():
    s = sheet({
        "A1": 4, "A2": 10, "A3": 1,
        "B1": "=MIN(A1:A3)", "B2": "=MAX(A1:A3)", "B3": "=AVG(A1:A3)",
    })
    r = pub.evaluate_sheet(s)
    mn, mx, av = cell_of(r, "B1"), cell_of(r, "B2"), cell_of(r, "B3")
    ok = num_eq(mn, 1) and num_eq(mx, 10) and num_eq(av, 5)
    return ok, f"MIN={mn!r} MAX={mx!r} AVG={av!r}"


def chk_rectangular_range():
    # A1:C2 spans columns A,B,C across rows 1,2 -> 6 cells summing to 21
    s = sheet({
        "A1": 1, "B1": 2, "C1": 3,
        "A2": 4, "B2": 5, "C2": 6,
        "D1": "=SUM(A1:C2)",
    })
    r = pub.evaluate_sheet(s)
    d1 = cell_of(r, "D1")
    return num_eq(d1, 21), f"D1(=SUM(A1:C2))={d1!r}"


def chk_strings():
    # a literal string cell round-trips unchanged
    s = sheet({"A1": "hello", "A2": "=A1"})
    r = pub.evaluate_sheet(s)
    a1, a2 = cell_of(r, "A1"), cell_of(r, "A2")
    ok = (a1 == "hello") and (a2 == "hello")
    return ok, f"A1={a1!r} A2(=A1)={a2!r}"


def chk_if_and_comparison():
    # IF with > comparison; mirrors the brief's own B1/B2 example
    s = sheet({
        "A1": 10, "A2": 20, "A3": "=A1+A2",
        "B1": "=SUM(A1:A3)",
        "B2": '=IF(B1>40,"high","low")',
        "B3": '=IF(B1<40,"high","low")',
    })
    r = pub.evaluate_sheet(s)
    b1, b2, b3 = cell_of(r, "B1"), cell_of(r, "B2"), cell_of(r, "B3")
    ok = num_eq(b1, 60) and (b2 == "high") and (b3 == "low")
    return ok, f"B1={b1!r} B2(>40)={b2!r} B3(<40)={b3!r}"


def chk_comparison_operators():
    # each comparison operator drives IF; check =, !=, <=, >=
    s = sheet({
        "A1": 5, "A2": 5, "A3": 7,
        "E": '=IF(A1=A2,"y","n")',
        "NE": '=IF(A1!=A3,"y","n")',
        "LE": '=IF(A1<=A2,"y","n")',
        "GE": '=IF(A3>=A1,"y","n")',
    })
    r = pub.evaluate_sheet(s)
    vals = {k: cell_of(r, k) for k in ("E", "NE", "LE", "GE")}
    ok = all(v == "y" for v in vals.values())
    return ok, f"{vals!r}"


def chk_nested_formulas():
    # deep chain A1 -> A2 -> A3 -> A4 must resolve transitively
    s = sheet({"A1": 2, "A2": "=A1*3", "A3": "=A2+4", "A4": "=A3*2"})
    r = pub.evaluate_sheet(s)
    a4 = cell_of(r, "A4")  # ((2*3)+4)*2 = 20
    return num_eq(a4, 20), f"A4={a4!r} (expected 20)"


def chk_missing_in_formula_is_zero():
    # a missing cell referenced INSIDE a numeric formula contributes 0
    s = sheet({"A1": 5, "A2": "=A1+Z9"})
    r = pub.evaluate_sheet(s)
    a2 = cell_of(r, "A2")
    return num_eq(a2, 5), f"A2(=A1+Z9, Z9 missing)={a2!r} (expected 5)"


def chk_direct_missing_reported():
    # a DIRECT request for a missing cell must be reported, NOT coerced to 0.
    # Contract: KeyError raised, OR None returned. 0 / any number is a FAIL.
    s = sheet({"A1": 5})
    try:
        v = pub.get_cell_value(s, "Z9")
    except KeyError:
        return True, "raised KeyError (reported missing)"
    except Exception as e:  # noqa: BLE001
        return False, f"raised {type(e).__name__} (contract: KeyError or None)"
    if v is None:
        return True, "returned None (reported missing)"
    return False, f"returned {v!r} (must report missing, not coerce)"


def chk_get_cell_value_present():
    s = sheet({"A1": 10, "A2": 20, "A3": "=A1+A2"})
    v = pub.get_cell_value(s, "A3")
    return num_eq(v, 30), f"get_cell_value(A3)={v!r}"


def chk_cycle_detected_evaluate():
    # A1 -> A2 -> A1 is circular; evaluate_sheet must report it (no crash)
    s = sheet({"A1": "=A2+1", "A2": "=A1+1"})
    r = pub.evaluate_sheet(s)  # must not raise
    errs = errors_of(r)
    reported = has_circular(errs) or has_circular(r)
    return (isinstance(r, dict) and reported), f"result={r!r}"


def chk_cycle_detected_get_cell():
    # get_cell_value on a cyclic cell: raise OR return a circular descriptor
    s = sheet({"A1": "=A2", "A2": "=A1"})
    try:
        v = pub.get_cell_value(s, "A1")
    except Exception as e:  # noqa: BLE001 - any raise satisfies "reported"
        return True, f"raised {type(e).__name__}"
    if has_circular(v):
        return True, f"returned circular descriptor {v!r}"
    return False, f"returned {v!r} (no cycle reported)"


def chk_explain_references():
    # explain_cell exposes the direct references as a set-comparable list
    s = sheet({"A1": 10, "A2": 20, "A3": "=A1+A2"})
    info = pub.explain_cell(s, "A3")
    if not isinstance(info, dict):
        return False, f"type={type(info).__name__}"
    refs = info.get("references")
    if not isinstance(refs, list):
        return False, f"references={refs!r} (expected list)"
    ok = set(refs) == {"A1", "A2"}
    return ok, f"references={refs!r} (expected {{A1,A2}})"


def chk_explain_range_expands():
    # a range reference expands to its member cells in explain.references
    # ASSUMES range members are surfaced as individual cell names (pinned in Contract).
    s = sheet({"A1": 1, "A2": 2, "A3": 3, "B1": "=SUM(A1:A3)"})
    info = pub.explain_cell(s, "B1")
    refs = info.get("references") if isinstance(info, dict) else None
    if not isinstance(refs, list):
        return False, f"references={refs!r}"
    ok = {"A1", "A2", "A3"} <= set(refs)
    return ok, f"references={refs!r} (expected superset of A1,A2,A3)"


def chk_explain_value():
    s = sheet({"A1": 10, "A2": 20, "A3": "=A1+A2"})
    info = pub.explain_cell(s, "A3")
    val = info.get("value") if isinstance(info, dict) else MISSING
    cellname = info.get("cell") if isinstance(info, dict) else None
    ok = num_eq(val, 30) and cellname == "A3"
    return ok, f"cell={cellname!r} value={val!r}"


def chk_explain_cycle_no_raise():
    # explain_cell on a cyclic cell must NOT raise; reports circular or value None
    s = sheet({"A1": "=A2", "A2": "=A1"})
    info = pub.explain_cell(s, "A1")  # must not raise
    if not isinstance(info, dict):
        return False, f"type={type(info).__name__}"
    ok = has_circular(info) or (info.get("value") is None)
    return ok, f"info={info!r}"


def chk_determinism():
    s = sheet({"A1": 3, "A2": "=A1*7", "B1": "=SUM(A1:A2)"})
    r1 = json.dumps(pub.evaluate_sheet(s), sort_keys=True, default=str)
    r2 = json.dumps(pub.evaluate_sheet(s), sort_keys=True, default=str)
    return (r1 == r2), ("stable" if r1 == r2 else "differs across runs")


def chk_no_python_eval_injection():
    # If the engine used Python eval(), a formula like this would blow up or do
    # something exotic. A safe parser treats unknown bare names as cell refs
    # (missing -> 0). We require: no crash, and a NUMERIC result. This is a
    # behavioral proxy for "not eval" pinned by "must not use Python eval".
    s = sheet({"A1": "=__import__"})  # a real eval() would yield a builtin/func
    r = pub.evaluate_sheet(s)
    if not isinstance(r, dict):
        return False, f"type={type(r).__name__}"
    v = cell_of(r, "A1")
    # treated as a missing cell ref -> 0, OR reported as an error: both are fine.
    # what must NOT happen: a Python function/object leaking through.
    in_errors = "A1" in errors_of(r)
    ok = in_errors or (v is MISSING) or is_number(v) or isinstance(v, str)
    leaked = (v is not MISSING) and not is_number(v) and not isinstance(v, (str, bool))
    return (ok and not leaked), f"A1={v!r} in_errors={in_errors}"


# The FIXED check list (id, human description, fn). Order is stable.
BEHAVIOR_CHECKS = [
    ("arith_precedence", "arithmetic precedence: =2+3*4 -> 14, =(2+3)*4 -> 20", chk_arith_precedence),
    ("cell_refs_decimal", "cell references resolve; decimal division yields a float", chk_cell_refs_and_decimal),
    ("sub_div", "subtraction and division over cell references", chk_subtraction_division),
    ("sum_range", "SUM over a column range A1:A3", chk_sum_range),
    ("min_max_avg", "MIN/MAX/AVG over a range", chk_min_max_avg),
    ("rect_range", "ranges cover rectangular regions (A1:C2)", chk_rectangular_range),
    ("strings", "string literals and string-valued references round-trip", chk_strings),
    ("if_comparison", "IF with > comparison selects the right branch", chk_if_and_comparison),
    ("comparison_ops", "comparison operators =, !=, <=, >= each work", chk_comparison_operators),
    ("nested_formulas", "nested/transitive formula chains resolve", chk_nested_formulas),
    ("missing_in_formula_zero", "a missing cell inside a numeric formula is 0", chk_missing_in_formula_is_zero),
    ("direct_missing_reported", "a directly requested missing cell is reported, not 0", chk_direct_missing_reported),
    ("get_cell_present", "get_cell_value returns an existing cell's value", chk_get_cell_value_present),
    ("cycle_evaluate", "evaluate_sheet detects & reports a circular reference (no crash)", chk_cycle_detected_evaluate),
    ("cycle_get_cell", "get_cell_value reports a circular reference (raise or descriptor)", chk_cycle_detected_get_cell),
    ("explain_refs", "explain_cell lists a cell's direct references", chk_explain_references),
    ("explain_range_expands", "explain_cell expands a range into member cells", chk_explain_range_expands),
    ("explain_value", "explain_cell returns the cell name and computed value", chk_explain_value),
    ("explain_cycle_no_raise", "explain_cell on a cyclic cell does not raise", chk_explain_cycle_no_raise),
    ("determinism", "evaluate_sheet is deterministic across repeated runs", chk_determinism),
    ("no_python_eval", "formulas are parsed, not run through Python eval()", chk_no_python_eval_injection),
]


def run_behavior_check(cid, desc, fn):
    if not import_ok:
        record(cid, desc, False, f"import failed: {import_detail}")
        return
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, desc, ok, detail)


for _cid, _desc, _fn in BEHAVIOR_CHECKS:
    run_behavior_check(_cid, _desc, _fn)


# --- CLI checks: all output must be JSON. Behind the import guard so an --------
# import failure cannot let CLI checks float the score up.

def write_sheet(cells):
    fd, path = tempfile.mkstemp(suffix=".json", dir=ROOT)
    with os.fdopen(fd, "w") as f:
        json.dump(sheet(cells), f)
    return path


def run_cli(args):
    proc = subprocess.run(
        [sys.executable, "-m", "cellsim", *args],
        capture_output=True, text=True, timeout=60, cwd=ROOT,
    )
    return proc


def chk_cli_eval():
    path = write_sheet({"A1": 10, "A2": 20, "A3": "=A1+A2"})
    try:
        proc = run_cli(["eval", path])
        json.loads(proc.stdout)  # raises if not JSON
        return True, f"rc={proc.returncode}"
    finally:
        try:
            os.remove(path)
        except OSError:
            pass


def chk_cli_cell():
    path = write_sheet({"A1": 10, "A2": 20, "A3": "=A1+A2"})
    try:
        proc = run_cli(["cell", path, "A3"])
        blob = json.loads(proc.stdout)  # must be JSON
        # ASSUMES the value 30 appears somewhere in the JSON payload.
        text = json.dumps(blob, default=str)
        return ("30" in text), f"rc={proc.returncode} out={text}"
    finally:
        try:
            os.remove(path)
        except OSError:
            pass


def chk_cli_explain():
    path = write_sheet({"A1": 10, "A2": 20, "A3": "=A1+A2"})
    try:
        proc = run_cli(["explain", path, "A3"])
        json.loads(proc.stdout)  # must be JSON
        return True, f"rc={proc.returncode}"
    finally:
        try:
            os.remove(path)
        except OSError:
            pass


CLI_CHECKS = [
    ("cli_eval_json", "`python -m cellsim eval` emits JSON", chk_cli_eval),
    ("cli_cell_value", "`python -m cellsim cell` emits JSON carrying the value", chk_cli_cell),
    ("cli_explain_json", "`python -m cellsim explain` emits JSON", chk_cli_explain),
]

for _cid, _desc, _fn in CLI_CHECKS:
    run_behavior_check(_cid, _desc, _fn)


passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "cellsim",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": round(passed / total, 4) if total else 0.0,
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
