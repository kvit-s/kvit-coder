"""Exponential-backoff delay schedule with optional "full jitter".

A :class:`Backoff` turns a zero-based retry *attempt* number into the delay (in
seconds) to wait before that attempt. The un-jittered delay grows
*exponentially* with the attempt number and is clamped to a ceiling::

    delay(attempt) = min(cap, base * factor ** attempt)

so attempt 0 waits ``base`` seconds, attempt 1 waits ``base * factor``, and so
on, until the schedule flattens out at ``cap``.

On top of that, AWS-style "full jitter" randomises each wait uniformly over the
whole interval ``[0, delay(attempt)]`` -- the actual sleep is a random draw in
that range. To keep the schedule deterministic and unit-testable, this class
does NOT draw the random number itself: it exposes the *un-jittered* delay via
:meth:`delay` and the inclusive jitter *bounds* via :meth:`bounds`. A caller
that wants a concrete sleep draws ``random.uniform(low, high)`` itself.

Time is in seconds, as a float. ``attempt`` is a zero-based non-negative integer.

Example
-------
    >>> b = Backoff(base=0.5, factor=2.0, cap=10.0)
    >>> b.delay(0)        # base
    0.5
    >>> b.delay(1)        # base * factor
    1.0
    >>> b.delay(5)        # 0.5 * 32 = 16 -> clamped to cap
    10.0
    >>> b.bounds(1)       # full jitter over [0, delay(1)]
    (0.0, 1.0)
"""

from __future__ import annotations


class Backoff:
    """Exponential backoff with a ceiling and full-jitter bounds.

    Parameters
    ----------
    base:
        Delay for attempt 0, in seconds (the exponential's scale). A float.
    factor:
        Multiplicative growth per attempt (e.g. ``2.0`` doubles each time).
    cap:
        Ceiling, in seconds: the un-jittered delay is never larger than this.
    """

    def __init__(self, base: float, factor: float, cap: float) -> None:
        self.base = float(base)
        self.factor = float(factor)
        self.cap = float(cap)

    def delay(self, attempt: int) -> float:
        """Un-jittered delay (seconds) before the zero-based ``attempt``.

        ``min(cap, base * factor ** attempt)``: attempt 0 is ``base``; the
        exponential is computed in full and only THEN clamped to ``cap``. The
        result is always a float.
        """
        raw = self.base * (self.factor ** attempt)
        return raw if raw < self.cap else self.cap

    def bounds(self, attempt: int) -> tuple[float, float]:
        """Inclusive full-jitter bounds ``(low, high)`` for ``attempt``.

        Full jitter spreads the wait uniformly over the whole interval from 0
        up to the (already capped) un-jittered delay, so ``low`` is always
        ``0.0`` and ``high`` is :meth:`delay`. Both are floats. Because ``high``
        is the capped delay, the jitter window itself never exceeds ``cap``.
        """
        return (0.0, self.delay(attempt))
