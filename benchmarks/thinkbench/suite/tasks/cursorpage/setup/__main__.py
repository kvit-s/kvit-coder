"""Tiny demo / smoke CLI for the cursorpage package.

Run with ``python -m cursorpage`` to exercise offset pagination. Not part of
the graded contract; provided only as a convenience.
"""

import json

from .public import Paginator


def main() -> None:
    rows = [{"id": 3, "score": 5}, {"id": 1, "score": 5}, {"id": 2, "score": 9}]
    p = Paginator(rows, key="score")
    print(json.dumps({
        "page0": [r["id"] for r in p.page(0, 2)],
        "page1": [r["id"] for r in p.page(1, 2)],
    }))


if __name__ == "__main__":
    main()
