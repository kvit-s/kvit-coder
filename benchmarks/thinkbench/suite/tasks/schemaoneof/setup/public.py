"""schemaoneof — a tiny validator for a subset of JSON Schema.

``validate(instance, schema)`` walks ``instance`` against ``schema`` and returns
a list of error dicts. An EMPTY list means the instance is valid; a non-empty
list means it failed one or more constraints.

Supported keywords (a deliberate subset of JSON Schema draft semantics):

``type``
    One of ``"object"``, ``"array"``, ``"string"``, ``"number"``,
    ``"integer"``, ``"boolean"``, ``"null"``. Checks the instance's JSON type.
    (Note: ``bool`` is NOT a ``number``/``integer`` here, matching JSON Schema.)
``enum``
    A list of allowed values; the instance must be deep-equal to one of them.
``required``
    A list of property names that must be present (only meaningful for objects).
``properties``
    A mapping of property name -> subschema; each present property is validated
    against its subschema. Properties not named here are left unconstrained.

Each error is a dict of the shape::

    {"path": "<json-pointer-ish path>", "keyword": "<failing keyword>",
     "message": "<human readable>"}

The ``path`` is a slash-joined location (``""`` is the root). Subschema errors
carry the nested path so callers can see *where* the failure occurred.

Example
-------
    >>> validate(5, {"type": "integer"})
    []
    >>> validate("x", {"type": "integer"})  # doctest: +ELLIPSIS
    [{'path': '', 'keyword': 'type', ...}]
    >>> validate({"a": 1}, {"type": "object", "required": ["a", "b"]})  # doctest: +ELLIPSIS
    [{'path': '', 'keyword': 'required', ...}]
"""

from __future__ import annotations

from typing import Any


def _json_type(value: Any) -> str:
    """Return the JSON-Schema type name for a Python value.

    Booleans are reported as ``"boolean"`` (never ``"integer"``/``"number"``),
    matching JSON Schema's treatment of ``true``/``false``.
    """
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
    """Whether ``value`` satisfies the schema ``type`` name ``expected``.

    ``"number"`` accepts integers as well as floats (an int IS a number), but
    ``"integer"`` does not accept floats. Booleans satisfy only ``"boolean"``.
    """
    actual = _json_type(value)
    if expected == "number":
        return actual in ("integer", "number")
    return actual == expected


def _error(path: str, keyword: str, message: str) -> dict:
    return {"path": path, "keyword": keyword, "message": message}


def _join(path: str, token: str) -> str:
    """Append ``token`` to a slash-joined path (root is the empty string)."""
    return f"{path}/{token}" if path else token


def validate(instance: Any, schema: Any, path: str = "") -> list[dict]:
    """Validate ``instance`` against ``schema``; return a list of error dicts.

    An empty list means valid. ``path`` is used internally to report nested
    locations and normally defaults to the root.
    """
    errors: list[dict] = []

    if not isinstance(schema, dict):
        # A non-dict schema is meaningless here; treat as "no constraints".
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

    return errors


def _deep_equal(a: Any, b: Any) -> bool:
    """Structural equality that does NOT conflate booleans with numbers.

    ``True == 1`` in plain Python, but for enum membership we want ``1`` to
    match only the number ``1`` and ``True`` to match only ``true``.
    """
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
