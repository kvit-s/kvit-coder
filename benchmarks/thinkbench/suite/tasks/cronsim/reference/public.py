"""Reference cronsim.public — a five-field cron parser and simulator.

Semantics (standard Vixie/POSIX cron):
  - Five fields: minute hour day_of_month month day_of_week.
  - minute 0-59, hour 0-23, dom 1-31, month 1-12, dow 0-7 (0 and 7 both Sunday).
  - "*", comma lists, "a-b" ranges, and "*/n" / "a-b/n" steps.
  - day_of_month / day_of_week OR-combine: when BOTH are restricted (neither is
    "*"), a calendar day matches if EITHER field matches. When one is "*", the
    other alone restricts (the "*" side imposes no constraint).
  - Schedules are evaluated in the supplied timezone (zoneinfo); emitted instants
    are ISO-8601 strings in UTC (suffix "Z").
"""
from datetime import datetime, timedelta
from datetime import timezone as _tz
from zoneinfo import ZoneInfo

# (min, max) inclusive bounds per field, in field order.
_BOUNDS = [(0, 59), (0, 23), (1, 31), (1, 12), (0, 7)]
_NAMES = ["minute", "hour", "day_of_month", "month", "day_of_week"]


def _parse_field(spec, lo, hi):
    """Expand one cron field into a sorted set of allowed integers."""
    allowed = set()
    for part in spec.split(","):
        part = part.strip()
        if part == "":
            raise ValueError(f"empty field component in {spec!r}")
        step = 1
        if "/" in part:
            base, _, step_s = part.partition("/")
            if step_s == "" or not step_s.isdigit() or int(step_s) == 0:
                raise ValueError(f"bad step in {part!r}")
            step = int(step_s)
        else:
            base = part

        if base == "*":
            start, end = lo, hi
        elif "-" in base:
            a, _, b = base.partition("-")
            if not (a.isdigit() and b.isdigit()):
                raise ValueError(f"bad range in {part!r}")
            start, end = int(a), int(b)
        else:
            if not base.isdigit():
                raise ValueError(f"bad value in {part!r}")
            start = end = int(base)
            if step != 1:
                # "n/step" means n, n+step, ... up to hi (single-anchored range).
                end = hi

        if start > end:
            raise ValueError(f"descending range in {part!r}")
        if start < lo or end > hi:
            raise ValueError(f"value out of bounds [{lo},{hi}] in {part!r}")
        for v in range(start, end + 1, step):
            allowed.add(v)
    if not allowed:
        raise ValueError(f"no values matched for {spec!r}")
    return allowed


def parse_cron(expr):
    """Parse a five-field cron expression into a structured schedule dict."""
    if not isinstance(expr, str):
        raise ValueError("cron expression must be a string")
    fields = expr.split()
    if len(fields) != 5:
        raise ValueError(f"expected 5 fields, got {len(fields)}: {expr!r}")

    minute = _parse_field(fields[0], *_BOUNDS[0])
    hour = _parse_field(fields[1], *_BOUNDS[1])
    dom = _parse_field(fields[2], *_BOUNDS[2])
    month = _parse_field(fields[3], *_BOUNDS[3])
    dow_raw = _parse_field(fields[4], *_BOUNDS[4])

    # Normalise day-of-week so 7 == 0 == Sunday.
    dow = {0 if d == 7 else d for d in dow_raw}

    return {
        "minute": sorted(minute),
        "hour": sorted(hour),
        "day_of_month": sorted(dom),
        "month": sorted(month),
        "day_of_week": sorted(dow),
        # Whether each day field is restricted (not full "*"); drives OR-semantics.
        "dom_restricted": fields[2].strip() != "*",
        "dow_restricted": fields[4].strip() != "*",
        "expr": expr,
    }


def _day_matches(sched, dt):
    """Does this calendar day satisfy the dom/dow fields (with OR semantics)?"""
    # Python weekday(): Mon=0..Sun=6. Cron dow: Sun=0..Sat=6. Convert.
    cron_dow = (dt.weekday() + 1) % 7
    dom_ok = dt.day in sched["day_of_month"]
    dow_ok = cron_dow in sched["day_of_week"]
    if sched["dom_restricted"] and sched["dow_restricted"]:
        return dom_ok or dow_ok
    if sched["dom_restricted"]:
        return dom_ok
    if sched["dow_restricted"]:
        return dow_ok
    return True  # both "*"


def _matches(sched, dt):
    """Does a fully-specified local datetime (minute resolution) fire?"""
    return (
        dt.minute in sched["minute"]
        and dt.hour in sched["hour"]
        and dt.month in sched["month"]
        and _day_matches(sched, dt)
    )


def _parse_iso(s):
    """Parse an ISO-8601 instant; accept a trailing 'Z' as UTC."""
    if s.endswith("Z"):
        s = s[:-1] + "+00:00"
    return datetime.fromisoformat(s)


def _to_utc_iso(dt):
    """Render an aware datetime as a UTC ISO-8601 string with a 'Z' suffix."""
    return dt.astimezone(_tz.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def next_runs(expr, start_iso, count, timezone="UTC"):
    """Return the next `count` fire times at or after `start_iso`, as UTC ISO strings.

    The schedule is interpreted in `timezone` (a zoneinfo key). `start_iso` may carry
    its own offset/Z; if naive it is taken to be in `timezone`. Each result is the
    UTC ISO rendering of a local wall-clock minute that satisfies the expression.
    """
    sched = parse_cron(expr)
    tz = ZoneInfo(timezone)
    start = _parse_iso(start_iso)
    if start.tzinfo is None:
        start = start.replace(tzinfo=tz)
    start_utc = start.astimezone(_tz.utc)

    # Walk local wall-clock minutes from the start, rounding up to a whole minute.
    local = start.astimezone(tz).replace(second=0, microsecond=0)

    results = []
    # Bound the walk so a pathological/empty schedule can't loop forever (~8 years).
    max_minutes = 8 * 366 * 24 * 60
    steps = 0
    while len(results) < count and steps < max_minutes:
        # Re-anchor the wall time in tz each tick so its UTC offset is resolved with
        # the correct DST rule for that calendar day (offsets shift across DST, e.g.
        # noon EST=17:00Z before spring-forward, noon EDT=16:00Z after).
        wall = datetime(
            local.year, local.month, local.day, local.hour, local.minute,
            tzinfo=tz, fold=0,
        )
        # Spring-forward gap times (e.g. 02:30 on a US spring day) keep their nominal
        # wall hour under zoneinfo and resolve via the pre-transition offset (fold=0).
        if wall.astimezone(_tz.utc) >= start_utc and _matches(sched, local):
            results.append(_to_utc_iso(wall))
        local += timedelta(minutes=1)
        steps += 1
    return results


def should_run(expr, instant_iso, timezone="UTC"):
    """True iff `instant_iso` (to the minute) satisfies `expr` in `timezone`."""
    sched = parse_cron(expr)
    tz = ZoneInfo(timezone)
    inst = _parse_iso(instant_iso)
    if inst.tzinfo is None:
        inst = inst.replace(tzinfo=tz)
    local = inst.astimezone(tz)
    return _matches(sched, local)
