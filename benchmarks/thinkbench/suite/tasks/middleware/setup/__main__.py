"""Tiny demo / smoke CLI for the middleware package.

Run with ``python -m middleware`` to exercise the router. Not part of the graded
contract; provided only as a convenience.
"""

import json

from .public import Router


def main() -> None:
    r = Router()
    r.add("/greet", lambda req: "hello")
    print(json.dumps({"greet": r.dispatch("/greet")}))


if __name__ == "__main__":
    main()
