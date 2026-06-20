"""cronmatch.public — a tiny 5-field cron matcher (stdlib only).

``matches(cron_expr, dt)`` parses a 5-field cron expression and returns whether
``dt`` is "due" under it. The five fields, in order, are::

    minute  hour  day-of-month  month  day-of-week

Each field is a comma list of terms, where a term is ``*``, a single value
``v``, a range ``a-b``, a step ``*/n``, or a stepped range ``a-b/n``. ``*/n``
enumerates the field's full range starting at the field minimum (month ``*/3`` =
1,4,7,10), and ``a-b/n`` takes every nth value across ``a..b`` (``10-30/10`` =
10,20,30). The day-of-week field uses 0 = Sunday .. 6 = Saturday.

A datetime matches the whole expression when minute, hour and month each match
their field and the day-of-month / day-of-week pair matches. When BOTH of those
two fields are restricted, standard cron OR-s them: the day is due if either the
day-of-month clause or the day-of-week clause matches.

Standard library only (``datetime``).

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from __future__ import annotations

from datetime import datetime
from typing import List, Set, Tuple

# (name, low, high) for the five fields, in cron order.
_FIELDS: List[Tuple[str, int, int]] = [
    ("minute", 0, 59),
    ("hour", 0, 23),
    ("day-of-month", 1, 31),
    ("month", 1, 12),
    ("day-of-week", 0, 6),
]


class CronError(ValueError):
    """Raised when a cron expression is malformed or unsupported."""


def _parse_field(spec: str, lo: int, hi: int) -> Set[int]:
    """Expand one cron field into the set of integer values it allows.

    A field is a comma list of terms; the field matches a value if ANY term
    matches it, so the result is the union over the terms.

    Each term is one of ``*``, ``v``, ``a-b``, ``*/n`` or ``a-b/n``.
    """
    allowed: Set[int] = set()
    for term in spec.split(","):
        term = term.strip()
        if not term:
            raise CronError(f"empty term in field {spec!r}")

        step = 1
        if "/" in term:
            base, step_s = term.split("/", 1)
            try:
                step = int(step_s)
            except ValueError:
                raise CronError(f"bad step {step_s!r} in field {spec!r}")
            if step < 1:
                raise CronError(f"step must be >= 1, got {step}")
        else:
            base = term

        if base == "*":
            # ``*`` / ``*/n`` spans the field's full range, starting at the field
            # MINIMUM (so month ``*/3`` = 1,4,7,10 and day-of-month ``*/10`` =
            # 1,11,21,31), not at 0.
            start, end = lo, hi
        elif "-" in base:
            a_s, b_s = base.split("-", 1)
            try:
                start, end = int(a_s), int(b_s)
            except ValueError:
                raise CronError(f"bad range {base!r} in field {spec!r}")
        else:
            try:
                start = end = int(base)
            except ValueError:
                raise CronError(f"bad value {base!r} in field {spec!r}")

        # ``step`` applies uniformly: a bare value steps over a single element, a
        # range/``*`` steps every nth value across ``start..end`` inclusive.
        for v in range(start, end + 1, step):
            allowed.add(v)

    return allowed


def matches(cron_expr: str, dt: datetime) -> bool:
    """Return True iff ``dt`` is due under the 5-field cron expression
    ``cron_expr``."""
    if not isinstance(cron_expr, str):
        raise CronError(f"cron_expr must be a str, got {type(cron_expr).__name__}")
    if not isinstance(dt, datetime):
        raise CronError(f"dt must be a datetime, got {type(dt).__name__}")

    parts = cron_expr.split()
    if len(parts) != 5:
        raise CronError(f"cron expression must have 5 fields, got {len(parts)}")

    minute_set = _parse_field(parts[0], *_FIELDS[0][1:])
    hour_set = _parse_field(parts[1], *_FIELDS[1][1:])
    dom_set = _parse_field(parts[2], *_FIELDS[2][1:])
    month_set = _parse_field(parts[3], *_FIELDS[3][1:])
    dow_set = _parse_field(parts[4], *_FIELDS[4][1:])

    # Python's weekday(): Mon=0 .. Sun=6. Cron's day-of-week: Sun=0 .. Sat=6.
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
        # Standard cron OR: when both the day-of-month and day-of-week fields are
        # restricted, the day is due if EITHER clause matches.
        return dom_ok or dow_ok
    elif dom_restricted:
        return dom_ok
    elif dow_restricted:
        return dow_ok
    return True
