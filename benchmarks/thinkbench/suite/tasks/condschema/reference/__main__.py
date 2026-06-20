"""Tiny demo / smoke CLI for the condschema package.

Run with ``python -m condschema`` to exercise nested + conditional validation.
Not part of the graded contract; provided only as a convenience.
"""

import json

from .public import validate


def main() -> None:
    schema = {
        "country": {"type": "string"},
        "state": {"type": "string", "requiredIf": ["country", "US"]},
        "address": {"type": "object", "fields": {
            "zip": {"type": "string", "required": True},
        }},
        "items": {"type": "list", "items": {
            "type": "object", "fields": {
                "sku": {"type": "string", "required": True},
            },
        }},
    }
    data = {"country": "US", "address": {}, "items": [{"sku": "A1"}, {}, {"sku": 5}]}
    print(json.dumps(validate(data, schema), indent=2))


if __name__ == "__main__":
    main()
