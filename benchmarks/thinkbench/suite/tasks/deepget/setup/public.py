"""deepget -- nested access by dotted path.

Two helpers walk an arbitrarily nested structure of dicts and lists using a
dotted *path* string such as ``"a.b.0.c"``:

* :func:`get` returns the value at ``path``, or ``default`` if any segment of
  the path cannot be resolved.
* :func:`set_` stores ``value`` at ``path``, auto-creating any missing
  intermediate containers along the way, and returns the (possibly newly
  created) root object.

A path is a ``.``-separated string of segments. Each segment selects into the
*current* container:

* If the current container is a ``dict``, the segment is used as a key. The raw
  string is tried first; if that key is absent and the segment is all digits,
  the integer form is tried too (so dicts keyed by ints are reachable).
* If the current container is a ``list``, the segment must be a non-negative
  integer index within range; anything else (non-numeric, negative, or out of
  range) fails to resolve.

Example
-------
    >>> obj = {"a": {"b": [{"c": 1}, {"c": 2}]}}
    >>> get(obj, "a.b.1.c")
    2
    >>> get(obj, "a.b.9.c", default="?")
    '?'
    >>> set_({}, "x.0.y", 7)
    {'x': [{'y': 7}]}
"""

from __future__ import annotations


def _is_index(seg: str) -> bool:
    """True if ``seg`` looks like an integer index."""
    return seg.isdigit()


def _split(path: str) -> list[str]:
    """Split a dotted path into its segments. An empty path has no segments."""
    if path == "":
        return []
    return path.split(".")


def _step(container, seg: str):
    """Resolve one segment against ``container``; return ``None`` if absent."""
    # A numeric segment is an index into a sequence.
    if _is_index(seg):
        return container[int(seg)]
    # Otherwise it is a mapping key.
    if isinstance(container, dict):
        return container.get(seg)
    return None


def get(obj, path: str, default=None):
    """Return the value at dotted ``path`` in ``obj``, else ``default``."""
    cur = obj
    for seg in _split(path):
        try:
            cur = _step(cur, seg)
        except (KeyError, IndexError, TypeError):
            return default
        if cur is None:
            return default
    return cur


def set_(obj, path: str, value):
    """Store ``value`` at dotted ``path``, creating intermediate dicts."""
    segs = _split(path)
    if not segs:
        return obj

    cur = obj
    for seg in segs[:-1]:
        if seg not in cur:
            cur[seg] = {}
        cur = cur[seg]

    cur[segs[-1]] = value
    return obj
