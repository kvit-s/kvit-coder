#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_tokenbucket`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced
`tokenbucket` package against the BRIEF'S CONTRACT (a continuous-refill token
bucket whose `TokenBucket(capacity, refill_per_sec).allow(now, cost=1) -> bool`
refills for the elapsed time, caps AFTER refilling, consumes `cost` only when
the request is allowed, and never goes backwards in time), NOT against any
particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- it caps the token count to capacity BEFORE crediting the elapsed
    refill instead of after, so a long idle gap blows the bucket far past
    capacity (a permanent over-credit).
  * BUG 2 -- it truncates each refill with `int(elapsed * rate)`, so many tiny
    sub-token steps each round down to zero and the fractional credit is lost
    forever even though `_last` advances.
  * BUG 3 -- a DENIED request still subtracts `cost`, draining the bucket (and
    clamping to 0) even though nothing was granted.
  * BUG 4 -- elapsed time is not clamped, so a backwards `now` (clock blip)
    subtracts tokens and rewinds the internal clock.

Basic allow/deny on coarse, whole-second steps still looks correct, so a
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

TOL = 1e-6  # token-count comparison tolerance (fractional bucket)

# --- fixed denominator: the full roster of checks, declared before any run ----
# (id, human description). Kept in lockstep with the checks registered below so
# that an import failure can still report a result line for every single check.
CHECK_SPECS = [
    ("starts_full_allows", "a fresh full bucket allows up to capacity immediately"),
    ("empties_then_denies", "draining the bucket then asking again is denied"),
    ("coarse_refill", "a whole-second wait refills the expected whole tokens"),
    ("denied_consumes_nothing", "a DENIED request leaves the token count unchanged"),
    ("denied_then_exact", "after a denial the still-present tokens are spendable"),
    ("cost_gt_one_allow", "cost>1 is allowed and consumes exactly cost tokens"),
    ("cost_gt_one_boundary", "cost equal to available tokens is allowed (>=, not >)"),
    ("cost_gt_one_over", "cost one above available is denied without draining"),
    ("cap_after_refill", "a long idle gap caps the bucket AT capacity, not above"),
    ("cap_exact_no_overflow", "refill that exactly reaches capacity does not overflow"),
    ("fractional_refill_accrues", "many tiny sub-token steps accrue real fractional credit"),
    ("fractional_partial", "a fractional refill grants the proportional token amount"),
    ("monotonic_clamp_no_gain", "a backwards `now` neither adds nor removes tokens"),
    ("monotonic_clamp_recovers", "after a backwards blip, forward time refills from the real last"),
    ("burst_then_wait_recovery", "a burst that empties the bucket recovers after waiting"),
    ("sustained_rate", "over a long run the allow-rate tracks refill_per_sec"),
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


# --- import the produced package (contract: tokenbucket.public, fallback pkg) --
import_ok = True
import_detail = ""
TokenBucket = None
try:
    try:
        mod = importlib.import_module("tokenbucket.public")
    except Exception:
        mod = importlib.import_module("tokenbucket")
    TokenBucket = getattr(mod, "TokenBucket")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. baseline: a fresh bucket starts full and grants up to capacity at once.
    def c_starts_full_allows():
        b = TokenBucket(capacity=5, refill_per_sec=1.0)
        got = [b.allow(now=0.0) for _ in range(5)]
        return all(got), f"first 5 allows @t=0 -> {got}"

    check("starts_full_allows", c_starts_full_allows)

    # 2. once empty (no time passed), further requests are denied.
    def c_empties_then_denies():
        b = TokenBucket(capacity=3, refill_per_sec=1.0)
        for _ in range(3):
            b.allow(now=0.0)
        denied = b.allow(now=0.0)
        return denied is False, f"4th allow @t=0 -> {denied!r} (expected False)"

    check("empties_then_denies", c_empties_then_denies)

    # 3. coarse whole-second refill: drain, wait 4s -> exactly 4 grants then deny.
    def c_coarse_refill():
        b = TokenBucket(capacity=10, refill_per_sec=1.0)
        for _ in range(10):
            b.allow(now=0.0)            # empty
        granted = sum(1 for _ in range(4) if b.allow(now=4.0))
        # at t=4 the bucket holds 4 tokens -> exactly 4 grants, 5th denied
        fifth = b.allow(now=4.0)
        return (granted == 4 and fifth is False), f"granted={granted} fifth={fifth!r}"

    check("coarse_refill", c_coarse_refill)

    # 4. BUG 3: a denied request must consume NOTHING.
    def c_denied_consumes_nothing():
        b = TokenBucket(capacity=10, refill_per_sec=1.0)
        b.allow(now=0.0, cost=8)        # 2 left
        before = b.tokens
        res = b.allow(now=0.0, cost=5)  # need 5, have 2 -> deny
        after = b.tokens
        return (res is False and approx(before, 2.0) and approx(after, 2.0)), \
            f"deny={res!r} before={before!r} after={after!r} (expected 2 unchanged)"

    check("denied_consumes_nothing", c_denied_consumes_nothing)

    # 5. BUG 3 corollary: the tokens survive a denial and remain spendable.
    def c_denied_then_exact():
        b = TokenBucket(capacity=10, refill_per_sec=1.0)
        b.allow(now=0.0, cost=8)        # 2 left
        b.allow(now=0.0, cost=5)        # denied (must not drain)
        a = b.allow(now=0.0, cost=2)    # the 2 should still be there
        b2 = b.allow(now=0.0, cost=1)   # now truly empty
        return (a is True and b2 is False), f"spend2={a!r} thenSpend1={b2!r}"

    check("denied_then_exact", c_denied_then_exact)

    # 6. cost>1 is granted and consumes exactly that many tokens.
    def c_cost_gt_one_allow():
        b = TokenBucket(capacity=10, refill_per_sec=1.0)
        res = b.allow(now=0.0, cost=4)
        return (res is True and approx(b.tokens, 6.0)), f"allow(cost=4)->{res!r} tokens={b.tokens!r}"

    check("cost_gt_one_allow", c_cost_gt_one_allow)

    # 7. cost exactly equal to the available tokens is allowed (>= boundary).
    def c_cost_gt_one_boundary():
        b = TokenBucket(capacity=6, refill_per_sec=1.0)
        res = b.allow(now=0.0, cost=6)  # exactly capacity
        empty = b.allow(now=0.0, cost=1)
        return (res is True and empty is False and approx(b.tokens, 0.0)), \
            f"allow(cost=6)->{res!r} next->{empty!r} tokens={b.tokens!r}"

    check("cost_gt_one_boundary", c_cost_gt_one_boundary)

    # 8. cost one above the available count is denied -- and (BUG 3) not drained.
    def c_cost_gt_one_over():
        b = TokenBucket(capacity=5, refill_per_sec=1.0)
        b.allow(now=0.0, cost=2)        # 3 left
        res = b.allow(now=0.0, cost=4)  # need 4, have 3 -> deny, keep 3
        return (res is False and approx(b.tokens, 3.0)), f"allow(cost=4)->{res!r} tokens={b.tokens!r}"

    check("cost_gt_one_over", c_cost_gt_one_over)

    # 9. BUG 1: a long idle gap must cap AT capacity (refill THEN cap), not above.
    def c_cap_after_refill():
        b = TokenBucket(capacity=10, refill_per_sec=5.0)
        b.allow(now=0.0, cost=1)        # 9 tokens, last=0
        b.allow(now=1000.0, cost=1)     # +5000 then cap to 10, consume 1 -> 9
        # bucket must never exceed capacity; after consuming 1 it should hold 9.
        return approx(b.tokens, 9.0), f"tokens after 1000s idle (cap=10) -> {b.tokens!r} (expected 9)"

    check("cap_after_refill", c_cap_after_refill)

    # 10. BUG 1 sharper: refilling to exactly capacity must not overflow capacity.
    def c_cap_exact_no_overflow():
        b = TokenBucket(capacity=8, refill_per_sec=2.0)
        for _ in range(8):
            b.allow(now=0.0)            # empty, last=0
        # wait 10s -> +20 tokens, capped at 8. Then exactly 8 grants, 9th denied.
        granted = sum(1 for _ in range(8) if b.allow(now=10.0))
        ninth = b.allow(now=10.0)
        return (granted == 8 and ninth is False), f"granted={granted} ninth={ninth!r} (expected 8/False)"

    check("cap_exact_no_overflow", c_cap_exact_no_overflow)

    # 11. BUG 2: many tiny sub-token steps must accrue REAL fractional credit.
    def c_fractional_refill_accrues():
        b = TokenBucket(capacity=100, refill_per_sec=1.0)
        b.allow(now=0.0, cost=100)      # drain to 0, last=0
        t = 0.0
        for _ in range(500):
            t += 0.01                    # 0.01 token/step, each alone < 1 -> denied
            b.allow(now=t, cost=1)
        # 5.0s total at rate 1 => ~5 tokens accrued; the allow()s consumed ~4,
        # leaving ~1. Buggy truncation leaves ~0. Require materially > 0.5.
        return (b.tokens > 0.5), f"tokens after 500 x 0.01s steps -> {b.tokens!r} (expected ~1, buggy ~0)"

    check("fractional_refill_accrues", c_fractional_refill_accrues)

    # 12. BUG 2 sharper: a single fractional wait grants the proportional amount.
    def c_fractional_partial():
        b = TokenBucket(capacity=10, refill_per_sec=2.0)
        b.allow(now=0.0, cost=10)       # empty, last=0
        # wait 1.5s at 2 tok/s -> 3.0 tokens. cost=3 must be allowed; cost=4 denied.
        ok3 = b.allow(now=1.5, cost=3)
        # after consuming 3 of 3, empty -> a 1-cost must be denied.
        empty = b.allow(now=1.5, cost=1)
        return (ok3 is True and empty is False), f"allow(cost=3 @1.5s)->{ok3!r} next->{empty!r}"

    check("fractional_partial", c_fractional_partial)

    # 13. BUG 4: a backwards `now` must not add OR remove tokens.
    def c_monotonic_clamp_no_gain():
        b = TokenBucket(capacity=10, refill_per_sec=2.0)
        b.allow(now=5.0, cost=10)       # empty at t=5, last=5
        before = b.tokens               # 0.0
        res = b.allow(now=2.0, cost=1)  # clock blips backward -> deny, no change
        after = b.tokens
        return (res is False and approx(before, 0.0) and approx(after, 0.0)), \
            f"backward allow->{res!r} before={before!r} after={after!r} (expected 0 unchanged)"

    check("monotonic_clamp_no_gain", c_monotonic_clamp_no_gain)

    # 14. BUG 4 corollary: forward progress after a blip refills from the REAL last.
    def c_monotonic_clamp_recovers():
        b = TokenBucket(capacity=10, refill_per_sec=2.0)
        b.allow(now=5.0, cost=10)       # empty, last=5
        b.allow(now=2.0, cost=1)        # backward blip (denied); last must stay 5
        # advance to t=6: that's 1s past the REAL last (5) -> +2 tokens, not from t=2.
        ok2 = b.allow(now=6.0, cost=2)  # exactly 2 available
        empty = b.allow(now=6.0, cost=1)
        return (ok2 is True and empty is False), f"recover allow(cost=2 @t=6)->{ok2!r} next->{empty!r}"

    check("monotonic_clamp_recovers", c_monotonic_clamp_recovers)

    # 15. burst-then-wait recovery: empty in a burst, wait, partial refill grants.
    def c_burst_then_wait_recovery():
        b = TokenBucket(capacity=20, refill_per_sec=4.0)
        burst = sum(1 for _ in range(20) if b.allow(now=0.0))   # 20 grants
        denied = b.allow(now=0.0)                                # 21st denied
        # wait 2.5s at 4 tok/s -> +10 tokens.
        recov = sum(1 for _ in range(10) if b.allow(now=2.5))    # exactly 10
        eleventh = b.allow(now=2.5)
        return (burst == 20 and denied is False and recov == 10 and eleventh is False), \
            f"burst={burst} denied={denied!r} recov={recov} 11th={eleventh!r}"

    check("burst_then_wait_recovery", c_burst_then_wait_recovery)

    # 16. sustained-rate sanity: over many seconds the long-run allow rate tracks
    #     refill_per_sec (this is where lost-fraction / over-credit bugs show up).
    #     Capacity is large so the cap never interferes; the rate is the only
    #     limit. 1000 attempts at 0.05s spacing -> 50s of wall time at 10 tok/s,
    #     so the bucket can grant ~500 (one grant per ~0.1s). The lost-fraction
    #     bug accrues ~nothing (far below); any over-credit bug exceeds 500.
    def c_sustained_rate():
        b = TokenBucket(capacity=1000, refill_per_sec=10.0)
        b.allow(now=0.0, cost=1000)     # drain to empty, last=0
        granted = 0
        t = 0.0
        for _ in range(1000):
            t += 0.05
            if b.allow(now=t):
                granted += 1
        return (495 <= granted <= 500), f"granted={granted} over 50s @10tok/s cap=1000 (expected ~500)"

    check("sustained_rate", c_sustained_rate)


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
    "task": "fix_tokenbucket",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
