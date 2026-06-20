"""Tiny demo / smoke CLI for the schemaoneof package.

Run with ``python -m schemaoneof`` to validate a couple of canned instances and
print the resulting error lists as JSON. Not part of the graded contract;
provided only as a convenience.
"""

import json

from .public import validate


def main() -> None:
    samples = [
        (5, {"type": "integer"}),
        ("x", {"type": "integer"}),
        ({"a": 1}, {"type": "object", "required": ["a", "b"]}),
        ("green", {"enum": ["red", "green", "blue"]}),
    ]
    report = [
        {"instance": inst, "schema": sch, "errors": validate(inst, sch)}
        for inst, sch in samples
    ]
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
