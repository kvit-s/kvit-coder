#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `calceval`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``calceval`` package against the BRIEF'S CONTRACT (the ``calceval.public``
``evaluate`` API), NOT against any particular internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE associativity / precedence / unary / parenthesis edge cases, all with
expected values computed HERE (never read from the agent's tests, never via
``eval``). The FIXED reference passes every check; the planted-bug starter fails
a discriminating subset, so a partial fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``calceval.public``:
  1. ``-`` and ``/`` parse RIGHT-associative instead of LEFT — ``10-2-3`` comes
     out ``11`` (``10-(2-3)``) and ``100/10/2`` comes out ``20`` (``100/(10/2)``)
     instead of ``5``;
  2. ``^`` parses LEFT-associative instead of RIGHT — ``2^3^2`` comes out ``64``
     (``(2^3)^2``) instead of ``512`` (``2^(3^2)``);
  3. unary minus binds TIGHTER than ``^`` instead of looser — ``-2^2`` comes out
     ``4`` (``(-2)^2``) instead of ``-4`` (``-(2^2)``).

They interact through the shared precedence cascade: the unary/exponent fixes
(2 and 3) live in the same code path, and ``-2^2^2`` only yields ``-16`` once all
of unary-binding, ``^`` right-associativity, and (for nested cases) the
subtraction associativity are correct.

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


# --- import the produced package ---------------------------------------------
# Contract path is ``calceval.public``; fall back to the package root
# ``calceval`` so a submission that re-exports ``evaluate`` from ``__init__`` (but
# moved it off ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
evaluate = None
CalcError = None
try:
    try:
        mod = importlib.import_module("calceval.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("calceval")
    evaluate = getattr(mod, "evaluate")
    CalcError = getattr(mod, "CalcError", None)
    if not (isinstance(CalcError, type) and issubclass(CalcError, BaseException)):
        CalcError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- an independent reference oracle computed HERE (NO eval) ------------------
# A self-contained shunting-yard evaluator so the grader never trusts the
# submission's own arithmetic and never calls ``eval``. Left/right associativity
# and the unary-vs-``^`` binding are encoded explicitly and correctly here.
_BINPREC = {"+": 1, "-": 1, "*": 2, "/": 2, "^": 4}
_RIGHT = {"^"}  # right-associative binary operators
_UNARY_PREC = 3  # unary minus: looser than '^' (4), tighter than '*'/'/' (2)


def _otokens(expr):
    out = []
    i, n = 0, len(expr)
    while i < n:
        c = expr[i]
        if c.isspace():
            i += 1
            continue
        if c in "+-*/^()":
            out.append(c)
            i += 1
            continue
        if c.isdigit() or c == ".":
            j = i
            while j < n and (expr[j].isdigit() or expr[j] == "."):
                j += 1
            out.append(expr[i:j])
            i = j
            continue
        raise ValueError(f"bad char {c!r}")
    return out


def oracle(expr):
    """Evaluate ``expr`` with correct precedence/associativity, returning float."""
    toks = _otokens(expr)
    if not toks:
        raise ValueError("empty")
    values, ops = [], []  # ops entries: ('bin', op) or ('u',)
    prev = None  # None | 'num' | 'op' | '(' | ')'

    def apply(top):
        if top[0] == "u":
            values.append(-values.pop())
        else:
            op = top[1]
            b = values.pop()
            a = values.pop()
            if op == "+":
                values.append(a + b)
            elif op == "-":
                values.append(a - b)
            elif op == "*":
                values.append(a * b)
            elif op == "/":
                values.append(a / b)
            else:
                values.append(float(a ** b))

    def prec(top):
        return _UNARY_PREC if top[0] == "u" else _BINPREC[top[1]]

    for t in toks:
        if t == "(":
            ops.append(("(",))
            prev = "("
        elif t == ")":
            while ops and ops[-1][0] != "(":
                apply(ops.pop())
            if not ops:
                raise ValueError("unbalanced )")
            ops.pop()  # discard '('
            prev = ")"
        elif t in "+-*/^":
            if t == "-" and prev in (None, "op", "("):
                # Unary minus is a right-associative PREFIX operator: it binds
                # looser than '^' (so '-2^2' is '-(2^2)') yet is itself the right
                # operand of any pending binary op (so '2^-1' keeps '^' stacked).
                # A prefix operator pops nothing on push — it just stacks and is
                # applied (innermost first) once its operand is on the value
                # stack. This makes '--2' == 2 and '2^-1' == 0.5 both work.
                ops.append(("u",))
            elif t == "+" and prev in (None, "op", "("):
                pass  # unary plus: no-op
            else:
                p = _BINPREC[t]
                right = t in _RIGHT
                while ops and ops[-1][0] != "(":
                    tp = prec(ops[-1])
                    if tp > p or (tp == p and not right):
                        apply(ops.pop())
                    else:
                        break
                ops.append(("bin", t))
            prev = "op"
        else:
            values.append(float(t))
            prev = "num"

    while ops:
        top = ops.pop()
        if top[0] == "(":
            raise ValueError("unbalanced (")
        apply(top)
    if len(values) != 1:
        raise ValueError("malformed")
    return values[0]


def expect(expr):
    """Check ``evaluate(expr)`` equals the oracle's value (to a float epsilon)."""
    def _fn():
        want = oracle(expr)
        got = evaluate(expr)
        ok = isinstance(got, float) and abs(got - want) <= 1e-9 * max(1.0, abs(want))
        return ok, f"{expr!r}: got {got!r}, expected {want!r}"

    return _fn


def raises(expr):
    """Check ``evaluate(expr)`` raises CalcError (not some other exception)."""
    def _fn():
        try:
            got = evaluate(expr)
        except CalcError:
            return True, f"{expr!r}: raised CalcError"
        except Exception as e:  # noqa: BLE001
            return False, f"{expr!r}: raised {type(e).__name__}, expected CalcError"
        return False, f"{expr!r}: did not raise, returned {got!r}"

    return _fn


if import_ok:
    # --- baseline precedence (pass even buggy; guards against regressions) ----
    check("basic_mul_over_add", "* binds tighter than + (2+3*4 == 14)", expect("2+3*4"))
    check("basic_add_mul_chain", "mixed +/* precedence (2*3+4*5 == 26)", expect("2*3+4*5"))
    check("basic_sub_mul", "* binds tighter than - (10-2*3 == 4)", expect("10-2*3"))
    check("basic_parens", "parentheses override precedence ((2+3)*4 == 20)", expect("(2+3)*4"))
    check("basic_pow_over_mul", "^ binds tighter than * (2*3^2 == 18)", expect("2*3^2"))
    check("basic_single_number", "a lone number evaluates to itself (42 -> 42.0)", expect("42"))
    check("basic_decimal", "decimals parse and compute (3.5*2 == 7.0)", expect("3.5*2"))
    check("basic_leading_dot", "leading-dot decimals parse (.5+.5 == 1.0)", expect(".5+.5"))

    # --- BUG 1: '-' and '/' must be LEFT-associative -------------------------
    check("left_assoc_sub", "subtraction is left-associative (10-2-3 == 5)", expect("10-2-3"))
    check("left_assoc_sub_long", "long subtraction chain (2-3-4-5 == -10)", expect("2-3-4-5"))
    check("left_assoc_div", "division is left-associative (100/10/2 == 5)", expect("100/10/2"))
    check("left_assoc_div_long", "long division chain (64/4/2/2 == 4)", expect("64/4/2/2"))
    check("left_assoc_mixed", "mixed +/- left to right (1+2-3+4 == 4)", expect("1+2-3+4"))
    check("left_assoc_sub_then_mul", "left-assoc minus around a product (20-2-3*2 == 12)",
          expect("20-2-3*2"))

    # --- BUG 2: '^' must be RIGHT-associative --------------------------------
    check("right_assoc_pow", "exponent is right-associative (2^3^2 == 512)", expect("2^3^2"))
    check("right_assoc_pow2", "exponent right-assoc again (2^2^3 == 256)", expect("2^2^3"))
    check("right_assoc_pow_triple", "triple exponent right-assoc (2^2^2^2 == 65536)",
          expect("2^2^2^2"))
    check("right_assoc_pow_zero", "right-assoc with a zero exponent (4^3^0 == 4)", expect("4^3^0"))

    # --- BUG 3: unary minus binds LOOSER than '^' ----------------------------
    check("unary_pow_binding", "unary minus binds looser than ^ (-2^2 == -4)", expect("-2^2"))
    check("unary_pow_even", "unary over even power (-2^4 == -16)", expect("-2^4"))
    check("paren_unary_pow", "parens flip the binding ((-2)^2 == 4)", expect("(-2)^2"))
    check("unary_simple", "plain unary minus (-3+5 == 2)", expect("-3+5"))
    check("unary_group", "unary minus over a group (-(2+3) == -5)", expect("-(2+3)"))
    check("pow_negative_exp", "negative exponent via unary (2^-1 == 0.5)", expect("2^-1"))

    # --- the three-way interaction (needs ALL the precedence rules right) ----
    check("interaction_unary_right_pow", "unary + right-assoc ^ (-2^2^2 == -16)",
          expect("-2^2^2"))
    check("interaction_sub_pow", "left-assoc minus around right-assoc power (1-2^3^0 == -1)",
          expect("1-2^3^0"))
    check("interaction_full", "all rules at once (10-2-3^1^2*2 == 2)", expect("10-2-3^1^2*2"))

    # --- validation: malformed input raises CalcError ------------------------
    check("err_empty", "empty string raises CalcError", raises(""))
    check("err_blank", "all-whitespace raises CalcError", raises("   "))
    check("err_trailing_op", "a trailing operator raises CalcError", raises("2+"))
    check("err_unbalanced", "unbalanced parens raise CalcError", raises("(1+2"))
    check("err_bad_char", "an unknown character raises CalcError", raises("2&3"))
    check("err_div_zero", "division by zero raises CalcError", raises("1/0"))


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 33

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "calceval",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
