"""tmploop — a tiny string templater.

Public API is re-exported here for convenience; the implementation lives in
``tmploop.public``.

    >>> from tmploop import render
    >>> render("Hi {{ name }}!", {"name": "Ada"})
    'Hi Ada!'
"""

from .public import render

__all__ = ["render"]
