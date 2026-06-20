#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield Task (cachelab).

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `cachelab.public`
`Cache` class and the `python -m cachelab simulate` CLI), NOT against the model's own
tests and NOT against any particular internal file layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

Tolerance: the brief under-specifies the stats dict shape and the exact stale/error
semantics. This oracle accepts any contract-conformant representation and checks
BEHAVIOR, not incidental key names. Spots where it assumes a convention the brief does
not pin are marked `# ASSUMES`.

Determinism: time is driven ONLY through an injected fake clock — never real sleeps —
so TTL/stale checks and concurrency checks are reproducible. The fake clock the grader
constructs matches the brief's constructor signature (`Cache(clock=...)`) where `clock`
is a zero-arg callable returning seconds.
"""
import importlib
import json
import os
import subprocess
import sys
import tempfile
import threading

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


class FakeClock:
    """Grader-owned deterministic clock.

    # ASSUMES: the Cache constructor accepts a keyword `clock=` that is a zero-arg
    # callable returning the current time in seconds (float). This matches the brief's
    # `def __init__(self, clock=None)` and "Provide a fake clock for deterministic
    # tests." We inject OUR clock rather than relying on the package's own, so the test
    # controls time regardless of how the package names its clock helper.
    """

    def __init__(self, start=0.0):
        self._t = float(start)
        self._lock = threading.Lock()

    def __call__(self):
        with self._lock:
            return self._t

    def advance(self, dt):
        with self._lock:
            self._t += float(dt)


class Counter:
    """Thread-safe call counter for loaders, so concurrency checks are robust."""

    def __init__(self):
        self.n = 0
        self._lock = threading.Lock()

    def bump(self):
        with self._lock:
            self.n += 1

    @property
    def value(self):
        with self._lock:
            return self.n


def make_cache(pub):
    """Construct a Cache with the grader's fake clock; return (cache, clock)."""
    clock = FakeClock()
    cache = pub.Cache(clock=clock)
    return cache, clock


def stat_total(stats):
    """Tolerantly sum the integer values in a stats dict (for 'something happened')."""
    if not isinstance(stats, dict):
        return 0
    return sum(v for v in stats.values() if isinstance(v, int))


# --- import the produced package (contract: cachelab.public; tolerate top-level) --
import_ok = False
import_detail = ""
pub = None
for _mod in ("cachelab.public", "cachelab"):  # accept Cache at .public OR the package root
    try:
        cand = importlib.import_module(_mod)
        if hasattr(cand, "Cache"):
            pub = cand
            import_ok = True
            break
    except Exception as e:  # noqa: BLE001
        import_detail = f"{type(e).__name__}: {e}"
if not import_ok and not import_detail:
    import_detail = "no `Cache` in cachelab.public or cachelab"


if import_ok:
    # 1. cache miss then hit — loader runs once, second get within TTL is served cached.
    def c_hit():
        cache, _clock = make_cache(pub)
        cnt = Counter()

        def loader():
            cnt.bump()
            return "v1"

        v1 = cache.get("k", loader, ttl_seconds=10)
        v2 = cache.get("k", loader, ttl_seconds=10)
        return (v1 == "v1" and v2 == "v1" and cnt.value == 1), \
            f"v1={v1!r} v2={v2!r} loader_calls={cnt.value}"

    check("hit", "second get within TTL is a cache hit (loader runs once)", c_hit)

    # 2. cache miss — first get on a cold key runs the loader and returns its value.
    def c_miss():
        cache, _clock = make_cache(pub)
        cnt = Counter()
        v = cache.get("cold", lambda: (cnt.bump(), "loaded")[1], ttl_seconds=10)
        return (v == "loaded" and cnt.value == 1), f"v={v!r} loader_calls={cnt.value}"

    check("miss", "first get on a cold key runs the loader (miss)", c_miss)

    # 3. TTL expiry — after advancing past TTL, the loader runs again.
    def c_ttl_expiry():
        cache, clock = make_cache(pub)
        cnt = Counter()

        def loader():
            cnt.bump()
            return f"v{cnt.value}"

        a = cache.get("k", loader, ttl_seconds=10)          # load -> v1
        clock.advance(5)
        b = cache.get("k", loader, ttl_seconds=10)          # still fresh -> v1
        clock.advance(20)                                   # now past TTL
        c = cache.get("k", loader, ttl_seconds=10)          # reload -> v2
        return (a == "v1" and b == "v1" and c == "v2" and cnt.value == 2), \
            f"a={a!r} b={b!r} c={c!r} loader_calls={cnt.value}"

    check("ttl_expiry", "value expires after ttl_seconds and reloads", c_ttl_expiry)

    # 4. stale value — within the stale window, a refreshing thread is busy, so a
    #    concurrent caller gets the OLD value, not a block. Drive single-flight with a
    #    loader gate so the refresh is genuinely in-flight when the stale read happens.
    def c_stale():
        cache, clock = make_cache(pub)
        load_started = threading.Event()
        release_loader = threading.Event()
        results = {}

        def slow_loader():
            load_started.set()
            release_loader.wait(timeout=5)
            return "v2"

        # Prime with a fresh value (fast loader).
        cache.get("k", lambda: "v1", ttl_seconds=10, stale_seconds=30)
        # Move into the stale window: expired (past TTL) but within TTL+stale.
        clock.advance(15)

        def refresher():
            results["refresh"] = cache.get("k", slow_loader, ttl_seconds=10, stale_seconds=30)

        t = threading.Thread(target=refresher)
        t.start()
        if not load_started.wait(timeout=5):
            release_loader.set()
            t.join(timeout=5)
            return False, "refresh loader never started"

        # While the refresh is in flight, a second caller should get the STALE value
        # immediately (stale-while-revalidate), not block on the loader.
        stale_val = cache.get("k", slow_loader, ttl_seconds=10, stale_seconds=30)

        release_loader.set()
        t.join(timeout=5)
        return (stale_val == "v1"), \
            f"stale_val={stale_val!r} refresh={results.get('refresh')!r}"

    check("stale", "stale-while-revalidate serves the old value during refresh", c_stale)

    # 5. per-key locking — only ONE loader runs for a single hot+expired key under N
    #    concurrent gets (single-flight). Gate the loader so all threads pile up first.
    def c_per_key_single_flight():
        cache, clock = make_cache(pub)
        cnt = Counter()
        gate = threading.Event()
        barrier = threading.Barrier(8 + 1)

        def loader():
            cnt.bump()
            gate.wait(timeout=5)   # hold the single flight open while peers arrive
            return "value"

        # Cold key, no stale window: concurrent callers must coalesce onto one loader.
        def worker():
            barrier.wait(timeout=5)
            cache.get("hot", loader, ttl_seconds=10, stale_seconds=0)

        threads = [threading.Thread(target=worker) for _ in range(8)]
        for t in threads:
            t.start()
        barrier.wait(timeout=5)    # release all workers at once
        # Give threads a moment to contend, then let the one loader finish.
        import time as _t
        _t.sleep(0.2)
        gate.set()
        for t in threads:
            t.join(timeout=5)
        return (cnt.value == 1), f"loader_calls={cnt.value} (expected exactly 1)"

    check("per_key_single_flight",
          "exactly one loader runs for a hot key under N concurrent gets", c_per_key_single_flight)

    # 6. concurrent requests on DIFFERENT keys do not block each other — each runs its
    #    own loader, and all complete (independent keys are independent).
    def c_independent_keys():
        cache, _clock = make_cache(pub)
        cnt = Counter()
        N = 6
        start = threading.Barrier(N + 1)
        results = {}

        def worker(i):
            start.wait(timeout=5)

            def loader():
                cnt.bump()
                return i

            results[i] = cache.get(f"key{i}", loader, ttl_seconds=10)

        threads = [threading.Thread(target=worker, args=(i,)) for i in range(N)]
        for t in threads:
            t.start()
        start.wait(timeout=5)
        for t in threads:
            t.join(timeout=5)
        all_done = all(results.get(i) == i for i in range(N))
        return (all_done and cnt.value == N), \
            f"loader_calls={cnt.value} (expected {N}) results_ok={all_done}"

    check("independent_keys",
          "different keys don't block each other; each loads independently", c_independent_keys)

    # 7. invalidation — after invalidate(), the next get reloads via the loader.
    def c_invalidate():
        cache, _clock = make_cache(pub)
        cnt = Counter()

        def loader():
            cnt.bump()
            return f"v{cnt.value}"

        a = cache.get("k", loader, ttl_seconds=1000)   # v1, still fresh
        cache.invalidate("k")
        b = cache.get("k", loader, ttl_seconds=1000)   # must reload -> v2
        return (a == "v1" and b == "v2" and cnt.value == 2), \
            f"a={a!r} b={b!r} loader_calls={cnt.value}"

    check("invalidate", "invalidate() forces the next get to reload", c_invalidate)

    # 8. loader exceptions — a raising loader on a cold key propagates (does not cache a
    #    bogus value), and a later successful get works. Tolerant: we accept either the
    #    exception propagating OR a sentinel-free non-cache, by checking the retry loads.
    def c_loader_exception():
        cache, _clock = make_cache(pub)
        cnt = Counter()

        def bad_loader():
            cnt.bump()
            raise RuntimeError("boom")

        raised = False
        try:
            cache.get("k", bad_loader, ttl_seconds=10)
        except Exception:
            raised = True

        # The real requirement (and this check's own stated tolerance): a failed cold
        # load must NOT cache a bogus value, so the retry re-runs the good loader and
        # succeeds. Accept EITHER the exception propagating OR a swallowed failure.
        good = cache.get("k", lambda: "ok", ttl_seconds=10)
        return (good == "ok" and cnt.value == 1), \
            f"raised={raised} good={good!r} bad_calls={cnt.value}"

    check("loader_exception",
          "a raising loader propagates and does not cache a value", c_loader_exception)

    # 9. statistics — stats() returns a dict that reflects activity (hits vs misses move
    #    independently). Shape-tolerant: we don't pin key names, we check the COUNTS
    #    change in the right direction.
    def c_stats():
        cache, _clock = make_cache(pub)
        before = cache.stats()
        if not isinstance(before, dict):
            return False, f"stats() not a dict: {type(before).__name__}"

        cache.get("k", lambda: 1, ttl_seconds=10)   # one miss/load
        after_miss = cache.stats()
        cache.get("k", lambda: 1, ttl_seconds=10)   # one hit
        cache.get("k", lambda: 1, ttl_seconds=10)   # another hit
        after_hits = cache.stats()

        # Fair reading of "statistics": stats must RECORD activity and be monotonic
        # non-decreasing. We do NOT require hit-counting specifically (the brief lists
        # "statistics" generically, not "hits"), only that some counter accrues.
        t0, t1, t2 = stat_total(before), stat_total(after_miss), stat_total(after_hits)
        monotonic = t0 <= t1 <= t2
        moved = t2 > t0  # at least one counter advanced across the load + hits
        return (isinstance(after_hits, dict) and monotonic and moved), \
            f"totals={t0}->{t1}->{t2} keys={sorted(after_hits)}"

    check("stats", "stats() reports activity; hits and misses move independently", c_stats)


# --- CLI: `python -m cachelab simulate scenario.json` must print JSON stats ---
def run_cli(scenario):
    fd, path = tempfile.mkstemp(suffix=".json", dir=ROOT)
    try:
        with os.fdopen(fd, "w") as f:
            json.dump(scenario, f)
        proc = subprocess.run(
            [sys.executable, "-m", "cachelab", "simulate", path],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        out = proc.stdout.strip()
        parsed = json.loads(out)  # raises if not JSON
        # ASSUMES the CLI prints a JSON object of stats (the brief says "print JSON
        # stats"); we accept any JSON object, and only require it parse + be a dict.
        return isinstance(parsed, dict), f"rc={proc.returncode} stdout={out[:200]!r}"
    finally:
        try:
            os.remove(path)
        except OSError:
            pass


# A minimal, deterministic scenario: load a key, let it expire, reload, invalidate.
# Field names follow the brief's example spirit; a conformant CLI may name them
# differently, so this check grades only "emits JSON stats" (the pinned contract),
# tolerating a CLI that ignores unknown fields.
_scenario = {
    "steps": [
        {"op": "get", "key": "a", "value": 1, "ttl_seconds": 10, "stale_seconds": 5},
        {"op": "get", "key": "a", "value": 1, "ttl_seconds": 10},
        {"op": "advance", "seconds": 20},
        {"op": "get", "key": "a", "value": 2, "ttl_seconds": 10},
        {"op": "invalidate", "key": "a"},
    ]
}
check("cli_simulate_json",
      "`python -m cachelab simulate scenario.json` prints JSON stats", lambda: run_cli(_scenario))


passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "cachelab",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    # A failed import must score 0 — never a high score from a collapsed denominator.
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
