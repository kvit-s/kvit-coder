"""Tiny demo / smoke CLI for the querygroup package.

Run with ``python -m querygroup`` to exercise the query engine. Not part of the
graded contract; provided only as a convenience.
"""

import json

from .public import Query


def main() -> None:
    q = Query([
        {"dept": "eng", "pay": 10},
        {"dept": "eng", "pay": 30},
        {"dept": "ops", "pay": 20},
    ])
    high = q.where(lambda r: r["pay"] >= 20).order_by("pay", reverse=True).rows()
    print(json.dumps(high))


if __name__ == "__main__":
    main()
