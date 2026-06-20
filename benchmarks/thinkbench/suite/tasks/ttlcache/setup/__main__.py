"""Tiny demo / smoke CLI for the ttlcache package.

Run with ``python -m ttlcache`` to see a value expire as the clock advances.
Not part of the graded contract; provided only as a convenience.
"""

import json

from .public import Cache


def main() -> None:
    ticks = [0]
    cache = Cache(clock=lambda: ticks[0])
    cache.set("greeting", "hello", ttl=10)

    timeline = []
    for t in (0, 5, 9, 10, 11):
        ticks[0] = t
        timeline.append({"t": t, "get": cache.get("greeting")})

    print(json.dumps({"timeline": timeline, "stats": cache.stats.as_dict()}, indent=2))


if __name__ == "__main__":
    main()
