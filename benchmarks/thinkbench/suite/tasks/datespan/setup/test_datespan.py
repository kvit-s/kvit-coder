"""Visible test suite for datespan.

Run it from the directory that CONTAINS the ``datespan`` package:

    python -m unittest datespan.test_datespan

These tests currently FAIL because of bugs in ``datespan.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).

Dates below carry their weekday in the comment so the business-day intent is
readable: Jun 15 2026 is a Monday, Jun 19 a Friday, Jun 20 a Saturday, Jun 21 a
Sunday, Jun 22 the next Monday.
"""
import unittest
from datetime import date

from datespan.public import add_business_days, business_days_between


class TestAddBusinessDays(unittest.TestCase):
    # --- simple within-week cases already work (sanity baseline) ------------
    def test_forward_within_week(self):
        # Mon + 1 = Tue, Mon + 4 = Fri
        self.assertEqual(add_business_days(date(2026, 6, 15), 1), date(2026, 6, 16))
        self.assertEqual(add_business_days(date(2026, 6, 15), 4), date(2026, 6, 19))

    def test_forward_skips_weekend(self):
        # Mon + 5 lands on the next Mon (Sat/Sun are skipped)
        self.assertEqual(add_business_days(date(2026, 6, 15), 5), date(2026, 6, 22))
        # Fri + 1 = next Mon
        self.assertEqual(add_business_days(date(2026, 6, 19), 1), date(2026, 6, 22))

    def test_zero_on_business_day(self):
        self.assertEqual(add_business_days(date(2026, 6, 15), 0), date(2026, 6, 15))

    # --- BUG 2: weekend start must be normalized forward --------------------
    def test_weekend_start_is_normalized(self):
        # Sat + 0 snaps forward to Mon; Sun + 2 anchors on Mon then steps to Wed
        self.assertEqual(add_business_days(date(2026, 6, 20), 0), date(2026, 6, 22))
        self.assertEqual(add_business_days(date(2026, 6, 21), 2), date(2026, 6, 24))

    # --- BUG 1: negative n must step backward over weekends -----------------
    def test_negative_within_week(self):
        # Fri - 1 = Thu (no weekend crossed)
        self.assertEqual(add_business_days(date(2026, 6, 19), -1), date(2026, 6, 18))

    def test_negative_crosses_weekend(self):
        # Mon - 1 = previous Fri (step back over the weekend)
        self.assertEqual(add_business_days(date(2026, 6, 15), -1), date(2026, 6, 12))
        # Mon - 5 = the Monday a week earlier
        self.assertEqual(add_business_days(date(2026, 6, 15), -5), date(2026, 6, 8))


class TestBusinessDaysBetween(unittest.TestCase):
    # --- simple within-week cases already work ------------------------------
    def test_forward_within_week(self):
        # exclusive of a, inclusive of b: Mon->Tue is 1, Mon->Fri is 4
        self.assertEqual(business_days_between(date(2026, 6, 15), date(2026, 6, 16)), 1)
        self.assertEqual(business_days_between(date(2026, 6, 15), date(2026, 6, 19)), 4)

    def test_same_day_is_zero(self):
        self.assertEqual(business_days_between(date(2026, 6, 15), date(2026, 6, 15)), 0)

    def test_forward_skips_weekend(self):
        # Mon -> next Mon is 5 business days (Sat/Sun excluded)
        self.assertEqual(business_days_between(date(2026, 6, 15), date(2026, 6, 22)), 5)
        # Fri -> Sat is 0 (Saturday is not a business day)
        self.assertEqual(business_days_between(date(2026, 6, 19), date(2026, 6, 20)), 0)

    # --- BUG 3: sign when a is after b --------------------------------------
    def test_reverse_is_negative(self):
        # walking from the later date to the earlier one is the negation
        self.assertEqual(business_days_between(date(2026, 6, 16), date(2026, 6, 15)), -1)
        self.assertEqual(business_days_between(date(2026, 6, 22), date(2026, 6, 15)), -5)


if __name__ == "__main__":
    unittest.main()
