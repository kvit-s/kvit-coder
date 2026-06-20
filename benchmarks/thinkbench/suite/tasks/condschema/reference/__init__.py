"""condschema — a data validator with nested schemas and conditional fields.

Public API is re-exported here for convenience; the implementation lives in
``condschema.public``.

    >>> from condschema import validate
    >>> validate({"name": "Ada"}, {"name": {"type": "string", "required": True}})
    []
"""

from .public import validate

__all__ = ["validate"]
