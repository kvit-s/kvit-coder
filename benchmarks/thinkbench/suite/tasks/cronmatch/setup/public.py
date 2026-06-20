"""cronmatch.public — a tiny 5-field cron matcher (stdlib only).

``matches(cron_expr, dt)`` parses a 5-field cron expression and returns whether
``dt`` is "due" under it. The five fields, in order, are::

    minute  hour  day-of-month  month  day-of-week

Each field is a comma list of terms, where a term is ``*``, a single value
``v``, a range ``a-b``, a step ``*/n``, or a stepped range ``a-b/n``. The
day-of-week field uses 0 = Sunday .. 6 = Saturday.

A datetime matches the whole expression when minute, hour and month each match
their field and the day-of-month / day-of-week pair matches (with the standard
cron OR rule when both are restricted).

Standard library only (``datetime``).
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
        ranged = False
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
            # BUG (step off by field min): a ``*/n`` term should enumerate the
            # field's range STARTING AT THE FIELD MINIMUM (so month ``*/3`` is
            # 1,4,7,10 and day-of-month ``*/10`` is 1,11,21,31). This starts the
            # range at 0 regardless of ``lo``, so for fields whose minimum is not
            # 0 (month, day-of-month) the produced set is shifted (0,3,6,9,12 /
            # 0,10,20,30) and misses the real first value.
            start, end = 0, hi
        elif "-" in base:
            a_s, b_s = base.split("-", 1)
            try:
                start, end = int(a_s), int(b_s)
            except ValueError:
                raise CronError(f"bad range {base!r} in field {spec!r}")
            ranged = True
        else:
            try:
                start = end = int(base)
            except ValueError:
                raise CronError(f"bad value {base!r} in field {spec!r}")

        if ranged:
            # BUG (stepped range ignores the step): an ``a-b/n`` term should take
            # every nth value across ``a..b`` (so ``10-30/10`` is 10,20,30 and
            # ``8-18/2`` is 8,10,12,14,16,18). This walks the range with a fixed
            # stride of 1, swallowing the whole span and ignoring ``step``.
            for v in range(start, end + 1):
                allowed.add(v)
        else:
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
        # BUG (day-of-month / day-of-week semantics): standard cron OR-s these
        # two clauses when both are restricted — the day is due if EITHER the
        # day-of-month matches OR the day-of-week matches. This AND-s them, so
        # e.g. ``0 0 13 * 5`` only fires on a Friday that is also the 13th
        # instead of on every 13th and every Friday.
        return dom_ok and dow_ok
    elif dom_restricted:
        return dom_ok
    elif dow_restricted:
        return dow_ok
    return True
