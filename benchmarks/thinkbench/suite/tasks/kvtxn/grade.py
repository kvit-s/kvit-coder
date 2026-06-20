#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `kvtxn`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `kvtxn`
package against the BRIEF'S CONTRACT (nested transactions with savepoint
semantics, plus the unchanged core API), NOT against any particular internal
file layout.

The defining behaviors under test are the ones a SUPERFICIAL implementation gets
wrong:

  * a NESTED rollback restores only the inner savepoint, leaving the enclosing
    scope's changes intact (a single-snapshot store rewinds too far or loses the
    outer scope);
  * a nested COMMIT folds into the parent but does NOT make changes durable, so
    a subsequent outer rollback still undoes everything (a store that treats
    commit as "drop the snapshot / make durable" fails this);
  * undo must cover deletes, not just sets.

The shipped flat attempt keeps a single whole-store snapshot and so fails the
nesting checks while passing the single-level and regression checks — that's
what makes the task discriminate (naive lands well under 1.0, a careful nested
implementation lands at 1.0).

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
    ("single_commit_durable", "begin; set; commit makes the change durable"),
    ("single_rollback_restores", "begin; set; rollback restores the prior value"),
    ("rollback_restores_absent", "rollback of a set on a NEW key removes it again"),
    ("nested_rollback_inner_only", "nested rollback restores only the inner savepoint"),
    ("nested_commit_then_outer_rollback", "inner commit folds to parent; outer rollback still undoes all"),
    ("nested_commit_visible_in_parent", "after inner commit the value is visible within the outer txn"),
    ("nested_rollback_keeps_outer", "nested rollback does not discard the outer scope's earlier change"),
    ("rollback_after_delete_restores", "rollback restores a value deleted inside the txn"),
    ("commit_after_delete_durable", "a delete committed at top level stays deleted"),
    ("three_level_partial", "3 levels deep: rollback one, commit one, rollback outer rewinds all"),
    ("error_commit_no_txn", "commit with no open transaction raises TransactionError"),
    ("error_rollback_no_txn", "rollback with no open transaction raises TransactionError"),
    ("regression_get_set_delete", "get/set/delete still work with no active transaction"),
    ("regression_delete_return", "delete reports presence (True/False) with no txn"),
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


# --- import the produced package (contract: kvtxn.public, fallback kvtxn) ------
import_ok = True
import_detail = ""
Store = None
TxnError = None
try:
    try:
        mod = importlib.import_module("kvtxn.public")
    except Exception:
        mod = importlib.import_module("kvtxn")
    Store = getattr(mod, "Store")
    # TransactionError is part of the contract; fall back to RuntimeError so the
    # error checks still grade something sensible if the name is missing.
    try:
        pkg = importlib.import_module("kvtxn")
        TxnError = getattr(pkg, "TransactionError", None)
    except Exception:
        TxnError = None
    if TxnError is None:
        TxnError = getattr(mod, "TransactionError", RuntimeError)
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. single begin/commit makes the change durable.
    def c_single_commit_durable():
        s = Store()
        s.set("a", 1)
        s.begin()
        s.set("a", 2)
        s.commit()
        v = s.get("a")
        return v == 2, f"get('a') after commit -> {v!r} (expected 2)"

    check("single_commit_durable", c_single_commit_durable)

    # 2. single begin/rollback restores the prior value.
    def c_single_rollback_restores():
        s = Store()
        s.set("a", 1)
        s.begin()
        s.set("a", 2)
        s.rollback()
        v = s.get("a")
        return v == 1, f"get('a') after rollback -> {v!r} (expected 1)"

    check("single_rollback_restores", c_single_rollback_restores)

    # 3. rollback of a set on a previously-absent key removes it again.
    def c_rollback_restores_absent():
        s = Store()
        s.begin()
        s.set("new", 5)
        s.rollback()
        v = s.get("new", "GONE")
        return v == "GONE", f"get('new') after rollback -> {v!r} (expected absent)"

    check("rollback_restores_absent", c_rollback_restores_absent)

    # 4. THE nesting check: a nested rollback restores ONLY the inner savepoint.
    def c_nested_rollback_inner_only():
        s = Store()
        s.set("a", 1)
        s.begin()           # outer
        s.set("a", 2)
        s.begin()           # inner
        s.set("a", 3)
        s.rollback()        # undo inner only -> back to outer's value (2), NOT 1
        v = s.get("a")
        return v == 2, f"get('a') after inner rollback -> {v!r} (expected 2, not 1)"

    check("nested_rollback_inner_only", c_nested_rollback_inner_only)

    # 5. inner commit folds into parent; a later OUTER rollback still undoes all.
    def c_nested_commit_then_outer_rollback():
        s = Store()
        s.set("a", 1)
        s.begin()           # outer
        s.set("a", 2)
        s.begin()           # inner
        s.set("a", 3)
        s.commit()          # fold inner into outer (NOT durable)
        s.rollback()        # undo outer -> all the way back to 1
        v = s.get("a")
        return v == 1, f"get('a') after outer rollback of folded commit -> {v!r} (expected 1)"

    check("nested_commit_then_outer_rollback", c_nested_commit_then_outer_rollback)

    # 6. after an inner commit, the folded value is visible inside the outer txn.
    def c_nested_commit_visible_in_parent():
        s = Store()
        s.set("a", 1)
        s.begin()           # outer
        s.begin()           # inner
        s.set("a", 42)
        s.commit()          # fold into outer
        v = s.get("a")      # still inside outer txn
        return v == 42, f"get('a') within outer after inner commit -> {v!r} (expected 42)"

    check("nested_commit_visible_in_parent", c_nested_commit_visible_in_parent)

    # 7. a nested rollback must not discard the outer scope's EARLIER change.
    def c_nested_rollback_keeps_outer():
        s = Store()
        s.begin()           # outer
        s.set("x", "outer")
        s.begin()           # inner
        s.set("y", "inner")
        s.rollback()        # drop only inner: 'y' gone, 'x' kept
        x = s.get("x", "MISSING")
        y = s.get("y", "MISSING")
        return (x == "outer" and y == "MISSING"), f"x={x!r} y={y!r} (expected 'outer'/MISSING)"

    check("nested_rollback_keeps_outer", c_nested_rollback_keeps_outer)

    # 8. rollback restores a value deleted inside the transaction.
    def c_rollback_after_delete_restores():
        s = Store()
        s.set("x", 10)
        s.begin()
        existed = s.delete("x")
        mid = s.get("x", "GONE")
        s.rollback()
        v = s.get("x", "GONE")
        return (existed is True and mid == "GONE" and v == 10), \
            f"deleted={existed!r} mid={mid!r} after_rollback={v!r} (expected True/GONE/10)"

    check("rollback_after_delete_restores", c_rollback_after_delete_restores)

    # 9. a delete committed at top level stays deleted.
    def c_commit_after_delete_durable():
        s = Store()
        s.set("x", 10)
        s.begin()
        s.delete("x")
        s.commit()
        v = s.get("x", "GONE")
        return v == "GONE", f"get('x') after committed delete -> {v!r} (expected absent)"

    check("commit_after_delete_durable", c_commit_after_delete_durable)

    # 10. three levels: rollback the deepest, commit the middle, rollback outer.
    def c_three_level_partial():
        s = Store()
        s.set("k", 0)
        s.begin()                 # L1
        s.set("k", 1)
        s.begin()                 # L2
        s.set("k", 2)
        s.begin()                 # L3
        s.set("k", 3)
        s.rollback()              # drop L3 -> k == 2 (L2 value)
        after_l3 = s.get("k")
        s.commit()                # fold L2 into L1 -> k == 2 within L1
        after_l2_commit = s.get("k")
        s.rollback()              # drop L1 -> back to 0
        after_l1 = s.get("k")
        ok = (after_l3 == 2 and after_l2_commit == 2 and after_l1 == 0)
        return ok, f"afterL3={after_l3!r} afterL2commit={after_l2_commit!r} afterL1={after_l1!r} (expected 2/2/0)"

    check("three_level_partial", c_three_level_partial)

    # 11. commit with no open txn raises TransactionError.
    def c_error_commit_no_txn():
        s = Store()
        try:
            s.commit()
            return False, "commit() with no txn did not raise"
        except Exception as e:  # noqa: BLE001
            ok = isinstance(e, TxnError) and isinstance(e, RuntimeError)
            return ok, f"raised {type(e).__name__} (want TransactionError(RuntimeError))"

    check("error_commit_no_txn", c_error_commit_no_txn)

    # 12. rollback with no open txn raises TransactionError.
    def c_error_rollback_no_txn():
        s = Store()
        try:
            s.rollback()
            return False, "rollback() with no txn did not raise"
        except Exception as e:  # noqa: BLE001
            ok = isinstance(e, TxnError) and isinstance(e, RuntimeError)
            return ok, f"raised {type(e).__name__} (want TransactionError(RuntimeError))"

    check("error_rollback_no_txn", c_error_rollback_no_txn)

    # 13. REGRESSION: get/set/delete with no active txn behave normally.
    def c_regression_get_set_delete():
        s = Store()
        miss = s.get("nope", "DEF")
        s.set("a", 1)
        got = s.get("a")
        s.set("a", 2)
        over = s.get("a")
        s.delete("a")
        gone = s.get("a", "GONE")
        ok = (miss == "DEF" and got == 1 and over == 2 and gone == "GONE")
        return ok, f"miss={miss!r} get={got!r} overwrite={over!r} deleted={gone!r}"

    check("regression_get_set_delete", c_regression_get_set_delete)

    # 14. REGRESSION: delete reports presence with no txn.
    def c_regression_delete_return():
        s = Store()
        s.set("a", 1)
        first = s.delete("a")
        second = s.delete("a")
        return (first is True and second is False), \
            f"first={first!r} second={second!r} (expected True/False)"

    check("regression_delete_return", c_regression_delete_return)


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
    "task": "kvtxn",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
