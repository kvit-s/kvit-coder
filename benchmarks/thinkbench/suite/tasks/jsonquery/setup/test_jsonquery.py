"""Visible test suite for jsonquery.

Run it from the directory that CONTAINS the ``jsonquery`` package:

    python -m unittest jsonquery.test_jsonquery

These tests currently FAIL because of bugs in ``jsonquery.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).
"""
import unittest

from jsonquery.public import select, SelectError


# A small document reused across the tests.
DOC = {
    "users": [
        {"name": "ada", "id": 1, "roles": [{"id": 10}, {"id": 11}]},
        {"name": "linus", "id": 2, "roles": [{"id": 20}]},
    ],
    "owner": {"name": "grace", "id": 3},
}


class TestJsonquery(unittest.TestCase):
    # --- simple .a.b descent already works (sanity baseline) ----------------
    def test_simple_key_chain(self):
        self.assertEqual(select(DOC, ".owner.name"), ["grace"])

    def test_index_then_key(self):
        self.assertEqual(select(DOC, ".users[0].name"), ["ada"])

    def test_leading_bare_key(self):
        # a leading key without a dot is sugar for ".owner"
        self.assertEqual(select(DOC, "owner.id"), [3])

    # --- BUG 1: [*] must fan out flat, in order -----------------------------
    def test_wildcard_fans_out(self):
        # [*] yields each element of the list, flat — NOT the list nested
        self.assertEqual(
            select(DOC, ".users[*].name"),
            ["ada", "linus"],
        )

    def test_wildcard_terminal(self):
        # a terminal [*] returns the elements themselves, flat
        self.assertEqual(
            select({"xs": [1, 2, 3]}, ".xs[*]"),
            [1, 2, 3],
        )

    # --- BUG 2: ..key recursive descent, pre-order, into lists too ----------
    def test_recursive_descent_all_ids(self):
        # every id anywhere, top-down (pre-order), including ids nested inside
        # list elements: top users' ids before their roles' ids, owner last.
        self.assertEqual(
            select(DOC, "..id"),
            [1, 10, 11, 2, 20, 3],
        )

    def test_recursive_descent_names(self):
        self.assertEqual(
            select(DOC, "..name"),
            ["ada", "linus", "grace"],
        )

    # --- BUG 3: a missing key / index raises (does not return []) -----------
    def test_missing_key_raises(self):
        with self.assertRaises(SelectError):
            select(DOC, ".owner.missing")

    def test_index_out_of_range_raises(self):
        with self.assertRaises(SelectError):
            select(DOC, ".users[5]")

    # --- interaction: [*] fan-out + missing key under one branch ------------
    def test_wildcard_then_missing_key_raises(self):
        # .users[*].name works, but .users[*].missing must raise because the
        # second user (and the first) lack 'missing' — fanning out then hitting
        # a missing key has to surface the mismatch, not silently drop it.
        with self.assertRaises(SelectError):
            select(DOC, ".users[*].missing")

    # --- interaction: descend into a fanned-out frontier --------------------
    def test_wildcard_then_recursive_descent(self):
        # fan out the users, then collect every id at/under each — pre-order,
        # into the nested roles lists.
        self.assertEqual(
            select(DOC, ".users[*]..id"),
            [1, 10, 11, 2, 20],
        )


if __name__ == "__main__":
    unittest.main()
