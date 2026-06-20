"""Visible test suite for textflow.

Run it from the directory that CONTAINS the ``textflow`` package:

    python -m unittest textflow.test_textflow

These tests currently FAIL because of bugs in ``textflow.public``. Your job is
to fix the package so every test below passes (the hidden grading suite checks
the same behavior on more cases).
"""
import unittest

from textflow.public import justify, JustifyError


class TestTextflow(unittest.TestCase):
    # --- baseline that already works (sanity; even-fit lines look right) -----
    def test_even_fit_line(self):
        # "a b c" into width 5 fills exactly with single spaces: no padding, no
        # remainder, so even the buggy distributor sets it correctly. The single
        # last line is left-justified and (here) already full width.
        self.assertEqual(justify(["a", "b", "c"], 5), ["a b c"])

    def test_each_line_is_exactly_width(self):
        out = justify(["the", "quick", "brown", "fox", "jumps"], 11)
        self.assertTrue(all(len(line) == 11 for line in out))

    # --- BUG 1: extra spaces go to the LEFT gaps ----------------------------
    def test_uneven_extra_to_left(self):
        # First (interior) line is "a","b","c" = 3 chars into width 8 -> 5 spaces
        # over 2 gaps: base 2, extra 1; the EXTRA space goes to the LEFT gap ->
        # "a   b  c". ("next" is bumped to the ragged last line.)
        self.assertEqual(
            justify(["a", "b", "c", "next"], 8), ["a   b  c", "next    "]
        )

    def test_uneven_three_gaps(self):
        # First (interior) line is "aa","bb","cc","dd" = 8 chars into width 13 ->
        # 5 spaces over 3 gaps: base 1, extra 2; the two extra spaces go to the
        # first two gaps -> "aa  bb  cc dd". ("ee" lands on the last line.)
        self.assertEqual(
            justify(["aa", "bb", "cc", "dd", "ee"], 13),
            ["aa  bb  cc dd", "ee           "],
        )

    # --- BUG 2: a single-word (non-last) line is left-justified + padded -----
    def test_single_word_line_padded(self):
        # "longword" alone fills a line of width 12 by left-justify (4 trailing
        # spaces); "tail" is the last line, left-justified to width.
        out = justify(["longword", "tail"], 12)
        self.assertEqual(out, ["longword    ", "tail        "])

    # --- BUG 3: the last line is left-justified, not fully justified ---------
    def test_last_line_left_justified(self):
        # two lines: first ("alpha beta") is fully justified across width 14;
        # the last ("gamma") is left-justified and padded, NOT stretched.
        out = justify(["alpha", "beta", "gamma"], 14)
        self.assertEqual(out, ["alpha     beta", "gamma         "])

    def test_last_line_multiword_single_spaced(self):
        # the final line keeps SINGLE spaces between its words then pads right.
        out = justify(["one", "two", "three", "tiny", "end"], 9)
        # greedy pack: "one two" (7) | "three" alone (5<9) | "tiny end" last.
        self.assertEqual(out, ["one   two", "three    ", "tiny end "])

    # --- interaction: all three at once -------------------------------------
    def test_interaction_all_three(self):
        # forces an uneven interior line, a single-word interior line, and a
        # left-justified last line in one paragraph at width 10.
        words = ["practical", "no", "gap", "x", "the", "final", "row"]
        out = justify(words, 10)
        # pack: "practical"(9) alone | "no gap x"(8) | "the final"(9) | "row".
        #  - "practical" single interior word -> left-justify + pad
        #  - "no gap x": 6 chars, 4 spaces over 2 gaps -> base 2 extra 0 -> even
        #  - "the final": last? no -> 9 chars, 1 space, 1 gap -> "the   final"
        #    wait: only two words "the","final" = 8 chars, 2 spaces, 1 gap.
        #  - "row" last -> left-justify + pad
        self.assertEqual(
            out,
            ["practical ", "no  gap  x", "the  final", "row       "],
        )

    # --- validation ----------------------------------------------------------
    def test_empty_words(self):
        self.assertEqual(justify([], 10), [])

    def test_word_longer_than_width_raises(self):
        with self.assertRaises(JustifyError):
            justify(["toolongword"], 4)


if __name__ == "__main__":
    unittest.main()
