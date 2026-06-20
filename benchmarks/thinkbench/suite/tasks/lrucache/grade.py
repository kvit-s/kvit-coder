#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_lrucache`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced
`lrucache` package against the BRIEF'S CONTRACT (a fixed-capacity
least-recently-used cache whose ``LRUCache(capacity)`` exposes
``get(key) -> value | MISSING`` and ``put(key, value) -> None``, where a
*successful* get OR put refreshes a key's recency, eviction always removes the
true least-recently-used entry, the cache never holds more than ``capacity``
entries, and a ``capacity`` of 0 stores nothing), NOT against any particular
internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- a successful ``get`` returns the value but does NOT refresh the
    key's recency, so a key that is read repeatedly still ages out and can be
    evicted even though it is in active use.
  * BUG 2 -- eviction removes the MOST-recently-used entry (``popitem(last=True)``)
    instead of the least-recently-used one, so the wrong key is dropped.
  * BUG 3 -- a ``put`` that OVERWRITES an existing key updates the stored value
    but does NOT refresh that key's recency, so a freshly-rewritten key can be
    evicted as if it were stale.
  * BUG 4 -- the capacity guard is off by one (``len > capacity`` checked
    BEFORE the insert), so the cache grows to ``capacity + 1`` entries, and a
    ``capacity`` of 0 wrongly stores one entry.

Basic fill-and-read within capacity still looks correct, so a superficial fix
can pass the easy checks while still failing the edge cases.

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
# (id, human description). Kept in lockstep with the checks registered below so
# that an import failure can still report a result line for every single check.
CHECK_SPECS = [
    ("put_get_roundtrip", "a stored key reads back its value"),
    ("miss_returns_sentinel", "an absent key returns the MISSING sentinel"),
    ("update_existing_value", "putting an existing key overwrites its value in place"),
    ("fill_to_capacity_keeps_all", "filling exactly to capacity keeps every entry"),
    ("evict_lru_basic", "inserting past capacity evicts the least-recently-used key"),
    ("evict_keeps_most_recent", "the most-recently-inserted key survives an eviction"),
    ("len_never_exceeds_capacity", "the cache never holds more than capacity entries"),
    ("get_refreshes_recency", "a successful get protects its key from the next eviction"),
    ("get_miss_no_recency_side_effect", "a missing get does not alter eviction order"),
    ("put_update_refreshes_recency", "overwriting an existing key protects it from eviction"),
    ("evict_picks_true_lru_after_gets", "after mixed gets the evicted key is the real LRU"),
    ("capacity_zero_stores_nothing", "a capacity-0 cache never retains an entry"),
    ("capacity_one_replaces", "a capacity-1 cache holds only the latest key"),
    ("repeated_get_hot_key_survives", "a repeatedly-read hot key is never evicted"),
    ("eviction_sequence_order", "a sequence of inserts evicts keys in LRU order"),
    ("update_does_not_grow", "overwriting an existing key does not evict anyone"),
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


# --- import the produced package (contract: lrucache.public, fallback pkg) -----
import_ok = True
import_detail = ""
LRUCache = None
MISSING = None
try:
    try:
        mod = importlib.import_module("lrucache.public")
    except Exception:
        mod = importlib.import_module("lrucache")
    LRUCache = getattr(mod, "LRUCache")
    # The sentinel is part of the contract; reach it via the class attribute so
    # we never depend on a particular module-level name.
    MISSING = getattr(LRUCache, "MISSING")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def is_miss(v):
    """True iff ``v`` is the cache's documented miss sentinel."""
    return v is MISSING


if import_ok:
    # 1. baseline: a stored key reads back exactly.
    def c_put_get_roundtrip():
        c = LRUCache(capacity=3)
        c.put("a", 1)
        c.put("b", 2)
        return (c.get("a") == 1 and c.get("b") == 2), \
            f"get a={c.get('a')!r} b={c.get('b')!r} (expected 1, 2)"

    check("put_get_roundtrip", c_put_get_roundtrip)

    # 2. an absent key returns the MISSING sentinel (not None, not KeyError).
    def c_miss_returns_sentinel():
        c = LRUCache(capacity=3)
        c.put("a", 1)
        got = c.get("zzz")
        return is_miss(got), f"get(absent)->{got!r} (expected MISSING sentinel)"

    check("miss_returns_sentinel", c_miss_returns_sentinel)

    # 3. putting an existing key overwrites the value in place.
    def c_update_existing_value():
        c = LRUCache(capacity=3)
        c.put("a", 1)
        c.put("a", 99)
        return (c.get("a") == 99), f"after re-put get a={c.get('a')!r} (expected 99)"

    check("update_existing_value", c_update_existing_value)

    # 4. filling EXACTLY to capacity keeps every entry (no premature eviction).
    def c_fill_to_capacity_keeps_all():
        c = LRUCache(capacity=3)
        c.put("a", 1)
        c.put("b", 2)
        c.put("c", 3)
        vals = (c.get("a"), c.get("b"), c.get("c"))
        return (vals == (1, 2, 3)), f"a,b,c -> {vals!r} (expected (1,2,3))"

    check("fill_to_capacity_keeps_all", c_fill_to_capacity_keeps_all)

    # 5. BUG 2/4: one insert past capacity evicts the LRU (first-inserted) key.
    #    No gets in between, so "a" is unambiguously the least-recently-used.
    def c_evict_lru_basic():
        c = LRUCache(capacity=2)
        c.put("a", 1)
        c.put("b", 2)
        c.put("c", 3)            # full -> evict the LRU, which is "a"
        a, b, cc = c.get("a"), c.get("b"), c.get("c")
        return (is_miss(a) and b == 2 and cc == 3), \
            f"a={a!r}(expect MISSING) b={b!r}(expect 2) c={cc!r}(expect 3)"

    check("evict_lru_basic", c_evict_lru_basic)

    # 6. BUG 2: the most-recently-inserted key must SURVIVE the eviction.
    def c_evict_keeps_most_recent():
        c = LRUCache(capacity=2)
        c.put("a", 1)
        c.put("b", 2)
        c.put("c", 3)           # evicting MRU instead of LRU would drop "c"
        return (c.get("c") == 3), f"get c={c.get('c')!r} (expected 3, MRU must survive)"

    check("evict_keeps_most_recent", c_evict_keeps_most_recent)

    # 7. BUG 4: the cache must NEVER exceed capacity entries.
    def c_len_never_exceeds_capacity():
        c = LRUCache(capacity=2)
        present = 0
        for i, k in enumerate(["a", "b", "c", "d", "e"]):
            c.put(k, i)
            # count how many of the inserted-so-far keys are still present.
            here = sum(1 for kk in ["a", "b", "c", "d", "e"] if not is_miss(c.get(kk)))
            present = max(present, here)
        return (present <= 2), f"max simultaneously-present entries = {present} (cap=2, expected <=2)"

    check("len_never_exceeds_capacity", c_len_never_exceeds_capacity)

    # 8. BUG 1: a successful get must refresh recency, shielding that key from the
    #    very next eviction. cap=2: put a,b; get a (now a is MRU); put c -> b (LRU)
    #    goes, a stays.
    def c_get_refreshes_recency():
        c = LRUCache(capacity=2)
        c.put("a", 1)
        c.put("b", 2)
        c.get("a")              # touch "a": now "b" is the LRU
        c.put("c", 3)           # evict the LRU == "b"
        a, b, cc = c.get("a"), c.get("b"), c.get("c")
        return (a == 1 and is_miss(b) and cc == 3), \
            f"a={a!r}(expect 1) b={b!r}(expect MISSING) c={cc!r}(expect 3)"

    check("get_refreshes_recency", c_get_refreshes_recency)

    # 9. a MISSING get must NOT reorder anything: the LRU is still the first key.
    def c_get_miss_no_recency_side_effect():
        c = LRUCache(capacity=2)
        c.put("a", 1)
        c.put("b", 2)
        c.get("nope")           # miss: must be a no-op for recency
        c.put("c", 3)           # LRU is still "a" -> "a" evicted
        a, b = c.get("a"), c.get("b")
        return (is_miss(a) and b == 2), f"a={a!r}(expect MISSING) b={b!r}(expect 2)"

    check("get_miss_no_recency_side_effect", c_get_miss_no_recency_side_effect)

    # 10. BUG 3: overwriting an existing key must refresh its recency too.
    #     cap=2: put a,b; re-put a (a now MRU); put c -> b (LRU) evicted, a stays.
    def c_put_update_refreshes_recency():
        c = LRUCache(capacity=2)
        c.put("a", 1)
        c.put("b", 2)
        c.put("a", 11)          # overwrite "a": this must mark it most-recent
        c.put("c", 3)           # evict the LRU == "b"
        a, b, cc = c.get("a"), c.get("b"), c.get("c")
        return (a == 11 and is_miss(b) and cc == 3), \
            f"a={a!r}(expect 11) b={b!r}(expect MISSING) c={cc!r}(expect 3)"

    check("put_update_refreshes_recency", c_put_update_refreshes_recency)

    # 11. combined recency: a run of gets reorders things, eviction picks the true
    #     LRU. cap=3: put a,b,c; get a; get b -> recency order (LRU..MRU) is c,a,b;
    #     put d -> evict "c". a,b,d remain.
    def c_evict_picks_true_lru_after_gets():
        c = LRUCache(capacity=3)
        c.put("a", 1)
        c.put("b", 2)
        c.put("c", 3)
        c.get("a")              # order -> b, c, a
        c.get("b")              # order -> c, a, b ; LRU is "c"
        c.put("d", 4)           # evict "c"
        a, b, cc, d = c.get("a"), c.get("b"), c.get("c"), c.get("d")
        return (a == 1 and b == 2 and is_miss(cc) and d == 4), \
            f"a={a!r} b={b!r} c={cc!r}(expect MISSING) d={d!r}"

    check("evict_picks_true_lru_after_gets", c_evict_picks_true_lru_after_gets)

    # 12. BUG 4: a capacity-0 cache must store NOTHING.
    def c_capacity_zero_stores_nothing():
        c = LRUCache(capacity=0)
        c.put("a", 1)
        c.put("b", 2)
        a, b = c.get("a"), c.get("b")
        return (is_miss(a) and is_miss(b)), \
            f"a={a!r} b={b!r} (both expected MISSING in a cap-0 cache)"

    check("capacity_zero_stores_nothing", c_capacity_zero_stores_nothing)

    # 13. a capacity-1 cache keeps only the most recent key.
    def c_capacity_one_replaces():
        c = LRUCache(capacity=1)
        c.put("a", 1)
        c.put("b", 2)           # must evict "a"
        a, b = c.get("a"), c.get("b")
        return (is_miss(a) and b == 2), f"a={a!r}(expect MISSING) b={b!r}(expect 2)"

    check("capacity_one_replaces", c_capacity_one_replaces)

    # 14. BUG 1 sharper: a key read on EVERY round must never be evicted.
    #     cap=3, keep touching "hot" while churning cold keys past it.
    def c_repeated_get_hot_key_survives():
        c = LRUCache(capacity=3)
        c.put("hot", 1)
        survived = True
        for i in range(20):
            c.get("hot")                 # keep "hot" fresh each round
            c.put(f"cold{i}", i)         # churn a fresh cold key in
            if is_miss(c.get("hot")):
                survived = False
                break
        return survived, f"hot key survived 20 cold-churn rounds = {survived} (expected True)"

    check("repeated_get_hot_key_survives", c_repeated_get_hot_key_survives)

    # 15. eviction ORDER: with no gets, inserts evict strictly oldest-first.
    #     cap=3: insert a,b,c,d,e -> a then b evicted; c,d,e remain.
    def c_eviction_sequence_order():
        c = LRUCache(capacity=3)
        for k, v in [("a", 1), ("b", 2), ("c", 3), ("d", 4), ("e", 5)]:
            c.put(k, v)
        got = {k: c.get(k) for k in ["a", "b", "c", "d", "e"]}
        ok = (is_miss(got["a"]) and is_miss(got["b"])
              and got["c"] == 3 and got["d"] == 4 and got["e"] == 5)
        return ok, f"got={ {k: (v if not is_miss(v) else 'MISSING') for k, v in got.items()} }"

    check("eviction_sequence_order", c_eviction_sequence_order)

    # 16. BUG 3 corollary: overwriting an existing key must not evict anyone --
    #     it is not a new entry, so the count is unchanged.
    def c_update_does_not_grow():
        c = LRUCache(capacity=2)
        c.put("a", 1)
        c.put("b", 2)
        c.put("a", 10)          # overwrite, NOT a new insert -> no eviction
        c.put("b", 20)          # overwrite, NOT a new insert -> no eviction
        a, b = c.get("a"), c.get("b")
        return (a == 10 and b == 20), \
            f"a={a!r}(expect 10) b={b!r}(expect 20) -- overwrites must not evict"

    check("update_does_not_grow", c_update_does_not_grow)


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
    "task": "fix_lrucache",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
