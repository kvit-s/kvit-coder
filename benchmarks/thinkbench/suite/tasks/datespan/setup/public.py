"""datespan.public — business-day date math (stdlib only).

Business days are Monday through Friday. There is NO holiday calendar: only
Saturday and Sunday are skipped.

``add_business_days(start, n)`` is supposed to return the date ``n`` business
days from ``start`` (normalizing a weekend ``start`` forward first, and
honoring negative ``n`` by walking backward over weekends).

``business_days_between(a, b)`` is supposed to return the number of business
days strictly after ``a`` up to and including ``b``, negative when ``a`` is
after ``b``.

Standard library only (``datetime``). Inputs are :class:`datetime.date`.
"""

from __future__ import annotations

from datetime import date, timedelta


def _is_weekend(d: date) -> bool:
    """True for Saturday (5) or Sunday (6)."""
    return d.weekday() >= 5


def add_business_days(start: date, n: int) -> date:
    """Return ``start`` advanced by ``n`` business days (Mon–Fri)."""
    # BUG 2 (no weekend normalization): a `start` that falls on a Saturday or
    # Sunday should be snapped FORWARD to the next business day before counting.
    # Here `start` is used as-is, so a weekend start anchors the whole result on
    # a non-business day (and `n == 0` returns the weekend date unchanged).
    cur = start
    if n == 0:
        return cur

    step = 1 if n > 0 else -1
    remaining = abs(n)
    while remaining > 0:
        cur += timedelta(days=step)
        # BUG 1 (wrong direction over weekends): when the cursor lands on a
        # weekend the skip ALWAYS nudges forward (+1 day), regardless of the
        # step direction. For positive `n` that happens to be correct, but for
        # negative `n` it walks the wrong way — instead of landing on the
        # previous Friday it bounces forward back onto/over the weekend.
        while _is_weekend(cur):
            cur += timedelta(days=1)
        remaining -= 1
    return cur


def business_days_between(a: date, b: date) -> int:
    """Return the count of business days between ``a`` and ``b``."""
    if a == b:
        return 0

    # The forward count below is correct (exclusive of the low endpoint,
    # inclusive of the high endpoint), so a simple Mon→Fri span reads right.
    lo, hi = (a, b) if a <= b else (b, a)
    count = 0
    cur = lo
    while cur < hi:
        cur += timedelta(days=1)
        if not _is_weekend(cur):
            count += 1
    # BUG 3 (sign dropped when a > b): the magnitude is computed on the ordered
    # span but the result is never negated, so `business_days_between(a, b)`
    # with `a` AFTER `b` returns a positive count instead of a negative one.
    return count
