"""Tiny demo / smoke CLI for the condschema package.

Run with ``python -m condschema`` to exercise the validator. Not part of the
graded contract; provided only as a convenience.
"""

import json

from .public import validate


def main() -> None:
    schema = {
        "name": {"type": "string", "required": True},
        "age": {"type": "integer"},
    }
    data = {"name": "Ada", "age": "old"}
    print(json.dumps(validate(data, schema)))


if __name__ == "__main__":
    main()
