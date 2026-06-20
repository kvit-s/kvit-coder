"""permgen — lexicographic permutations of a sorted list of distinct items.

Public API lives in :mod:`permgen.public`:

* ``nth_permutation(items, n)`` returns the ``n``-th permutation (0-indexed) of
  ``items`` in lexicographic order.
* ``permutation_rank(perm, items)`` is supposed to be its exact inverse: the
  0-indexed rank of ``perm`` among the lexicographic permutations of ``items``.
"""

from .public import nth_permutation, permutation_rank, PermError

__all__ = ["nth_permutation", "permutation_rank", "PermError"]
