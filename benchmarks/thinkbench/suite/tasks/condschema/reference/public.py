"""condschema — a data validator with NESTED schemas and CONDITIONAL fields.

``validate(data, schema)`` walks ``data`` against ``schema`` and returns a list
of error dicts. An empty list means valid.

A ``schema`` maps ``field name -> field spec``. A spec may declare:

``type``
    One of ``"string"`` / ``"number"`` / ``"integer"`` / ``"bool"`` /
    ``"object"`` / ``"list"``. A ``bool`` matches only ``"bool"``; ``"number"``
    accepts ints and floats; ``"integer"`` rejects floats.
``required``
    If truthy, the field must be present.
``requiredIf``
    A ``[sibling_field, value]`` pair. The field is required only when, in the
    SAME object, ``sibling_field`` is present and EQUALS ``value`` (exact match;
    booleans never equal numbers). Otherwise the field is optional.
``fields`` (on ``type: "object"``)
    A nested schema validated recursively against the field's value when it is a
    dict. Inner paths are dotted under the field name (``address.zip``).
``items`` (on ``type: "list"``)
    A single spec applied to every element of the field's value when it is a
    list. An element's path is ``field.<index>`` plus any inner path
    (``items.2.sku``).

Each error is ``{"path": <dotted str>, "code": <"required"|"type">,
"message": <str>}``. Errors are emitted in a stable pre-order walk: fields in
schema-declared order; list elements by ascending index; a field's nested
errors before the next sibling field.

Example
-------
    >>> validate({"name": "Ada"}, {"name": {"type": "string", "required": True}})
    []
    >>> schema = {"country": {"type": "string"},
    ...           "state": {"type": "string", "requiredIf": ["country", "US"]}}
    >>> validate({"country": "US"}, schema)  # doctest: +ELLIPSIS
    [{'path': 'state', 'code': 'required', ...}]
    >>> validate({"country": "CA"}, schema)
    []
"""

from __future__ import annotations

from typing import Any


def _value_type(value: Any) -> str:
    """Return this validator's type name for a Python value.

    Booleans are reported as ``"bool"`` (never ``"integer"``/``"number"``).
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
    """Whether ``value`` satisfies the declared ``type`` name ``expected``."""
    actual = _value_type(value)
    if expected == "number":
        return actual in ("integer", "number")
    return actual == expected


def _exact_equal(a: Any, b: Any) -> bool:
    """Equality for ``requiredIf`` that does NOT conflate bools with numbers.

    In plain Python ``True == 1``; here ``True`` matches only ``True`` and ``1``
    matches only ``1``. Other values compare with ``==`` after a type guard so
    e.g. the string ``"1"`` never matches the number ``1``.
    """
    if isinstance(a, bool) or isinstance(b, bool):
        return isinstance(a, bool) and isinstance(b, bool) and a == b
    # int vs float: 1 == 1.0 in Python; keep them distinct for an exact match.
    if type(a) is not type(b):
        return False
    return a == b


def _error(path: str, code: str, message: str) -> dict:
    return {"path": path, "code": code, "message": message}


def _join(prefix: str, token: str) -> str:
    """Append ``token`` to a dotted path (``""`` is the root)."""
    return f"{prefix}.{token}" if prefix else token


def _is_required(spec: dict, obj: dict) -> bool:
    """Whether ``spec`` makes its field required within object ``obj``.

    ``required: True`` always requires it. ``requiredIf: [sib, val]`` requires
    it only when ``sib`` is present in ``obj`` and equals ``val`` exactly.
    """
    if spec.get("required"):
        return True
    cond = spec.get("requiredIf")
    if isinstance(cond, (list, tuple)) and len(cond) == 2:
        sib, val = cond
        return sib in obj and _exact_equal(obj[sib], val)
    return False


def _validate_object(data: dict, schema: dict, prefix: str) -> list[dict]:
    """Validate the object ``data`` against ``schema``, prefixing every path."""
    errors: list[dict] = []
    for name, spec in schema.items():
        if not isinstance(spec, dict):
            continue
        path = _join(prefix, name)

        if name not in data:
            # Absent: a required (plain or conditional) field is the only error;
            # an absent field is never type-checked.
            if _is_required(spec, data):
                errors.append(
                    _error(path, "required", f"missing required field {name!r}")
                )
            continue

        value = data[name]
        expected = spec.get("type")

        # Present: type-check it if a type is declared. A type mismatch on an
        # object/list field also means we must NOT recurse into it.
        if expected is not None and not _type_matches(value, expected):
            errors.append(
                _error(
                    path,
                    "type",
                    f"expected type {expected!r}, got {_value_type(value)!r}",
                )
            )
            continue

        # Recurse into nested objects / list items (value type already verified
        # above when a type was declared; guard again for untyped specs).
        if expected == "object" and isinstance(spec.get("fields"), dict):
            if isinstance(value, dict):
                errors.extend(_validate_object(value, spec["fields"], path))
        elif expected == "list" and isinstance(spec.get("items"), dict):
            if isinstance(value, list):
                item_spec = spec["items"]
                for idx, element in enumerate(value):
                    errors.extend(
                        _validate_element(element, item_spec, _join(path, str(idx)))
                    )

    return errors


def _validate_element(value: Any, spec: dict, path: str) -> list[dict]:
    """Validate one list element against ``spec`` at location ``path``.

    A list ``items`` spec applies to a value directly (it has no name of its
    own), so there is no presence/``requiredIf`` decision here — only a type
    check and any nested recursion.
    """
    errors: list[dict] = []
    expected = spec.get("type")
    if expected is not None and not _type_matches(value, expected):
        errors.append(
            _error(
                path,
                "type",
                f"expected type {expected!r}, got {_value_type(value)!r}",
            )
        )
        return errors

    if expected == "object" and isinstance(spec.get("fields"), dict):
        if isinstance(value, dict):
            errors.extend(_validate_object(value, spec["fields"], path))
    elif expected == "list" and isinstance(spec.get("items"), dict):
        if isinstance(value, list):
            item_spec = spec["items"]
            for idx, element in enumerate(value):
                errors.extend(
                    _validate_element(element, item_spec, _join(path, str(idx)))
                )
    return errors


def validate(data: Any, schema: Any) -> list[dict]:
    """Validate ``data`` against ``schema``; return a list of error dicts.

    An empty list means valid. Supports flat fields, nested objects (via
    ``fields``), lists of items (via ``items``), and conditional requirements
    (via ``requiredIf``). See the module docstring for the full contract.
    """
    if not isinstance(schema, dict):
        return []
    if not isinstance(data, dict):
        # No named fields to walk against; nothing the schema can constrain.
        return []
    return _validate_object(data, schema, "")


__all__ = ["validate"]
