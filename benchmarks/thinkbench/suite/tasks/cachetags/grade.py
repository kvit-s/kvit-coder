#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `cachetags`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `cachetags`
package against the BRIEF'S CONTRACT (per-entry TTL expiry + tag-based
invalidation, on top of the unchanged core API), NOT against any particular
internal file layout.

The defining behaviors under test are the ones a SUPERFICIAL implementation gets
wrong:

  * the TTL boundary is HALF-OPEN — an entry set with ttl=T at t0 is a hit on
    [t0, t0+T) and a miss at and after t0+T (off-by-one impls use `>` and serve
    one tick late, or expire one tick early);
  * an EXPIRED entry loses its tag membership — `invalidate_tag` on a former
    tag of an already-expired entry counts 0 and the entry never resurfaces (a
    naive tag index keeps pointing at dead keys and either mis-counts the
    invalidation or revives the entry);
  * re-`set`ting a key REPLACES its tags wholesale — invalidating an OLD tag
    must not touch the re-set entry (a naive union-of-tags index keeps the old
    tag and wrongly drops it).

Plain `get` / `set` / `delete` with no ttl and no tags must keep working — those
are the regression checks. A naive feature-add typically passes the core,
single-tag and basic-TTL checks but trips the subtle ones, landing well under
1.0; a careful implementation lands at 1.0.

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
    ("set_get_basic", "a value set with no ttl is returned later"),
    ("ttl_hit_before", "an entry is a HIT one tick before its TTL elapses"),
    ("ttl_miss_exact", "an entry is a MISS at the exact instant its TTL elapses"),
    ("ttl_miss_after", "an entry is a MISS after its TTL has elapsed"),
    ("ttl_none_never_expires", "ttl=None entry never expires"),
    ("ttl_zero_already_expired", "ttl=0 entry is already expired the instant it is set"),
    ("ttl_negative_already_expired", "negative-ttl entry is already expired"),
    ("invalidate_single_tag", "invalidate_tag drops a live entry carrying that tag"),
    ("invalidate_returns_count", "invalidate_tag returns the number of live entries dropped"),
    ("invalidate_other_tag_untouched", "invalidate_tag leaves entries without that tag alone"),
    ("invalidate_multiple_with_tag", "invalidate_tag drops every live entry carrying the tag"),
    ("expired_loses_tag_membership", "invalidate_tag of a former tag of an EXPIRED entry counts 0"),
    ("expired_not_resurfaced_after_invalidate", "an expired entry never resurfaces via the tag index"),
    ("reset_replaces_tags", "re-set replaces tags: an OLD tag no longer invalidates the entry"),
    ("reset_keeps_new_tag", "re-set keeps the NEW tag: it still invalidates the entry"),
    ("invalidate_unknown_tag_zero", "invalidate_tag on an unknown tag returns 0 and changes nothing"),
    ("regression_get_set_overwrite", "plain get/set/overwrite still work with no ttl/tags"),
    ("regression_delete_return", "delete reports presence (True/False) with no ttl/tags"),
    ("regression_get_default", "get returns the supplied default on a miss"),
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


# --- import the produced package (contract: cachetags.public, fallback cachetags) ---
import_ok = True
import_detail = ""
Cache = None
try:
    try:
        mod = importlib.import_module("cachetags.public")
    except Exception:
        mod = importlib.import_module("cachetags")
    Cache = getattr(mod, "Cache")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. basic set/get with no ttl.
    def c_set_get_basic():
        c = Cache()
        c.set("a", 1, now=0)
        v = c.get("a", now=100)
        return v == 1, f"get('a') -> {v!r} (expected 1)"

    check("set_get_basic", c_set_get_basic)

    # 2. half-open boundary: HIT one tick before expiry.
    def c_ttl_hit_before():
        c = Cache()
        c.set("a", 1, now=0, ttl=10)
        v = c.get("a", now=9)
        return v == 1, f"get('a', now=9) -> {v!r} (expected 1 — still fresh)"

    check("ttl_hit_before", c_ttl_hit_before)

    # 3. half-open boundary: MISS at the exact expiry instant (t0+ttl).
    def c_ttl_miss_exact():
        c = Cache()
        c.set("a", 1, now=0, ttl=10)
        v = c.get("a", now=10, default="GONE")
        return v == "GONE", f"get('a', now=10) -> {v!r} (expected miss at exact boundary)"

    check("ttl_miss_exact", c_ttl_miss_exact)

    # 4. MISS after expiry.
    def c_ttl_miss_after():
        c = Cache()
        c.set("a", 1, now=0, ttl=10)
        v = c.get("a", now=11, default="GONE")
        return v == "GONE", f"get('a', now=11) -> {v!r} (expected miss)"

    check("ttl_miss_after", c_ttl_miss_after)

    # 5. ttl=None (the default) means never expires.
    def c_ttl_none_never_expires():
        c = Cache()
        c.set("a", 1, now=0)             # no ttl
        c.set("b", 2, now=0, ttl=None)  # explicit None
        a = c.get("a", now=10 ** 9)
        b = c.get("b", now=10 ** 9)
        return (a == 1 and b == 2), f"a={a!r} b={b!r} (expected 1/2, never expire)"

    check("ttl_none_never_expires", c_ttl_none_never_expires)

    # 6. ttl=0 is already expired.
    def c_ttl_zero_already_expired():
        c = Cache()
        c.set("a", 1, now=5, ttl=0)
        v = c.get("a", now=5, default="GONE")
        return v == "GONE", f"get('a', now=5) with ttl=0 -> {v!r} (expected already expired)"

    check("ttl_zero_already_expired", c_ttl_zero_already_expired)

    # 7. negative ttl is already expired.
    def c_ttl_negative_already_expired():
        c = Cache()
        c.set("a", 1, now=5, ttl=-3)
        v = c.get("a", now=5, default="GONE")
        return v == "GONE", f"get('a', now=5) with ttl=-3 -> {v!r} (expected already expired)"

    check("ttl_negative_already_expired", c_ttl_negative_already_expired)

    # 8. invalidate_tag drops a live tagged entry.
    def c_invalidate_single_tag():
        c = Cache()
        c.set("a", 1, now=0, tags=["red"])
        c.invalidate_tag("red", now=0)
        v = c.get("a", now=0, default="GONE")
        return v == "GONE", f"get('a') after invalidate('red') -> {v!r} (expected gone)"

    check("invalidate_single_tag", c_invalidate_single_tag)

    # 9. invalidate_tag returns the count of LIVE entries it dropped.
    def c_invalidate_returns_count():
        c = Cache()
        c.set("a", 1, now=0, tags=["red"])
        c.set("b", 2, now=0, tags=["red"])
        c.set("c", 3, now=0, tags=["blue"])
        n = c.invalidate_tag("red", now=0)
        return n == 2, f"invalidate_tag('red') -> {n!r} (expected 2)"

    check("invalidate_returns_count", c_invalidate_returns_count)

    # 10. invalidate_tag leaves entries WITHOUT that tag alone.
    def c_invalidate_other_tag_untouched():
        c = Cache()
        c.set("a", 1, now=0, tags=["red"])
        c.set("b", 2, now=0, tags=["blue"])
        c.invalidate_tag("red", now=0)
        a = c.get("a", now=0, default="GONE")
        b = c.get("b", now=0, default="GONE")
        return (a == "GONE" and b == 2), f"a={a!r} b={b!r} (expected GONE/2)"

    check("invalidate_other_tag_untouched", c_invalidate_other_tag_untouched)

    # 11. invalidate_tag drops EVERY live entry carrying the tag (incl. multi-tag).
    def c_invalidate_multiple_with_tag():
        c = Cache()
        c.set("a", 1, now=0, tags=["red", "x"])
        c.set("b", 2, now=0, tags=["y", "red"])
        c.set("c", 3, now=0, tags=["red"])
        c.invalidate_tag("red", now=0)
        gone = [c.get(k, now=0, default="GONE") for k in ("a", "b", "c")]
        return all(g == "GONE" for g in gone), f"after invalidate('red'): {gone!r} (expected all GONE)"

    check("invalidate_multiple_with_tag", c_invalidate_multiple_with_tag)

    # 12. SUBTLE: an EXPIRED entry has lost its tag membership — invalidate counts 0.
    def c_expired_loses_tag_membership():
        c = Cache()
        c.set("a", 1, now=0, ttl=5, tags=["green"])
        # at now=10 the entry has expired; invalidating its former tag is a no-op.
        n = c.invalidate_tag("green", now=10)
        return n == 0, f"invalidate_tag('green', now=10) after expiry -> {n!r} (expected 0)"

    check("expired_loses_tag_membership", c_expired_loses_tag_membership)

    # 13. SUBTLE: an expired entry must not resurface via the tag index.
    def c_expired_not_resurfaced_after_invalidate():
        c = Cache()
        c.set("a", 1, now=0, ttl=5, tags=["green"])
        c.invalidate_tag("green", now=10)      # prunes the expired entry
        # Even if the clock somehow rewinds, the entry is GONE — never resurfaces.
        v_then = c.get("a", now=10, default="GONE")
        v_rewind = c.get("a", now=0, default="GONE")
        return (v_then == "GONE" and v_rewind == "GONE"), \
            f"after-invalidate get -> {v_then!r}, rewound get -> {v_rewind!r} (expected GONE/GONE)"

    check("expired_not_resurfaced_after_invalidate", c_expired_not_resurfaced_after_invalidate)

    # 14. SUBTLE: re-set replaces tags — an OLD tag no longer invalidates the entry.
    def c_reset_replaces_tags():
        c = Cache()
        c.set("d", 4, now=0, tags=["old"])
        c.set("d", 5, now=0, tags=["new"])     # "d" loses "old", carries "new"
        n = c.invalidate_tag("old", now=0)     # must not touch "d"
        v = c.get("d", now=0, default="GONE")
        return (n == 0 and v == 5), f"invalidate('old') -> {n!r}, get('d') -> {v!r} (expected 0/5)"

    check("reset_replaces_tags", c_reset_replaces_tags)

    # 15. re-set keeps the NEW tag — invalidating it still drops the entry.
    def c_reset_keeps_new_tag():
        c = Cache()
        c.set("d", 4, now=0, tags=["old"])
        c.set("d", 5, now=0, tags=["new"])
        n = c.invalidate_tag("new", now=0)
        v = c.get("d", now=0, default="GONE")
        return (n == 1 and v == "GONE"), f"invalidate('new') -> {n!r}, get('d') -> {v!r} (expected 1/GONE)"

    check("reset_keeps_new_tag", c_reset_keeps_new_tag)

    # 16. invalidate_tag on an unknown tag is a harmless no-op.
    def c_invalidate_unknown_tag_zero():
        c = Cache()
        c.set("a", 1, now=0, tags=["red"])
        n = c.invalidate_tag("nope", now=0)
        v = c.get("a", now=0)
        return (n == 0 and v == 1), f"invalidate('nope') -> {n!r}, get('a') -> {v!r} (expected 0/1)"

    check("invalidate_unknown_tag_zero", c_invalidate_unknown_tag_zero)

    # 17. REGRESSION: plain get/set/overwrite with no ttl/tags.
    def c_regression_get_set_overwrite():
        c = Cache()
        miss = c.get("nope", now=0, default="DEF")
        c.set("a", 1, now=0)
        got = c.get("a", now=0)
        c.set("a", 2, now=0)
        over = c.get("a", now=0)
        ok = (miss == "DEF" and got == 1 and over == 2)
        return ok, f"miss={miss!r} get={got!r} overwrite={over!r} (expected DEF/1/2)"

    check("regression_get_set_overwrite", c_regression_get_set_overwrite)

    # 18. REGRESSION: delete reports presence with no ttl/tags.
    def c_regression_delete_return():
        c = Cache()
        c.set("a", 1, now=0)
        first = c.delete("a")
        second = c.delete("a")
        gone = c.get("a", now=0, default="GONE")
        return (first is True and second is False and gone == "GONE"), \
            f"first={first!r} second={second!r} gone={gone!r} (expected True/False/GONE)"

    check("regression_delete_return", c_regression_delete_return)

    # 19. REGRESSION: get returns the supplied default on a miss.
    def c_regression_get_default():
        c = Cache()
        v = c.get("absent", now=0, default=42)
        return v == 42, f"get('absent', default=42) -> {v!r} (expected 42)"

    check("regression_get_default", c_regression_get_default)


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
    "task": "cachetags",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
