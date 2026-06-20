"""Visible test suite for decimalfmt.

Run it from the directory that CONTAINS the ``decimalfmt`` package:

    python -m unittest decimalfmt.test_decimalfmt

These tests currently FAIL because of bugs in ``decimalfmt.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).
"""
import unittest

from decimalfmt.public import MoneyError, format_amount, parse_amount


class TestDecimalfmt(unittest.TestCase):
    # --- small positive amounts already render correctly (sanity baseline) ---
    def test_small_positive(self):
        self.assertEqual(format_amount(12345), "123.45")

    def test_under_a_thousand(self):
        self.assertEqual(format_amount(99999), "999.99")

    def test_whole_cents_two_digit_frac(self):
        self.assertEqual(format_amount(50), "0.50")

    # --- BUG 1: negative sign placement --------------------------------------
    def test_negative_sign_in_front(self):
        # the sign belongs in FRONT of the whole number, not after the decimal
        self.assertEqual(format_amount(-1234567), "-12,345.67")

    def test_small_negative(self):
        self.assertEqual(format_amount(-5), "-0.05")

    # --- BUG 2: fractional zero-padding --------------------------------------
    def test_pad_single_cent(self):
        # 5 cents -> ".05", not ".5"
        self.assertEqual(format_amount(5), "0.05")

    def test_pad_whole_dollar(self):
        # a whole-dollar amount keeps both fractional digits
        self.assertEqual(format_amount(100), "1.00")

    # --- BUG 3: thousands grouping from the right ----------------------------
    def test_grouping_from_right(self):
        self.assertEqual(format_amount(1234567), "12,345.67")

    def test_grouping_millions(self):
        self.assertEqual(format_amount(100000000), "1,000,000.00")

    # --- round-trip: parse strips the separators -----------------------------
    def test_round_trip_grouped(self):
        self.assertEqual(parse_amount(format_amount(1234567)), 1234567)

    def test_round_trip_negative(self):
        self.assertEqual(parse_amount(format_amount(-1234567)), -1234567)

    # --- interaction: negative + grouping + padding + round-trip -------------
    def test_negative_grouped_padded_round_trip(self):
        s = format_amount(-100000005)
        self.assertEqual(s, "-1,000,000.05")
        self.assertEqual(parse_amount(s), -100000005)

    # --- validation ----------------------------------------------------------
    def test_bad_cents_raises(self):
        with self.assertRaises(MoneyError):
            format_amount("100")


if __name__ == "__main__":
    unittest.main()
