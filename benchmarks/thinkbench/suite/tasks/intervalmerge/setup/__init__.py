"""intervalmerge -- half-open interval algebra (merge / subtract).

Public API is re-exported here for convenience; the implementation lives in
``intervalmerge.public``.

    >>> from intervalmerge import merge, subtract
    >>> merge([(1, 3), (2, 4)])
    [(1, 4)]
    >>> subtract([(0, 10)], [(3, 5)])
    [(0, 3), (5, 10)]
"""

from .public import merge, subtract

__all__ = ["merge", "subtract"]
