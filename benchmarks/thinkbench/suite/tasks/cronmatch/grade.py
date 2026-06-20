#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `cronmatch`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``cronmatch`` package against the BRIEF'S CONTRACT (the ``cronmatch.public``
``matches`` API), NOT against any particular internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE step / stepped-range / day-of-week edge cases, all with expected values
computed HERE (never read from the agent's tests). The FIXED reference passes
every check; the planted-bug starter fails a discriminating subset, so a partial
fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``cronmatch.public``:
  1. ``*/n`` off by the field minimum — the step range starts at 0 regardless of
     the field's low bound, so month ``*/3`` becomes {0,3,6,9,12} instead of
     {1,4,7,10} and day-of-month ``*/10`` becomes {0,10,20,30} instead of
     {1,11,21,31};
  2. stepped range ``a-b/n`` ignores the step — it expands the whole ``a..b``
     span with stride 1, so ``10-30/10`` becomes 10..30 instead of {10,20,30};
  3. day-of-month / day-of-week AND-ed instead of OR-ed — when BOTH are
     restricted standard cron fires if EITHER matches, but the starter demands
     both, so ``0 0 13 * 5`` only matches a Friday that is also the 13th.

These interact: a stepped range (or stepped month) feeding a restricted
day-of-month while the day-of-week is also restricted exercises bugs 1/2 and 3 at
once, so some checks need more than one fix to pass.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs).
"""
import importlib
import json
import sys
from datetime import datetime

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --- import the produced package ---------------------------------------------
# Contract path is ``cronmatch.public``; fall back to the package root
# ``cronmatch`` so a submission that re-exports ``matches`` from ``__init__`` (but
# moved it off ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
matches = None
CronError = None
try:
    try:
        mod = importlib.import_module("cronmatch.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("cronmatch")
    matches = getattr(mod, "matches")
    CronError = getattr(mod, "CronError", None)
    if not (isinstance(CronError, type) and issubclass(CronError, BaseException)):
        CronError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
_RANGES = {
    "minute": (0, 59),
    "hour": (0, 23),
    "dom": (1, 31),
    "month": (1, 12),
    "dow": (0, 6),
}


def _field_set(spec, lo, hi):
    """Reference expansion of one cron field into its allowed-value set."""
    allowed = set()
    for term in spec.split(","):
        term = term.strip()
        step = 1
        if "/" in term:
            base, step_s = term.split("/", 1)
            step = int(step_s)
        else:
            base = term
        if base == "*":
            start, end = lo, hi
        elif "-" in base:
            a_s, b_s = base.split("-", 1)
            start, end = int(a_s), int(b_s)
        else:
            start = end = int(base)
        for v in range(start, end + 1, step):
            allowed.add(v)
    return allowed


def oracle(expr, dt):
    """Reference matcher, independent of the submission under test."""
    parts = expr.split()
    if len(parts) != 5:
        raise ValueError("cron expression must have 5 fields")
    minute_set = _field_set(parts[0], *_RANGES["minute"])
    hour_set = _field_set(parts[1], *_RANGES["hour"])
    dom_set = _field_set(parts[2], *_RANGES["dom"])
    month_set = _field_set(parts[3], *_RANGES["month"])
    dow_set = _field_set(parts[4], *_RANGES["dow"])

    cron_dow = (dt.weekday() + 1) % 7
    if dt.minute not in minute_set:
        return False
    if dt.hour not in hour_set:
        return False
    if dt.month not in month_set:
        return False

    dom_restricted = parts[2] != "*"
    dow_restricted = parts[4] != "*"
    dom_ok = dt.day in dom_set
    dow_ok = cron_dow in dow_set
    if dom_restricted and dow_restricted:
        return dom_ok or dow_ok
    if dom_restricted:
        return dom_ok
    if dow_restricted:
        return dow_ok
    return True


def _dt(s):
    return datetime.strptime(s, "%Y-%m-%d %H:%M")


def expect(label, expr, dts):
    """Check ``matches(expr, dt)`` equals the oracle's verdict for ``dt``."""
    dt = _dt(dts)

    def _fn():
        want = oracle(expr, dt)
        got = matches(expr, dt)
        return (bool(got) == want), f"{label}: {expr!r} @ {dts} -> got {got!r}, expected {want!r}"

    return _fn


if import_ok:
    # --- baseline: pass even fully buggy (regression guards) -----------------
    check("every_minute", "plain * * * * * matches any datetime",
          expect("every", "* * * * *", "2026-06-18 13:47"))
    check("exact_match_true", "exact minute+hour matches",
          expect("exact_t", "30 9 * * *", "2026-06-18 09:30"))
    check("exact_minute_false", "wrong minute fails an exact match",
          expect("exact_m", "30 9 * * *", "2026-06-18 09:31"))
    check("exact_hour_false", "wrong hour fails an exact match",
          expect("exact_h", "30 9 * * *", "2026-06-18 10:30"))
    check("list_minute_true", "comma list matches a listed minute",
          expect("list_t", "0,30,45 * * * *", "2026-06-18 14:45"))
    check("list_minute_false", "comma list rejects an unlisted minute",
          expect("list_f", "0,30,45 * * * *", "2026-06-18 14:31"))
    check("simple_range_in", "a-b range matches inside the range",
          expect("rng_in", "10-20 * * * *", "2026-06-18 14:15"))
    check("simple_range_out", "a-b range rejects outside the range",
          expect("rng_out", "10-20 * * * *", "2026-06-18 14:21"))
    check("month_field_true", "month field matches the right month",
          expect("mon_t", "0 0 1 6 *", "2026-06-01 00:00"))
    check("month_field_false", "month field rejects the wrong month",
          expect("mon_f", "0 0 1 6 *", "2026-07-01 00:00"))
    check("dow_only_true", "day-of-week-only rule matches that weekday",
          expect("dow_t", "0 0 * * 1", "2026-06-15 00:00"))  # Monday
    check("dow_only_false", "day-of-week-only rule rejects other weekdays",
          expect("dow_f", "0 0 * * 1", "2026-06-16 00:00"))  # Tuesday
    check("dom_only_true", "day-of-month-only rule matches that day",
          expect("dom_t", "0 0 15 * *", "2026-06-15 00:00"))
    check("dom_only_false", "day-of-month-only rule rejects other days",
          expect("dom_f", "0 0 15 * *", "2026-06-16 00:00"))
    check("dow_sunday_zero", "day-of-week 0 means Sunday",
          expect("dow0", "0 0 * * 0", "2026-06-21 00:00"))  # Sunday
    check("step_minute_min0_hit", "*/15 on minute (min 0) matches 30",
          expect("smin_h", "*/15 * * * *", "2026-06-18 00:30"))
    check("step_minute_min0_miss", "*/15 on minute (min 0) rejects 31",
          expect("smin_m", "*/15 * * * *", "2026-06-18 00:31"))

    # --- BUG 1: */n off by the field minimum (month / day-of-month) ----------
    check("step_month_q_jan", "month */3 includes January (1,4,7,10)",
          expect("smq_jan", "0 0 1 */3 *", "2026-01-01 00:00"))
    check("step_month_q_jul", "month */3 includes July",
          expect("smq_jul", "0 0 1 */3 *", "2026-07-01 00:00"))
    check("step_month_q_jun_miss", "month */3 excludes June",
          expect("smq_jun", "0 0 1 */3 *", "2026-06-01 00:00"))
    check("step_month_q_mar_miss", "month */3 excludes March",
          expect("smq_mar", "0 0 1 */3 *", "2026-03-01 00:00"))
    check("step_month_2_mar", "month */2 includes March, excludes February",
          expect("sm2_mar", "0 0 1 */2 *", "2026-03-01 00:00"))
    check("step_month_2_feb_miss", "month */2 excludes February (1,3,5,...)",
          expect("sm2_feb", "0 0 1 */2 *", "2026-02-01 00:00"))
    check("step_dom_10_hit", "day-of-month */10 includes the 11th (1,11,21,31)",
          expect("sd10_h", "0 0 */10 * *", "2026-06-11 00:00"))
    check("step_dom_10_miss", "day-of-month */10 excludes the 10th",
          expect("sd10_m", "0 0 */10 * *", "2026-06-10 00:00"))
    check("step_dom_10_first", "day-of-month */10 includes the 1st",
          expect("sd10_1", "0 0 */10 * *", "2026-06-01 00:00"))
    check("step_dom_7_hit", "day-of-month */7 includes the 8th (1,8,15,22,29)",
          expect("sd7_h", "0 0 */7 * *", "2026-06-08 00:00"))

    # --- BUG 2: stepped range a-b/n honors the step --------------------------
    check("range_step_min_hit", "minute 10-30/10 matches 20",
          expect("rsm_h", "10-30/10 * * * *", "2026-06-18 00:20"))
    check("range_step_min_miss15", "minute 10-30/10 rejects 15",
          expect("rsm_15", "10-30/10 * * * *", "2026-06-18 00:15"))
    check("range_step_min_miss25", "minute 10-30/10 rejects 25",
          expect("rsm_25", "10-30/10 * * * *", "2026-06-18 00:25"))
    check("range_step_hour_hit", "hour 8-18/2 matches 14",
          expect("rsh_h", "0 8-18/2 * * *", "2026-06-18 14:00"))
    check("range_step_hour_miss9", "hour 8-18/2 rejects 9",
          expect("rsh_9", "0 8-18/2 * * *", "2026-06-18 09:00"))
    check("range_step_hour_miss11", "hour 8-18/2 rejects 11",
          expect("rsh_11", "0 8-18/2 * * *", "2026-06-18 11:00"))
    check("range_step_dom_hit", "day-of-month 5-25/10 matches the 15th",
          expect("rsd_h", "0 0 5-25/10 * *", "2026-06-15 00:00"))
    check("range_step_dom_miss", "day-of-month 5-25/10 rejects the 10th",
          expect("rsd_m", "0 0 5-25/10 * *", "2026-06-10 00:00"))

    # --- BUG 3: day-of-month / day-of-week OR when both restricted -----------
    check("or_dow_fires", "0 0 13 * 5: a Friday that is not the 13th still matches",
          expect("or_dow", "0 0 13 * 5", "2026-06-19 00:00"))  # Fri 19th
    check("or_dom_fires", "0 0 13 * 5: the 13th that is not a Friday still matches",
          expect("or_dom", "0 0 13 * 5", "2026-06-13 00:00"))  # Sat 13th
    check("or_both", "0 0 13 * 5: a Friday the 13th matches",
          expect("or_both", "0 0 13 * 5", "2026-02-13 00:00"))  # Fri 13th
    check("or_neither", "0 0 13 * 5: a non-13th non-Friday does not match",
          expect("or_neither", "0 0 13 * 5", "2026-06-18 00:00"))  # Thu 18th
    check("or_dow_wed_fires", "0 0 1 * 3: a Wednesday that is not the 1st matches",
          expect("or_wed", "0 0 1 * 3", "2026-06-17 00:00"))  # Wed 17th
    check("or_dom_first_fires", "0 0 1 * 3: the 1st that is not a Wednesday matches",
          expect("or_1st", "0 0 1 * 3", "2026-06-01 00:00"))  # Mon 1st

    # --- interaction: needs more than one fix --------------------------------
    check("ix_step_range_or_dom", "10-20/5 dom OR Fri: the 15th fires via dom (needs step+OR)",
          expect("ix_a", "0 0 10-20/5 * 5", "2026-06-15 00:00"))  # Mon 15th
    check("ix_step_range_or_dow", "10-20/5 dom OR Fri: a Friday off the set fires via dow",
          expect("ix_b", "0 0 10-20/5 * 5", "2026-06-19 00:00"))  # Fri 19th
    check("ix_step_range_or_none", "10-20/5 dom OR Fri: neither -> no match",
          expect("ix_c", "0 0 10-20/5 * 5", "2026-06-17 00:00"))  # Wed 17th
    check("ix_step_month_or_dow", "*/3 month + 1st-OR-Fri: July Friday fires (needs step-min+OR)",
          expect("ix_d", "0 0 1 */3 5", "2026-07-03 00:00"))  # Jul 3rd, Fri
    check("ix_step_month_or_dom", "*/3 month + 1st-OR-Fri: April 1st fires (needs step-min+OR)",
          expect("ix_e", "0 0 1 */3 5", "2026-04-01 00:00"))  # Apr 1st

    # --- validation ----------------------------------------------------------
    def c_bad_field_count():
        try:
            matches("* * * *", _dt("2026-06-18 00:00"))
        except CronError:
            return True, "raised CronError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected CronError"
        return False, "did not raise"

    check("bad_field_count_raises", "a non-5-field expression raises CronError", c_bad_field_count)

    def c_bad_token():
        try:
            matches("oops * * * *", _dt("2026-06-18 00:00"))
        except CronError:
            return True, "raised CronError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected CronError"
        return False, "did not raise"

    check("bad_token_raises", "an unparseable field token raises CronError", c_bad_token)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 48

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "cronmatch",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
