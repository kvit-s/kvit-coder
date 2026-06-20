#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield Task (cronsim).

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `cronsim.public` API
and the `python -m cronsim` CLI), NOT against the model's own tests and NOT against
any particular internal file layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

Tolerance: the brief fixes the public SHAPES (next_runs -> list[str] of UTC ISO,
should_run -> bool, parse_cron -> dict) but under-specifies parse_cron's key names
and the CLI's exact stdout wording. This oracle checks BEHAVIOR — it computes every
expected next_runs / should_run value ITSELF from the standard library (zoneinfo) and
compares, never trusting the candidate's own numbers. ISO results are compared after
normalising to a UTC instant, so "...Z" vs "+00:00" and optional seconds all pass.
Spots where it assumes a convention the brief does not pin are marked `# ASSUMES`.
"""
import importlib
import json
import os
import subprocess
import sys
from datetime import datetime, timedelta
from datetime import timezone as _tz
from zoneinfo import ZoneInfo

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


# --- grader-internal ground truth (independent of the candidate) -------------
_BOUNDS = [(0, 59), (0, 23), (1, 31), (1, 12), (0, 7)]


def _expand(spec, lo, hi):
    out = set()
    for part in spec.split(","):
        step = 1
        base = part
        if "/" in part:
            base, _, s = part.partition("/")
            step = int(s)
        if base == "*":
            a, b = lo, hi
        elif "-" in base:
            x, _, y = base.partition("-")
            a, b = int(x), int(y)
        else:
            a = int(base)
            b = hi if step != 1 else a
        out.update(range(a, b + 1, step))
    return out


def _parse(expr):
    f = expr.split()
    minute = _expand(f[0], *_BOUNDS[0])
    hour = _expand(f[1], *_BOUNDS[1])
    dom = _expand(f[2], *_BOUNDS[2])
    month = _expand(f[3], *_BOUNDS[3])
    dow = {0 if d == 7 else d for d in _expand(f[4], *_BOUNDS[4])}
    return {
        "minute": minute, "hour": hour, "dom": dom, "month": month, "dow": dow,
        "dom_r": f[2] != "*", "dow_r": f[4] != "*",
    }


def _day_ok(s, dt):
    cron_dow = (dt.weekday() + 1) % 7
    dom_ok = dt.day in s["dom"]
    dow_ok = cron_dow in s["dow"]
    if s["dom_r"] and s["dow_r"]:
        return dom_ok or dow_ok
    if s["dom_r"]:
        return dom_ok
    if s["dow_r"]:
        return dow_ok
    return True


def _fires(s, dt):
    return (
        dt.minute in s["minute"] and dt.hour in s["hour"]
        and dt.month in s["month"] and _day_ok(s, dt)
    )


def _expected_next(expr, start_iso, count, tzname="UTC"):
    """Grader's own reference for next_runs: list of UTC instants (aware datetimes)."""
    s = _parse(expr)
    tz = ZoneInfo(tzname)
    raw = start_iso[:-1] + "+00:00" if start_iso.endswith("Z") else start_iso
    start = datetime.fromisoformat(raw)
    if start.tzinfo is None:
        start = start.replace(tzinfo=tz)
    start_utc = start.astimezone(_tz.utc)
    local = start.astimezone(tz).replace(second=0, microsecond=0)
    out = []
    steps = 0
    while len(out) < count and steps < 8 * 366 * 24 * 60:
        wall = datetime(local.year, local.month, local.day, local.hour, local.minute,
                        tzinfo=tz, fold=0)
        if wall.astimezone(_tz.utc) >= start_utc and _fires(s, local):
            out.append(wall.astimezone(_tz.utc))
        local += timedelta(minutes=1)
        steps += 1
    return out


def _to_instant(iso):
    """Normalise any UTC-ISO string the candidate emits to an aware UTC datetime."""
    s = iso.strip()
    if s.endswith("Z"):
        s = s[:-1] + "+00:00"
    dt = datetime.fromisoformat(s)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=_tz.utc)  # ASSUMES a naive output string is already UTC
    return dt.astimezone(_tz.utc)


def _cmp_runs(got, expr, start_iso, count, tzname="UTC"):
    """Compare a candidate next_runs list to the grader's own expected instants."""
    if not isinstance(got, list):
        return False, f"not a list: {type(got).__name__}"
    exp = _expected_next(expr, start_iso, count, tzname)
    if len(got) != len(exp):
        return False, f"len got={len(got)} exp={len(exp)} got={got!r}"
    for g, e in zip(got, exp):
        try:
            gi = _to_instant(g)
        except Exception as ex:  # noqa: BLE001
            return False, f"unparseable item {g!r}: {ex}"
        if gi != e:
            return False, f"mismatch got={gi.isoformat()} exp={e.isoformat()}"
    return True, f"{len(exp)} instants match"


# --- import the produced package (contract: cronsim.public) ------------------
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("cronsim.public")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. ranges + steps + lists — the brief's own example, in a DST-free month.
    #    Verifies hour-range 9-17, minute-step */15, dow-list/range 1-5, tz offset.
    def c_ranges_steps_lists():
        return _cmp_runs(
            pub.next_runs("*/15 9-17 * * 1-5", "2026-01-05T00:00:00Z", 12, "America/New_York"),
            "*/15 9-17 * * 1-5", "2026-01-05T00:00:00Z", 12, "America/New_York",
        )

    check("ranges_steps_lists", "ranges, */n steps and list/range fields fire correctly", c_ranges_steps_lists)

    # 2. comma lists specifically (0,30 minutes; explicit month list)
    def c_lists():
        return _cmp_runs(
            pub.next_runs("0,30 0 1,15 * *", "2026-01-01T00:00:00Z", 6, "UTC"),
            "0,30 0 1,15 * *", "2026-01-01T00:00:00Z", 6, "UTC",
        )

    check("lists", "comma lists in minute and day-of-month fields", c_lists)

    # 3. stepped range 1-10/2 (a step over an explicit range, not just */n)
    def c_stepped_range():
        return _cmp_runs(
            pub.next_runs("1-10/2 * * * *", "2026-01-01T00:00:00Z", 5, "UTC"),
            "1-10/2 * * * *", "2026-01-01T00:00:00Z", 5, "UTC",
        )

    check("stepped_range", "stepped range like 1-10/2 expands correctly", c_stepped_range)

    # 4. invalid expressions must raise (not return junk). Tolerant to exception type.
    def c_invalid():
        bad = [
            "* * * *",          # too few fields
            "* * * * * *",      # too many fields
            "60 * * * *",       # minute out of range
            "* 24 * * *",       # hour out of range
            "* * 0 * *",        # dom below 1
            "* * * 13 *",       # month above 12
            "5-2 * * * *",      # descending range
            "*/0 * * * *",      # zero step
            "abc * * * *",      # non-numeric
            "* * * * 8",        # dow above 7
        ]
        raised = 0
        survived = []
        for e in bad:
            try:
                pub.parse_cron(e)
                survived.append(e)
            except Exception:  # noqa: BLE001 - any error is acceptable rejection
                raised += 1
        # ASSUMES rejection surfaces as a raised exception (the brief says "invalid
        # expressions" without pinning a sentinel return); require ALL to be rejected.
        return (raised == len(bad)), f"rejected {raised}/{len(bad)}; survived={survived!r}"

    check("invalid", "invalid expressions are rejected (raise)", c_invalid)

    # 5. leap year — Feb 29 only fires in leap years; next from 2026 is 2028-02-29.
    def c_leap():
        return _cmp_runs(
            pub.next_runs("0 12 29 2 *", "2026-01-01T00:00:00Z", 2, "UTC"),
            "0 12 29 2 *", "2026-01-01T00:00:00Z", 2, "UTC",  # 2028 then 2032
        )

    check("leap_year", "Feb 29 schedule only matches leap years", c_leap)

    # 6a. DST spring-forward — noon daily is gap-free but its UTC offset shifts
    #     from EST(17:00Z) to EDT(16:00Z) across 2026-03-08.
    def c_dst_spring():
        return _cmp_runs(
            pub.next_runs("0 12 * * *", "2026-03-07T00:00:00Z", 3, "America/New_York"),
            "0 12 * * *", "2026-03-07T00:00:00Z", 3, "America/New_York",
        )

    check("dst_spring", "noon's UTC offset shifts correctly across spring-forward", c_dst_spring)

    # 6b. DST fall-back — noon shifts back EDT(16:00Z) -> EST(17:00Z) on 2026-11-01.
    def c_dst_fall():
        return _cmp_runs(
            pub.next_runs("0 12 * * *", "2026-10-31T00:00:00Z", 3, "America/New_York"),
            "0 12 * * *", "2026-10-31T00:00:00Z", 3, "America/New_York",
        )

    check("dst_fall", "noon's UTC offset shifts correctly across fall-back", c_dst_fall)

    # 7. weekday numbering — Sunday is both 0 and 7; Mon=1..Sat=6.
    #    "0 0 * * 0" and "0 0 * * 7" must produce identical Sunday-midnight runs.
    def c_weekday_numbering():
        r0 = pub.next_runs("0 0 * * 0", "2026-06-01T00:00:00Z", 4, "UTC")
        r7 = pub.next_runs("0 0 * * 7", "2026-06-01T00:00:00Z", 4, "UTC")
        ok0, d0 = _cmp_runs(r0, "0 0 * * 0", "2026-06-01T00:00:00Z", 4, "UTC")
        if not ok0:
            return False, f"dow=0 wrong: {d0}"
        # 7 must equal 0 (both Sunday); compare normalised instants.
        try:
            same = [_to_instant(x) for x in r0] == [_to_instant(x) for x in r7]
        except Exception as ex:  # noqa: BLE001
            return False, f"dow=7 unparseable: {ex}"
        # Spot-check the first hit really is a Sunday.
        first = _to_instant(r0[0])
        is_sun = first.weekday() == 6
        return (same and is_sun), f"0==7:{same} first={first.isoformat()} sunday={is_sun}"

    check("weekday_numbering", "Sunday is both dow 0 and dow 7; numbering is correct", c_weekday_numbering)

    # 8. day-of-month OR day-of-week semantics when BOTH restricted.
    #    "0 0 13 * 5": fires on the 13th OR on any Friday. Verify via should_run on a
    #    13th-not-Friday, a Friday-not-13th, and a neither day.
    def c_dom_dow_or():
        # 2026-01-13 is Tuesday (13th, not Friday) -> True via DOM.
        a = pub.should_run("0 0 13 * 5", "2026-01-13T00:00:00Z", "UTC")
        # 2026-01-02 is Friday (not the 13th) -> True via DOW.
        b = pub.should_run("0 0 13 * 5", "2026-01-02T00:00:00Z", "UTC")
        # 2026-01-06 is Tuesday (neither) -> False.
        c = pub.should_run("0 0 13 * 5", "2026-01-06T00:00:00Z", "UTC")
        ok = (a is True or a == True) and (b is True or b == True) and not c  # noqa: E712
        return ok, f"13th-not-fri={a} fri-not-13th={b} neither={c}"

    check("dom_dow_or", "dom/dow use OR semantics when both restricted", c_dom_dow_or)

    # 9. should_run agrees with next_runs (a fired instant returns True; the minute
    #    after a once-daily run returns False).
    def c_should_run_consistency():
        # Same timezone for the run and the check: 14:30 New-York-local fires; the
        # returned instant is in UTC, and should_run must agree when told the same tz.
        tzname = "America/New_York"
        runs = pub.next_runs("30 14 * * *", "2026-07-01T00:00:00Z", 1, tzname)
        if not runs:
            return False, "next_runs returned nothing"
        inst = _to_instant(runs[0])  # aware UTC instant of the fired run
        on = pub.should_run("30 14 * * *", inst.strftime("%Y-%m-%dT%H:%M:%SZ"), tzname)
        off = pub.should_run("30 14 * * *", (inst + timedelta(minutes=1)).strftime("%Y-%m-%dT%H:%M:%SZ"), tzname)
        return (bool(on) and not bool(off)), f"on={on} off={off} at={inst.isoformat()}"

    check("should_run_consistency", "should_run is True at a fired instant, False one minute later", c_should_run_consistency)

    # 10. deterministic output — identical inputs give identical results.
    def c_determinism():
        args = ("*/15 9-17 * * 1-5", "2026-01-05T00:00:00Z", 10, "America/New_York")
        r1 = pub.next_runs(*args)
        r2 = pub.next_runs(*args)
        return (r1 == r2), "stable" if r1 == r2 else f"differs: {r1!r} vs {r2!r}"

    check("determinism", "next_runs is deterministic across repeated calls", c_determinism)

    # 11. count is honored and order is strictly ascending.
    def c_count_and_order():
        r = pub.next_runs("*/5 * * * *", "2026-02-10T00:00:00Z", 7, "UTC")
        if len(r) != 7:
            return False, f"expected 7, got {len(r)}"
        inst = [_to_instant(x) for x in r]
        ascending = all(inst[i] < inst[i + 1] for i in range(len(inst) - 1))
        return ascending, f"ascending={ascending} n={len(r)}"

    check("count_and_order", "next_runs honors count and returns strictly ascending instants", c_count_and_order)


# --- CLI: grade the documented contract, not the file layout ------------------
def run_cli(args, timeout=60):
    proc = subprocess.run(
        [sys.executable, "-m", "cronsim", *args],
        capture_output=True, text=True, timeout=timeout, cwd=ROOT,
    )
    return proc


def c_cli_next():
    # The brief's documented invocation. ASSUMES `next` emits the UTC ISO instants on
    # stdout (one per line, or as a JSON array) — we extract every UTC-ISO token and
    # compare the SET/ORDER to the grader's expected instants, tolerant of framing.
    proc = run_cli([
        "next", "*/15 9-17 * * 1-5",
        "--start", "2026-01-05T00:00:00Z", "--count", "10",
        "--timezone", "America/New_York",
    ])
    if proc.returncode != 0:
        return False, f"rc={proc.returncode} stderr={proc.stderr[:200]!r}"
    import re
    toks = re.findall(r"\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(?::\d{2})?(?:Z|\+00:?00)?", proc.stdout)
    exp = _expected_next("*/15 9-17 * * 1-5", "2026-01-05T00:00:00Z", 10, "America/New_York")
    if len(toks) < len(exp):
        return False, f"found {len(toks)} timestamps, expected {len(exp)}; stdout={proc.stdout[:200]!r}"
    try:
        got = [_to_instant(t) for t in toks[:len(exp)]]
    except Exception as ex:  # noqa: BLE001
        return False, f"unparseable CLI token: {ex}"
    return (got == exp), f"got[0]={got[0].isoformat()} exp[0]={exp[0].isoformat()}"


check("cli_next", "`python -m cronsim next ...` emits the correct UTC instants", c_cli_next)


def c_cli_invalid():
    # A malformed expression must NOT exit 0 (the CLI surfaces the error).
    proc = run_cli(["next", "not a valid expr x", "--start", "2026-01-01T00:00:00Z", "--count", "3"])
    return (proc.returncode != 0), f"rc={proc.returncode}"


check("cli_invalid", "the CLI exits nonzero on an invalid expression", c_cli_invalid)


passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "cronsim",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
