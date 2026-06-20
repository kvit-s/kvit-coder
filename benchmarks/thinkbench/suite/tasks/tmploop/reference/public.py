"""A tiny string templater with BLOCK TAGS (each / if), plus dotted variables.

``render(template, context)`` substitutes ``{{ var }}`` placeholders AND honours
two block tags that may nest arbitrarily:

* ``{{#each items}} BODY {{/each}}`` iterates ``items`` in order. Inside BODY:
  - ``{{ this }}``   -> the current element,
  - ``{{ @index }}`` -> its 0-based position,
  - ``{{ @first }}`` / ``{{ @last }}`` -> booleans (usable as ``#if`` conditions),
  - ``{{ this.field }}`` walks into the element, and when the element is a dict
    its keys are also reachable bare (``{{ field }}``), shadowing the outer scope.
  An ``items`` that is missing, empty, or not iterable yields ZERO iterations.

* ``{{#if cond}} A {{else}} B {{/if}}`` renders A when ``cond`` is truthy, else B.
  The ``{{else}}`` arm is optional. Truthiness is Python truthiness of the resolved
  value; a missing variable (and ``None``/``False``/``0``/``""``/empty collection)
  is falsy.

Blocks nest both ways (``each`` inside ``if``, ``if`` inside ``each``); matching is
done with a STACK so the right closer pairs with the right opener. Literal text
outside any tag is preserved verbatim — the tag delimiters are removed exactly
where they sit and nothing around them is trimmed.

Plain ``{{ var }}`` substitution (with dotted lookups, missing -> "") is unchanged.

Example
-------
    >>> render("{{#each xs}}[{{ @index }}:{{ this }}]{{/each}}", {"xs": ["a", "b"]})
    '[0:a][1:b]'
    >>> render("{{#if ok}}Y{{else}}N{{/if}}", {"ok": False})
    'N'
"""

from __future__ import annotations

import re
from typing import Any, Iterable, List, Mapping, Tuple

# One token: a block-open ({{#each x}} / {{#if x}}), {{else}}, a block-close
# ({{/each}} / {{/if}}), or a plain variable placeholder {{ x }}. We capture the
# whole tag span so literal text between tags is whatever the splitter leaves.
_TAG = re.compile(
    r"\{\{\s*"
    r"(?:"
    r"(#each|#if)\s+(.*?)"      # 1=kind 2=arg     (open)
    r"|(/each|/if)"            # 3=closer
    r"|(else)"                # 4=else
    r"|([^#/].*?)"            # 5=variable expr (must not start with # or /)
    r")"
    r"\s*\}\}",
    re.DOTALL,
)

_MISSING = object()


# --------------------------------------------------------------------------- #
# Lexer: turn the template into a flat token stream of TEXT and TAG tokens.
# --------------------------------------------------------------------------- #

def _tokenize(template: str) -> List[Tuple[str, Any]]:
    """Return a list of ('text', s) / ('open', (kind, arg)) / ('close', kind) /
    ('else', None) / ('var', expr) tokens, in source order."""
    toks: List[Tuple[str, Any]] = []
    pos = 0
    for m in _TAG.finditer(template):
        if m.start() > pos:
            toks.append(("text", template[pos:m.start()]))
        if m.group(1):       # #each / #if
            toks.append(("open", (m.group(1)[1:], m.group(2).strip())))
        elif m.group(3):     # /each / /if
            toks.append(("close", m.group(3)[1:]))
        elif m.group(4):     # else
            toks.append(("else", None))
        else:                # variable
            toks.append(("var", m.group(5).strip()))
        pos = m.end()
    if pos < len(template):
        toks.append(("text", template[pos:]))
    return toks


# --------------------------------------------------------------------------- #
# Parser: fold the flat token stream into a nested AST using an explicit stack.
# A node is one of:
#   ('text', s)
#   ('var', expr)
#   ('each', arg, body_nodes)
#   ('if', arg, then_nodes, else_nodes)
# --------------------------------------------------------------------------- #

class TemplateError(ValueError):
    """Raised on a malformed template (mismatched / unclosed block tags)."""


def _parse(toks: List[Tuple[str, Any]]) -> List[Any]:
    # Each stack frame collects the nodes of an open block. The bottom frame is
    # the document root. For an `if`, we track which arm ('then'/'else') is filling.
    root: List[Any] = []
    # stack of frames: (kind, arg, then_list, else_list_or_None, arm)
    stack: List[Tuple[str, str, List[Any], Any, str]] = []

    def emit(node: Any) -> None:
        if not stack:
            root.append(node)
            return
        kind, arg, then, els, arm = stack[-1]
        (then if arm == "then" else els).append(node)

    for kind, payload in toks:
        if kind == "text":
            if payload:
                emit(("text", payload))
        elif kind == "var":
            emit(("var", payload))
        elif kind == "open":
            blk, arg = payload
            stack.append((blk, arg, [], [] if blk == "if" else None, "then"))
        elif kind == "else":
            if not stack or stack[-1][0] != "if":
                raise TemplateError("{{else}} outside of an {{#if}} block")
            f = stack[-1]
            if f[4] == "else":
                raise TemplateError("duplicate {{else}} in one {{#if}} block")
            stack[-1] = (f[0], f[1], f[2], f[3], "else")
        elif kind == "close":
            if not stack:
                raise TemplateError(f"stray {{{{/{payload}}}}} with no open block")
            f = stack.pop()
            if f[0] != payload:
                raise TemplateError(
                    f"mismatched closer: {{{{/{payload}}}}} closes a {{{{#{f[0]}}}}}")
            if f[0] == "each":
                node = ("each", f[1], f[2])
            else:
                node = ("if", f[1], f[2], f[3] or [])
            # close pops THIS block, then emits it into whatever now tops the stack
            stack_was = stack
            if not stack_was:
                root.append(node)
            else:
                pk, pa, pthen, pels, parm = stack_was[-1]
                (pthen if parm == "then" else pels).append(node)
    if stack:
        raise TemplateError(f"unclosed {{{{#{stack[-1][0]}}}}} block")
    return root


# --------------------------------------------------------------------------- #
# Lookup + evaluation against a layered scope.
# --------------------------------------------------------------------------- #

def _lookup(expr: str, scope: Mapping[str, Any]) -> Any:
    """Resolve a (possibly dotted) ``expr`` against ``scope``; ``_MISSING`` if any
    segment is absent. ``this``/``@index``/``@first``/``@last`` are honoured as
    scope keys when present (the each-renderer installs them)."""
    parts = expr.split(".")
    cur: Any = scope
    first = parts[0]
    # Resolve the head segment against the scope mapping.
    if isinstance(cur, Mapping):
        if first in cur:
            cur = cur[first]
        else:
            return _MISSING
    else:
        return _MISSING
    # Walk the remaining segments into the resolved value.
    for seg in parts[1:]:
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


def _truthy(val: Any) -> bool:
    if val is _MISSING or val is None:
        return False
    return bool(val)


def _as_iterable(val: Any) -> Iterable[Any]:
    """A missing / None / non-iterable value yields nothing. Strings and mappings
    are treated as NON-iterable for #each (you iterate a list, not a string)."""
    if val is _MISSING or val is None:
        return ()
    if isinstance(val, (str, bytes, Mapping)):
        return ()
    try:
        iter(val)
    except TypeError:
        return ()
    return val


# --------------------------------------------------------------------------- #
# Renderer: walk the AST against a stack of scopes (innermost wins).
# --------------------------------------------------------------------------- #

def _resolve(expr: str, scopes: List[Mapping[str, Any]]) -> Any:
    for sc in reversed(scopes):
        v = _lookup(expr, sc)
        if v is not _MISSING:
            return v
    return _MISSING


def _render_nodes(nodes: List[Any], scopes: List[Mapping[str, Any]]) -> str:
    out: List[str] = []
    for node in nodes:
        tag = node[0]
        if tag == "text":
            out.append(node[1])
        elif tag == "var":
            v = _resolve(node[1], scopes)
            if v is _MISSING or v is None:
                out.append("")
            elif v is True:
                out.append("true")
            elif v is False:
                out.append("false")
            else:
                out.append(str(v))
        elif tag == "if":
            _, arg, then, els = node
            branch = then if _truthy(_resolve(arg, scopes)) else els
            out.append(_render_nodes(branch, scopes))
        elif tag == "each":
            _, arg, body = node
            items = list(_as_iterable(_resolve(arg, scopes)))
            n = len(items)
            for i, item in enumerate(items):
                frame: dict = {
                    "this": item,
                    "@index": i,
                    "@first": i == 0,
                    "@last": i == n - 1,
                }
                # When the element is a dict, its keys are reachable bare and
                # shadow the outer scope for the duration of this iteration.
                if isinstance(item, Mapping):
                    frame.update(item)
                out.append(_render_nodes(body, scopes + [frame]))
    return "".join(out)


def render(template: str, context: Mapping[str, Any]) -> str:
    """Render ``template`` against ``context``, honouring ``{{ var }}`` plus the
    ``{{#each}}`` and ``{{#if}}`` block tags. Raises ``TemplateError`` on a
    malformed template (mismatched / unclosed blocks)."""
    if context is None:
        context = {}
    ast = _parse(_tokenize(template))
    return _render_nodes(ast, [context])
