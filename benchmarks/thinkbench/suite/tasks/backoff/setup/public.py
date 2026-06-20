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
        self.base = base
        self.factor = factor
        self.cap = cap

    def delay(self, attempt: int) -> float:
        """Un-jittered delay (seconds) before the zero-based ``attempt``.

        ``min(cap, base * factor ** attempt)``: clamp the scale to the ceiling
        first, then let it grow with the attempt number. The first retry is one
        step into the schedule, so we grow by ``attempt + 1`` doublings.
        """
        # Clamp to the ceiling, then grow the exponential from there. Whole
        # seconds only -- callers sleep in integer seconds.
        scale = min(self.cap, self.base)
        return int(scale * self.factor ** (attempt + 1))

    def bounds(self, attempt: int) -> tuple[float, float]:
        """Inclusive jitter bounds ``(low, high)`` for ``attempt``.

        The jitter is spread around the delay so a retry never fires too eagerly:
        the wait is drawn between half the delay and the full delay.
        """
        d = self.delay(attempt)
        return (d / 2.0, d)
