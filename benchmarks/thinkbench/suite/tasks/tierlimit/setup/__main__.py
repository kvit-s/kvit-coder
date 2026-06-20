"""Tiny demo / smoke CLI for the tierlimit package.

Run with ``python -m tierlimit`` to exercise the limiter. Not part of the graded
contract; provided only as a convenience.
"""

import json

from .public import RateLimiter


def main() -> None:
    r = RateLimiter(limit=2, window=10.0)
    verdicts = [r.allow(t) for t in (0.0, 1.0, 2.0, 10.0)]
    print(json.dumps({"verdicts": verdicts}))  # [True, True, False, True]


if __name__ == "__main__":
    main()
