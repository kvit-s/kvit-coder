"""A fixed-window rate limiter with a single GLOBAL limit.

The limiter admits at most ``limit`` requests in each fixed wall-clock window of
``window`` seconds. The window that contains a timestamp ``now`` is the
half-open interval ``[w0, w0 + window)`` where
``w0 = floor(now / window) * window``. When a new window begins the count starts
over at zero.

Time is supplied by the caller as an absolute, non-decreasing ``now`` (seconds,
a float), so behaviour is deterministic and needs no real clock.

This first version tracks ONE global counter — every call to :meth:`allow`
shares the same budget. There is no notion of a caller "key" and no per-caller
tiers yet; adding those is the task (see ``brief.txt``).

Example
-------
    >>> r = RateLimiter(limit=2, window=10.0)
    >>> r.allow(0.0)      # 1st in window [0,10)
    True
    >>> r.allow(3.0)      # 2nd in window [0,10)
    True
    >>> r.allow(5.0)      # 3rd — over the limit
    False
    >>> r.allow(10.0)     # new window [10,20) — count starts over
    True
"""

from __future__ import annotations

import math


class RateLimiter:
    """Fixed-window limiter with a single global budget.

    Parameters
    ----------
    limit:
        Maximum number of requests admitted per window (a positive integer).
    window:
        Window length in seconds (a positive float).
    """

    def __init__(self, limit: int, window: float) -> None:
        self.limit = limit
        self.window = window
        # The window currently being counted, and how many requests it has seen.
        self._window_start: float | None = None
        self._count = 0

    def _window_for(self, now: float) -> float:
        """Return the start of the fixed window that contains ``now``."""
        return math.floor(now / self.window) * self.window

    def allow(self, now: float) -> bool:
        """Admit a request at time ``now`` against the single global budget.

        Returns ``True`` if the request fits inside the current window's
        remaining budget (and counts it), ``False`` otherwise (and counts
        nothing).
        """
        w0 = self._window_for(now)
        if self._window_start is None or w0 != self._window_start:
            # A new window has begun: reset the counter.
            self._window_start = w0
            self._count = 0
        if self._count < self.limit:
            self._count += 1
            return True
        return False
