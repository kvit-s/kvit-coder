#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_ttlcache`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `ttlcache`
package against the BRIEF'S CONTRACT (correct TTL-boundary expiry on an
injectable fake clock, plus the unchanged public API), NOT against any
particular internal file layout.

The defining behavior under test: an entry set with ttl=T at time t0 must be a
HIT for every clock value in [t0, t0+T) and a MISS at and after t0+T. The
shipped (buggy) code serves entries one tick PAST their TTL (it uses
`now > expires_at` instead of `now >= expires_at`), so the boundary checks here
FAIL on the buggy code and PASS on the fixed code — that's what makes the task
discriminate the fix.

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

# --- fixed denominator: the full roster of checks, declared before any run ----
# (id, human description). Kept in lockstep with the CHECKS table below so that
# an import failure can still report a result line for every single check.
CHECK_SPECS = [
    ("set_get_basic", "a freshly set value is returned before its TTL elapses"),
    ("boundary_just_before", "value is a HIT one tick before the TTL elapses"),
    ("boundary_exact", "value is a MISS at the exact instant the TTL elapses"),
    ("boundary_after", "value is a MISS after the TTL has elapsed"),
    ("ttl_zero_expired", "ttl=0 entry is already expired (never served)"),
    ("ttl_negative_expired", "negative ttl entry is already expired"),
    ("miss_unknown_key", "an unknown key is a miss / returns the default"),
    ("default_returned", "the supplied default is returned on a miss"),
    ("stats_hits_misses", "hits/misses counters reflect the corrected boundary"),
    ("stats_expiration_counted", "an expired-on-access entry increments expirations"),
    ("contains_boundary", "membership (`in`) expires exactly at the TTL boundary"),
    ("ttl_remaining_boundary", "ttl_remaining is None at and after the TTL boundary"),
    ("len_counts_live_only", "len() counts only live (non-expired) entries"),
    ("purge_removes_expired", "purge() evicts entries that are at/after their TTL"),
    ("overwrite_resets_ttl", "re-setting a key resets its TTL from the new set time"),
    ("delete_removes", "delete() removes a present key and reports it"),
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


class FakeClock:
    """Deterministic, manually-advanced clock for the injectable `clock=`."""

    def __init__(self, start=0):
        self.t = start

    def __call__(self):
        return self.t

    def set(self, t):
        self.t = t


# --- import the produced package (contract: ttlcache.public, fallback ttlcache)
import_ok = True
import_detail = ""
Cache = None
try:
    try:
        mod = importlib.import_module("ttlcache.public")
    except Exception:
        mod = importlib.import_module("ttlcache")
    Cache = getattr(mod, "Cache")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. baseline: a value set is readable while clearly fresh.
    def c_set_get_basic():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", 1, ttl=10)
        clk.set(3)
        v = c.get("a")
        return v == 1, f"get @t=3 -> {v!r}"

    check("set_get_basic", c_set_get_basic)

    # 2. HIT one tick before the boundary (t0+T-1).
    def c_boundary_just_before():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", "v", ttl=10)
        clk.set(9)
        v = c.get("a")
        return v == "v", f"get @t=9 (ttl=10) -> {v!r} (expected hit)"

    check("boundary_just_before", c_boundary_just_before)

    # 3. THE bug: MISS exactly at the boundary (t0+T). Buggy code returns the value.
    def c_boundary_exact():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", "v", ttl=10)
        clk.set(10)
        v = c.get("a")
        return v is None, f"get @t=10 (ttl=10) -> {v!r} (expected miss/None)"

    check("boundary_exact", c_boundary_exact)

    # 4. MISS strictly after the boundary.
    def c_boundary_after():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", "v", ttl=10)
        clk.set(11)
        v = c.get("a")
        return v is None, f"get @t=11 (ttl=10) -> {v!r} (expected miss/None)"

    check("boundary_after", c_boundary_after)

    # 5. ttl=0 means already-expired: never served, even at the same instant.
    def c_ttl_zero_expired():
        clk = FakeClock(5)
        c = Cache(clock=clk)
        c.set("a", "v", ttl=0)
        v = c.get("a")  # same tick
        return v is None, f"get @same tick (ttl=0) -> {v!r} (expected None)"

    check("ttl_zero_expired", c_ttl_zero_expired)

    # 6. negative ttl is already expired.
    def c_ttl_negative_expired():
        clk = FakeClock(5)
        c = Cache(clock=clk)
        c.set("a", "v", ttl=-3)
        v = c.get("a")
        return v is None, f"get (ttl=-3) -> {v!r} (expected None)"

    check("ttl_negative_expired", c_ttl_negative_expired)

    # 7. unknown key is a miss.
    def c_miss_unknown_key():
        c = Cache(clock=FakeClock(0))
        v = c.get("nope")
        return v is None, f"get unknown -> {v!r}"

    check("miss_unknown_key", c_miss_unknown_key)

    # 8. supplied default flows through on a miss.
    def c_default_returned():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", "v", ttl=10)
        clk.set(10)  # boundary -> expired
        v = c.get("a", default="SENT")
        miss_unknown = c.get("ghost", default="SENT2")
        return (v == "SENT" and miss_unknown == "SENT2"), f"expired->{v!r} unknown->{miss_unknown!r}"

    check("default_returned", c_default_returned)

    # 9. hits/misses counters reflect the corrected boundary.
    def c_stats_hits_misses():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", 1, ttl=10)
        clk.set(9)
        c.get("a")              # hit
        clk.set(10)
        c.get("a")              # miss (boundary)
        c.get("ghost")          # miss
        s = c.stats
        hits = getattr(s, "hits", None)
        misses = getattr(s, "misses", None)
        return (hits == 1 and misses == 2), f"hits={hits!r} misses={misses!r} (expected 1/2)"

    check("stats_hits_misses", c_stats_hits_misses)

    # 10. an expired-on-access entry increments the expirations counter.
    def c_stats_expiration_counted():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", 1, ttl=10)
        clk.set(10)             # boundary -> expired on access
        c.get("a")
        exp = getattr(c.stats, "expirations", None)
        return (exp == 1), f"expirations={exp!r} (expected 1)"

    check("stats_expiration_counted", c_stats_expiration_counted)

    # 11. membership (`in`) honors the same corrected boundary.
    def c_contains_boundary():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", 1, ttl=10)
        clk.set(9)
        before = ("a" in c)
        clk.set(10)
        at = ("a" in c)
        return (before is True and at is False), f"in@9={before} in@10={at} (expected True/False)"

    check("contains_boundary", c_contains_boundary)

    # 12. ttl_remaining is None at and after the boundary, positive before it.
    def c_ttl_remaining_boundary():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", 1, ttl=10)
        clk.set(4)
        rem = c.ttl_remaining("a")
        clk.set(10)
        rem_at = c.ttl_remaining("a")
        ok = (rem is not None and rem > 0 and rem_at is None)
        return ok, f"remaining@4={rem!r} remaining@10={rem_at!r}"

    check("ttl_remaining_boundary", c_ttl_remaining_boundary)

    # 13. len() counts only live entries (boundary-expired ones drop out).
    def c_len_counts_live_only():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", 1, ttl=10)
        c.set("b", 2, ttl=20)
        clk.set(10)             # 'a' just reached its TTL; 'b' still live
        n = len(c)
        return n == 1, f"len@10={n} (expected 1; only 'b' live)"

    check("len_counts_live_only", c_len_counts_live_only)

    # 14. purge() evicts entries that are at/after their TTL.
    def c_purge_removes_expired():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", 1, ttl=10)
        c.set("b", 2, ttl=20)
        clk.set(10)             # 'a' is exactly at its boundary -> expired
        removed = c.purge()
        still_b = c.get("b")
        gone_a = c.get("a")
        return (removed == 1 and still_b == 2 and gone_a is None), \
            f"removed={removed!r} b={still_b!r} a={gone_a!r}"

    check("purge_removes_expired", c_purge_removes_expired)

    # 15. overwriting a key resets its TTL from the new set time.
    def c_overwrite_resets_ttl():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", 1, ttl=10)
        clk.set(5)
        c.set("a", 2, ttl=10)   # now expires at 15, not 10
        clk.set(14)
        v = c.get("a")
        clk.set(15)
        gone = c.get("a")
        return (v == 2 and gone is None), f"get@14={v!r} get@15={gone!r}"

    check("overwrite_resets_ttl", c_overwrite_resets_ttl)

    # 16. delete() removes a present key and reports presence.
    def c_delete_removes():
        clk = FakeClock(0)
        c = Cache(clock=clk)
        c.set("a", 1, ttl=10)
        existed = c.delete("a")
        absent = c.delete("a")
        v = c.get("a")
        return (existed is True and absent is False and v is None), \
            f"first={existed!r} second={absent!r} get={v!r}"

    check("delete_removes", c_delete_removes)


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
    "task": "fix_ttlcache",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
