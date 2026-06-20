"""Visible test suite for rrulelite.

Run it from the directory that CONTAINS the ``rrulelite`` package:

    python -m unittest rrulelite.test_rrulelite

These tests currently FAIL because of bugs in ``rrulelite.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).
"""
import unittest
from datetime import date

from rrulelite.public import expand, RRuleError


class TestRrulelite(unittest.TestCase):
    # --- daily / weekly already work (sanity baseline) ----------------------
    def test_daily_simple(self):
        self.assertEqual(
            expand({"freq": "daily"}, date(2026, 1, 1), 3),
            [date(2026, 1, 1), date(2026, 1, 2), date(2026, 1, 3)],
        )

    def test_daily_interval(self):
        self.assertEqual(
            expand({"freq": "daily", "interval": 2}, date(2026, 1, 1), 3),
            [date(2026, 1, 1), date(2026, 1, 3), date(2026, 1, 5)],
        )

    def test_weekly_interval(self):
        self.assertEqual(
            expand({"freq": "weekly", "interval": 2}, date(2026, 1, 1), 3),
            [date(2026, 1, 1), date(2026, 1, 15), date(2026, 1, 29)],
        )

    # --- monthly basics ------------------------------------------------------
    def test_monthly_simple(self):
        self.assertEqual(
            expand({"freq": "monthly"}, date(2026, 1, 15), 3),
            [date(2026, 1, 15), date(2026, 2, 15), date(2026, 3, 15)],
        )

    # --- BUG 1: interval ignored for monthly --------------------------------
    def test_monthly_interval_two(self):
        # every other month from Jan -> Jan, Mar, May
        self.assertEqual(
            expand({"freq": "monthly", "interval": 2}, date(2026, 1, 10), 3),
            [date(2026, 1, 10), date(2026, 3, 10), date(2026, 5, 10)],
        )

    # --- BUG 2: month-end overflow (clamp, not spill) -----------------------
    def test_monthly_month_end_clamp(self):
        # Jan 31 stepping monthly clamps to each month's last day; Feb 2026 is
        # NOT a leap year, so the second date is Feb 28 (not Mar 3).
        self.assertEqual(
            expand({"freq": "monthly"}, date(2026, 1, 31), 3),
            [date(2026, 1, 31), date(2026, 2, 28), date(2026, 3, 31)],
        )

    # --- BUG 3: until is inclusive ------------------------------------------
    def test_until_inclusive(self):
        # the date landing exactly on `until` must be KEPT
        self.assertEqual(
            expand(
                {"freq": "daily", "until": date(2026, 1, 3)},
                date(2026, 1, 1),
                10,
            ),
            [date(2026, 1, 1), date(2026, 1, 2), date(2026, 1, 3)],
        )

    # --- interaction: interval + clamp + inclusive until --------------------
    def test_monthly_interval_clamp_until(self):
        # every other month from Jan 31, inclusive until Jul 31:
        # Jan 31, Mar 31, May 31, Jul 31 (Sep 30 would be past until)
        self.assertEqual(
            expand(
                {"freq": "monthly", "interval": 2, "until": date(2026, 7, 31)},
                date(2026, 1, 31),
                10,
            ),
            [date(2026, 1, 31), date(2026, 3, 31), date(2026, 5, 31), date(2026, 7, 31)],
        )

    # --- limit cap and validation -------------------------------------------
    def test_limit_cap(self):
        self.assertEqual(len(expand({"freq": "daily"}, date(2026, 1, 1), 5)), 5)

    def test_bad_freq_raises(self):
        with self.assertRaises(RRuleError):
            expand({"freq": "yearly"}, date(2026, 1, 1), 3)


if __name__ == "__main__":
    unittest.main()
