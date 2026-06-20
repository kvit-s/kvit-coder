"""Reference cachelab.public — an in-memory cache with TTLs, stale-while-revalidate,
and per-key stampede protection (single-flight).

Time is read exclusively through an injectable clock so tests are deterministic and
never depend on wall-clock sleeps. The clock is any zero-arg callable returning a
monotonic-ish float "now" in seconds; `FakeClock` is provided for tests.

Concurrency model:
  * A single short-lived `_state_lock` guards the shared entry table (fast critical
    sections only — never held while a loader runs).
  * Each key has its own `_KeyState` with a dedicated `loader_lock`. Only the thread
    that wins that lock runs the loader for a hot+expired key (single-flight). Because
    the lock is per key, different keys never block one another.
  * Stale-while-revalidate: while an entry is expired but still within its stale
    window, a concurrent caller that cannot immediately win the loader lock may return
    the stale value instead of blocking, as long as another thread is already
    refreshing it.
"""
import threading
import time


class FakeClock:
    """Deterministic, manually-advanced clock for tests.

    Callable: ``clock()`` returns the current time in seconds. Advance time with
    ``clock.advance(dt)`` or set it absolutely with ``clock.set(t)``. Thread-safe so
    concurrency tests can advance time from one thread while others read it.
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
            return self._t

    def set(self, t):
        with self._lock:
            self._t = float(t)
            return self._t


class _Entry:
    """A cached value with the absolute times at which it goes fresh→stale→dead."""

    __slots__ = ("value", "fresh_until", "stale_until")

    def __init__(self, value, fresh_until, stale_until):
        self.value = value
        self.fresh_until = fresh_until      # < now  => expired (needs refresh)
        self.stale_until = stale_until      # < now  => fully dead (cannot serve)


class _KeyState:
    """Per-key concurrency state: the current entry plus that key's loader lock."""

    __slots__ = ("entry", "loader_lock")

    def __init__(self):
        self.entry = None
        self.loader_lock = threading.Lock()


class Cache:
    def __init__(self, clock=None):
        # ASSUMES: clock is a zero-arg callable returning seconds-as-float; default
        # to the real monotonic clock when none is injected.
        self._clock = clock if clock is not None else time.monotonic
        self._keys = {}
        self._state_lock = threading.Lock()
        self._stats = {
            "hits": 0,           # served a fresh entry without calling the loader
            "misses": 0,         # had to run the loader (cold or hard-expired)
            "stale_hits": 0,     # served a stale value during revalidation
            "loads": 0,          # total loader invocations that completed
            "errors": 0,         # loader invocations that raised
            "invalidations": 0,  # explicit invalidate() calls that removed an entry
            "keys": 0,           # distinct keys currently holding an entry
        }
        self._stats_lock = threading.Lock()

    # -- stats helpers --------------------------------------------------------
    def _bump(self, name, delta=1):
        with self._stats_lock:
            self._stats[name] += delta

    def _key_state(self, key):
        with self._state_lock:
            ks = self._keys.get(key)
            if ks is None:
                ks = _KeyState()
                self._keys[key] = ks
            return ks

    # -- public API -----------------------------------------------------------
    def get(self, key, loader, ttl_seconds, stale_seconds=0):
        now = self._clock()
        ks = self._key_state(key)

        entry = ks.entry
        if entry is not None and now < entry.fresh_until:
            # Fresh hit — no loader, no locking on the hot path beyond the table read.
            self._bump("hits")
            return entry.value

        # Expired (or cold). Exactly one thread should run the loader per key.
        # Try to win the loader lock without blocking first so stale-while-revalidate
        # callers can bail out and serve the stale value instead of waiting.
        got_lock = ks.loader_lock.acquire(blocking=False)
        if not got_lock:
            # Someone else is already refreshing this key.
            if entry is not None and now < entry.stale_until:
                # Within the stale window: serve stale immediately, don't block.
                self._bump("stale_hits")
                return entry.value
            # No serveable stale value: we must wait for the in-flight refresh, then
            # return whatever it produced (or re-run if it failed).
            ks.loader_lock.acquire(blocking=True)
            got_lock = True

        try:
            # Re-check under the loader lock: another thread may have just refreshed,
            # making our load unnecessary (double-checked locking).
            entry = ks.entry
            now = self._clock()
            if entry is not None and now < entry.fresh_until:
                self._bump("hits")
                return entry.value

            # We are the single flight. Run the loader.
            try:
                value = loader()
            except Exception:
                self._bump("errors")
                # On failure, surface a still-serveable stale value rather than the
                # exception when the stale window permits it.
                if entry is not None and now < entry.stale_until:
                    self._bump("stale_hits")
                    return entry.value
                raise

            self._bump("loads")
            self._bump("misses")
            new_entry = _Entry(
                value=value,
                fresh_until=now + ttl_seconds,
                stale_until=now + ttl_seconds + max(0, stale_seconds),
            )
            with self._state_lock:
                was_present = ks.entry is not None
                ks.entry = new_entry
                if not was_present:
                    self._bump("keys")
            return value
        finally:
            if got_lock:
                ks.loader_lock.release()

    def invalidate(self, key):
        with self._state_lock:
            ks = self._keys.get(key)
            had_entry = ks is not None and ks.entry is not None
            if ks is not None:
                ks.entry = None
        if had_entry:
            self._bump("invalidations")
            self._bump("keys", -1)

    def stats(self):
        with self._stats_lock:
            return dict(self._stats)
