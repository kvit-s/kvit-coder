#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_movavg`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced `movavg`
package against the BRIEF'S CONTRACT (a fixed-size sliding window whose
`Window(size).add(x)` retains the most recent `size` values and whose
`mean()`/`min()`/`max()` report statistics over exactly the values currently in
the window), NOT against any particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- eviction is off by one (`len > size + 1`), so the window keeps
    `size + 1` values instead of `size`; every full-window statistic is computed
    over one stale extra sample.
  * BUG 2 -- a partial window (fewer than `size` values added) divides the sum
    by `size` instead of by the count actually present, under-reporting the
    mean before the window fills.
  * BUG 3 -- `min()`/`max()` are cached and only updated on `add`, never
    recomputed on eviction, so once the sample holding the extreme slides out
    the reported extreme goes stale.
  * BUG 4 -- `mean()` uses integer floor division, truncating a fractional mean
    (1, 2 -> 1 instead of 1.5).

Basic stats on a short, all-increasing sequence still look correct, so a
superficial fix can pass the easy checks while still failing the edge cases.

Output: a single JSON scorecard on stdout. Each check runs in isolation, so the
score is continuous (passed / total), never all-or-nothing. FIXED DENOMINATOR:
the full check list is registered up front, so an import failure records every
check as failed and forces score 0.0. Exit code is 0 whenever grading ran to
completion (even score 0.0); the process never raises out.
"""
import importlib
import json
import os
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

TOL = 1e-9  # mean comparison tolerance (float division)

# --- fixed denominator: the full roster of checks, declared before any run ----
# (id, human description). Kept in lockstep with the checks registered below so
# that an import failure can still report a result line for every single check.
CHECK_SPECS = [
    ("basic_full_mean", "a filled window reports the mean of exactly the last `size` values"),
    ("basic_min_max", "min/max over a simple filled window are correct"),
    ("len_tracks_count", "len() is the count present: < size while filling, == size once full"),
    ("evicts_to_size", "after many adds the window holds exactly `size`, not `size+1`"),
    ("rolling_mean_window", "the rolling mean uses only the last `size` values (no stale extra)"),
    ("partial_mean_two", "a partial window means over the values present, not over `size`"),
    ("partial_mean_one", "a single value gives mean == that value (divide by 1, not size)"),
    ("partial_then_full", "mean is right both before and after the window first fills"),
    ("min_recovers_after_evict", "min recovers once the sample holding it is evicted"),
    ("max_recovers_after_evict", "max recovers once the sample holding it is evicted"),
    ("extreme_tracks_window", "min/max reflect the live window across a long shifting stream"),
    ("fractional_mean", "a fractional mean is returned exactly (1,2 -> 1.5), not truncated"),
    ("fractional_mean_float", "mean returns a float type, not a truncated int"),
    ("negative_values", "min/max/mean are correct with negative values in the window"),
    ("size_one_window", "a size-1 window always reflects only the most recent value"),
    ("empty_raises", "mean/min/max on an empty window raise (no value yet)"),
]
CHECK_IDS = [cid for cid, _ in CHECK_SPECS]
DESC = dict(CHECK_SPECS)

results = {}  # cid -> {"passed": bool, "detail": str}


def record(cid, passed, detail=""):
    results[cid] = {"passed": bool(passed), "detail": str(detail or "")}


def check(cid, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


def approx(a, b, tol=TOL):
    return abs(a - b) <= tol


def feed(Window, size, values):
    """Build a window of `size`, push every value in `values`, return it."""
    w = Window(size)
    for v in values:
        w.add(v)
    return w


# --- import the produced package (contract: movavg.public, fallback pkg) ------
import_ok = True
import_detail = ""
Window = None
try:
    try:
        mod = importlib.import_module("movavg.public")
    except Exception:
        mod = importlib.import_module("movavg")
    Window = getattr(mod, "Window")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. baseline: a window filled to exactly `size` means over those `size`.
    def c_basic_full_mean():
        w = feed(Window, 3, [10, 20, 30])
        exp = sum([10, 20, 30]) / 3
        return approx(w.mean(), exp), f"mean={w.mean()!r} expected={exp!r}"

    check("basic_full_mean", c_basic_full_mean)

    # 2. baseline: min/max on a simple filled window.
    def c_basic_min_max():
        w = feed(Window, 4, [7, 3, 9, 5])
        return (w.min() == 3 and w.max() == 9), f"min={w.min()!r} max={w.max()!r} (expected 3/9)"

    check("basic_min_max", c_basic_min_max)

    # 3. len() reflects how many values are present: rises while filling, then
    #    pins at size.
    def c_len_tracks_count():
        w = Window(3)
        seen = []
        for v in [1, 2, 3, 4, 5]:
            w.add(v)
            seen.append(len(w))
        return seen == [1, 2, 3, 3, 3], f"len progression={seen} (expected [1,2,3,3,3])"

    check("len_tracks_count", c_len_tracks_count)

    # 4. BUG 1: after many adds the window holds EXACTLY size, never size+1.
    def c_evicts_to_size():
        w = feed(Window, 3, list(range(100)))
        return len(w) == 3, f"len after 100 adds (size=3) -> {len(w)!r} (expected 3)"

    check("evicts_to_size", c_evicts_to_size)

    # 5. BUG 1 sharper: the rolling mean must be over the LAST `size` values only.
    #    With size=3 and a long run, the window is the last 3 -> [97,98,99].
    def c_rolling_mean_window():
        vals = list(range(100))
        w = feed(Window, 3, vals)
        exp = sum(vals[-3:]) / 3  # mean of [97,98,99] = 98.0
        # A keep-one-too-many bug averages [96,97,98,99]/3 or similar -> wrong.
        return approx(w.mean(), exp), f"rolling mean={w.mean()!r} expected={exp!r}"

    check("rolling_mean_window", c_rolling_mean_window)

    # 6. BUG 2: a partial window (2 of 5) means over the 2 present, not over 5.
    def c_partial_mean_two():
        w = feed(Window, 5, [10, 20])
        exp = 30 / 2  # 15.0, NOT 30/5 == 6.0
        return approx(w.mean(), exp), f"partial mean={w.mean()!r} expected={exp!r} (not /size)"

    check("partial_mean_two", c_partial_mean_two)

    # 7. BUG 2 sharp: one value -> mean is that value (divide by 1).
    def c_partial_mean_one():
        w = feed(Window, 10, [42])
        return approx(w.mean(), 42.0), f"mean of one value (size=10) -> {w.mean()!r} (expected 42.0)"

    check("partial_mean_one", c_partial_mean_one)

    # 8. BUG 2 continuity: correct both before AND after the window fills.
    def c_partial_then_full():
        w = Window(3)
        w.add(6)
        m1 = w.mean()                 # 6.0
        w.add(12)
        m2 = w.mean()                 # 9.0  (18/2, not 18/3)
        w.add(0)
        m3 = w.mean()                 # 6.0  (18/3, full)
        w.add(9)                      # evict 6 -> [12,0,9]
        m4 = w.mean()                 # 7.0
        ok = (approx(m1, 6.0) and approx(m2, 9.0) and approx(m3, 6.0) and approx(m4, 7.0))
        return ok, f"means={[m1, m2, m3, m4]} expected [6.0, 9.0, 6.0, 7.0]"

    check("partial_then_full", c_partial_then_full)

    # 9. BUG 3: min must recover after the sample holding it is evicted.
    def c_min_recovers_after_evict():
        # size 3. Push a tiny value first, then push it out of the window.
        w = feed(Window, 3, [1, 50, 60, 70])  # window now [50,60,70]; the 1 is gone
        return w.min() == 50, f"min after evicting the 1 -> {w.min()!r} (expected 50, not stale 1)"

    check("min_recovers_after_evict", c_min_recovers_after_evict)

    # 10. BUG 3 mirror: max must recover after the sample holding it is evicted.
    def c_max_recovers_after_evict():
        w = feed(Window, 3, [100, 5, 6, 7])  # window now [5,6,7]; the 100 is gone
        return w.max() == 7, f"max after evicting the 100 -> {w.max()!r} (expected 7, not stale 100)"

    check("max_recovers_after_evict", c_max_recovers_after_evict)

    # 11. BUG 3 sustained: extremes track a long shifting stream at every step.
    def c_extreme_tracks_window():
        size = 4
        w = Window(size)
        vals = [5, 1, 9, 3, 8, 2, 7, 0, 6, 4, 10, 1, 1, 1, 1]
        bad = []
        for i, v in enumerate(vals):
            w.add(v)
            window = vals[max(0, i - size + 1): i + 1]
            if w.min() != min(window) or w.max() != max(window):
                bad.append((i, w.min(), w.max(), min(window), max(window)))
        return not bad, f"mismatches (idx,gotmin,gotmax,expmin,expmax)={bad[:3]}"

    check("extreme_tracks_window", c_extreme_tracks_window)

    # 12. BUG 4: a fractional mean is returned exactly, not floor-truncated.
    def c_fractional_mean():
        w = feed(Window, 2, [1, 2])
        return approx(w.mean(), 1.5), f"mean of [1,2] -> {w.mean()!r} (expected 1.5, not 1)"

    check("fractional_mean", c_fractional_mean)

    # 13. BUG 4 type: mean must be a real float, not an int from // truncation.
    def c_fractional_mean_float():
        w = feed(Window, 3, [1, 2, 2])  # 5/3 = 1.666...
        m = w.mean()
        return (isinstance(m, float) and approx(m, 5 / 3)), \
            f"mean={m!r} type={type(m).__name__} (expected float ~1.6667)"

    check("fractional_mean_float", c_fractional_mean_float)

    # 14. negative values: min/max/mean stay correct with negatives in-window.
    def c_negative_values():
        w = feed(Window, 4, [-5, -1, -8, -3])
        okmin = w.min() == -8
        okmax = w.max() == -1
        okmean = approx(w.mean(), (-5 + -1 + -8 + -3) / 4)  # -4.25
        return (okmin and okmax and okmean), \
            f"min={w.min()!r} max={w.max()!r} mean={w.mean()!r} (expected -8/-1/-4.25)"

    check("negative_values", c_negative_values)

    # 15. size-1 window always reflects only the most recent value (every stat).
    def c_size_one_window():
        w = Window(1)
        w.add(3)
        w.add(8)
        w.add(2)
        ok = (len(w) == 1 and approx(w.mean(), 2.0) and w.min() == 2 and w.max() == 2)
        return ok, f"len={len(w)} mean={w.mean()!r} min={w.min()!r} max={w.max()!r} (all reflect 2)"

    check("size_one_window", c_size_one_window)

    # 16. an empty window (nothing added yet) raises on every statistic.
    def c_empty_raises():
        w = Window(3)
        raised = []
        for name in ("mean", "min", "max"):
            try:
                getattr(w, name)()
                raised.append((name, False))
            except Exception:  # noqa: BLE001 - any raise satisfies the contract
                raised.append((name, True))
        ok = all(r for _, r in raised)
        return ok, f"raised={raised} (each statistic must raise on empty)"

    check("empty_raises", c_empty_raises)


# --- assemble the scorecard with a FIXED denominator -------------------------
checks_out = []
for cid in CHECK_IDS:
    r = results.get(cid)
    if r is None:
        # Not run (e.g. import failed): record as a failed check, keep denominator.
        r = {"passed": False, "detail": "not run (import failed)" if not import_ok else "not run"}
    checks_out.append({"id": cid, "desc": DESC[cid], "passed": r["passed"], "detail": r["detail"]})

passed = sum(1 for c in checks_out if c["passed"])
total = len(checks_out)  # always len(CHECK_SPECS): fixed denominator
card = {
    "task": "fix_movavg",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
