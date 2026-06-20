"""A pure, in-process path router (with wildcard / catch-all segments).

`Router` maps registered URL-style paths to opaque handler objects. Nothing here
touches the network: it is a plain data structure you query by hand. A path is a
``/``-separated string; each segment is one of:

  - a STATIC segment that matches itself literally (``users``, ``v1``);
  - a PARAM segment written ``{name}`` that matches exactly one non-empty
    segment and captures it under ``name``; or
  - a WILDCARD / catch-all segment written ``{name:*}`` that matches the entire
    REMAINING path (including slashes) and captures it under ``name``. A
    wildcard is only meaningful as the LAST segment of a route.

Usage
-----
    >>> r = Router()
    >>> r.add("/files/{path:*}", "serve")
    >>> r.match("/files/a/b/c")
    ('serve', {'path': 'a/b/c'})

Precedence
----------
When more than one registered route can match a path, the most specific wins:
STATIC beats PARAM beats WILDCARD at the same position. So a literal route always
wins over a param, and a param/static prefix always wins over a catch-all:

    >>> r = Router()
    >>> r.add("/files/{path:*}", "wild")
    >>> r.add("/files/readme", "exact")
    >>> r.match("/files/readme")
    ('exact', {})
    >>> r.match("/files/a/b")
    ('wild', {'path': 'a/b'})

``match`` returns ``(handler, params)`` on a hit, or ``(None, {})`` when nothing
matches. Matching never raises on an unknown path; it just misses.

Standard library only.
"""
from __future__ import annotations

from typing import Any, Optional


def _split(path: str) -> list[str]:
    """Split a path into its non-empty segments.

    Leading/trailing slashes and empty segments (from ``//``) are ignored, so
    ``/users/42/`` and ``users/42`` both yield ``["users", "42"]``. The root
    path ``/`` yields ``[]``.
    """
    return [seg for seg in path.split("/") if seg != ""]


class _Node:
    """One node in the route trie.

    Children are keyed by literal segment text for static routes. A single param
    child (if any) matches any one segment. A single wildcard child (if any)
    captures the remaining path; it is terminal.
    """

    __slots__ = (
        "static",
        "param_child",
        "param_name",
        "wildcard_name",
        "wildcard_handler",
        "has_wildcard",
        "handler",
        "has_handler",
    )

    def __init__(self) -> None:
        # literal segment -> child node
        self.static: dict[str, "_Node"] = {}
        # the (single) "{name}" child, if a param route passes through here
        self.param_child: Optional["_Node"] = None
        self.param_name: Optional[str] = None
        # a "{name:*}" catch-all registered at this node (captures rest of path)
        self.wildcard_name: Optional[str] = None
        self.wildcard_handler: Any = None
        self.has_wildcard: bool = False
        # handler registered at exactly this node (end of a route)
        self.handler: Any = None
        self.has_handler: bool = False


def _is_param(seg: str) -> bool:
    return seg.startswith("{") and seg.endswith("}") and ":" not in seg


def _is_wildcard(seg: str) -> bool:
    return seg.startswith("{") and seg.endswith("}") and seg[1:-1].endswith(":*")


class Router:
    """A trie-backed router with static, ``{param}`` and ``{name:*}`` segments."""

    def __init__(self) -> None:
        self._root = _Node()

    def add(self, path: str, handler: Any) -> None:
        """Register ``handler`` for ``path``.

        Segments are static text, ``{name}`` params, or a trailing ``{name:*}``
        wildcard that captures the rest of the path. Re-adding the same path
        replaces its handler.
        """
        node = self._root
        for seg in _split(path):
            if _is_wildcard(seg):
                # "{name:*}" — terminal catch-all hung off the current node.
                name = seg[1:-1][: -len(":*")]
                node.wildcard_name = name
                node.wildcard_handler = handler
                node.has_wildcard = True
                return
            if _is_param(seg):
                name = seg[1:-1]
                if node.param_child is None:
                    node.param_child = _Node()
                node.param_child.param_name = name
                node = node.param_child
            else:
                child = node.static.get(seg)
                if child is None:
                    child = _Node()
                    node.static[seg] = child
                node = child
        node.handler = handler
        node.has_handler = True

    def match(self, path: str) -> tuple[Any, dict]:
        """Match ``path`` against the registered routes.

        Returns ``(handler, params)`` for the best (most specific) match, or
        ``(None, {})`` if nothing matches. Precedence at each position is
        static > param > wildcard, so an exact prefix always beats a catch-all.
        """
        segments = _split(path)
        params: dict[str, str] = {}
        result = self._match(self._root, segments, 0, params)
        if result is None:
            return (None, {})
        return result

    def _match(
        self, node: "_Node", segments: list[str], i: int, params: dict
    ) -> Optional[tuple[Any, dict]]:
        """Recursively match ``segments[i:]`` under ``node``.

        Tries static, then param, then wildcard — that ordering is what gives
        static > param > wildcard precedence, because the first branch to reach a
        registered handler wins. Returns ``(handler, params_copy)`` or ``None``.
        """
        if i == len(segments):
            if node.has_handler:
                return (node.handler, dict(params))
            # An empty remainder can still feed a catch-all (captures "").
            if node.has_wildcard:
                out = dict(params)
                out[node.wildcard_name] = ""
                return (node.wildcard_handler, out)
            return None

        seg = segments[i]

        # 1. static segment — highest precedence.
        static_child = node.static.get(seg)
        if static_child is not None:
            hit = self._match(static_child, segments, i + 1, params)
            if hit is not None:
                return hit

        # 2. param segment — matches exactly one segment.
        if node.param_child is not None:
            name = node.param_child.param_name
            params[name] = seg
            hit = self._match(node.param_child, segments, i + 1, params)
            if hit is not None:
                return hit
            del params[name]

        # 3. wildcard / catch-all — lowest precedence; captures the rest of the
        #    path verbatim, slashes included.
        if node.has_wildcard:
            out = dict(params)
            out[node.wildcard_name] = "/".join(segments[i:])
            return (node.wildcard_handler, out)

        return None
