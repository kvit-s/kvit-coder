#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_backoff`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced `backoff`
package against the BRIEF'S CONTRACT (an exponential-backoff schedule whose
``Backoff(base, factor, cap).delay(attempt) -> float`` returns
``min(cap, base * factor ** attempt)`` in full floating-point precision, and
whose ``bounds(attempt) -> (low, high)`` returns the inclusive full-jitter range
``(0.0, delay(attempt))``), NOT against any particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- attempt indexing is shifted by one (``factor ** (attempt + 1)``),
    so attempt 0 already returns ``base * factor`` instead of ``base`` and the
    whole schedule waits one step too long.
  * BUG 2 -- the cap is applied to ``base`` BEFORE the exponential
    (``min(cap, base) * factor ** attempt``) instead of to the final product,
    so for the usual ``base < cap`` case the clamp never bites and the delay
    grows without bound, blowing far past ``cap``.
  * BUG 3 -- the delay is truncated with ``int(...)``, so a fractional ``base``
    (e.g. 0.5) collapses to 0 and the schedule jumps in whole-second steps,
    throwing away the fractional precision.
  * BUG 4 -- ``bounds`` returns ``(delay / 2, delay)`` (equal jitter) instead of
    the full-jitter ``(0.0, delay)``, so the low bound is never 0.

The general shape (delays grow, then stop growing near the ceiling) still looks
plausible on a casual glance, so a superficial fix can pass a few checks while
still failing the edge cases (attempt-0 exponent, cap-after-exponent, fractional
precision, the [0, delay] jitter window, and the cap interacting with jitter).

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

TOL = 1e-9  # delay/bounds comparison tolerance (float seconds)

# --- fixed denominator: the full roster of checks, declared before any run ----
# (id, human description). Kept in lockstep with the checks registered below so
# that an import failure can still report a result line for every single check.
CHECK_SPECS = [
    ("attempt_zero_is_base", "delay(0) == base exactly (exponent is attempt, not attempt+1)"),
    ("attempt_one_is_base_factor", "delay(1) == base * factor (one step into the schedule)"),
    ("exponential_growth", "delays double each attempt below the cap (factor**attempt)"),
    ("cap_clamps_high_attempt", "a high attempt is clamped down to cap, not grown past it"),
    ("cap_after_exponent", "small base * big exponent is still capped (cap applied to the product)"),
    ("never_exceeds_cap", "no attempt 0..40 ever returns more than cap"),
    ("delay_below_cap_exact", "an un-capped delay equals base*factor**attempt exactly"),
    ("fractional_base_preserved", "a fractional base (0.5) yields 0.5, not 0 (no truncation)"),
    ("fractional_growth_preserved", "fractional delays keep precision as they grow (0.5,1.0,2.0...)"),
    ("delay_returns_float", "delay always returns a float (even at base and at cap)"),
    ("factor_non_integer", "a non-integer factor (1.5) gives exact float powers, not truncated"),
    ("bounds_low_is_zero", "bounds(attempt) low is 0.0 (full jitter starts at zero)"),
    ("bounds_high_is_delay", "bounds(attempt) high equals delay(attempt)"),
    ("bounds_high_capped", "bounds high at a capped attempt equals cap, not the uncapped delay"),
    ("bounds_are_floats", "both bounds are floats"),
    ("bounds_match_delay_series", "bounds high tracks the full delay series across attempts"),
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


def expected_delay(base, factor, cap, attempt):
    """The contract's reference delay, computed independently of the package."""
    return min(cap, base * factor ** attempt)


# --- import the produced package (contract: backoff.public, fallback pkg) -----
import_ok = True
import_detail = ""
Backoff = None
try:
    try:
        mod = importlib.import_module("backoff.public")
    except Exception:
        mod = importlib.import_module("backoff")
    Backoff = getattr(mod, "Backoff")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. BUG 1: attempt 0 waits exactly base (factor ** 0 == 1).
    def c_attempt_zero_is_base():
        b = Backoff(base=2.0, factor=2.0, cap=1000.0)
        d = b.delay(0)
        return approx(d, 2.0), f"delay(0)={d!r} (expected base=2.0)"

    check("attempt_zero_is_base", c_attempt_zero_is_base)

    # 2. BUG 1 corollary: attempt 1 is base*factor, not base*factor**2.
    def c_attempt_one_is_base_factor():
        b = Backoff(base=2.0, factor=2.0, cap=1000.0)
        d = b.delay(1)
        return approx(d, 4.0), f"delay(1)={d!r} (expected base*factor=4.0)"

    check("attempt_one_is_base_factor", c_attempt_one_is_base_factor)

    # 3. exponential growth below the cap: 3,6,12,24 for base=3 factor=2.
    def c_exponential_growth():
        b = Backoff(base=3.0, factor=2.0, cap=10000.0)
        got = [b.delay(a) for a in range(4)]
        exp = [3.0, 6.0, 12.0, 24.0]
        ok = all(approx(g, e) for g, e in zip(got, exp))
        return ok, f"delays 0..3 = {got} (expected {exp})"

    check("exponential_growth", c_exponential_growth)

    # 4. BUG 2: a high attempt is clamped to cap (base=1, factor=2, attempt=20
    #    => 2**20 = 1048576, must clamp to cap=30).
    def c_cap_clamps_high_attempt():
        b = Backoff(base=1.0, factor=2.0, cap=30.0)
        d = b.delay(20)
        return approx(d, 30.0), f"delay(20)={d!r} (expected cap=30.0)"

    check("cap_clamps_high_attempt", c_cap_clamps_high_attempt)

    # 5. BUG 2 sharper: a SMALL base with a big exponent is still capped. If the
    #    cap is wrongly applied to base first (min(cap,base)=base) it never bites.
    def c_cap_after_exponent():
        b = Backoff(base=0.1, factor=3.0, cap=5.0)
        # 0.1 * 3**10 = 0.1 * 59049 = 5904.9 -> capped to 5.0
        d = b.delay(10)
        return approx(d, 5.0), f"delay(10)={d!r} (expected cap=5.0, not the uncapped 5904.9)"

    check("cap_after_exponent", c_cap_after_exponent)

    # 6. BUG 2 invariant: NOTHING in a long attempt range may exceed cap.
    def c_never_exceeds_cap():
        b = Backoff(base=0.5, factor=2.0, cap=20.0)
        over = [(a, b.delay(a)) for a in range(41) if b.delay(a) > 20.0 + TOL]
        return (not over), f"attempts exceeding cap=20: {over[:5]} (expected none)"

    check("never_exceeds_cap", c_never_exceeds_cap)

    # 7. an UN-capped delay matches the exact exponential (no early clamp, no
    #    off-by-one). base=2 factor=2 attempt=4 -> 32, cap high enough.
    def c_delay_below_cap_exact():
        b = Backoff(base=2.0, factor=2.0, cap=1000.0)
        d = b.delay(4)
        e = expected_delay(2.0, 2.0, 1000.0, 4)  # 32.0
        return approx(d, e), f"delay(4)={d!r} (expected {e})"

    check("delay_below_cap_exact", c_delay_below_cap_exact)

    # 8. BUG 3: a fractional base must survive (0.5 -> 0.5, not int()'d to 0).
    def c_fractional_base_preserved():
        b = Backoff(base=0.5, factor=2.0, cap=100.0)
        d = b.delay(0)
        return approx(d, 0.5), f"delay(0)={d!r} (expected 0.5, buggy truncates to 0)"

    check("fractional_base_preserved", c_fractional_base_preserved)

    # 9. BUG 3 sharper: the whole fractional series keeps precision.
    def c_fractional_growth_preserved():
        b = Backoff(base=0.5, factor=2.0, cap=100.0)
        got = [b.delay(a) for a in range(4)]
        exp = [0.5, 1.0, 2.0, 4.0]
        ok = all(approx(g, e) for g, e in zip(got, exp))
        return ok, f"fractional delays 0..3 = {got} (expected {exp})"

    check("fractional_growth_preserved", c_fractional_growth_preserved)

    # 10. delay always returns a float, both at base and clamped at cap.
    def c_delay_returns_float():
        b = Backoff(base=1.0, factor=2.0, cap=4.0)
        d0, dcap = b.delay(0), b.delay(10)
        ok = isinstance(d0, float) and isinstance(dcap, float)
        return ok, f"type(delay(0))={type(d0).__name__} type(delay(10))={type(dcap).__name__}"

    check("delay_returns_float", c_delay_returns_float)

    # 11. BUG 3 corollary: a non-integer factor gives exact float powers, not a
    #     truncated/integer result. base=1 factor=1.5 attempt=3 -> 3.375.
    def c_factor_non_integer():
        b = Backoff(base=1.0, factor=1.5, cap=1000.0)
        d = b.delay(3)
        return approx(d, 3.375), f"delay(3)={d!r} (expected 1*1.5**3=3.375)"

    check("factor_non_integer", c_factor_non_integer)

    # 12. BUG 4: full jitter starts at 0 -- the low bound is 0.0, not delay/2.
    def c_bounds_low_is_zero():
        b = Backoff(base=4.0, factor=2.0, cap=1000.0)
        low, _ = b.bounds(2)
        return approx(low, 0.0), f"bounds(2) low={low!r} (expected 0.0, buggy gives delay/2)"

    check("bounds_low_is_zero", c_bounds_low_is_zero)

    # 13. the high bound equals the delay for that attempt.
    def c_bounds_high_is_delay():
        b = Backoff(base=4.0, factor=2.0, cap=1000.0)
        low, high = b.bounds(2)
        d = b.delay(2)
        return (approx(low, 0.0) and approx(high, d)), \
            f"bounds(2)=({low!r},{high!r}) delay(2)={d!r} (expected (0.0, {d!r}))"

    check("bounds_high_is_delay", c_bounds_high_is_delay)

    # 14. the cap interacts with jitter: at a capped attempt the high bound is
    #     cap (the CAPPED delay), not the uncapped exponential.
    def c_bounds_high_capped():
        b = Backoff(base=1.0, factor=2.0, cap=8.0)
        low, high = b.bounds(15)  # 2**15 huge -> capped to 8
        return (approx(low, 0.0) and approx(high, 8.0)), \
            f"bounds(15)=({low!r},{high!r}) (expected (0.0, 8.0))"

    check("bounds_high_capped", c_bounds_high_capped)

    # 15. both bounds are floats.
    def c_bounds_are_floats():
        b = Backoff(base=2.0, factor=2.0, cap=50.0)
        low, high = b.bounds(3)
        ok = isinstance(low, float) and isinstance(high, float)
        return ok, f"types=({type(low).__name__},{type(high).__name__})"

    check("bounds_are_floats", c_bounds_are_floats)

    # 16. across a range, bounds high == the full (contract) delay series and
    #     low stays 0 -- ties the jitter window to the corrected delay end to end.
    def c_bounds_match_delay_series():
        b = Backoff(base=0.5, factor=2.0, cap=10.0)
        bad = []
        for a in range(8):
            low, high = b.bounds(a)
            exp = expected_delay(0.5, 2.0, 10.0, a)
            if not (approx(low, 0.0) and approx(high, exp)):
                bad.append((a, low, high, exp))
        return (not bad), f"mismatched (attempt,low,high,expected): {bad[:4]} (expected none)"

    check("bounds_match_delay_series", c_bounds_match_delay_series)


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
    "task": "fix_backoff",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
