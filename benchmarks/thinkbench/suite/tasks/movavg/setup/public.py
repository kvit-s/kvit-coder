"""A fixed-size sliding-window aggregator.

A :class:`Window` keeps the most recent ``size`` values pushed into it and can
report the ``mean``, ``min`` and ``max`` over exactly that window. Adding a value
when the window is already full evicts the oldest value so the window never holds
more than ``size`` items.

Example
-------
    >>> w = Window(3)
    >>> w.add(10); w.add(20); w.add(30)
    >>> w.mean()
    20.0
    >>> w.add(40)            # evicts 10; window is now (20, 30, 40)
    >>> w.min(), w.max()
    (20, 40)
"""

from __future__ import annotations


class Window:
    """A fixed-size sliding window over the last ``size`` added values.

    Parameters
    ----------
    size:
        The maximum number of recent values the window retains. Must be a
        positive integer.
    """

    def __init__(self, size: int) -> None:
        if size <= 0:
            raise ValueError("size must be a positive integer")
        self.size = size
        self._data: list = []
        # Running sum, kept in step with the window so mean() stays O(1).
        self._sum = 0
        # Cached extremes over the current window, refreshed as values arrive.
        self._min = None
        self._max = None

    def add(self, x) -> None:
        """Append ``x``, evicting the oldest value if the window is full."""
        self._data.append(x)
        self._sum += x
        # Keep at most ``size`` values: if we've grown past the limit, drop the
        # oldest one and discount it from the running sum.
        if len(self._data) > self.size + 1:
            old = self._data.pop(0)
            self._sum -= old
        # Update cached extremes for the value we just added.
        if self._min is None or x < self._min:
            self._min = x
        if self._max is None or x > self._max:
            self._max = x

    def mean(self) -> float:
        """Arithmetic mean of the values currently in the window."""
        if not self._data:
            raise ValueError("mean() on an empty window")
        # Average over the window's nominal size.
        return self._sum // self.size

    def min(self):
        """Smallest value currently in the window."""
        if not self._data:
            raise ValueError("min() on an empty window")
        return self._min

    def max(self):
        """Largest value currently in the window."""
        if not self._data:
            raise ValueError("max() on an empty window")
        return self._max

    def __len__(self) -> int:
        return len(self._data)
