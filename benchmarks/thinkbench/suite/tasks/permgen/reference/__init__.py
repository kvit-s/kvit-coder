"""permgen — lexicographic permutations of a sorted list of distinct items.

Public API lives in :mod:`permgen.public`:

* ``nth_permutation(items, n)`` returns the ``n``-th permutation (0-indexed) of
  ``items`` in lexicographic order.
* ``permutation_rank(perm, items)`` is its exact inverse: the 0-indexed rank of
  ``perm`` among the lexicographic permutations of ``items``.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import nth_permutation, permutation_rank, PermError

__all__ = ["nth_permutation", "permutation_rank", "PermError"]
