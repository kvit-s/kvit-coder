"""permgen.public — lexicographic permutations of distinct items (stdlib only).

A permutation of ``items`` (a sequence of DISTINCT items, taken in the given
order as the lexicographic alphabet) can be addressed by its rank: the position
it occupies when all ``len(items)!`` permutations are listed in lexicographic
order, starting at rank 0.

* ``nth_permutation(items, n)`` -> the rank-``n`` permutation as a new list.
  ``nth_permutation(items, 0)`` is always ``list(items)`` (the identity).
* ``permutation_rank(perm, items)`` -> the rank of ``perm``; meant to be the
  exact inverse of ``nth_permutation``.

Both use the FACTORIAL NUMBER SYSTEM (a.k.a. factoradic): the rank decomposes
into digits with place values ``(k-1)!, (k-2)!, ..., 1!, 0!`` where ``k`` is the
number of items. Each digit selects an element from the items not yet used, so
the digit is an index into the REMAINING items.

Standard library only (``math``).
"""

from __future__ import annotations

from math import factorial
from typing import List, Sequence


class PermError(ValueError):
    """Raised when a permutation request is malformed or out of range."""


def nth_permutation(items: Sequence, n: int) -> List:
    """Return the ``n``-th lexicographic permutation (0-indexed) of ``items``."""
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

    # BUG (rank/unrank off-by-one): the permutations are treated as if they were
    # numbered starting from 1, so the index is shifted down by one before being
    # decoded. n == 0 still maps to the identity (m stays 0), but every other n
    # now decodes the permutation BELOW it, so this is no longer the exact
    # inverse of permutation_rank.
    m = n - 1 if n > 0 else 0

    avail = list(items)
    out: List = []
    for i in range(k, 0, -1):
        # BUG (factoradic place value): the digit for position with `i` items
        # remaining has place value (i-1)! = factorial(i - 1), but this uses
        # factorial(i) — one factorial too large. The leading digit collapses to
        # 0 and the lower digits are mis-scaled, so the wrong items are chosen.
        place = factorial(i)
        digit = m // place
        m = m % place
        if digit > len(avail) - 1:        # the inflated place value can over-index
            digit = len(avail) - 1
        out.append(avail.pop(digit))
    return out


def permutation_rank(perm: Sequence, items: Sequence) -> int:
    """Return the 0-indexed lexicographic rank of ``perm`` among permutations of
    ``items`` (meant to invert :func:`nth_permutation`)."""
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
            digit = avail.index(cur)
        except ValueError:
            raise PermError(f"{cur!r} is not an item of {items!r}")
        rank += digit * factorial(i - 1)
        # BUG (removing already-used elements): each chosen item must be removed
        # from `avail` so the next digit is an index into the REMAINING items.
        # This forgets to remove it, so `avail` never shrinks and every later
        # `index()` is taken against the full original list — inflating the rank
        # whenever an earlier (lower-positioned) item has already been used.
    return rank
