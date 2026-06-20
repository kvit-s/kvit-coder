"""Tiny demo / smoke CLI for the cachetags package.

Run with ``python -m cachetags`` to exercise the cache. Not part of the graded
contract; provided only as a convenience.
"""

import json

from .public import Cache


def main() -> None:
    c = Cache()
    c.set("a", 1, now=0)
    c.set("b", 2, now=0)
    c.delete("b")
    print(json.dumps({"a": c.get("a", now=5), "b": c.get("b", now=5)}))


if __name__ == "__main__":
    main()
