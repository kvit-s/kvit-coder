#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `rrulelite`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``rrulelite`` package against the BRIEF'S CONTRACT (the ``rrulelite.public``
``expand`` API), NOT against any particular internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE interval / month-end / until / leap-year edge cases, all with expected
values computed HERE (never read from the agent's tests). The FIXED reference
passes every check; the planted-bug starter fails a discriminating subset, so a
partial fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``rrulelite.public``:
  1. ``interval`` ignored for monthly — a monthly rule always steps ONE month,
     so ``interval == 2`` still yields consecutive months;
  2. month-end overflow spills instead of clamping — Jan 31 + 1 month asks for
     Feb 31 and is carried forward to Mar 3 rather than clamped to Feb 28/29;
  3. ``until`` off-by-one — it is treated as EXCLUSIVE (``>=``), dropping the
     date that lands exactly on ``until`` instead of keeping it (inclusive).

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs).
"""
import calendar
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
# Contract path is ``rrulelite.public``; fall back to the package root
# ``rrulelite`` so a submission that re-exports ``expand`` from ``__init__`` (but
# moved it off ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
expand = None
RRuleError = None
try:
    try:
        mod = importlib.import_module("rrulelite.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("rrulelite")
    expand = getattr(mod, "expand")
    RRuleError = getattr(mod, "RRuleError", None)
    if not (isinstance(RRuleError, type) and issubclass(RRuleError, BaseException)):
        RRuleError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
def _add_months(d, months):
    """``d`` advanced by ``months`` whole months, clamping the day to the last
    valid day of the target month."""
    total = (d.year * 12 + (d.month - 1)) + months
    year, month0 = divmod(total, 12)
    month = month0 + 1
    last = calendar.monthrange(year, month)[1]
    return date(year, month, min(d.day, last))


def oracle(freq, interval, until, start, limit):
    """Reference expansion, independent of the submission under test."""
    if limit <= 0:
        return []
    out = []
    n = 0
    while len(out) < limit:
        if freq == "daily":
            cur = start + timedelta(days=interval * n)
        elif freq == "weekly":
            cur = start + timedelta(weeks=interval * n)
        else:
            cur = _add_months(start, interval * n)
        if until is not None and cur > until:
            break
        out.append(cur)
        n += 1
    return out


def expect(label, rule, start, limit):
    """Check ``expand(rule, start, limit)`` equals the oracle's expansion."""
    freq = rule["freq"]
    interval = rule.get("interval", 1)
    until = rule.get("until")

    def _fn():
        want = oracle(freq, interval, until, start, limit)
        got = expand(dict(rule), start, limit)
        return (list(got) == want), f"{label}: got {got!r}, expected {want!r}"

    return _fn


if import_ok:
    # --- baseline: daily / weekly (pass even buggy; guards regressions) -------
    check("daily_simple", "daily interval=1 emits consecutive days from start",
          expect("daily", {"freq": "daily"}, date(2026, 1, 1), 4))
    check("daily_interval3", "daily interval=3 steps every 3rd day",
          expect("daily_i3", {"freq": "daily", "interval": 3}, date(2026, 1, 1), 4))
    check("weekly_interval2", "weekly interval=2 steps every other week",
          expect("weekly_i2", {"freq": "weekly", "interval": 2}, date(2026, 3, 2), 4))

    # --- monthly: interval (BUG 1) -------------------------------------------
    check("monthly_simple", "monthly interval=1 emits consecutive months",
          expect("monthly", {"freq": "monthly"}, date(2026, 1, 15), 4))
    check("monthly_interval2", "monthly interval=2 -> every other month",
          expect("monthly_i2", {"freq": "monthly", "interval": 2}, date(2026, 1, 10), 4))
    check("monthly_interval3_year_wrap", "monthly interval=3 wraps across year end",
          expect("monthly_i3", {"freq": "monthly", "interval": 3}, date(2026, 11, 5), 4))

    # --- monthly: month-end clamp (BUG 2) ------------------------------------
    check("clamp_jan31_to_feb28", "Jan 31 + 1 month clamps to Feb 28 (common year)",
          expect("clamp_common", {"freq": "monthly"}, date(2026, 1, 31), 3))
    check("clamp_leap_feb29", "Jan 31 + 1 month clamps to Feb 29 in a leap year (2028)",
          expect("clamp_leap", {"freq": "monthly"}, date(2028, 1, 31), 3))
    check("clamp_then_restore", "clamping is per-step: Jan 31 -> Feb 28 -> Mar 31",
          expect("clamp_restore", {"freq": "monthly"}, date(2026, 1, 31), 4))
    check("clamp_31_to_30day_month", "Mar 31 + 1 month clamps to Apr 30",
          expect("clamp_apr", {"freq": "monthly"}, date(2026, 3, 31), 2))
    check("clamp_interval2_endpoints", "interval=2 from Jan 31 -> Jan 31, Mar 31, May 31",
          expect("clamp_i2", {"freq": "monthly", "interval": 2}, date(2026, 1, 31), 3))

    # --- until: inclusive boundary (BUG 3) -----------------------------------
    check("until_inclusive_daily", "until keeps the date landing exactly on it (daily)",
          expect("until_daily", {"freq": "daily", "until": date(2026, 1, 3)},
                 date(2026, 1, 1), 10))
    check("until_inclusive_monthly", "until keeps the exact-boundary monthly date",
          expect("until_monthly", {"freq": "monthly", "until": date(2026, 4, 15)},
                 date(2026, 1, 15), 10))
    check("until_strict_past_stops", "a generated date strictly past until is excluded",
          expect("until_strict", {"freq": "daily", "interval": 2, "until": date(2026, 1, 4)},
                 date(2026, 1, 1), 10))  # Jan1, Jan3 (Jan5 > Jan4 stops); Jan4 never generated

    def c_until_start_past():
        # start already after until -> empty list (no crash)
        got = expand({"freq": "daily", "until": date(2026, 1, 1)}, date(2026, 1, 5), 5)
        return (list(got) == []), f"got {got!r}, expected []"

    check("until_start_past_until", "start beyond until yields empty list", c_until_start_past)

    # --- limit cap -----------------------------------------------------------
    def c_limit_cap():
        got = expand({"freq": "daily"}, date(2026, 1, 1), 5)
        return (len(got) == 5 and got[0] == date(2026, 1, 1)), f"got {got!r}"

    check("limit_cap", "limit caps the number of emitted dates", c_limit_cap)

    def c_limit_zero():
        got = expand({"freq": "daily"}, date(2026, 1, 1), 0)
        return (list(got) == []), f"got {got!r}, expected []"

    check("limit_zero_empty", "limit <= 0 returns empty list", c_limit_zero)

    # --- the three-way interaction (needs ALL three bugs fixed) --------------
    check(
        "interaction_interval_clamp_until",
        "interval=2 + month-end clamp + inclusive until together",
        expect(
            "interaction",
            {"freq": "monthly", "interval": 2, "until": date(2026, 7, 31)},
            date(2026, 1, 31),
            10,
        ),
    )

    # --- validation ----------------------------------------------------------
    def c_bad_freq():
        try:
            expand({"freq": "yearly"}, date(2026, 1, 1), 3)
        except RRuleError:
            return True, "raised RRuleError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected RRuleError"
        return False, "did not raise"

    check("bad_freq_raises", "unsupported freq raises RRuleError", c_bad_freq)

    def c_bad_interval():
        try:
            expand({"freq": "daily", "interval": 0}, date(2026, 1, 1), 3)
        except RRuleError:
            return True, "raised RRuleError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected RRuleError"
        return False, "did not raise"

    check("bad_interval_raises", "non-positive interval raises RRuleError", c_bad_interval)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 20

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "rrulelite",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
