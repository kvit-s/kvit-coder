"""tierlimit — a fixed-window rate limiter with per-key budgets and tiers.

Public API is re-exported here for convenience; the implementation lives in
``tierlimit.public``.

    >>> from tierlimit import RateLimiter
    >>> r = RateLimiter(limit=2, window=10.0)
    >>> r.allow(0.0)
    True
"""

from .public import RateLimiter

__all__ = ["RateLimiter"]
