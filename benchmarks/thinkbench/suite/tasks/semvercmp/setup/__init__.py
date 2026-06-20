"""semvercmp -- a Semantic Versioning 2.0 precedence comparator.

Public API is re-exported here for convenience; the implementation lives in
``semvercmp.public``.

    >>> from semvercmp import compare
    >>> compare("1.0.0-alpha", "1.0.0")
    -1
"""

from .public import compare

__all__ = ["compare"]
