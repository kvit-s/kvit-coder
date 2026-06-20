#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `repairmoney`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never reads or runs the agent's own tests (`repairmoney/test_*.py`). It
grades the produced `repairmoney` package against the BRIEF'S CONTRACT (the
`repairmoney` / `repairmoney.public` API), NOT against any particular internal
file layout and NOT against the visible test file.

The planted bugs (in the shipped `setup` package):
  1. format_cents puts the minus sign AFTER the `$` ("$-12.34") and/or drops it.
  2. format_cents does not zero-pad the cents (5 -> "$0.5" instead of "$0.05").
  3. split_evenly drops the remainder cent(s), so the parts do not sum to the
     total for a non-divisible split.
The checks below are a SUPERSET of the visible tests: more amounts (negative,
zero, large), and the split sum-to-total invariant across many n (divisible,
non-divisible, n=1). FIXED passes all; BUGGY fails several.

Expected values are computed HERE, independently of the package under test, so
the oracle never trusts the submission's own arithmetic.

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


# --- independent reference oracle (NOT imported from the submission) ----------
def expected_format(cents):
    """The contract's intended rendering: sign in front, cents zero-padded."""
    sign = "-" if cents < 0 else ""
    dollars, rem = divmod(abs(cents), 100)
    return f"{sign}${dollars}.{rem:02d}"


def expected_split(cents, n):
    """The contract's intended split: parts sum to cents, remainder spread to the
    earliest parts, all within one cent of each other."""
    base = cents // n  # floor division -> robust for negative totals too
    parts = [base] * n
    for i in range(cents - base * n):
        parts[i] += 1
    return parts


# --- import the produced package (contract: repairmoney.public) ---------------
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("repairmoney.public")
except Exception:  # noqa: BLE001 - fall back to the top-level package re-exports
    try:
        pub = importlib.import_module("repairmoney")
    except Exception as e2:  # noqa: BLE001
        import_ok = False
        import_detail = f"{type(e2).__name__}: {e2}"

if import_ok:
    format_cents = getattr(pub, "format_cents", None)
    split_evenly = getattr(pub, "split_evenly", None)
    if not callable(format_cents) or not callable(split_evenly):
        import_ok = False
        import_detail = (
            f"missing public API: format_cents={format_cents!r} "
            f"split_evenly={split_evenly!r}"
        )


if import_ok:
    # === format_cents ========================================================
    # 1. positive amount renders dollars.cents (the easy, already-passing case).
    def c_fmt_positive():
        got = format_cents(1234)
        return got == "$12.34", f"got={got!r} expected='$12.34'"

    check("fmt_positive", "format_cents renders a positive amount as $D.DD", c_fmt_positive)

    # 2. zero renders as $0.00 (both the dollar and the zero-pad must hold).
    def c_fmt_zero():
        got = format_cents(0)
        return got == "$0.00", f"got={got!r} expected='$0.00'"

    check("fmt_zero", "format_cents renders 0 as $0.00", c_fmt_zero)

    # 3. NEGATIVE amount: minus sign IN FRONT of the $ (the planted sign bug).
    def c_fmt_negative_sign():
        got = format_cents(-1234)
        return got == "-$12.34", f"got={got!r} expected='-$12.34' (sign before the $)"

    check(
        "fmt_negative_sign_in_front",
        "a negative amount puts the minus sign in front of the $ (-$12.34)",
        c_fmt_negative_sign,
    )

    # 4. cents under 10 are zero-padded to two digits (the planted pad bug).
    def c_fmt_pads_small_cents():
        got = format_cents(5)
        return got == "$0.05", f"got={got!r} expected='$0.05' (cents zero-padded)"

    check("fmt_pads_cents", "single-digit cents are zero-padded to two digits", c_fmt_pads_small_cents)

    # 5. negative AND small cents together: both fixes must compose.
    def c_fmt_negative_small():
        got = format_cents(-5)
        return got == "-$0.05", f"got={got!r} expected='-$0.05'"

    check("fmt_negative_small_cents", "a small negative amount renders -$0.05", c_fmt_negative_small)

    # 6. a battery of amounts must all match the independent oracle.
    def c_fmt_battery():
        cases = [1, 9, 10, 99, 100, 101, 999, 1000, 100000, 123456,
                 -1, -9, -99, -100, -1234, -100000]
        bad = []
        for c in cases:
            exp = expected_format(c)
            got = format_cents(c)
            if got != exp:
                bad.append(f"{c}->{got!r}!={exp!r}")
        return (not bad), ("all match" if not bad else "; ".join(bad[:6]))

    check("fmt_battery", "format_cents matches the oracle across many amounts", c_fmt_battery)

    # 7. large amount renders without truncation or grouping surprises.
    def c_fmt_large():
        got = format_cents(123456789)
        return got == "$1234567.89", f"got={got!r} expected='$1234567.89'"

    check("fmt_large", "a large amount renders all the dollar digits", c_fmt_large)

    # === split_evenly ========================================================
    # 8. an evenly-divisible split returns n equal parts summing to the total.
    def c_split_divisible():
        parts = split_evenly(1000, 4)
        return (parts == [250, 250, 250, 250] and sum(parts) == 1000), f"parts={parts!r}"

    check("split_divisible", "a divisible split yields n equal parts summing to total", c_split_divisible)

    # 9. a NON-divisible split still sums to the total (the planted remainder bug).
    def c_split_non_divisible_sum():
        parts = split_evenly(1000, 3)
        return (sum(parts) == 1000 and len(parts) == 3), \
            f"parts={parts!r} sum={sum(parts)} expected_sum=1000"

    check(
        "split_non_divisible_sums_to_total",
        "a non-divisible split's parts sum EXACTLY to the total",
        c_split_non_divisible_sum,
    )

    # 10. the remainder is distributed to the EARLIEST parts, within one cent.
    def c_split_remainder_shape():
        parts = split_evenly(1000, 3)
        ok_sum = sum(parts) == 1000
        ok_spread = (max(parts) - min(parts)) <= 1
        ok_front = parts == sorted(parts, reverse=True)  # bigger parts come first
        return (ok_sum and ok_spread and ok_front and parts == [334, 333, 333]), f"parts={parts!r}"

    check(
        "split_remainder_distributed",
        "the remainder cent goes to the earliest part(s), parts within one cent",
        c_split_remainder_shape,
    )

    # 11. n == 1 returns the whole total as a single part.
    def c_split_n_one():
        parts = split_evenly(100, 1)
        return parts == [100], f"parts={parts!r} expected=[100]"

    check("split_n_one", "split_evenly(total, 1) returns [total]", c_split_n_one)

    # 12. sum-to-total invariant across MANY (cents, n) combos vs the oracle.
    def c_split_invariant_battery():
        cases = [
            (1000, 3), (1000, 7), (10, 3), (1, 3), (0, 4), (7, 7), (5, 2),
            (99, 4), (100, 6), (12345, 11), (2, 3), (1, 1), (100000, 9),
        ]
        bad = []
        for cents, n in cases:
            parts = split_evenly(cents, n)
            exp = expected_split(cents, n)
            if not isinstance(parts, list):
                bad.append(f"({cents},{n})->type {type(parts).__name__}")
                continue
            if len(parts) != n:
                bad.append(f"({cents},{n})->len {len(parts)}")
                continue
            if sum(parts) != cents:
                bad.append(f"({cents},{n})->sum {sum(parts)}!={cents}")
                continue
            if parts != exp:
                bad.append(f"({cents},{n})->{parts!r}!={exp!r}")
        return (not bad), ("all match" if not bad else "; ".join(bad[:6]))

    check(
        "split_sum_invariant_battery",
        "across many (cents, n), parts have length n and sum to cents",
        c_split_invariant_battery,
    )

    # 13. a negative total splits into parts that still sum to the (negative) total.
    def c_split_negative_total():
        parts = split_evenly(-1000, 3)
        return (sum(parts) == -1000 and len(parts) == 3 and (max(parts) - min(parts)) <= 1), \
            f"parts={parts!r} sum={sum(parts)} expected_sum=-1000"

    check("split_negative_total", "a negative total splits into parts summing to it", c_split_negative_total)

    # 14. a large total with an awkward n still conserves every cent.
    def c_split_large():
        parts = split_evenly(1_000_000, 7)
        return (sum(parts) == 1_000_000 and len(parts) == 7 and (max(parts) - min(parts)) <= 1), \
            f"sum={sum(parts)} len={len(parts)} spread={max(parts) - min(parts)}"

    check("split_large_conserves", "a large awkward split conserves every cent", c_split_large)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 14

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "repairmoney",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
