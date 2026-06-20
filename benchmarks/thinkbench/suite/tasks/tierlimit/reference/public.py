"""A fixed-window rate limiter with per-key budgets and named tiers.

This extends the original single-global-limit limiter in two ways while keeping
the old behaviour intact:

* **Per-key limiting** — :meth:`allow_key` rate-limits each caller ``key``
  against its OWN independent fixed window. Two keys never share a budget, and
  their windows advance independently (a key's window is derived purely from the
  ``now`` values seen for that key).

* **Tiers** — :meth:`set_tier` assigns a key to a named tier, and each tier name
  maps to a per-window limit. A key that was never assigned a tier uses the
  ``default_tier``.

Window math is identical to the base limiter: the window containing ``now`` is
``[w0, w0 + window)`` with ``w0 = floor(now / window) * window``.

The single-global-limit API (:meth:`allow`) is unchanged and keeps its own
counter, entirely separate from the per-key machinery.

Subtle semantics (this is the whole task)
-----------------------------------------
* Each key's window and count are independent; one key crossing a window
  boundary or hitting its limit never disturbs another key.

* Changing a key's tier MID-WINDOW does NOT reset that key's current count or
  start a fresh window. The key keeps the requests it has already spent in the
  current window; only the *effective limit* used for the comparison changes,
  applied against that SAME window's existing count. So lowering a key's tier
  mid-window can immediately push it over (a key that spent 3 in a limit-5 tier
  is over a limit-2 tier and is denied for the rest of that window), and raising
  its tier mid-window immediately grants more room in the same window. The
  change takes hold from the next :meth:`allow_key`; it is not retroactive (it
  does not refund or revoke requests already decided).

Example
-------
    >>> r = RateLimiter(limit=5, window=10.0,
    ...                 tiers={"free": 2, "pro": 5}, default_tier="free")
    >>> r.allow_key("alice", 0.0)   # alice on default tier "free" (limit 2)
    True
    >>> r.allow_key("alice", 1.0)
    True
    >>> r.allow_key("alice", 2.0)   # 3rd in window — over free's limit of 2
    False
    >>> r.allow_key("bob", 2.0)     # bob is independent
    True
    >>> r.set_tier("alice", "pro")  # mid-window upgrade, count (2) is kept
    >>> r.allow_key("alice", 3.0)   # now under pro's limit of 5
    True
"""

from __future__ import annotations

import math


class RateLimiter:
    """Fixed-window limiter with a global budget, per-key budgets, and tiers.

    Parameters
    ----------
    limit:
        Maximum requests per window for the single GLOBAL :meth:`allow` path,
        unchanged from the base limiter. Also used as the limit of the implied
        ``"default"`` tier when ``tiers`` is not given.
    window:
        Window length in seconds (a positive float), shared by every key and by
        the global path.
    tiers:
        Optional mapping of ``tier name -> per-window limit`` (positive ints).
        When omitted, a single tier named ``"default"`` is created with the
        global ``limit``.
    default_tier:
        The tier name applied to any key that has not been assigned one with
        :meth:`set_tier`. Must name a tier present in ``tiers`` (or be
        ``"default"`` when ``tiers`` is omitted).
    """

    def __init__(
        self,
        limit: int,
        window: float,
        tiers: dict[str, int] | None = None,
        default_tier: str = "default",
    ) -> None:
        self.limit = limit
        self.window = window
        if tiers is None:
            tiers = {"default": limit}
        self._tiers: dict[str, int] = dict(tiers)
        if default_tier not in self._tiers:
            raise ValueError(f"default_tier {default_tier!r} is not a known tier")
        self._default_tier = default_tier

        # Global single-budget state (the original behaviour, untouched).
        self._g_window_start: float | None = None
        self._g_count = 0

        # Per-key tier assignment: key -> tier name. Absent => default tier.
        self._key_tier: dict[str, str] = {}
        # Per-key window state: key -> (window_start, count). Independent per key.
        self._key_state: dict[str, tuple[float, int]] = {}

    # -- window helper -----------------------------------------------------

    def _window_for(self, now: float) -> float:
        """Return the start of the fixed window that contains ``now``."""
        return math.floor(now / self.window) * self.window

    # -- global path (unchanged from the base limiter) ---------------------

    def allow(self, now: float) -> bool:
        """Admit a request at ``now`` against the single GLOBAL budget.

        Behaves exactly like the base limiter and shares nothing with the
        per-key machinery.
        """
        w0 = self._window_for(now)
        if self._g_window_start is None or w0 != self._g_window_start:
            self._g_window_start = w0
            self._g_count = 0
        if self._g_count < self.limit:
            self._g_count += 1
            return True
        return False

    # -- tiers -------------------------------------------------------------

    def set_tier(self, key: str, tier: str) -> None:
        """Assign ``key`` to the named ``tier``.

        Raises ``ValueError`` for an unknown tier name. This does NOT reset the
        key's current window or count: the new limit takes effect against the
        SAME window's existing count on the next :meth:`allow_key`.
        """
        if tier not in self._tiers:
            raise ValueError(f"unknown tier {tier!r}")
        self._key_tier[key] = tier

    def _limit_for(self, key: str) -> int:
        """Return the current per-window limit in effect for ``key``."""
        tier = self._key_tier.get(key, self._default_tier)
        return self._tiers[tier]

    # -- per-key path ------------------------------------------------------

    def allow_key(self, key: str, now: float) -> bool:
        """Admit a request from ``key`` at ``now`` against the key's own budget.

        Each key has an independent fixed window and count; the limit applied is
        the key's current tier limit, compared against the count already spent
        in the key's current window.
        """
        w0 = self._window_for(now)
        state = self._key_state.get(key)
        if state is None or w0 != state[0]:
            # First time we see this key, or it has crossed into a new window:
            # the count for the (new) window starts at zero.
            count = 0
        else:
            count = state[1]

        if count < self._limit_for(key):
            self._key_state[key] = (w0, count + 1)
            return True
        # Over the (possibly just-lowered) limit: deny, but still pin the key to
        # this window so its count is preserved for the rest of the window.
        self._key_state[key] = (w0, count)
        return False
