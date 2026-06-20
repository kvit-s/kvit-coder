"""schemaoneof — a small JSON-Schema-subset validator.

The public entry point is :func:`schemaoneof.validate`, re-exported here for
convenience; the implementation lives in :mod:`schemaoneof.public`.

    >>> from schemaoneof import validate
    >>> validate(5, {"type": "integer"})
    []
"""

from .public import validate

__all__ = ["validate"]
