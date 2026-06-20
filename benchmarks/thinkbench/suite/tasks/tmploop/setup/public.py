"""A tiny string templater (FLAT — variables only, no block tags yet).

``render(template, context)`` substitutes ``{{ var }}`` placeholders with values
looked up from ``context``. Lookups may be DOTTED: ``{{ user.name }}`` walks dict
keys (and falls back to object attributes). A missing lookup renders as the empty
string. Literal text outside ``{{ ... }}`` is preserved verbatim.

This is the FIRST cut: it understands variable placeholders ONLY. It does not yet
understand block tags such as ``{{#each items}}...{{/each}}`` or
``{{#if cond}}...{{else}}...{{/if}}`` — those are the task. See ``brief.txt``.

Example
-------
    >>> render("Hi {{ name }}!", {"name": "Ada"})
    'Hi Ada!'
    >>> render("{{ user.name }} <{{ user.email }}>",
    ...        {"user": {"name": "Ada", "email": "ada@x.io"}})
    'Ada <ada@x.io>'
"""

from __future__ import annotations

import re
from typing import Any, Mapping

# Matches a single ``{{ ... }}`` placeholder. The inner text is whatever sits
# between the braces, stripped of surrounding whitespace by the renderer.
_PLACEHOLDER = re.compile(r"\{\{(.*?)\}\}", re.DOTALL)

_MISSING = object()


def _lookup(expr: str, context: Mapping[str, Any]) -> Any:
    """Resolve a (possibly dotted) ``expr`` against ``context``.

    Each segment is looked up as a dict key first, then as an object attribute.
    Returns ``_MISSING`` if any segment along the path is absent.
    """
    cur: Any = context
    for seg in expr.split("."):
        if isinstance(cur, Mapping):
            if seg in cur:
                cur = cur[seg]
                continue
            return _MISSING
        if hasattr(cur, seg):
            cur = getattr(cur, seg)
            continue
        return _MISSING
    return cur


def _render_value(expr: str, context: Mapping[str, Any]) -> str:
    """Render one placeholder's expression to a string (missing -> '')."""
    val = _lookup(expr.strip(), context)
    if val is _MISSING or val is None:
        return ""
    return str(val)


def render(template: str, context: Mapping[str, Any]) -> str:
    """Render ``template``, substituting ``{{ var }}`` placeholders from ``context``.

    Only variable placeholders are understood. Any literal text is preserved
    exactly. (Block tags are NOT implemented yet — see the brief.)
    """
    if context is None:
        context = {}
    return _PLACEHOLDER.sub(lambda m: _render_value(m.group(1), context), template)
