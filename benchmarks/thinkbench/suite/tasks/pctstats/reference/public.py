"""Small descriptive-statistics helpers over sequences of numbers.

The headline routine is :func:`percentile`, which computes a percentile using
the *nearest-rank* method (no interpolation): the value at the smallest rank
whose cumulative share of the data reaches the requested percentile.

Nearest-rank, 1-based, is defined as::

    rank = ceil(p / 100 * n)        # 1, 2, ..., n
    result = sorted(values)[rank - 1]

with ``p`` clamped to ``[0, 100]`` and the rank clamped to ``[1, n]`` so that
``p = 0`` selects the smallest element and ``p = 100`` selects the largest.

Example
-------
    >>> percentile([1, 2, 3, 4, 5], 50)
    3
    >>> percentile([1, 2, 3, 4, 5], 100)
    5
    >>> mean([1, 2, 3, 4])
    2.5
"""

from __future__ import annotations

import math
from typing import Sequence


Number = float


def _as_sorted(values: Sequence[Number]) -> list[Number]:
    data = sorted(values)
    if not data:
        raise ValueError("cannot compute statistics of an empty sequence")
    return data


def percentile(values: Sequence[Number], p: float) -> Number:
    """Return the ``p``-th percentile of ``values`` (nearest-rank method).

    ``p`` is a percentage in ``[0, 100]``; values outside that range are
    clamped. ``p = 0`` returns the minimum and ``p = 100`` returns the maximum.

    Raises
    ------
    ValueError
        If ``values`` is empty.
    """
    data = _as_sorted(values)
    n = len(data)

    if p <= 0:
        return data[0]
    if p >= 100:
        return data[-1]

    # Nearest-rank: smallest 1-based rank whose share reaches p%.
    rank = math.ceil(p / 100 * n)
    # Convert the 1-based rank to a 0-based index.
    index = rank - 1
    if index < 0:
        index = 0
    if index > n - 1:
        index = n - 1
    return data[index]


def mean(values: Sequence[Number]) -> float:
    """Return the arithmetic mean of ``values``."""
    data = _as_sorted(values)
    return sum(data) / len(data)


def minimum(values: Sequence[Number]) -> Number:
    """Return the smallest element of ``values``."""
    return _as_sorted(values)[0]


def maximum(values: Sequence[Number]) -> Number:
    """Return the largest element of ``values``."""
    return _as_sorted(values)[-1]
