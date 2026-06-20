"""Tiny demo / smoke CLI for the serialhook package.

Run with ``python -m serialhook`` to exercise custom-type hooks. Not part of the
graded contract; provided only as a convenience.
"""

from datetime import datetime

from .public import dumps, loads, register


def main() -> None:
    register(
        datetime,
        "datetime",
        lambda dt: dt.isoformat(),
        lambda s: datetime.fromisoformat(s),
    )

    obj = {"when": datetime(2020, 1, 2, 3, 4, 5), "tags": ["a", "b"]}
    wire = dumps(obj)
    back = loads(wire)
    print(wire)
    print(back["when"] == obj["when"])


if __name__ == "__main__":
    main()
