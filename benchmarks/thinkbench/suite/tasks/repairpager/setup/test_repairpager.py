"""Visible test suite for repairpager — currently FAILING.

Run with::

    python -m pytest repairpager/test_repairpager.py
    # or, without pytest installed:
    python repairpager/test_repairpager.py

These tests describe the intended behaviour of ``paginate``. The package as
shipped does not satisfy them yet — fix the code in ``repairpager/public.py``
until every test passes. The hidden grader checks a superset of these cases.
"""

import os
import sys

# Make the `repairpager` package importable no matter how this file is invoked
# (``python -m pytest`` from the workspace root, or ``python
# repairpager/test_repairpager.py`` which would otherwise only put this file's
# own directory on sys.path). The workspace root is this package's parent dir.
_WS_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
if _WS_ROOT not in sys.path:
    sys.path.insert(0, _WS_ROOT)

from repairpager.public import paginate  # noqa: E402


def test_first_page_items():
    # page 1 must return the FIRST page_size items, in order, starting at item 0.
    r = paginate([10, 20, 30, 40, 50], page_size=2, page=1)
    assert r["items"] == [10, 20], r["items"]


def test_second_page_items():
    r = paginate([10, 20, 30, 40, 50], page_size=2, page=2)
    assert r["items"] == [30, 40], r["items"]


def test_last_partial_page_items():
    # 5 items, size 2 -> pages are [10,20] [30,40] [50]; page 3 holds the leftover.
    r = paginate([10, 20, 30, 40, 50], page_size=2, page=3)
    assert r["items"] == [50], r["items"]


def test_total_pages_counts_partial_page():
    # 5 items in chunks of 2 needs 3 pages (ceil), not 2 (floor).
    r = paginate([10, 20, 30, 40, 50], page_size=2, page=1)
    assert r["total_pages"] == 3, r["total_pages"]


def test_total_items():
    r = paginate([10, 20, 30, 40, 50], page_size=2, page=1)
    assert r["total_items"] == 5, r["total_items"]


def test_has_next_on_first_page():
    r = paginate([10, 20, 30, 40, 50], page_size=2, page=1)
    assert r["has_next"] is True, r["has_next"]
    assert r["has_prev"] is False, r["has_prev"]


def test_has_next_false_on_last_page():
    # the last page must report has_next == False.
    r = paginate([10, 20, 30, 40, 50], page_size=2, page=3)
    assert r["has_next"] is False, r["has_next"]
    assert r["has_prev"] is True, r["has_prev"]


def _run():
    fns = [v for k, v in sorted(globals().items()) if k.startswith("test_") and callable(v)]
    failures = []
    for fn in fns:
        try:
            fn()
        except AssertionError as e:
            failures.append(f"FAIL {fn.__name__}: {e}")
        except Exception as e:  # noqa: BLE001
            failures.append(f"ERROR {fn.__name__}: {type(e).__name__}: {e}")
    for line in failures:
        print(line)
    print(f"{len(fns) - len(failures)}/{len(fns)} passed")
    return 1 if failures else 0


if __name__ == "__main__":
    import sys

    sys.exit(_run())
