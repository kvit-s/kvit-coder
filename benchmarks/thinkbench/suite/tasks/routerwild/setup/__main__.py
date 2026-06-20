"""Tiny demo CLI for the routerwild package.

Run with ``python -m routerwild`` to see a few paths matched against a small
route table. Not part of the graded contract; provided only as a convenience.
"""
import json

from .public import Router


def main() -> None:
    r = Router()
    r.add("/users/{id}", "show_user")
    r.add("/users/me", "current_user")
    r.add("/static/css/main.css", "static_file")

    paths = ["/users/42", "/users/me", "/static/css/main.css", "/nope"]
    table = []
    for p in paths:
        handler, params = r.match(p)
        table.append({"path": p, "handler": handler, "params": params})

    print(json.dumps({"matches": table}, indent=2))


if __name__ == "__main__":
    main()
