"""Tiny demo / smoke CLI for the cachetags package.

Run with ``python -m cachetags`` to exercise TTL expiry + tag invalidation. Not
part of the graded contract; provided only as a convenience.
"""

import json

from .public import Cache


def main() -> None:
    c = Cache()
    c.set("a", 1, now=0, ttl=10, tags=["red"])
    fresh = c.get("a", now=5)        # 1 (still within TTL)
    expired = c.get("a", now=10)     # None (TTL elapsed -> miss)

    c.set("b", 2, now=0, tags=["red"])
    dropped = c.invalidate_tag("red", now=0)  # drops "b" (1)
    after = c.get("b", now=0)        # None

    print(json.dumps({
        "fresh": fresh,              # 1
        "expired": expired,          # None
        "invalidated": dropped,      # 1
        "after_invalidate": after,   # None
    }))


if __name__ == "__main__":
    main()
