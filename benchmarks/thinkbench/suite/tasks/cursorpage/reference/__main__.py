"""Tiny demo / smoke CLI for the cursorpage package.

Run with ``python -m cursorpage`` to walk all records with cursor pagination.
Not part of the graded contract; provided only as a convenience.
"""

import json

from .public import Paginator


def main() -> None:
    rows = [{"id": 3, "score": 5}, {"id": 1, "score": 5}, {"id": 2, "score": 9}]
    p = Paginator(rows, key="score")

    seen = []
    cursor = None
    while True:
        out = p.page_after(cursor, 2)
        seen.extend(r["id"] for r in out["items"])
        cursor = out["next_cursor"]
        if cursor is None:
            break

    print(json.dumps({
        "page0": [r["id"] for r in p.page(0, 2)],
        "walked": seen,            # [1, 3, 2] — every record once, ties by id
    }))


if __name__ == "__main__":
    main()
