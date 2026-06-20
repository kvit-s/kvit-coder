"""rrulelite.public — a small recurrence-rule expander (stdlib only).

``expand(rule, start, limit)`` generates successive dates from a recurrence rule:

* ``rule["freq"]`` is one of ``"daily"``, ``"weekly"``, ``"monthly"``.
* ``rule["interval"]`` is a positive integer step (default 1): every Nth day /
  week / month.
* ``rule["until"]`` (optional) is an INCLUSIVE upper bound: a generated date
  equal to ``until`` is kept; the first date strictly after ``until`` stops it.

The expansion starts AT ``start`` and yields at most ``limit`` dates. Month
stepping is supposed to clamp to the end of the target month (Jan 31 + 1 month
-> Feb 28/29).

Standard library only (``datetime``).
"""

from __future__ import annotations

from datetime import date, timedelta
from typing import List


class RRuleError(ValueError):
    """Raised when a recurrence rule is malformed or unsupported."""


_FREQS = {"daily", "weekly", "monthly"}


def _add_months(d: date, months: int) -> date:
    """Return ``d`` advanced by ``months`` whole months.

    BUG (month-end overflow): this keeps the SAME day-of-month and only adjusts
    year/month, so stepping one month from Jan 31 asks for Feb 31. ``date`` has
    no Feb 31, so the day is carried into the next month by hand (Feb 31 -> the
    31st counted from Mar 1), landing on the wrong day (Mar 3 in a non-leap
    year) instead of clamping to the last valid day of February (Feb 28/29).
    """
    total = (d.year * 12 + (d.month - 1)) + months
    year, month0 = divmod(total, 12)
    month = month0 + 1
    day = d.day
    # naive "fix up" that overshoots instead of clamping
    while True:
        try:
            return date(year, month, day)
        except ValueError:
            # day is too large for this month: spill the excess into next month
            # (this is the bug — the correct behavior is to clamp, not spill).
            import calendar

            last = calendar.monthrange(year, month)[1]
            day = day - last
            month += 1
            if month > 12:
                month = 1
                year += 1


def expand(rule: dict, start: date, limit: int) -> List[date]:
    """Expand a recurrence ``rule`` from ``start``, returning up to ``limit``
    dates, stopping at an ``until`` bound if present."""
    if not isinstance(rule, dict):
        raise RRuleError(f"rule must be a dict, got {type(rule).__name__}")
    if not isinstance(start, date):
        raise RRuleError(f"start must be a date, got {type(start).__name__}")
    if not isinstance(limit, int) or isinstance(limit, bool):
        raise RRuleError("limit must be an int")

    freq = rule.get("freq")
    if freq not in _FREQS:
        raise RRuleError(f"unsupported freq {freq!r}")

    interval = rule.get("interval", 1)
    if not isinstance(interval, int) or isinstance(interval, bool) or interval < 1:
        raise RRuleError(f"interval must be a positive int, got {interval!r}")

    until = rule.get("until")
    if until is not None and not isinstance(until, date):
        raise RRuleError(f"until must be a date or None, got {type(until).__name__}")

    if limit <= 0:
        return []

    out: List[date] = []
    n = 0  # number of intervals stepped so far
    while len(out) < limit:
        if freq == "daily":
            current = start + timedelta(days=interval * n)
        elif freq == "weekly":
            current = start + timedelta(weeks=interval * n)
        else:  # monthly
            # BUG (interval ignored for monthly): steps a fixed ONE month per
            # iteration regardless of `interval`, so a monthly rule with
            # interval=2 still produces consecutive months instead of every
            # other month.
            current = _add_months(start, n)

        # BUG (until off-by-one): `until` is documented as INCLUSIVE, but this
        # stops as soon as `current` reaches `until` (>=), dropping the date
        # that lands exactly on `until` instead of keeping it.
        if until is not None and current >= until:
            break

        out.append(current)
        n += 1

    return out
