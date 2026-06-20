"""Visible test suite for permgen.

Run it from the directory that CONTAINS the ``permgen`` package:

    python -m unittest permgen.test_permgen

These tests currently FAIL because of bugs in ``permgen.public``. Your job is to
fix the package so every test below passes (the hidden grading suite checks the
same behavior on more cases).
"""
import itertools
import unittest

from permgen.public import nth_permutation, permutation_rank, PermError


class TestPermgen(unittest.TestCase):
    # --- identity already works (sanity baseline) ---------------------------
    def test_identity_nth(self):
        # rank 0 is always the items in their given (lexicographic) order
        self.assertEqual(nth_permutation(["a", "b", "c", "d"], 0), ["a", "b", "c", "d"])

    # --- nth_permutation: factoradic decoding (BUGS 1 & 3) ------------------
    def test_nth_small(self):
        # the 6 permutations of [1,2,3] in lexicographic order, by rank
        expected = [
            [1, 2, 3], [1, 3, 2], [2, 1, 3], [2, 3, 1], [3, 1, 2], [3, 2, 1],
        ]
        self.assertEqual([nth_permutation([1, 2, 3], n) for n in range(6)], expected)

    def test_nth_last(self):
        # the final permutation is the fully reversed list
        self.assertEqual(
            nth_permutation(["a", "b", "c", "d"], 23), ["d", "c", "b", "a"]
        )

    def test_nth_mid(self):
        self.assertEqual(nth_permutation(["a", "b", "c", "d"], 5), ["a", "d", "c", "b"])

    # --- permutation_rank: index the REMAINING items (BUG 2) ----------------
    def test_rank_identity(self):
        self.assertEqual(permutation_rank(["a", "b", "c", "d"], ["a", "b", "c", "d"]), 0)

    def test_rank_values(self):
        items = ["a", "b", "c", "d"]
        self.assertEqual(permutation_rank(["b", "a", "c", "d"], items), 6)
        self.assertEqual(permutation_rank(["d", "c", "b", "a"], items), 23)

    # --- the two are exact inverses (needs ALL three bugs fixed) ------------
    def test_round_trip_nth_then_rank(self):
        items = ["a", "b", "c", "d"]
        for n in range(24):
            self.assertEqual(permutation_rank(nth_permutation(items, n), items), n)

    def test_round_trip_rank_then_nth(self):
        items = ["a", "b", "c", "d"]
        for n, perm in enumerate(itertools.permutations(items)):
            self.assertEqual(nth_permutation(items, permutation_rank(list(perm), items)), list(perm))

    # --- validation ---------------------------------------------------------
    def test_nth_out_of_range_raises(self):
        with self.assertRaises(PermError):
            nth_permutation(["a", "b", "c"], 999)


if __name__ == "__main__":
    unittest.main()
