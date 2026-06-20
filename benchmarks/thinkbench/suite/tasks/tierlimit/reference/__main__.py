"""Tiny demo / smoke CLI for the tierlimit package.

Run with ``python -m tierlimit`` to exercise per-key tiers. Not part of the
graded contract; provided only as a convenience.
"""

import json

from .public import RateLimiter


def main() -> None:
    r = RateLimiter(
        limit=5, window=10.0, tiers={"free": 2, "pro": 5}, default_tier="free"
    )
    a1 = r.allow_key("alice", 0.0)   # free, 1/2
    a2 = r.allow_key("alice", 1.0)   # free, 2/2
    a3 = r.allow_key("alice", 2.0)   # over free's 2 -> False
    r.set_tier("alice", "pro")       # mid-window upgrade, count (2) preserved
    a4 = r.allow_key("alice", 3.0)   # now under pro's 5 -> True

    print(json.dumps({
        "alice": [a1, a2, a3, a4],   # [True, True, False, True]
    }))


if __name__ == "__main__":
    main()
