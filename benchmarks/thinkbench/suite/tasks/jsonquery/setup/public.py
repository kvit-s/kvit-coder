"""jsonquery.public — a JSONPath-lite selector (stdlib only).

``select(obj, path)`` evaluates a small path expression against a nested
structure of dicts and lists and returns a flat ``list`` of every value the
path selects (in document order). Supported steps:

* ``.key``    — descend into the mapping value at ``key``.
* ``[index]`` — index into a list (non-negative integer).
* ``[*]``     — fan out to EVERY element of a list, in order.
* ``..key``   — recursive descent: every value stored under ``key`` ANYWHERE at
  or below the current node, found in pre-order (a node before its children),
  including ``key`` occurrences nested inside list elements.

A path is a sequence of those steps written together, e.g. ``.users[*].name``
or ``..id``. A leading ``.`` / ``..`` is optional sugar; ``users[0]`` and
``.users[0]`` mean the same thing.

A step that does not match the shape of the current value (a missing mapping
key, an out-of-range or non-integer list index, indexing a non-list, or keying
a non-mapping) raises :class:`SelectError`. The ONE exception is ``..key``,
whose whole purpose is to scan: it simply collects the matches that exist and
never raises for "not found" (an empty result is legal).

Standard library only (``re``).
"""

from __future__ import annotations

import re
from typing import Any, List, Tuple


class SelectError(ValueError):
    """Raised when a path step does not match the value it is applied to."""


# A step is one of:
#   ("key", name)        from .name
#   ("index", i)         from [i]
#   ("wild", None)       from [*]
#   ("descend", name)    from ..name
Step = Tuple[str, Any]

# Tokenizer: a leading ``..name`` (recursive descent), a ``.name`` key, a
# ``[*]`` wildcard, or a ``[<int>]`` index. ``name`` is a run of word chars.
_TOKEN = re.compile(
    r"""
      \.\.(?P<descend>\w+)      # ..key  (recursive descent) — check BEFORE .key
    | \.(?P<key>\w+)            # .key
    | \[\*\]                    # [*]
    | \[(?P<index>\d+)\]        # [123]
    | (?P<bare>\w+)             # leading bare key (sugar for a leading .key)
    """,
    re.VERBOSE,
)


def _parse(path: str) -> List[Step]:
    """Tokenize ``path`` into a list of steps, rejecting any leftover text."""
    if not isinstance(path, str):
        raise SelectError(f"path must be a str, got {type(path).__name__}")
    steps: List[Step] = []
    pos = 0
    n = len(path)
    while pos < n:
        m = _TOKEN.match(path, pos)
        if not m or m.end() == pos:
            raise SelectError(f"bad path syntax at {path[pos:]!r}")
        if m.group("descend") is not None:
            steps.append(("descend", m.group("descend")))
        elif m.group("key") is not None:
            steps.append(("key", m.group("key")))
        elif m.group("index") is not None:
            steps.append(("index", int(m.group("index"))))
        elif m.group("bare") is not None:
            steps.append(("key", m.group("bare")))
        else:  # [*]
            steps.append(("wild", None))
        pos = m.end()
    return steps


def _descend(node: Any, name: str, out: List[Any]) -> None:
    """Recursive descent: collect ``node[name]`` wherever it exists at or below
    ``node``.

    BUG (descent order + missing matches): this only recurses through MAPPING
    values, so any ``name`` stored inside a LIST element is never found, and it
    appends the current node's own match AFTER recursing into the children
    (post-order) instead of before, so the matches that do come back are in the
    wrong (bottom-up) order.
    """
    if isinstance(node, dict):
        for value in node.values():  # recurse first ...
            _descend(value, name, out)
        if name in node:
            out.append(node[name])   # ... then record this node (post-order: wrong)
    # NOTE: lists are not traversed at all, so matches nested inside list
    # elements are silently missed.


def _apply(step: Step, values: List[Any]) -> List[Any]:
    """Apply one step to the whole current frontier, preserving order."""
    kind, arg = step
    nxt: List[Any] = []
    if kind == "descend":
        for v in values:
            _descend(v, arg, nxt)
        return nxt
    for v in values:
        if kind == "key":
            if not isinstance(v, dict):
                raise SelectError(f"cannot read key {arg!r} of non-mapping {type(v).__name__}")
            if arg not in v:
                # BUG (missing key/index): a missing key should raise
                # SelectError, but this silently skips it, so a path that does
                # not match quietly yields fewer (or zero) results instead of
                # signalling the mismatch.
                continue
            nxt.append(v[arg])
        elif kind == "index":
            if not isinstance(v, list) or isinstance(v, bool):
                raise SelectError(f"cannot index non-list {type(v).__name__}")
            if arg >= len(v):
                # BUG (missing index): same defect for an out-of-range index —
                # it should raise SelectError, but is silently skipped.
                continue
            nxt.append(v[arg])
        elif kind == "wild":
            if not isinstance(v, list):
                raise SelectError(f"[*] needs a list, got {type(v).__name__}")
            # BUG ([*] flattening): the wildcard is supposed to FAN OUT, adding
            # each element of the list to the frontier (``nxt.extend(v)``).
            # Instead it appends the list itself as a single nested value, so it
            # never actually fans out — a terminal ``[*]`` returns ``[[...]]``
            # and a following ``.key`` is applied to the whole list, not each
            # element.
            nxt.append(v)
    return nxt


def select(obj: Any, path: str) -> List[Any]:
    """Evaluate ``path`` against ``obj`` and return the flat list of matches.

    See the module docstring for the supported step syntax and semantics. A
    structural mismatch raises :class:`SelectError`; ``..key`` never raises for
    "not found" and may legitimately return an empty list.
    """
    steps = _parse(path)
    values: List[Any] = [obj]
    for step in steps:
        values = _apply(step, values)
    return values
