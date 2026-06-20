"""A pure, in-process path router.

`Router` maps registered URL-style paths to opaque handler objects. Nothing here
touches the network: it is a plain data structure you query by hand. A path is a
``/``-separated string; each segment is one of:

  - a STATIC segment that matches itself literally (``users``, ``v1``); or
  - a PARAM segment written ``{name}`` that matches exactly one non-empty
    segment and captures it under ``name`` (so ``/users/{id}`` matches
    ``/users/42`` capturing ``{"id": "42"}``).

Usage
-----
    >>> r = Router()
    >>> r.add("/users/{id}", "show_user")
    >>> r.match("/users/42")
    ('show_user', {'id': '42'})
    >>> r.match("/users/42/extra")
    (None, {})

Precedence
----------
When more than one registered route can match a path, a STATIC segment is
preferred over a PARAM segment at the same position, so a literal route always
wins over a parameterised one:

    >>> r = Router()
    >>> r.add("/users/{id}", "by_id")
    >>> r.add("/users/me", "me")
    >>> r.match("/users/me")
    ('me', {})
    >>> r.match("/users/42")
    ('by_id', {'id': '42'})

``match`` returns ``(handler, params)`` on a hit, or ``(None, {})`` when nothing
matches. Matching never raises on an unknown path; it just misses.

Standard library only. No wildcard / catch-all segments are supported yet.
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
    child (if any) matches any one segment and records the capture name.
    """

    __slots__ = ("static", "param_child", "param_name", "handler", "has_handler")

    def __init__(self) -> None:
        # literal segment -> child node
        self.static: dict[str, "_Node"] = {}
        # the (single) "{name}" child, if a param route passes through here
        self.param_child: Optional["_Node"] = None
        self.param_name: Optional[str] = None
        # handler registered at exactly this node (end of a route)
        self.handler: Any = None
        self.has_handler: bool = False


class Router:
    """A trie-backed path router with static and ``{param}`` segments."""

    def __init__(self) -> None:
        self._root = _Node()

    def add(self, path: str, handler: Any) -> None:
        """Register ``handler`` for ``path``.

        ``path`` segments are either static text or ``{name}`` params. Re-adding
        the same path replaces its handler.
        """
        node = self._root
        for seg in _split(path):
            if seg.startswith("{") and seg.endswith("}"):
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

        Returns ``(handler, params)`` for the best match, or ``(None, {})`` if
        nothing matches. Static segments take precedence over param segments at
        the same position.
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

        Tries the static child first (higher precedence), then the param child.
        Returns ``(handler, params_copy)`` on success or ``None`` on a miss.
        """
        if i == len(segments):
            if node.has_handler:
                return (node.handler, dict(params))
            return None

        seg = segments[i]

        # 1. static segment — highest precedence.
        static_child = node.static.get(seg)
        if static_child is not None:
            hit = self._match(static_child, segments, i + 1, params)
            if hit is not None:
                return hit

        # 2. param segment — matches exactly one segment, lower precedence.
        if node.param_child is not None:
            name = node.param_child.param_name
            params[name] = seg
            hit = self._match(node.param_child, segments, i + 1, params)
            if hit is not None:
                return hit
            del params[name]

        return None
