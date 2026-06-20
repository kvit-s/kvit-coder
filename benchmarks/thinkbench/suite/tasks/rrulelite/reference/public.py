"""rrulelite.public — a small recurrence-rule expander (stdlib only).

``expand(rule, start, limit)`` generates successive dates from a recurrence rule:

* ``rule["freq"]`` is one of ``"daily"``, ``"weekly"``, ``"monthly"``.
* ``rule["interval"]`` is a positive integer step (default 1): every Nth day /
  week / month. ``interval == 2`` with ``freq == "monthly"`` means every other
  month.
* ``rule["until"]`` (optional) is an INCLUSIVE upper bound: a generated date
  equal to ``until`` is kept; the first date strictly after ``until`` stops the
  expansion.

The expansion always starts AT ``start`` (the first emitted date is ``start``
itself, provided it does not already exceed ``until``) and yields at most
``limit`` dates.

Month stepping clamps to the end of the target month: stepping one month from
Jan 31 lands on Feb 28 (or Feb 29 in a leap year), not on an invalid Feb 31 and
not on a rolled-over Mar 2/Mar 3.

Standard library only (``datetime``).
"""

from __future__ import annotations

import calendar
from datetime import date
from typing import List


class RRuleError(ValueError):
    """Raised when a recurrence rule is malformed or unsupported."""


_FREQS = {"daily", "weekly", "monthly"}


def _add_months(d: date, months: int) -> date:
    """Return ``d`` advanced by ``months`` whole months, clamping the day to the
    last valid day of the target month (Jan 31 + 1 month -> Feb 28/29)."""
    # 0-based month index makes the carry arithmetic clean.
    total = (d.year * 12 + (d.month - 1)) + months
    year, month0 = divmod(total, 12)
    month = month0 + 1
    last_day = calendar.monthrange(year, month)[1]
    day = min(d.day, last_day)
    return date(year, month, day)


def expand(rule: dict, start: date, limit: int) -> List[date]:
    """Expand a recurrence ``rule`` from ``start``, returning up to ``limit``
    dates, stopping at an inclusive ``until`` bound if present."""
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
            from datetime import timedelta

            current = start + timedelta(days=interval * n)
        elif freq == "weekly":
            from datetime import timedelta

            current = start + timedelta(weeks=interval * n)
        else:  # monthly
            current = _add_months(start, interval * n)

        # until is INCLUSIVE: keep a date equal to until, stop strictly past it.
        if until is not None and current > until:
            break

        out.append(current)
        n += 1

    return out
