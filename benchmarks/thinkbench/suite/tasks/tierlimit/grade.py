#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `tierlimit`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `tierlimit`
package against the BRIEF'S CONTRACT (per-key fixed-window limiting plus named
tiers, on top of the unchanged single-global-limit path), NOT against any
particular internal file layout.

The defining behaviors under test are the ones a SUPERFICIAL implementation gets
wrong:

  * PER-KEY independence — each key has its OWN window and count; a store that
    keeps one shared counter (or one shared window) lets one key spend another
    key's budget, or resets every key's window when any key crosses a boundary;
  * TIER CHANGE MID-WINDOW applies the new limit against the SAME window's
    EXISTING count, without resetting the count or starting a fresh window — a
    store that resets the count (or restarts the window) on `set_tier` mishandles
    both a downgrade (which should immediately push an over-budget key past its
    new, lower limit) and the not-retroactive rule (already-spent requests are
    kept, never refunded or revoked);
  * the GLOBAL path (`allow(now)`) and the bare `RateLimiter(limit, window)`
    constructor still work and stay isolated from the per-key machinery.

The shipped base limiter has only a single global counter and no `allow_key` /
`set_tier`, so it fails the whole feature while passing the global regression —
that is what makes the task discriminate (naive lands well under 1.0, a careful
per-key/tier implementation lands at 1.0).

Output: a single JSON scorecard on stdout. Each check runs in isolation, so the
score is continuous (passed / total), never all-or-nothing. FIXED DENOMINATOR:
the full check roster is declared up front, so an import failure records every
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

# --- fixed denominator: the full roster of checks, declared before any run ----
CHECK_SPECS = [
    ("default_tier_basic", "unassigned key uses the default tier's limit"),
    ("per_key_independent_count", "each key has its own count; one key's limit does not block another"),
    ("per_key_independent_window", "each key's window advances independently of other keys"),
    ("window_reset_per_key", "a key's count resets when it crosses into a new window"),
    ("explicit_tier_limit", "set_tier selects that tier's limit for the key"),
    ("upgrade_midwindow_keeps_count", "mid-window upgrade keeps the existing count and grants more room"),
    ("downgrade_midwindow_over", "mid-window downgrade pushes an over-budget key past the lower limit"),
    ("set_tier_not_retroactive", "set_tier does not refund/revoke already-decided requests in the window"),
    ("downgrade_then_new_window", "after a downgrade, the next window resets the count under the new tier"),
    ("tier_reassign_twice", "down-then-up mid-window tracks the live limit against the kept count"),
    ("boundary_window_math", "window boundary is half-open [w0, w0+window) via floor"),
    ("unknown_tier_raises", "set_tier with an unknown tier name raises ValueError"),
    ("bad_default_tier_raises", "constructing with a default_tier not in tiers raises ValueError"),
    ("regression_global_basic", "global allow(now) admits at most limit per window, resets each window"),
    ("regression_global_isolated", "global path and per-key path do not share budget"),
    ("regression_bare_constructor", "RateLimiter(limit, window) still constructs and runs the global path"),
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


# --- import the produced package (contract: tierlimit.public, fallback tierlimit)
import_ok = True
import_detail = ""
RateLimiter = None
try:
    try:
        mod = importlib.import_module("tierlimit.public")
    except Exception:
        mod = importlib.import_module("tierlimit")
    RateLimiter = getattr(mod, "RateLimiter")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# Standard tier table reused across checks: free=2, pro=5, default=free.
def make(limit=5, window=10.0, tiers=None, default_tier="free"):
    if tiers is None:
        tiers = {"free": 2, "pro": 5}
    return RateLimiter(limit, window, tiers=tiers, default_tier=default_tier)


if import_ok:
    # 1. an unassigned key falls back to the default tier (free, limit 2).
    def c_default_tier_basic():
        r = make()
        v = [r.allow_key("u", 0.0), r.allow_key("u", 1.0), r.allow_key("u", 2.0)]
        exp = [True, True, False]  # default free => 2 per window
        return v == exp, f"got {v} (expected {exp})"

    check("default_tier_basic", c_default_tier_basic)

    # 2. each key has its OWN count: exhausting one does not block another.
    def c_per_key_independent_count():
        r = make()
        a = [r.allow_key("a", 0.0), r.allow_key("a", 1.0), r.allow_key("a", 2.0)]  # T,T,F
        b = [r.allow_key("b", 3.0), r.allow_key("b", 4.0)]                          # T,T (fresh)
        return (a == [True, True, False] and b == [True, True]), \
            f"a={a} b={b} (expected a=[T,T,F] b=[T,T])"

    check("per_key_independent_count", c_per_key_independent_count)

    # 3. windows are per-key: key 'a' rolling to a new window must not reset 'b'.
    def c_per_key_independent_window():
        r = make()
        r.allow_key("a", 0.0)                 # a: 1/2 in [0,10)
        r.allow_key("b", 0.0)                 # b: 1/2 in [0,10)
        r.allow_key("b", 1.0)                 # b: 2/2 in [0,10)
        a_new = r.allow_key("a", 10.0)        # a crosses to [10,20): True
        # b is still in [0,10) and already at 2/2; it must STILL be denied
        # (a's window roll must not have reset b).
        b_again = r.allow_key("b", 2.0)
        return (a_new is True and b_again is False), \
            f"a_new={a_new} b_again={b_again} (expected True/False)"

    check("per_key_independent_window", c_per_key_independent_window)

    # 4. a single key's count resets when it crosses into the next window.
    def c_window_reset_per_key():
        r = make()
        v = [
            r.allow_key("u", 0.0),   # 1/2
            r.allow_key("u", 1.0),   # 2/2
            r.allow_key("u", 2.0),   # over -> False
            r.allow_key("u", 10.0),  # new window [10,20): 1/2 -> True
            r.allow_key("u", 11.0),  # 2/2 -> True
            r.allow_key("u", 12.0),  # over -> False
        ]
        exp = [True, True, False, True, True, False]
        return v == exp, f"got {v} (expected {exp})"

    check("window_reset_per_key", c_window_reset_per_key)

    # 5. an explicitly assigned tier selects that tier's limit.
    def c_explicit_tier_limit():
        r = make()
        r.set_tier("p", "pro")  # pro = 5
        v = [r.allow_key("p", float(i) * 0.1) for i in range(6)]  # all in [0,10)
        exp = [True, True, True, True, True, False]  # 5 allowed then denied
        return v == exp, f"got {v} (expected {exp})"

    check("explicit_tier_limit", c_explicit_tier_limit)

    # 6. THE upgrade trap: a mid-window upgrade KEEPS the existing count and
    #    grants more room in the SAME window (it does not reset to a fresh 5).
    def c_upgrade_midwindow_keeps_count():
        r = make()
        r.allow_key("u", 0.0)               # free: 1/2
        r.allow_key("u", 1.0)               # free: 2/2
        denied = r.allow_key("u", 2.0)      # over free -> False (count stays 2)
        r.set_tier("u", "pro")              # upgrade; count 2 preserved, limit now 5
        # room for exactly 3 more in this window (2 -> 5), then denied.
        after = [r.allow_key("u", 3.0 + i) for i in range(4)]  # T,T,T,F
        exp_after = [True, True, True, False]
        return (denied is False and after == exp_after), \
            f"denied={denied} after={after} (expected False, {exp_after})"

    check("upgrade_midwindow_keeps_count", c_upgrade_midwindow_keeps_count)

    # 7. THE downgrade trap: an over-budget key downgraded mid-window is at/over
    #    the new lower limit and is immediately denied for the rest of the window.
    def c_downgrade_midwindow_over():
        r = make()
        r.set_tier("u", "pro")              # pro = 5
        v = [r.allow_key("u", float(i)) for i in range(3)]  # 3 allowed (3/5), [0,10)
        r.set_tier("u", "free")             # downgrade to free=2; count already 3
        nxt = r.allow_key("u", 4.0)         # 3 >= 2 -> False, same window
        return (v == [True, True, True] and nxt is False), \
            f"first3={v} nxt={nxt} (expected [T,T,T], False)"

    check("downgrade_midwindow_over", c_downgrade_midwindow_over)

    # 8. set_tier is NOT retroactive: it neither refunds an earlier denial nor
    #    revokes an earlier grant; it only changes the limit going forward,
    #    measured against the count already spent.
    def c_set_tier_not_retroactive():
        r = make()
        r.allow_key("u", 0.0)               # free: 1/2
        r.allow_key("u", 1.0)               # free: 2/2
        r.set_tier("u", "pro")              # upgrade to 5; the earlier 2 are KEPT
        # If the count had been refunded to 0, this window would allow 5 here;
        # because the 2 are kept, only 3 more fit.
        after = [r.allow_key("u", 2.0 + i) for i in range(4)]  # T,T,T,F
        # And a fresh window starts clean at the live tier (pro=5).
        nxt_window = [r.allow_key("u", 10.0 + i) for i in range(6)]  # 5xT then F
        ok = (after == [True, True, True, False]
              and nxt_window == [True, True, True, True, True, False])
        return ok, f"after={after} nxt_window={nxt_window}"

    check("set_tier_not_retroactive", c_set_tier_not_retroactive)

    # 9. after a downgrade, the NEXT window resets the count cleanly under the
    #    new (lower) tier.
    def c_downgrade_then_new_window():
        r = make()
        r.set_tier("u", "pro")              # pro = 5
        for i in range(4):
            r.allow_key("u", float(i))      # 4/5 in [0,10)
        r.set_tier("u", "free")             # downgrade to 2; count 4 > 2
        over = r.allow_key("u", 5.0)        # denied in [0,10)
        # New window [10,20): count resets to 0, free=2 applies cleanly.
        fresh = [r.allow_key("u", 10.0), r.allow_key("u", 11.0), r.allow_key("u", 12.0)]
        return (over is False and fresh == [True, True, False]), \
            f"over={over} fresh={fresh} (expected False, [T,T,F])"

    check("downgrade_then_new_window", c_downgrade_then_new_window)

    # 10. reassigning the tier twice mid-window always uses the LIVE limit
    #     against the kept count (down then back up).
    def c_tier_reassign_twice():
        r = make()
        r.set_tier("u", "pro")              # 5
        r.allow_key("u", 0.0)               # 1/5
        r.allow_key("u", 1.0)               # 2/5
        r.allow_key("u", 2.0)               # 3/5
        r.set_tier("u", "free")             # 2; count 3 is over
        d = r.allow_key("u", 3.0)           # denied
        r.set_tier("u", "pro")              # back to 5; count still 3
        a = r.allow_key("u", 4.0)           # 3 < 5 -> True (4/5)
        return (d is False and a is True), f"after_down={d} after_up={a} (expected False/True)"

    check("tier_reassign_twice", c_tier_reassign_twice)

    # 11. window boundary is half-open [w0, w0+window): a request exactly at
    #     w0+window belongs to the NEXT window.
    def c_boundary_window_math():
        r = make(window=10.0)
        r.set_tier("u", "pro")              # 5
        r.allow_key("u", 9.999)             # 1/5 in [0,10)
        r.allow_key("u", 9.9999)            # 2/5 in [0,10)
        edge = r.allow_key("u", 10.0)       # exactly w0+window -> new window, 1/5
        # back-fill the FIRST window cannot happen (now is non-decreasing); the
        # point is that 10.0 started a fresh window, so it is allowed regardless
        # of the 2 already spent in [0,10).
        return edge is True, f"allow at 10.0 -> {edge} (expected True; fresh window)"

    check("boundary_window_math", c_boundary_window_math)

    # 12. set_tier with an unknown tier name raises ValueError.
    def c_unknown_tier_raises():
        r = make()
        try:
            r.set_tier("u", "platinum")
            return False, "set_tier with unknown tier did not raise"
        except ValueError:
            return True, "raised ValueError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, want ValueError"

    check("unknown_tier_raises", c_unknown_tier_raises)

    # 13. constructing with a default_tier absent from tiers raises ValueError.
    def c_bad_default_tier_raises():
        try:
            RateLimiter(5, 10.0, tiers={"free": 2}, default_tier="gold")
            return False, "bad default_tier did not raise"
        except ValueError:
            return True, "raised ValueError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, want ValueError"

    check("bad_default_tier_raises", c_bad_default_tier_raises)

    # 14. REGRESSION: the GLOBAL allow(now) path still works (limit per window,
    #     resets each window).
    def c_regression_global_basic():
        r = RateLimiter(2, 10.0)
        v = [
            r.allow(0.0),   # 1/2
            r.allow(1.0),   # 2/2
            r.allow(2.0),   # over -> False
            r.allow(10.0),  # new window -> True
            r.allow(11.0),  # 2/2 -> True
            r.allow(12.0),  # over -> False
        ]
        exp = [True, True, False, True, True, False]
        return v == exp, f"got {v} (expected {exp})"

    check("regression_global_basic", c_regression_global_basic)

    # 15. REGRESSION: the global path and the per-key path do not share budget.
    def c_regression_global_isolated():
        r = make(limit=2, window=10.0)   # global limit 2, tiers free=2/pro=5
        g1 = r.allow(0.0)                # global 1/2
        g2 = r.allow(1.0)                # global 2/2
        g3 = r.allow(2.0)                # global over -> False
        # The per-key budget for 'u' must be untouched by the global calls.
        k = [r.allow_key("u", 3.0), r.allow_key("u", 4.0), r.allow_key("u", 5.0)]
        # ...and the per-key calls must not have refilled the global budget.
        g4 = r.allow(6.0)               # still in [0,10), global already exhausted
        ok = (g1 is True and g2 is True and g3 is False
              and k == [True, True, False] and g4 is False)
        return ok, f"global=[{g1},{g2},{g3},{g4}] key={k}"

    check("regression_global_isolated", c_regression_global_isolated)

    # 16. REGRESSION: the bare RateLimiter(limit, window) constructor still works
    #     (implicit single 'default' tier; global path runs).
    def c_regression_bare_constructor():
        r = RateLimiter(3, 5.0)
        v = [r.allow(0.0), r.allow(1.0), r.allow(2.0), r.allow(3.0)]  # 3 then deny
        exp = [True, True, True, False]
        return v == exp, f"got {v} (expected {exp})"

    check("regression_bare_constructor", c_regression_bare_constructor)


# --- assemble the scorecard with a FIXED denominator -------------------------
checks_out = []
for cid in CHECK_IDS:
    r = results.get(cid)
    if r is None:
        r = {"passed": False, "detail": "not run (import failed)" if not import_ok else "not run"}
    checks_out.append({"id": cid, "desc": DESC[cid], "passed": r["passed"], "detail": r["detail"]})

passed = sum(1 for c in checks_out if c["passed"])
total = len(checks_out)  # always len(CHECK_SPECS): fixed denominator
card = {
    "task": "tierlimit",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
