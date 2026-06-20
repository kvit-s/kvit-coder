"""Tiny demo / smoke CLI for the querygroup package.

Run with ``python -m querygroup`` to exercise grouped aggregation. Not part of
the graded contract; provided only as a convenience.
"""

import json

from .public import Query


def main() -> None:
    q = Query([
        {"dept": "eng", "pay": 10},
        {"dept": "eng", "pay": 30},
        {"dept": "ops", "pay": 20},
        {"dept": "ops", "pay": None},
    ])
    grouped = (
        q.where(lambda r: True)
        .group_by("dept", [
            ("count", "pay", None),
            ("sum", "pay", None),
            ("avg", "pay", None),
        ])
        .rows()
    )
    print(json.dumps(grouped))


if __name__ == "__main__":
    main()
