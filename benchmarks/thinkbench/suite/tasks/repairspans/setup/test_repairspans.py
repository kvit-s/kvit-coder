"""Visible test suite for repairspans — RUN THIS, it currently FAILS.

These tests encode the contract from brief.txt: closed intervals, touching
endpoints merge / overlap, and ``merge`` must handle unsorted input. They fail
against the shipped (buggy) code. Fix the code in this package until every test
here passes.

Run from the workspace root::

    python -m repairspans.test_repairspans

(or with pytest, which discovers the ``test_*`` functions). The module is part of
the ``repairspans`` package, so it imports the code under test with a relative
import.
"""

from .public import merge, overlaps


def test_merge_disjoint_unchanged():
    assert merge([[1, 2], [5, 6]]) == [[1, 2], [5, 6]]


def test_merge_overlapping():
    assert merge([[1, 4], [2, 5]]) == [[1, 5]]


def test_merge_touching_adjacent():
    # Closed intervals that touch at an endpoint collapse into one.
    assert merge([[1, 2], [2, 3]]) == [[1, 3]]


def test_merge_unsorted_input():
    # Input is not guaranteed to be sorted by start.
    assert merge([[3, 4], [1, 2]]) == [[1, 2], [3, 4]]


def test_overlaps_true_on_overlap():
    assert overlaps([1, 4], [2, 5]) is True


def test_overlaps_true_on_touch():
    # Touching closed intervals share their endpoint, so they overlap.
    assert overlaps([1, 2], [2, 3]) is True


def test_overlaps_false_when_disjoint():
    assert overlaps([1, 2], [5, 6]) is False


def _run():
    tests = [v for k, v in sorted(globals().items()) if k.startswith("test_") and callable(v)]
    failures = []
    for t in tests:
        try:
            t()
        except AssertionError as e:
            failures.append((t.__name__, str(e) or "AssertionError"))
        except Exception as e:  # noqa: BLE001
            failures.append((t.__name__, f"{type(e).__name__}: {e}"))
    passed = len(tests) - len(failures)
    for name, detail in failures:
        print(f"FAIL {name}: {detail}")
    print(f"{passed}/{len(tests)} passed")
    return 1 if failures else 0


if __name__ == "__main__":
    import sys

    sys.exit(_run())
