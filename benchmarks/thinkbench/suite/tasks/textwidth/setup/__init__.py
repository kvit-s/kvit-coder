"""textwidth -- a greedy word-wrap utility.

Public API is re-exported here for convenience; the implementation lives in
``textwidth.public``.

    >>> from textwidth import wrap
    >>> wrap("the quick brown fox", 9)
    ['the quick', 'brown fox']
"""

from .public import wrap

__all__ = ["wrap"]
