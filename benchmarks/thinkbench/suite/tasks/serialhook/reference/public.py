"""A JSON-like serializer with custom-type hooks and circular-ref detection.

On top of the basic JSON types (``dict`` / ``list`` / ``str`` / ``int`` /
``float`` / ``bool`` / ``None``) this module supports:

* **Custom-type hooks** via ``register(type, tag, encode, decode)``. A value of
  a registered type is serialized to a *tagged form* — a JSON object
  ``{"__type__": tag, "value": <encoded>}`` — where ``<encoded>`` is whatever
  the ``encode`` callable returns. The encoded payload is itself serialized
  recursively, so it may contain basic types *or other registered types*.
  ``loads`` reverses the process: a tagged form whose ``tag`` is registered is
  routed to that type's ``decode`` callable (after recursively decoding the
  payload first).

* **Circular-reference detection** in ``dumps``. A structure that contains
  itself (directly or through a cycle of lists/dicts) raises
  ``CircularReferenceError`` with a clear message instead of recursing forever.
  Detection is by ANCESTRY along the current path, not "seen anywhere": a value
  that merely appears more than once in *sibling* positions (a shared / diamond
  reference) is NOT a cycle and serializes fine.

Design notes
------------
* The tagged form is just a plain dict on the wire, so it survives ``json``.
  To avoid confusing genuine user data with a tag, a dict is treated as a tagged
  form ONLY when it has EXACTLY the two keys ``__type__`` and ``value`` and its
  ``__type__`` is a string. Any other dict — including one that merely happens
  to contain a ``"__type__"`` key alongside others — is passed through as data.
* On ``loads``, a tagged form whose tag is NOT registered raises
  ``UnknownTagError`` rather than silently leaking the tagged dict to the caller.
* Basic-type output stays byte-identical to ``json.dumps`` default settings,
  because basic values are handed to ``json`` unchanged; only registered values
  and containers holding them are rewritten.
"""

from __future__ import annotations

import json
from typing import Any, Callable, Dict, Tuple

_TYPE_KEY = "__type__"
_VALUE_KEY = "value"


class SerializationError(ValueError):
    """Base class for serialhook errors."""


class CircularReferenceError(SerializationError):
    """Raised by ``dumps`` when the object graph contains a cycle."""


class UnknownTagError(SerializationError):
    """Raised by ``loads`` when a tagged form names an unregistered tag."""


# tag -> (encode, decode); type -> tag. Two maps so encode can look up by the
# value's exact type and decode can look up by the wire tag.
_ENCODERS: Dict[type, Tuple[str, Callable[[Any], Any]]] = {}
_DECODERS: Dict[str, Callable[[Any], Any]] = {}


def register(
    type_: type,
    tag: str,
    encode: Callable[[Any], Any],
    decode: Callable[[Any], Any],
) -> None:
    """Register a custom type for serialization.

    ``encode(value)`` must return a JSON-serializable payload (basic types or
    other registered types); ``decode(payload)`` must reverse it. ``tag`` is the
    string written to the wire under ``__type__`` and used to route decoding.
    """
    _ENCODERS[type_] = (tag, encode)
    _DECODERS[tag] = decode


def _to_jsonable(obj: Any, ancestors: set) -> Any:
    """Return a structure of pure basic types, encoding registered types and
    rewriting containers, while detecting cycles along the current path.

    ``ancestors`` holds the ``id()`` of every container currently open on the
    path from the root to ``obj``. A container whose id is already an ancestor
    is a cycle; ids are removed again when the container closes, so shared
    (sibling) references are fine.
    """
    # Registered custom type? (check BEFORE container handling so a registered
    # subtype of dict/list is still routed through its encoder).
    enc = _ENCODERS.get(type(obj))
    if enc is not None:
        tag, encode = enc
        # The encoded payload may itself contain registered types or
        # containers, so serialize it recursively too. It is a fresh value
        # produced by encode(), so it does not share identity with the path.
        payload = _to_jsonable(encode(obj), ancestors)
        return {_TYPE_KEY: tag, _VALUE_KEY: payload}

    # bool is a subclass of int — handle/pass basic scalars straight through.
    if obj is None or isinstance(obj, (str, bool, int, float)):
        return obj

    if isinstance(obj, dict):
        oid = id(obj)
        if oid in ancestors:
            raise CircularReferenceError("circular reference detected in dict")
        ancestors.add(oid)
        try:
            out = {}
            for k, v in obj.items():
                if not isinstance(k, str):
                    # match json's default: only str keys (json also coerces a
                    # few, but we keep it strict and predictable here).
                    raise SerializationError(
                        f"dict keys must be str, got {type(k).__name__}"
                    )
                out[k] = _to_jsonable(v, ancestors)
            return out
        finally:
            ancestors.discard(oid)

    if isinstance(obj, list):
        oid = id(obj)
        if oid in ancestors:
            raise CircularReferenceError("circular reference detected in list")
        ancestors.add(oid)
        try:
            return [_to_jsonable(v, ancestors) for v in obj]
        finally:
            ancestors.discard(oid)

    raise SerializationError(
        f"object of type {type(obj).__name__} is not serializable; "
        f"register it with register()"
    )


def dumps(obj: Any) -> str:
    """Serialize ``obj`` to a JSON string, honoring registered type hooks.

    Basic values produce output byte-identical to ``json.dumps`` defaults.
    Raises ``CircularReferenceError`` if the object graph contains a cycle.
    """
    jsonable = _to_jsonable(obj, set())
    return json.dumps(jsonable)


def _from_jsonable(obj: Any) -> Any:
    """Walk a parsed JSON structure, turning tagged forms back into custom
    types (decoding their payload first), bottom-up."""
    if isinstance(obj, list):
        return [_from_jsonable(v) for v in obj]
    if isinstance(obj, dict):
        if _is_tagged(obj):
            tag = obj[_TYPE_KEY]
            decode = _DECODERS.get(tag)
            if decode is None:
                raise UnknownTagError(f"no decoder registered for tag {tag!r}")
            # Decode the payload first (it may itself be / contain tagged forms).
            return decode(_from_jsonable(obj[_VALUE_KEY]))
        return {k: _from_jsonable(v) for k, v in obj.items()}
    return obj


def _is_tagged(d: dict) -> bool:
    """A dict is a tagged form iff it has EXACTLY ``__type__`` + ``value`` and a
    string tag. This keeps ordinary user dicts (even ones that contain a
    ``__type__`` key among others) from being mistaken for tagged forms."""
    return (
        len(d) == 2
        and _TYPE_KEY in d
        and _VALUE_KEY in d
        and isinstance(d[_TYPE_KEY], str)
    )


def loads(s: str) -> Any:
    """Parse a JSON string, restoring registered custom types from tagged forms.

    Raises ``UnknownTagError`` if a tagged form names an unregistered tag.
    """
    return _from_jsonable(json.loads(s))
