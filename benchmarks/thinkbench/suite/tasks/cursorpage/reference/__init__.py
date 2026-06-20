"""cursorpage — a small paginator over a list sorted by a key.

Public API is re-exported here for convenience; the implementation lives in
``cursorpage.public``.

    >>> from cursorpage import Paginator
    >>> p = Paginator([{"id": 1, "score": 10}], key="score")
    >>> p.page(0, 10)
    [{'id': 1, 'score': 10}]
"""

from .public import Paginator

__all__ = ["Paginator"]
