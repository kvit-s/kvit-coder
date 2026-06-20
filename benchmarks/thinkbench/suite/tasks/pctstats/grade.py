#!/usr/bin/env python3
"""Held-out oracle for the bug-fix task `fix_percentile` (package `pctstats`).

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never reads the agent's own tests. It imports the produced `pctstats`
package and checks the BEHAVIOUR demanded by the brief's contract: nearest-rank
percentiles (1-based `rank = ceil(p/100*n)`, clamped) plus the unchanged
`mean`/`minimum`/`maximum` helpers.

Expected values are computed here by hand (see comments), not by re-deriving them
from the package under test.

Output: one JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator). A failed import forces score
0.0. Exit code is 0 whenever grading ran to completion (even at 0.0); nonzero only
on a grader-internal failure.
"""
import importlib
import json
import math
import os
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

checks = []


def check(cid, desc, fn):
    """Run one behaviour check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


def approx(a, b, tol=1e-9):
    return abs(float(a) - float(b)) <= tol


# --- import the produced package (contract: top-level `pctstats`) -------------
import_ok = True
import_detail = ""
mod = None
try:
    mod = importlib.import_module("pctstats")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    percentile = getattr(mod, "percentile", None)
    mean = getattr(mod, "mean", None)
    minimum = getattr(mod, "minimum", None)
    maximum = getattr(mod, "maximum", None)

    # Skewed list: nineteen 1s then a single large outlier. n = 20.
    # Nearest-rank rank = ceil(p/100 * n); value = sorted[rank-1].
    SKEW = [1] * 19 + [100000]  # sorted is already ascending

    # 1. THE headline bug: p95 of the skewed list.
    #    rank = ceil(0.95*20) = 19 -> index 18 -> value 1 (NOT the max). The off-by-one
    #    bug saturates the index to n-1 here, returning the 100000 maximum instead.
    check(
        "p95_skewed_not_max",
        "p95 on [1]*19+[100000] is the small value 1, not the 100000 maximum",
        lambda: (percentile(SKEW, 95) == 1, f"got {percentile(SKEW, 95)!r}, want 1"),
    )

    # 2. p50 of the skewed list.
    #    rank = ceil(0.50*20) = 10 -> index 9 -> value 1.
    check(
        "p50_skewed",
        "p50 on the skewed list is 1",
        lambda: (percentile(SKEW, 50) == 1, f"got {percentile(SKEW, 50)!r}, want 1"),
    )

    # 3. p99 of the skewed list.
    #    rank = ceil(0.99*20) = ceil(19.8) = 20 -> index 19 -> value 100000.
    check(
        "p99_skewed",
        "p99 on the skewed list reaches the outlier 100000",
        lambda: (percentile(SKEW, 99) == 100000, f"got {percentile(SKEW, 99)!r}, want 100000"),
    )

    # A simple 1..10 list (n = 10) pins the mid-range ranks the bug gets wrong.
    TEN = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

    # 4. p30: rank = ceil(0.30*10) = 3 -> index 2 -> value 3.
    check(
        "p30_ten",
        "nearest-rank p30 of 1..10 is 3",
        lambda: (percentile(TEN, 30) == 3, f"got {percentile(TEN, 30)!r}, want 3"),
    )

    # 5. p50: rank = ceil(0.50*10) = 5 -> index 4 -> value 5.
    check(
        "p50_ten",
        "nearest-rank p50 of 1..10 is 5",
        lambda: (percentile(TEN, 50) == 5, f"got {percentile(TEN, 50)!r}, want 5"),
    )

    # 6. p90: rank = ceil(0.90*10) = 9 -> index 8 -> value 9.
    check(
        "p90_ten",
        "nearest-rank p90 of 1..10 is 9",
        lambda: (percentile(TEN, 90) == 9, f"got {percentile(TEN, 90)!r}, want 9"),
    )

    # 7. p100 is exactly the maximum (endpoint).
    check(
        "p100_is_max",
        "p100 of 1..10 is the maximum 10",
        lambda: (percentile(TEN, 100) == 10, f"got {percentile(TEN, 100)!r}, want 10"),
    )

    # 8. p0 is the smallest element (endpoint).
    check(
        "p0_is_min",
        "p0 of 1..10 is the minimum 1",
        lambda: (percentile(TEN, 0) == 1, f"got {percentile(TEN, 0)!r}, want 1"),
    )

    # 9. Single-value sequence: every percentile is that value.
    check(
        "single_value",
        "percentile of a single-element list is that element for any p",
        lambda: (
            percentile([42], 0) == 42 and percentile([42], 50) == 42 and percentile([42], 100) == 42,
            f"got {(percentile([42], 0), percentile([42], 50), percentile([42], 100))!r}, want (42, 42, 42)",
        ),
    )

    # 10. mean unchanged (correct in the buggy starter; must stay correct).
    check(
        "mean_correct",
        "mean of 1..10 is 5.5",
        lambda: (approx(mean(TEN), 5.5), f"got {mean(TEN)!r}, want 5.5"),
    )

    # 11. minimum unchanged.
    check(
        "minimum_correct",
        "minimum of the skewed list is 1",
        lambda: (minimum(SKEW) == 1, f"got {minimum(SKEW)!r}, want 1"),
    )

    # 12. maximum unchanged.
    check(
        "maximum_correct",
        "maximum of the skewed list is 100000",
        lambda: (maximum(SKEW) == 100000, f"got {maximum(SKEW)!r}, want 100000"),
    )


# FIXED DENOMINATOR: the suite always reports out of this many checks, so a package
# that fails to import (zero checks appended) still scores 0.0, never 0/0 -> 1.0.
TOTAL = 12

passed = sum(1 for c in checks if c["passed"])
card = {
    "task": "fix_percentile",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": TOTAL,
    "score": 0.0 if not import_ok else round(passed / TOTAL, 4),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
