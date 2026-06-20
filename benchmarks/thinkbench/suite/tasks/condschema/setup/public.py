"""condschema — a tiny data validator.

``validate(data, schema)`` walks ``data`` against ``schema`` and returns a list
of error dicts. An EMPTY list means the data is valid; a non-empty list means it
failed one or more rules.

Today the validator handles a FLAT schema only: a mapping of
``field name -> field spec``. Each spec may declare:

``type``
    One of ``"string"`` / ``"number"`` / ``"integer"`` / ``"bool"`` /
    ``"object"`` / ``"list"``. Checks the field's value type. A ``bool`` is NOT
    a number/integer; ``"number"`` accepts ints and floats; ``"integer"`` does
    not accept floats.
``required``
    If truthy, the field must be present in ``data``.

Each error is a dict::

    {"path": "<dotted path>", "code": "<rule>", "message": "<human readable>"}

``path`` is a dotted location into the data (a top-level field's path is just
its name). ``code`` is ``"required"`` (a required field is absent) or ``"type"``
(a present field has the wrong type). All errors are reported, in
schema-declared field order.

The task (see ``brief.txt``) is to ADD nested object/list schemas and
conditional requirements. Flat validation must keep working unchanged.

Example
-------
    >>> validate({"name": "Ada"}, {"name": {"type": "string", "required": True}})
    []
    >>> validate({}, {"name": {"type": "string", "required": True}})  # doctest: +ELLIPSIS
    [{'path': 'name', 'code': 'required', ...}]
"""

from __future__ import annotations

from typing import Any


def _value_type(value: Any) -> str:
    """Return this validator's type name for a Python value.

    Booleans are reported as ``"bool"`` (never ``"integer"``/``"number"``), so a
    ``bool`` satisfies only ``type: "bool"``.
    """
    if isinstance(value, bool):
        return "bool"
    if isinstance(value, int):
        return "integer"
    if isinstance(value, float):
        return "number"
    if isinstance(value, str):
        return "string"
    if isinstance(value, list):
        return "list"
    if isinstance(value, dict):
        return "object"
    return "unknown"


def _type_matches(value: Any, expected: str) -> bool:
    """Whether ``value`` satisfies the declared ``type`` name ``expected``.

    ``"number"`` accepts integers as well as floats; ``"integer"`` rejects
    floats; a ``bool`` matches only ``"bool"``.
    """
    actual = _value_type(value)
    if expected == "number":
        return actual in ("integer", "number")
    return actual == expected


def _error(path: str, code: str, message: str) -> dict:
    return {"path": path, "code": code, "message": message}


def validate(data: Any, schema: Any) -> list[dict]:
    """Validate ``data`` (a dict) against a FLAT ``schema``; return error dicts.

    An empty list means valid. Only ``type`` and ``required`` are understood;
    each top-level field is checked independently, in schema-declared order.
    """
    errors: list[dict] = []
    if not isinstance(schema, dict):
        return errors
    if not isinstance(data, dict):
        # Nothing to walk: a non-dict at the root has no named fields.
        return errors

    for name, spec in schema.items():
        if not isinstance(spec, dict):
            continue
        present = name in data
        if not present:
            if spec.get("required"):
                errors.append(
                    _error(name, "required", f"missing required field {name!r}")
                )
            # Absent and not required: nothing to check.
            continue
        # Present: type-check it if a type is declared.
        if "type" in spec:
            expected = spec["type"]
            if not _type_matches(data[name], expected):
                errors.append(
                    _error(
                        name,
                        "type",
                        f"expected type {expected!r}, got {_value_type(data[name])!r}",
                    )
                )

    return errors


__all__ = ["validate"]
