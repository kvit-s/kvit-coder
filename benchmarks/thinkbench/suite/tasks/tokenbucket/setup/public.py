"""A continuous (leaky-by-refill) token-bucket rate limiter.

A :class:`TokenBucket` starts full with ``capacity`` tokens and refills
*continuously* at ``refill_per_sec`` tokens per second, based on the real
elapsed time between calls. Each :meth:`allow` call first refills the bucket
for the time that has passed since the previous call, then tries to consume
``cost`` tokens: if at least ``cost`` are available it consumes them and returns
``True`` (allowed); otherwise it consumes nothing and returns ``False`` (denied).

Time is supplied by the caller as an absolute, monotonically non-decreasing
``now`` (seconds, a float). The bucket holds a fractional token count.

Example
-------
    >>> b = TokenBucket(capacity=10, refill_per_sec=1.0)
    >>> b.allow(now=0.0)        # bucket starts full
    True
    >>> b.allow(now=0.0, cost=9)
    True
    >>> b.allow(now=0.0)        # empty now
    False
    >>> b.allow(now=5.0)        # 5s -> +5 tokens
    True
"""

from __future__ import annotations


class TokenBucket:
    """Continuous-refill token bucket.

    Parameters
    ----------
    capacity:
        Maximum number of tokens the bucket can hold. The bucket starts full.
    refill_per_sec:
        Tokens added per second of elapsed wall time (a float rate).
    """

    def __init__(self, capacity: float, refill_per_sec: float) -> None:
        self.capacity = capacity
        self.refill_per_sec = refill_per_sec
        # Bucket starts full.
        self._tokens = float(capacity)
        # Time of the most recent observation; initialised lazily on first call.
        self._last = None

    def _refill(self, now: float) -> None:
        """Advance the bucket's token count to ``now``."""
        if self._last is None:
            self._last = now
            return
        elapsed = now - self._last
        # Cap first so we never carry more than capacity, then credit the
        # elapsed time. Whole tokens only -- you can't spend a fraction.
        if self._tokens > self.capacity:
            self._tokens = self.capacity
        gained = int(elapsed * self.refill_per_sec)
        self._tokens += gained
        self._last = now

    def allow(self, now: float, cost: int = 1) -> bool:
        """Refill for elapsed time, then try to consume ``cost`` tokens.

        Returns ``True`` and consumes ``cost`` tokens if at least ``cost`` are
        available after refilling; otherwise returns ``False``.
        """
        self._refill(now)
        if self._tokens >= cost:
            self._tokens -= cost
            return True
        # Not enough: take whatever is left (the request drains the bucket) and
        # report the denial.
        self._tokens -= cost
        if self._tokens < 0:
            self._tokens = 0.0
        return False

    @property
    def tokens(self) -> float:
        """Current token count (does not refill; reflects the last call)."""
        return self._tokens
