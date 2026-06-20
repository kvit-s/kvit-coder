"""permgen.public — lexicographic permutations of distinct items (stdlib only).

A permutation of ``items`` (a sequence of DISTINCT items, taken in the given
order as the lexicographic alphabet) can be addressed by its rank: the position
it occupies when all ``len(items)!`` permutations are listed in lexicographic
order, starting at rank 0.

* ``nth_permutation(items, n)`` -> the rank-``n`` permutation as a new list.
  ``nth_permutation(items, 0)`` is always ``list(items)`` (the identity).
* ``permutation_rank(perm, items)`` -> the rank of ``perm``; the exact inverse
  of ``nth_permutation`` so that, for every valid ``n``,
  ``permutation_rank(nth_permutation(items, n), items) == n``.

Both use the FACTORIAL NUMBER SYSTEM (a.k.a. factoradic): the rank decomposes
into digits with place values ``(k-1)!, (k-2)!, ..., 1!, 0!`` where ``k`` is the
number of items. Each digit selects (and removes) an element from the list of
items not yet used, so the digit is an index into the REMAINING items.

Standard library only (``math``).
"""

from __future__ import annotations

from math import factorial
from typing import List, Sequence


class PermError(ValueError):
    """Raised when a permutation request is malformed or out of range."""


def nth_permutation(items: Sequence, n: int) -> List:
    """Return the ``n``-th lexicographic permutation (0-indexed) of ``items``.

    ``items`` must contain distinct elements; ``n`` must satisfy
    ``0 <= n < len(items)!``. ``nth_permutation(items, 0)`` is the identity
    ``list(items)``.
    """
    if not isinstance(n, int) or isinstance(n, bool):
        raise PermError(f"n must be an int, got {type(n).__name__}")
    items = list(items)
    k = len(items)
    if k == 0:
        if n != 0:
            raise PermError(f"n={n} out of range for empty items")
        return []
    total = factorial(k)
    if not (0 <= n < total):
        raise PermError(f"n={n} out of range [0,{total})")

    avail = list(items)
    out: List = []
    rest = n
    for i in range(k, 0, -1):
        place = factorial(i - 1)          # place value (k-1)! ... 0!
        digit = rest // place             # index into the REMAINING items
        rest = rest % place
        out.append(avail.pop(digit))      # take + remove the chosen item
    return out


def permutation_rank(perm: Sequence, items: Sequence) -> int:
    """Return the 0-indexed lexicographic rank of ``perm`` among permutations of
    ``items``. Exact inverse of :func:`nth_permutation`.

    ``perm`` must be a permutation of ``items`` (same multiset, distinct
    elements). ``permutation_rank(list(items), items)`` is ``0``.
    """
    perm = list(perm)
    items = list(items)
    k = len(items)
    if len(perm) != k:
        raise PermError(f"perm has length {len(perm)}, expected {k}")

    avail = list(items)
    rank = 0
    for i in range(k, 0, -1):
        cur = perm[k - i]
        try:
            digit = avail.index(cur)      # position among the REMAINING items
        except ValueError:
            raise PermError(f"{cur!r} is not a remaining item of {items!r}")
        rank += digit * factorial(i - 1)
        avail.pop(digit)                  # remove it so later indices are correct
    return rank
