#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `datespan`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``datespan`` package against the BRIEF'S CONTRACT (the ``datespan.public``
``add_business_days`` / ``business_days_between`` API), NOT against any
particular internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE weekend / negative / reversed / span edge cases, all with expected values
computed HERE (never read from the agent's tests). The FIXED reference passes
every check; the planted-bug starter fails a discriminating subset, so a partial
fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``datespan.public``:
  1. negative ``n`` steps the WRONG WAY over weekends — the weekend skip always
     nudges forward (+1 day) regardless of step direction, so going back across
     a weekend bounces forward instead of landing on the previous Friday;
  2. a weekend ``start`` is NOT normalized — a Saturday/Sunday ``start`` is used
     as-is instead of being snapped forward to the next business day;
  3. ``business_days_between`` drops the SIGN when ``a`` is after ``b`` — it
     returns the positive magnitude instead of the negated count (the
     within-week forward count itself is correct, so simple cases still read
     right).

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs).
"""
import importlib
import json
import sys
from datetime import date, timedelta

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --- import the produced package ---------------------------------------------
# Contract path is ``datespan.public``; fall back to the package root
# ``datespan`` so a submission that re-exports the functions from ``__init__``
# (but moved them off ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
add_business_days = None
business_days_between = None
try:
    try:
        mod = importlib.import_module("datespan.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("datespan")
    add_business_days = getattr(mod, "add_business_days")
    business_days_between = getattr(mod, "business_days_between")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
def _is_weekend(d):
    return d.weekday() >= 5


def _normalize_forward(d):
    while _is_weekend(d):
        d += timedelta(days=1)
    return d


def oracle_add(start, n):
    """Reference ``add_business_days``, independent of the submission."""
    cur = _normalize_forward(start)
    if n == 0:
        return cur
    step = 1 if n > 0 else -1
    remaining = abs(n)
    while remaining > 0:
        cur += timedelta(days=step)
        if not _is_weekend(cur):
            remaining -= 1
    return cur


def oracle_between(a, b):
    """Reference ``business_days_between``, independent of the submission."""
    if a == b:
        return 0
    sign = 1
    lo, hi = a, b
    if a > b:
        lo, hi = b, a
        sign = -1
    count = 0
    cur = lo
    while cur < hi:
        cur += timedelta(days=1)
        if not _is_weekend(cur):
            count += 1
    return sign * count


def expect_add(label, start, n):
    """Check ``add_business_days(start, n)`` equals the oracle."""

    def _fn():
        want = oracle_add(start, n)
        got = add_business_days(start, n)
        return (got == want), f"{label}: got {got!r}, expected {want!r}"

    return _fn


def expect_between(label, a, b):
    """Check ``business_days_between(a, b)`` equals the oracle."""

    def _fn():
        want = oracle_between(a, b)
        got = business_days_between(a, b)
        return (got == want), f"{label}: got {got!r}, expected {want!r}"

    return _fn


if import_ok:
    # --- add: forward baseline (passes even buggy; guards regressions) -------
    check("add_fwd_within_week", "Mon + small n stays inside the week",
          expect_add("add_fwd", date(2026, 6, 15), 3))
    check("add_fwd_to_friday", "Mon + 4 lands on Fri",
          expect_add("add_fri", date(2026, 6, 15), 4))
    check("add_fwd_skips_weekend", "Mon + 5 skips the weekend to next Mon",
          expect_add("add_skip", date(2026, 6, 15), 5))
    check("add_fwd_friday_rolls", "Fri + 1 rolls to Mon",
          expect_add("add_frimon", date(2026, 6, 19), 1))
    check("add_zero_on_business_day", "n == 0 on a weekday returns that day",
          expect_add("add_zero", date(2026, 6, 17), 0))

    # --- add: weekend-start normalization (BUG 2) ----------------------------
    check("add_sat_zero_normalizes", "Sat + 0 snaps forward to Mon",
          expect_add("add_sat0", date(2026, 6, 20), 0))
    check("add_sun_zero_normalizes", "Sun + 0 snaps forward to Mon",
          expect_add("add_sun0", date(2026, 6, 21), 0))
    check("add_sat_forward", "Sat + 1 anchors on Mon then steps to Tue",
          expect_add("add_sat1", date(2026, 6, 20), 1))
    check("add_sun_forward2", "Sun + 2 anchors on Mon then steps to Wed",
          expect_add("add_sun2", date(2026, 6, 21), 2))

    # --- add: negative direction over weekends (BUG 1) -----------------------
    check("add_neg_within_week", "Fri - 1 = Thu (no weekend crossed)",
          expect_add("add_neg1", date(2026, 6, 19), -1))
    check("add_neg_crosses_weekend", "Mon - 1 = previous Fri",
          expect_add("add_negfri", date(2026, 6, 15), -1))
    check("add_neg_two", "Mon - 2 = previous Thu",
          expect_add("add_neg2", date(2026, 6, 15), -2))
    check("add_neg_full_week", "Mon - 5 = the Monday a week earlier",
          expect_add("add_neg5", date(2026, 6, 15), -5))
    check("add_neg_from_tuesday", "next Mon - 1 = the Friday before",
          expect_add("add_negtue", date(2026, 6, 22), -1))

    # --- between: forward baseline (passes even buggy) -----------------------
    check("btw_fwd_one", "Mon -> Tue is 1 business day",
          expect_between("btw1", date(2026, 6, 15), date(2026, 6, 16)))
    check("btw_fwd_four", "Mon -> Fri is 4 business days",
          expect_between("btw4", date(2026, 6, 15), date(2026, 6, 19)))
    check("btw_same_day_zero", "a == b is 0",
          expect_between("btwsame", date(2026, 6, 15), date(2026, 6, 15)))
    check("btw_fwd_skips_weekend", "Mon -> next Mon is 5 (weekend excluded)",
          expect_between("btwskip", date(2026, 6, 15), date(2026, 6, 22)))
    check("btw_fri_to_sat_zero", "Fri -> Sat is 0",
          expect_between("btwsat", date(2026, 6, 19), date(2026, 6, 20)))
    check("btw_fwd_month_span", "Mon -> a Wed a month later",
          expect_between("btwmonth", date(2026, 6, 15), date(2026, 7, 15)))

    # --- between: sign when a > b (BUG 3) ------------------------------------
    check("btw_reverse_one", "Tue -> Mon is -1",
          expect_between("btwr1", date(2026, 6, 16), date(2026, 6, 15)))
    check("btw_reverse_week", "next Mon -> earlier Mon is -5",
          expect_between("btwr5", date(2026, 6, 22), date(2026, 6, 15)))
    check("btw_reverse_month_span", "reversed month span is negative",
          expect_between("btwrmonth", date(2026, 7, 15), date(2026, 6, 15)))

    # --- the three-way interaction (needs ALL three bugs fixed) --------------
    # Weekend start (normalize) + negative n stepping back across the weekend
    # (direction), and a reversed count that crosses a weekend (sign).
    check(
        "interaction_add_sun_negative",
        "Sun start, normalize forward to Mon, then step back across the weekend",
        expect_add("interact_add", date(2026, 6, 21), -1),
    )
    check(
        "interaction_between_reverse_weekend",
        "reversed span across a weekend is the negated weekend-skipping count",
        expect_between("interact_btw", date(2026, 6, 23), date(2026, 6, 18)),
    )


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 25

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "datespan",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
