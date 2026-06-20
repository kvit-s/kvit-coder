"""datespan.public — business-day date math (stdlib only).

Business days are Monday through Friday. There is NO holiday calendar: only
Saturday and Sunday are skipped.

``add_business_days(start, n)``
    Return the date that is ``n`` business days from ``start``.

    * ``start`` is first NORMALIZED onto a business day: if it falls on a
      weekend it is moved FORWARD to the following Monday. The normalized date
      is the anchor for the whole computation, including ``n == 0`` and ``n <
      0``.
    * With ``n == 0`` the (normalized) anchor itself is returned.
    * With ``n > 0`` the result is the anchor advanced by ``n`` business days,
      skipping weekends.
    * With ``n < 0`` the result is the anchor moved BACKWARD by ``abs(n)``
      business days, again skipping weekends (so going back across a weekend
      lands on the previous Friday, not on a Saturday/Sunday).

``business_days_between(a, b)``
    Return the number of business days in the half-open span ``(a, b]`` —
    EXCLUSIVE of ``a``, INCLUSIVE of ``b``. Equivalently: how many business-day
    steps it takes to walk from ``a`` to ``b``.

    * ``a == b`` is ``0``.
    * If ``a`` is after ``b`` the result is NEGATIVE (the negation of the count
      from ``b`` to ``a``), so ``business_days_between(a, b) ==
      -business_days_between(b, a)``.
    * Weekends never count: a Friday-to-Saturday span is ``0``.

Standard library only (``datetime``). Inputs are :class:`datetime.date`.
"""

from __future__ import annotations

from datetime import date, timedelta


def _is_weekend(d: date) -> bool:
    """True for Saturday (5) or Sunday (6)."""
    return d.weekday() >= 5


def _normalize_forward(d: date) -> date:
    """Snap ``d`` forward to the next business day (a no-op on Mon–Fri)."""
    while _is_weekend(d):
        d += timedelta(days=1)
    return d


def add_business_days(start: date, n: int) -> date:
    """Return ``start`` advanced by ``n`` business days (Mon–Fri).

    ``start`` is normalized forward onto a business day first; ``n`` may be
    zero, positive, or negative.
    """
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


def business_days_between(a: date, b: date) -> int:
    """Return the count of business days in ``(a, b]`` (negative if ``a > b``)."""
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
