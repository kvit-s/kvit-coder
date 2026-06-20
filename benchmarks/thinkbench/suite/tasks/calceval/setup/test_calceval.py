"""Visible test suite for calceval.

Run it from the directory that CONTAINS the ``calceval`` package:

    python -m unittest calceval.test_calceval

These tests currently FAIL because of bugs in ``calceval.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).
"""
import unittest

from calceval.public import evaluate, CalcError


class TestCalceval(unittest.TestCase):
    # --- precedence basics already work (sanity baseline) -------------------
    def test_mul_over_add(self):
        self.assertEqual(evaluate("2+3*4"), 14.0)

    def test_parens(self):
        self.assertEqual(evaluate("(2+3)*4"), 20.0)

    def test_pow_over_mul(self):
        self.assertEqual(evaluate("2*3^2"), 18.0)

    def test_decimal(self):
        self.assertEqual(evaluate("3.5*2"), 7.0)

    # --- BUG 1: '-' and '/' must be LEFT-associative ------------------------
    def test_left_assoc_sub(self):
        # 10-2-3 == (10-2)-3 == 5, NOT 10-(2-3) == 11
        self.assertEqual(evaluate("10-2-3"), 5.0)

    def test_left_assoc_div(self):
        # 100/10/2 == (100/10)/2 == 5, NOT 100/(10/2) == 20
        self.assertEqual(evaluate("100/10/2"), 5.0)

    # --- BUG 2: '^' must be RIGHT-associative -------------------------------
    def test_right_assoc_pow(self):
        # 2^3^2 == 2^(3^2) == 512, NOT (2^3)^2 == 64
        self.assertEqual(evaluate("2^3^2"), 512.0)

    # --- BUG 3: unary minus binds LOOSER than '^' --------------------------
    def test_unary_pow_binding(self):
        # -2^2 == -(2^2) == -4, NOT (-2)^2 == 4
        self.assertEqual(evaluate("-2^2"), -4.0)

    def test_paren_flips_unary(self):
        # parentheses still let you force (-2)^2 == 4
        self.assertEqual(evaluate("(-2)^2"), 4.0)

    # --- interaction: all three rules at once ------------------------------
    def test_interaction(self):
        # -2^2^2 == -(2^(2^2)) == -(2^4) == -16
        self.assertEqual(evaluate("-2^2^2"), -16.0)

    # --- validation ---------------------------------------------------------
    def test_empty_raises(self):
        with self.assertRaises(CalcError):
            evaluate("")

    def test_div_zero_raises(self):
        with self.assertRaises(CalcError):
            evaluate("1/0")


if __name__ == "__main__":
    unittest.main()
