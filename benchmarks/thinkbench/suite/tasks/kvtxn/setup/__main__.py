"""Tiny demo / smoke CLI for the kvtxn package.

Run with ``python -m kvtxn`` to exercise the store. Not part of the graded
contract; provided only as a convenience.
"""

import json

from .public import Store


def main() -> None:
    s = Store()
    s.set("a", 1)
    s.set("b", 2)
    s.delete("b")
    print(json.dumps({"a": s.get("a"), "b": s.get("b")}))


if __name__ == "__main__":
    main()
