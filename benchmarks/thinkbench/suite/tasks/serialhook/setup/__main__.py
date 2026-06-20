"""Tiny demo / smoke CLI for the serialhook package.

Run with ``python -m serialhook`` to exercise the serializer. Not part of the
graded contract; provided only as a convenience.
"""

from .public import dumps, loads


def main() -> None:
    s = dumps({"a": 1, "b": [True, None, 1.5, "x"]})
    print(s)
    print(loads(s))


if __name__ == "__main__":
    main()
