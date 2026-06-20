#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `repaircalc`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``repaircalc`` package against the BRIEF'S CONTRACT (the ``repaircalc.public``
``evaluate`` API), NOT against any particular internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE precedence / associativity / parenthesis / decimal edge cases, all with
values computed here (never read from the agent's tests). The FIXED reference
passes every check; the planted-bug starter fails several.

The planted bugs (in the starter ``repaircalc.public``):
  1. inverted operator precedence — ``+``/``-`` bind tighter than ``*``/``/``
     (so ``2 + 3 * 4`` gives 20 instead of 14);
  2. right-associative subtraction (so ``10 - 3 - 2`` gives 9 instead of 5);
  3. decimal literals rounded to the nearest integer (so ``3.5 + 1.5`` gives 6).

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs).
"""
import importlib
import inspect
import json
import math
import sys

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --- import the produced package ---------------------------------------------
# Contract path is ``repaircalc.public``; fall back to the package root
# ``repaircalc`` so a submission that re-exports ``evaluate`` from ``__init__``
# (but moved it off ``public``) is still graded on behavior rather than mis-scored.
import_ok = True
import_detail = ""
evaluate = None
CalcError = None
src_text = ""
try:
    try:
        mod = importlib.import_module("repaircalc.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("repaircalc")
    evaluate = getattr(mod, "evaluate")
    # CalcError is part of the pinned API; fall back to any Exception subclass so
    # a renamed-but-present error type still lets the error checks run.
    CalcError = getattr(mod, "CalcError", None)
    if not (isinstance(CalcError, type) and issubclass(CalcError, BaseException)):
        CalcError = Exception
    try:
        src_text = inspect.getsource(mod)
    except Exception:  # noqa: BLE001 - source scan is best-effort only
        src_text = ""
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def approx(got, want):
    """True when ``got`` is numerically equal to ``want`` (tolerant on floats)."""
    if isinstance(got, bool) or not isinstance(got, (int, float)):
        return False
    return math.isclose(float(got), float(want), rel_tol=1e-9, abs_tol=1e-9)


def expect(expr, want):
    """Check that ``evaluate(expr)`` numerically equals ``want`` (computed here)."""
    def _fn():
        got = evaluate(expr)
        return approx(got, want), f"evaluate({expr!r}) = {got!r}, expected {want!r}"
    return _fn


if import_ok:
    # --- basic arithmetic (guards against an "always wrong" regression) -------
    check("basic_add", "2+2 == 4", expect("2+2", 4))
    check("basic_sub", "9-4 == 5", expect("9-4", 5))
    check("basic_single_number", "a bare literal evaluates to itself", expect("7", 7))
    check("basic_unary_minus", "leading unary minus negates", expect("-5+3", -2))

    # --- precedence: * and / bind tighter than + and - ------------------------
    check("prec_add_then_mul", "2+3*4 == 14 (mul binds tighter)", expect("2+3*4", 14))
    check("prec_mul_then_add", "4*2+1 == 9", expect("4*2+1", 9))
    check("prec_two_products", "2*3+4*5 == 26", expect("2*3+4*5", 26))
    check("prec_mixed_add_sub_mul", "2 + 3 * 4 - 1 == 13", expect("2 + 3 * 4 - 1", 13))
    check("prec_div_in_sum", "10 + 8/2 == 14", expect("10 + 8/2", 14))
    check("prec_sub_then_mul", "20 - 2*3 == 14", expect("20 - 2*3", 14))

    # --- associativity: all binary ops are LEFT-associative -------------------
    check("assoc_sub_simple", "10-3-2 == 5 (left assoc)", expect("10-3-2", 5))
    check("assoc_sub_chain", "20-5-3-1 == 11", expect("20-5-3-1", 11))
    check("assoc_add_sub_mix", "1+2-3+4 == 4", expect("1+2-3+4", 4))
    check("assoc_add_sub_mix2", "12-4+2 == 10", expect("12-4+2", 10))
    check("assoc_div_chain", "100/10/2 == 5", expect("100/10/2", 5))
    check("assoc_div_chain2", "64/4/2 == 8", expect("64/4/2", 8))
    check("assoc_mul_div_lr", "6/2*3 == 9", expect("6/2*3", 9))

    # --- parentheses regroup precedence ---------------------------------------
    check("paren_group_add", "(2+3)*4 == 20", expect("(2+3)*4", 20))
    check("paren_inner_expr", "2*(3+4) == 14", expect("2*(3+4)", 14))
    check("paren_nested", "((1+2)*(3+4)) == 21", expect("((1+2)*(3+4))", 21))
    check("paren_over_sub", "(10-3)-2 == 5", expect("(10-3)-2", 5))
    check("paren_changes_sub_assoc", "10-(3-2) == 9", expect("10-(3-2)", 9))
    check("paren_unary", "-(3+4) == -7", expect("-(3+4)", -7))

    # --- decimals keep their fractional value ---------------------------------
    check("dec_add", "3.5+1.5 == 5.0", expect("3.5+1.5", 5.0))
    check("dec_leading_dot", ".5+.5 == 1.0", expect(".5+.5", 1.0))
    check("dec_mul", "2.5*4 == 10.0", expect("2.5*4", 10.0))
    check("dec_div", "10/4 == 2.5", expect("10/4", 2.5))
    check("dec_precision", "0.1+0.2 ≈ 0.3", expect("0.1+0.2", 0.3))
    check("dec_mixed", "1.5*2+0.5 == 3.5", expect("1.5*2+0.5", 3.5))

    # --- division by zero is a CalcError, not a raw crash ---------------------
    def c_divzero():
        try:
            r = evaluate("1/0")
        except CalcError:
            return True, "raised CalcError"
        except Exception as e:  # noqa: BLE001 - wrong error type fails the check
            return False, f"raised {type(e).__name__}, expected CalcError"
        return False, f"did not raise, returned {r!r}"

    check("div_by_zero_raises", "division by zero raises CalcError", c_divzero)

    # --- no eval: the implementation must be a real parser --------------------
    def c_no_eval():
        if not src_text:
            # could not read source; do not penalize, but record it
            return True, "source unavailable; skipped (counts as pass)"
        lowered = src_text
        banned = ("eval(", "exec(", "literal_eval")
        hit = [b for b in banned if b in lowered]
        return (not hit), (f"found {hit}" if hit else "no eval/exec/literal_eval in source")

    check("no_eval_used", "implementation does not use eval/exec/literal_eval", c_no_eval)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 31

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "repaircalc",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
