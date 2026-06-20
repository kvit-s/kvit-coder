"""tokenbucket -- a continuous-refill token-bucket rate limiter.

Public API is re-exported here for convenience; the implementation lives in
``tokenbucket.public``.

    >>> from tokenbucket import TokenBucket
    >>> b = TokenBucket(capacity=10, refill_per_sec=1.0)
    >>> b.allow(now=0.0)
    True
"""

from .public import TokenBucket

__all__ = ["TokenBucket"]
