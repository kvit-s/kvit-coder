"""A tiny JSON-like serializer: ``dumps(obj)`` / ``loads(s)``.

Handles the JSON basic types — ``dict`` / ``list`` / ``str`` / ``int`` /
``float`` / ``bool`` / ``None`` — and nothing else. The wire format is plain
JSON (this module is a thin wrapper over the standard library's ``json``), so
output is byte-for-byte what ``json.dumps`` produces with its default
settings, and round-tripping a basic value returns an equal value.

    >>> dumps({"b": 1, "a": [True, None, 1.5]})
    '{"b": 1, "a": [true, null, 1.5]}'
    >>> loads('{"b": 1, "a": [true, null, 1.5]}')
    {'b': 1, 'a': [True, None, 1.5]}

There is NO support for custom Python types (datetime, Decimal, …): passing one
to ``dumps`` raises ``TypeError`` exactly as ``json`` would. There is also no
guard against circular references — handing ``dumps`` a structure that contains
itself will recurse until the interpreter gives up. The task is to add both: a
``register`` hook for custom types, and circular-reference detection. See
``brief.txt`` for the contract.
"""

from __future__ import annotations

import json
from typing import Any


def dumps(obj: Any) -> str:
    """Serialize ``obj`` (basic JSON types only) to a JSON string.

    Output is byte-identical to ``json.dumps(obj)`` with default settings.
    """
    return json.dumps(obj)


def loads(s: str) -> Any:
    """Parse a JSON string back into Python basic types."""
    return json.loads(s)
