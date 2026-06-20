"""Tiny demo / smoke CLI for the middleware package.

Run with ``python -m middleware`` to exercise the onion middleware. Not part of
the graded contract; provided only as a convenience.
"""

import json

from .public import Router


def main() -> None:
    r = Router()
    r.add("/greet", lambda req: "hello")

    trail = []

    def outer(request, next):
        trail.append("outer-before")
        resp = next()
        trail.append("outer-after")
        return resp + "!"

    def inner(request, next):
        trail.append("inner-before")
        resp = next()
        trail.append("inner-after")
        return resp.upper()

    r.use(outer)
    r.use(inner)

    result = r.dispatch("/greet")
    print(json.dumps({"result": result, "trail": trail}))


if __name__ == "__main__":
    main()
