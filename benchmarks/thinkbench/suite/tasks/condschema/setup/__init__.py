"""condschema — a tiny data validator (flat schema).

Public API is re-exported here for convenience; the implementation lives in
``condschema.public``.

    >>> from condschema import validate
    >>> validate({"name": "Ada"}, {"name": {"type": "string", "required": True}})
    []
"""

from .public import validate

__all__ = ["validate"]
