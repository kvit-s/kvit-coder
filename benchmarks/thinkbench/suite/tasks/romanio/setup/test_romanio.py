"""Visible test suite for romanio.

Run it from the directory that CONTAINS the ``romanio`` package:

    python -m unittest romanio.test_romanio

These tests currently FAIL because of bugs in ``romanio.public``. Your job is to
fix the package so every test below passes (the hidden grading suite checks the
same behavior on more cases).
"""
import unittest

from romanio.public import to_roman, from_roman, RomanError


class TestRomanio(unittest.TestCase):
    # --- simple additive numerals already work (sanity baseline) ------------
    def test_to_roman_additive_basics(self):
        self.assertEqual(to_roman(1), "I")
        self.assertEqual(to_roman(2), "II")
        self.assertEqual(to_roman(3), "III")
        self.assertEqual(to_roman(6), "VI")
        self.assertEqual(to_roman(8), "VIII")

    def test_from_roman_additive_basics(self):
        self.assertEqual(from_roman("III"), 3)
        self.assertEqual(from_roman("VI"), 6)
        self.assertEqual(from_roman("VIII"), 8)
        self.assertEqual(from_roman("XXX"), 30)

    # --- BUG 1: to_roman must use subtractive notation ----------------------
    def test_to_roman_subtractive(self):
        # 4=IV (not IIII), 9=IX, 40=XL, 90=XC, 400=CD, 900=CM
        self.assertEqual(to_roman(4), "IV")
        self.assertEqual(to_roman(9), "IX")
        self.assertEqual(to_roman(40), "XL")
        self.assertEqual(to_roman(90), "XC")
        self.assertEqual(to_roman(400), "CD")
        self.assertEqual(to_roman(900), "CM")
        # a composite that exercises several subtractive pairs at once
        self.assertEqual(to_roman(1994), "MCMXCIV")

    # --- BUG 2: from_roman must honour subtractive pairs --------------------
    def test_from_roman_subtractive(self):
        self.assertEqual(from_roman("IV"), 4)
        self.assertEqual(from_roman("IX"), 9)
        self.assertEqual(from_roman("XL"), 40)
        self.assertEqual(from_roman("XC"), 90)
        self.assertEqual(from_roman("CD"), 400)
        self.assertEqual(from_roman("CM"), 900)
        self.assertEqual(from_roman("MCMXCIV"), 1994)

    # --- round trip: the two are inverses -----------------------------------
    def test_round_trip(self):
        for n in (4, 9, 14, 40, 49, 90, 444, 900, 1994, 2026, 3888):
            self.assertEqual(from_roman(to_roman(n)), n)

    # --- BUG 3: out-of-range must raise -------------------------------------
    def test_to_roman_out_of_range(self):
        with self.assertRaises(RomanError):
            to_roman(0)
        with self.assertRaises(RomanError):
            to_roman(4000)

    def test_from_roman_bad_symbol(self):
        with self.assertRaises(RomanError):
            from_roman("IZ")


if __name__ == "__main__":
    unittest.main()
