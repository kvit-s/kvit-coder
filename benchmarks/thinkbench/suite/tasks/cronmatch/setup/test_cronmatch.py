"""Visible test suite for cronmatch.

Run it from the directory that CONTAINS the ``cronmatch`` package:

    python -m unittest cronmatch.test_cronmatch

These tests currently FAIL because of bugs in ``cronmatch.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).
"""
import unittest
from datetime import datetime

from cronmatch.public import matches, CronError


def dt(s):
    return datetime.strptime(s, "%Y-%m-%d %H:%M")


class TestCronmatch(unittest.TestCase):
    # --- basics that already work (sanity baseline) -------------------------
    def test_every_minute(self):
        self.assertTrue(matches("* * * * *", dt("2026-06-18 13:47")))

    def test_exact_value(self):
        self.assertTrue(matches("30 9 * * *", dt("2026-06-18 09:30")))
        self.assertFalse(matches("30 9 * * *", dt("2026-06-18 09:31")))
        self.assertFalse(matches("30 9 * * *", dt("2026-06-18 10:30")))

    def test_list_and_simple_range(self):
        self.assertTrue(matches("0,30,45 * * * *", dt("2026-06-18 14:45")))
        self.assertFalse(matches("0,30,45 * * * *", dt("2026-06-18 14:31")))
        self.assertTrue(matches("10-20 * * * *", dt("2026-06-18 14:15")))
        self.assertFalse(matches("10-20 * * * *", dt("2026-06-18 14:21")))

    def test_minute_step_field_min_zero(self):
        # */15 on minute (min 0) -> {0,15,30,45}; this already works.
        self.assertTrue(matches("*/15 * * * *", dt("2026-06-18 00:30")))
        self.assertFalse(matches("*/15 * * * *", dt("2026-06-18 00:31")))

    # --- BUG 1: */n is off by the field minimum -----------------------------
    def test_step_month_starts_at_one(self):
        # month */3 -> {1,4,7,10}; January and July are in the set, June is not.
        self.assertTrue(matches("0 0 1 */3 *", dt("2026-01-01 00:00")))
        self.assertTrue(matches("0 0 1 */3 *", dt("2026-07-01 00:00")))
        self.assertFalse(matches("0 0 1 */3 *", dt("2026-06-01 00:00")))

    def test_step_day_of_month_starts_at_one(self):
        # day-of-month */10 -> {1,11,21,31}; the 11th matches, the 10th does not.
        self.assertTrue(matches("0 0 */10 * *", dt("2026-06-11 00:00")))
        self.assertFalse(matches("0 0 */10 * *", dt("2026-06-10 00:00")))

    # --- BUG 2: stepped range a-b/n must honor the step ---------------------
    def test_stepped_range_minute(self):
        # 10-30/10 -> {10,20,30}; 20 matches, 15 does not.
        self.assertTrue(matches("10-30/10 * * * *", dt("2026-06-18 00:20")))
        self.assertFalse(matches("10-30/10 * * * *", dt("2026-06-18 00:15")))

    def test_stepped_range_hour(self):
        # 8-18/2 -> {8,10,12,14,16,18}; 14 matches, 9 and 11 do not.
        self.assertTrue(matches("0 8-18/2 * * *", dt("2026-06-18 14:00")))
        self.assertFalse(matches("0 8-18/2 * * *", dt("2026-06-18 09:00")))

    # --- BUG 3: day-of-month / day-of-week OR when both restricted ----------
    def test_dom_dow_or_semantics(self):
        # "0 0 13 * 5" = the 13th OR a Friday.
        self.assertTrue(matches("0 0 13 * 5", dt("2026-06-13 00:00")))   # Sat 13th -> dom
        self.assertTrue(matches("0 0 13 * 5", dt("2026-06-19 00:00")))   # Fri 19th -> dow
        self.assertTrue(matches("0 0 13 * 5", dt("2026-02-13 00:00")))   # Fri 13th -> both
        self.assertFalse(matches("0 0 13 * 5", dt("2026-06-18 00:00")))  # Thu 18th -> neither

    def test_only_one_of_dom_dow_restricted(self):
        # only day-of-week restricted -> only it constrains the day.
        self.assertTrue(matches("0 0 * * 1", dt("2026-06-15 00:00")))    # Monday
        self.assertFalse(matches("0 0 * * 1", dt("2026-06-16 00:00")))   # Tuesday
        # only day-of-month restricted.
        self.assertTrue(matches("0 0 15 * *", dt("2026-06-15 00:00")))
        self.assertFalse(matches("0 0 15 * *", dt("2026-06-16 00:00")))

    # --- interaction: stepped range / stepped month + dom/dow OR ------------
    def test_interaction_stepped_range_and_or(self):
        # day-of-month 10-20/5 = {10,15,20} OR Friday(5).
        self.assertTrue(matches("0 0 10-20/5 * 5", dt("2026-06-15 00:00")))   # 15 -> dom
        self.assertTrue(matches("0 0 10-20/5 * 5", dt("2026-06-19 00:00")))   # Fri -> dow
        self.assertFalse(matches("0 0 10-20/5 * 5", dt("2026-06-17 00:00")))  # Wed 17th -> neither

    def test_interaction_stepped_month_and_or(self):
        # month */3 = {1,4,7,10}; day-of-month 1 OR Friday(5).
        self.assertTrue(matches("0 0 1 */3 5", dt("2026-07-03 00:00")))   # Jul, Fri -> month ok, dow fires
        self.assertTrue(matches("0 0 1 */3 5", dt("2026-04-01 00:00")))   # Apr, 1st -> month ok, dom fires

    # --- validation ---------------------------------------------------------
    def test_bad_field_count_raises(self):
        with self.assertRaises(CronError):
            matches("* * * *", dt("2026-06-18 00:00"))

    def test_bad_token_raises(self):
        with self.assertRaises(CronError):
            matches("oops * * * *", dt("2026-06-18 00:00"))


if __name__ == "__main__":
    unittest.main()
