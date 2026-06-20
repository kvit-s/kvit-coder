"""schemaoneof — a tiny validator for a subset of JSON Schema.

REFERENCE implementation: identical to the shipped ``setup`` package PLUS the
``oneOf`` and ``not`` combinators. Never shown to the model.

``validate(instance, schema)`` walks ``instance`` against ``schema`` and returns
a list of error dicts. An EMPTY list means the instance is valid; a non-empty
list means it failed one or more constraints.

Supported keywords:

``type`` / ``enum`` / ``required`` / ``properties``
    As in the shipped package (unchanged behavior).
``oneOf``
    A list of subschemas. The instance must match EXACTLY ONE of them. Zero
    matches is an error; two-or-more matches is an error; exactly one is valid.
``not``
    A single subschema. The instance must NOT match it. If it does match, that
    is an error.

Each error is a dict of the shape::

    {"path": "<json-pointer-ish path>", "keyword": "<failing keyword>",
     "message": "<human readable>"}
"""

from __future__ import annotations

from typing import Any


def _json_type(value: Any) -> str:
    """Return the JSON-Schema type name for a Python value."""
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "boolean"
    if isinstance(value, int):
        return "integer"
    if isinstance(value, float):
        return "number"
    if isinstance(value, str):
        return "string"
    if isinstance(value, list):
        return "array"
    if isinstance(value, dict):
        return "object"
    return "unknown"


def _type_matches(value: Any, expected: str) -> bool:
    actual = _json_type(value)
    if expected == "number":
        return actual in ("integer", "number")
    return actual == expected


def _error(path: str, keyword: str, message: str) -> dict:
    return {"path": path, "keyword": keyword, "message": message}


def _join(path: str, token: str) -> str:
    return f"{path}/{token}" if path else token


def _matches(instance: Any, schema: Any) -> bool:
    """True iff ``instance`` validates cleanly against ``schema`` (no errors).

    Used by ``oneOf``/``not`` to count subschema matches. The path is irrelevant
    here — we only care whether the subschema produced any error.
    """
    return len(validate(instance, schema)) == 0


def validate(instance: Any, schema: Any, path: str = "") -> list[dict]:
    """Validate ``instance`` against ``schema``; return a list of error dicts."""
    errors: list[dict] = []

    if not isinstance(schema, dict):
        return errors

    # -- type ---------------------------------------------------------------
    if "type" in schema:
        expected = schema["type"]
        if not _type_matches(instance, expected):
            errors.append(
                _error(
                    path,
                    "type",
                    f"expected type {expected!r}, got {_json_type(instance)!r}",
                )
            )

    # -- enum ---------------------------------------------------------------
    if "enum" in schema:
        allowed = schema["enum"]
        if not any(_deep_equal(instance, a) for a in allowed):
            errors.append(
                _error(path, "enum", f"value {instance!r} is not one of {allowed!r}")
            )

    # -- required -----------------------------------------------------------
    if "required" in schema and isinstance(instance, dict):
        for name in schema["required"]:
            if name not in instance:
                errors.append(
                    _error(path, "required", f"missing required property {name!r}")
                )

    # -- properties ---------------------------------------------------------
    if "properties" in schema and isinstance(instance, dict):
        props = schema["properties"]
        if isinstance(props, dict):
            for name, subschema in props.items():
                if name in instance:
                    errors.extend(
                        validate(instance[name], subschema, _join(path, name))
                    )

    # -- oneOf --------------------------------------------------------------
    if "oneOf" in schema:
        subschemas = schema["oneOf"]
        if isinstance(subschemas, list):
            match_count = sum(1 for sub in subschemas if _matches(instance, sub))
            if match_count != 1:
                errors.append(
                    _error(
                        path,
                        "oneOf",
                        f"instance must match exactly one subschema, "
                        f"matched {match_count}",
                    )
                )

    # -- not ----------------------------------------------------------------
    if "not" in schema:
        subschema = schema["not"]
        if _matches(instance, subschema):
            errors.append(
                _error(
                    path,
                    "not",
                    "instance must NOT match the 'not' subschema, but it did",
                )
            )

    return errors


def _deep_equal(a: Any, b: Any) -> bool:
    """Structural equality that does NOT conflate booleans with numbers."""
    if isinstance(a, bool) or isinstance(b, bool):
        return isinstance(a, bool) and isinstance(b, bool) and a == b
    if isinstance(a, dict) and isinstance(b, dict):
        if a.keys() != b.keys():
            return False
        return all(_deep_equal(a[k], b[k]) for k in a)
    if isinstance(a, list) and isinstance(b, list):
        return len(a) == len(b) and all(_deep_equal(x, y) for x, y in zip(a, b))
    return type(a) == type(b) and a == b


__all__ = ["validate"]
