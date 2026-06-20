"""tmploop — a tiny string templater with nestable ``each`` / ``if`` block tags.

Public API is re-exported here for convenience; the implementation lives in
``tmploop.public``.

    >>> from tmploop import render
    >>> render("{{#each xs}}{{ this }}{{/each}}", {"xs": [1, 2, 3]})
    '123'
"""

from .public import render, TemplateError

__all__ = ["render", "TemplateError"]
