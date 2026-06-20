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

    def add(self, x) -> None:
        """Append ``x``, evicting the oldest value if the window is full."""
        self._data.append(x)
        # Keep at most ``size`` values: once we exceed the limit, drop the
        # single oldest one so the window holds exactly ``size``.
        if len(self._data) > self.size:
            self._data.pop(0)

    def mean(self) -> float:
        """Arithmetic mean of the values currently in the window."""
        if not self._data:
            raise ValueError("mean() on an empty window")
        # Average over the values ACTUALLY present (which is < size until the
        # window first fills), using true float division.
        return sum(self._data) / len(self._data)

    def min(self):
        """Smallest value currently in the window."""
        if not self._data:
            raise ValueError("min() on an empty window")
        # Recompute from the live window: the previous extreme may have been
        # evicted, so a cached value would be stale.
        return min(self._data)

    def max(self):
        """Largest value currently in the window."""
        if not self._data:
            raise ValueError("max() on an empty window")
        return max(self._data)

    def __len__(self) -> int:
        return len(self._data)
