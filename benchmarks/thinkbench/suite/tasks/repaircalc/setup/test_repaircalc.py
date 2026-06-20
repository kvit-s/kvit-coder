"""Visible test suite for repaircalc.

Run it from the directory that CONTAINS the ``repaircalc`` package:

    python -m unittest repaircalc.test_repaircalc

These tests currently FAIL because of bugs in ``repaircalc.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).
"""
import unittest

from repaircalc.public import evaluate, CalcError


class TestRepaircalc(unittest.TestCase):
    def test_simple_add(self):
        self.assertEqual(evaluate("2+2"), 4)

    def test_simple_sub(self):
        self.assertEqual(evaluate("9-4"), 5)

    def test_precedence_mul_over_add(self):
        # multiplication must bind tighter than addition: 2 + (3*4)
        self.assertEqual(evaluate("2+3*4"), 14)

    def test_precedence_mixed(self):
        self.assertEqual(evaluate("2*3+4*5"), 26)

    def test_precedence_with_sub(self):
        self.assertEqual(evaluate("2 + 3 * 4 - 1"), 13)

    def test_left_assoc_subtraction(self):
        # subtraction is left-associative: (10 - 3) - 2 == 5, not 10 - (3 - 2)
        self.assertEqual(evaluate("10-3-2"), 5)

    def test_left_assoc_chain(self):
        self.assertEqual(evaluate("20-5-3-1"), 11)

    def test_add_sub_left_to_right(self):
        self.assertEqual(evaluate("1+2-3+4"), 4)

    def test_parentheses_group(self):
        self.assertEqual(evaluate("(2+3)*4"), 20)

    def test_parentheses_inner_expr(self):
        self.assertEqual(evaluate("2*(3+4)"), 14)

    def test_decimal_add(self):
        self.assertEqual(evaluate("3.5+1.5"), 5.0)

    def test_decimal_leading_dot(self):
        self.assertEqual(evaluate(".5+.5"), 1.0)

    def test_decimal_mul(self):
        self.assertEqual(evaluate("2.5*4"), 10.0)

    def test_division(self):
        self.assertEqual(evaluate("10/4"), 2.5)

    def test_division_by_zero_raises(self):
        with self.assertRaises(CalcError):
            evaluate("1/0")


if __name__ == "__main__":
    unittest.main()
