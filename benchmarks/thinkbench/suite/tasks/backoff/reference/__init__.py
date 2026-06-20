"""backoff -- exponential-backoff delay schedule with full-jitter bounds.

Public API is re-exported here for convenience; the implementation lives in
``backoff.public``.

    >>> from backoff import Backoff
    >>> b = Backoff(base=0.5, factor=2.0, cap=10.0)
    >>> b.delay(0)
    0.5
"""

from .public import Backoff

__all__ = ["Backoff"]
