"""movavg -- a fixed-size sliding-window aggregator.

Public API is re-exported here for convenience; the implementation lives in
``movavg.public``.

    >>> from movavg import Window
    >>> w = Window(3)
    >>> w.add(1); w.add(2); w.add(3)
    >>> w.mean()
    2.0
"""

from .public import Window

__all__ = ["Window"]
