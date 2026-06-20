"""Reference cachelab CLI — `python -m cachelab simulate scenario.json` prints JSON stats.

A scenario is JSON describing a deterministic sequence of cache operations driven by
the fake clock, e.g.:

    {
      "steps": [
        {"op": "get", "key": "a", "value": 1, "ttl_seconds": 10, "stale_seconds": 5},
        {"op": "advance", "seconds": 12},
        {"op": "get", "key": "a", "value": 2, "ttl_seconds": 10},
        {"op": "invalidate", "key": "a"}
      ]
    }

`get` steps use a loader that returns the step's "value" (so simulations stay
deterministic and side-effect free). The program prints the cache's final stats as
JSON to stdout.
"""
import json
import sys

from .public import Cache, FakeClock


def run_scenario(scenario):
    clock = FakeClock(start=float(scenario.get("start", 0.0)))
    cache = Cache(clock=clock)

    for step in scenario.get("steps", []):
        op = step.get("op")
        if op == "advance":
            clock.advance(float(step.get("seconds", 0)))
        elif op == "set" or op == "set_time":
            clock.set(float(step.get("seconds", 0)))
        elif op == "get":
            value = step.get("value")
            ttl = int(step.get("ttl_seconds", 0))
            stale = int(step.get("stale_seconds", 0))
            try:
                cache.get(step["key"], lambda v=value: v, ttl, stale)
            except Exception:
                # Loader exceptions are part of the simulation; reflected in stats.
                pass
        elif op == "invalidate":
            cache.invalidate(step["key"])
        # Unknown ops are ignored so scenarios stay forward-compatible.

    return cache.stats()


def main(argv):
    if len(argv) < 2 or argv[0] != "simulate":
        print(json.dumps({"error": "usage: cachelab simulate <scenario.json>"}))
        return 2
    path = argv[1]
    try:
        with open(path) as f:
            scenario = json.load(f)
    except Exception as e:  # noqa: BLE001
        print(json.dumps({"error": f"{type(e).__name__}: {e}"}))
        return 2

    stats = run_scenario(scenario)
    print(json.dumps(stats))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
