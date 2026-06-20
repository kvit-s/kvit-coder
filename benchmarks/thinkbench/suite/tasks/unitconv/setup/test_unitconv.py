"""Visible test suite for unitconv.

Run it from the directory that CONTAINS the ``unitconv`` package:

    python -m unittest unitconv.test_unitconv

These tests currently FAIL because of bugs in ``unitconv.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).
"""
import unittest

from unitconv.public import convert, UnitError


class TestUnitconv(unittest.TestCase):
    # --- simple same-dimension conversions already work (sanity baseline) ----
    def test_length_mm_to_m(self):
        self.assertAlmostEqual(convert(1000, "mm", "m"), 1.0)

    def test_length_km_to_m(self):
        self.assertAlmostEqual(convert(2, "km", "m"), 2000.0)

    def test_time_min_to_h(self):
        self.assertAlmostEqual(convert(90, "min", "h"), 1.5)

    def test_time_h_to_s(self):
        self.assertAlmostEqual(convert(1, "h", "s"), 3600.0)

    # --- BUG 1: compound numerator scale dropped ----------------------------
    def test_compound_km_per_s(self):
        # 2 km/s = 2000 m/s; the "km" -> 1000 scale must survive parsing.
        self.assertAlmostEqual(convert(2, "km/s", "m/s"), 2000.0)

    # --- BUG 2: compound denominator composed with the wrong operator -------
    def test_compound_m_per_h(self):
        # 3600 m/h = 1 m/s (divide by 3600 seconds, not multiply).
        self.assertAlmostEqual(convert(3600, "m/h", "m/s"), 1.0)

    # --- BUGS 1 + 2 together: a real speed conversion -----------------------
    def test_kmh_to_ms(self):
        # 36 km/h = 36000 m / 3600 s = 10 m/s.
        self.assertAlmostEqual(convert(36, "km/h", "m/s"), 10.0)

    def test_ms_to_kmh(self):
        # 10 m/s = 36 km/h.
        self.assertAlmostEqual(convert(10, "m/s", "km/h"), 36.0)

    # --- BUG 3: incompatible dimensions must raise --------------------------
    def test_length_to_time_raises(self):
        with self.assertRaises(UnitError):
            convert(1, "m", "s")

    def test_length_to_speed_raises(self):
        with self.assertRaises(UnitError):
            convert(1, "m", "km/h")

    # --- unknown unit still raises ------------------------------------------
    def test_unknown_unit_raises(self):
        with self.assertRaises(UnitError):
            convert(1, "m", "ly")


if __name__ == "__main__":
    unittest.main()
