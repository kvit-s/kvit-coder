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

_MISSING = object()


def _is_index(seg: str) -> bool:
    """True if ``seg`` is a plain non-negative integer literal (no sign, digits)."""
    return seg.isdigit()


def _split(path: str) -> list[str]:
    """Split a dotted path into its segments. An empty path has no segments."""
    if path == "":
        return []
    return path.split(".")


def _step(container, seg: str):
    """Resolve one segment against ``container``; return ``_MISSING`` if absent.

    A present value of ``None`` resolves to ``None`` (NOT missing): the segment
    was found, the stored value just happens to be ``None``.
    """
    if isinstance(container, dict):
        if seg in container:
            return container[seg]
        # Fall back to an int key only when the string key is absent.
        if _is_index(seg):
            ikey = int(seg)
            if ikey in container:
                return container[ikey]
        return _MISSING
    if isinstance(container, list):
        if _is_index(seg):
            idx = int(seg)
            if 0 <= idx < len(container):
                return container[idx]
        return _MISSING
    return _MISSING


def get(obj, path: str, default=None):
    """Return the value at dotted ``path`` in ``obj``, else ``default``.

    A stored ``None`` is returned as-is (it is a real, present value). ``default``
    is returned ONLY when a segment cannot be resolved (missing key, bad/out-of-
    range index, or a path that descends into a non-container).
    """
    cur = obj
    for seg in _split(path):
        nxt = _step(cur, seg)
        if nxt is _MISSING:
            return default
        cur = nxt
    return cur


def set_(obj, path: str, value):
    """Store ``value`` at dotted ``path``, auto-creating intermediates.

    Walks ``obj`` segment by segment. When an intermediate container is missing
    it is created: a ``list`` if the NEXT segment is a numeric index, otherwise a
    ``dict``. Returns the root object (mutated in place when possible).

    An empty path replaces nothing meaningful and simply returns ``obj``
    unchanged (there is no segment to assign).
    """
    segs = _split(path)
    if not segs:
        return obj

    # Seed a root container if the caller passed None (nothing to walk into).
    if obj is None:
        obj = [] if _is_index(segs[0]) else {}

    cur = obj
    for i, seg in enumerate(segs[:-1]):
        nxt_seg = segs[i + 1]
        want_list = _is_index(nxt_seg)
        cur = _descend_for_set(cur, seg, want_list)

    _assign(cur, segs[-1], value)
    return obj


def _descend_for_set(cur, seg: str, want_list: bool):
    """Return the child container at ``seg`` in ``cur``, creating it if missing.

    ``want_list`` decides the type of a freshly created child (list vs dict),
    based on whether the following segment is a numeric index.
    """
    if isinstance(cur, list):
        idx = int(seg)  # caller guarantees a numeric seg when cur is a list
        _grow_list(cur, idx)
        child = cur[idx]
        if not isinstance(child, (dict, list)):
            child = [] if want_list else {}
            cur[idx] = child
        return child
    # treat as dict
    key = seg
    child = cur.get(key, _MISSING)
    if child is _MISSING or not isinstance(child, (dict, list)):
        child = [] if want_list else {}
        cur[key] = child
    return child


def _assign(cur, seg: str, value) -> None:
    """Assign ``value`` at the final segment of a set_ walk."""
    if isinstance(cur, list):
        idx = int(seg)
        _grow_list(cur, idx)
        cur[idx] = value
    else:
        cur[seg] = value


def _grow_list(lst: list, idx: int) -> None:
    """Pad ``lst`` with ``None`` so that index ``idx`` is assignable."""
    while len(lst) <= idx:
        lst.append(None)
