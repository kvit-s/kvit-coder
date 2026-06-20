"""Visible test suite for repairmoney — currently FAILING.

Run from the workspace root with either::

    python -m pytest repairmoney/test_repairmoney.py
    python repairmoney/test_repairmoney.py        # plain-asserts fallback

These tests describe the intended behaviour. They fail against the shipped
(buggy) code; your job is to fix ``repairmoney/public.py`` so they all pass.
"""

import os
import sys

# Make ``repairmoney`` importable whether this file is run via
# ``python -m pytest`` / ``python -m repairmoney.test_repairmoney`` (cwd already
# on the path) or as a plain script ``python repairmoney/test_repairmoney.py``
# (where sys.path[0] is this file's own directory). We add the PARENT of the
# package directory so ``import repairmoney`` resolves either way.
_PKG_PARENT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
if _PKG_PARENT not in sys.path:
    sys.path.insert(0, _PKG_PARENT)

from repairmoney.public import format_cents, split_evenly


def test_format_positive():
    assert format_cents(1234) == "$12.34"


def test_format_negative_sign_in_front():
    # The minus sign belongs IN FRONT of the dollar sign.
    assert format_cents(-1234) == "-$12.34"


def test_format_zero():
    assert format_cents(0) == "$0.00"


def test_format_pads_cents():
    # Small cent amounts must be zero-padded to two digits.
    assert format_cents(5) == "$0.05"
    assert format_cents(-5) == "-$0.05"


def test_split_divisible_sums_to_total():
    parts = split_evenly(1000, 4)
    assert parts == [250, 250, 250, 250]
    assert sum(parts) == 1000


def test_split_non_divisible_sums_to_total():
    parts = split_evenly(1000, 3)
    # Remainder cent must be distributed, not dropped.
    assert sum(parts) == 1000
    assert parts == [334, 333, 333]


def test_split_n_is_one():
    assert split_evenly(100, 1) == [100]


def _run():
    fns = [v for k, v in sorted(globals().items()) if k.startswith("test_")]
    failures = 0
    for fn in fns:
        try:
            fn()
            print(f"PASS {fn.__name__}")
        except AssertionError as e:
            failures += 1
            print(f"FAIL {fn.__name__}: {e}")
    print(f"\n{len(fns) - failures}/{len(fns)} passed")
    return failures


if __name__ == "__main__":
    import sys

    sys.exit(1 if _run() else 0)
