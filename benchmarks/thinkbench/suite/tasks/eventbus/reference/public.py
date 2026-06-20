"""In-memory publish/subscribe message bus with WILDCARD subscriptions.

Topics are dot-delimited strings of one or more non-empty segments. A
subscription topic may contain wildcard segments:

* ``*`` matches exactly ONE segment in that position (``"a.*"`` matches ``"a.b"``
  but not ``"a"`` or ``"a.b.c"``).
* ``#`` matches ONE OR MORE trailing segments and is only meaningful as the LAST
  segment (``"a.#"`` matches ``"a.b"`` and ``"a.b.c"`` but not ``"a"``; ``"#"``
  alone matches every topic of one or more segments).

``*`` and ``#`` are wildcards ONLY in subscription topics; in a published topic
they are ordinary literal text.

Delivery is deterministic: when several subscriptions match a publish, their
callbacks fire in the order the subscriptions were registered (globally), and
each subscription fires at most once per publish.

Implementation: subscriptions are kept in a single global list in registration
order. ``publish`` walks that list once, tests each subscription's compiled
pattern against the concrete topic, and invokes the matches in order — so global
subscription order and fire-once-per-subscription both fall out for free.

Example
-------
    >>> bus = EventBus()
    >>> got = []
    >>> _ = bus.subscribe("order.#", lambda t, d: got.append(t))
    >>> bus.publish("order.created.late", None)
    1
    >>> got
    ['order.created.late']
"""

from __future__ import annotations

from typing import Any, Callable, List


def _matches(pattern_segs: List[str], topic_segs: List[str]) -> bool:
    """Return True iff the concrete ``topic_segs`` match the subscription
    ``pattern_segs`` under the ``*`` / ``#`` wildcard rules.

    ``*`` consumes exactly one segment; a trailing ``#`` consumes one or more of
    the remaining segments. ``#`` is only treated as a wildcard when it is the
    final pattern segment (which is the only position the contract gives it
    meaning); any earlier ``#`` is matched literally.
    """
    i = 0
    for pi, pseg in enumerate(pattern_segs):
        if pseg == "#" and pi == len(pattern_segs) - 1:
            # Trailing '#': must have AT LEAST ONE remaining topic segment.
            return (len(topic_segs) - i) >= 1
        if i >= len(topic_segs):
            return False  # pattern still has required segments, topic exhausted
        if pseg == "*":
            i += 1  # matches exactly this one segment, whatever it is
            continue
        if pseg != topic_segs[i]:
            return False  # literal segment must match exactly
        i += 1
    # Pattern fully consumed: match iff the topic was fully consumed too.
    return i == len(topic_segs)


class Subscription:
    """An opaque handle returned by :meth:`EventBus.subscribe`."""

    __slots__ = ("topic", "fn", "order", "_segs")

    def __init__(self, topic: str, fn: Callable[[str, Any], Any], order: int) -> None:
        self.topic = topic
        self.fn = fn
        self.order = order
        self._segs = topic.split(".")


class EventBus:
    """A minimal in-memory pub/sub bus with wildcard topic matching."""

    def __init__(self) -> None:
        self._subs: List[Subscription] = []
        self._counter = 0

    def subscribe(self, topic: str, fn: Callable[[str, Any], Any]) -> Subscription:
        """Register ``fn`` to be called when a publish matches ``topic``.

        ``topic`` may contain ``*`` (single-segment) or a trailing ``#``
        (multi-segment) wildcard. Returns an opaque subscription handle; each
        call is a distinct subscription that fires at most once per publish.
        """
        sub = Subscription(topic, fn, self._counter)
        self._counter += 1
        self._subs.append(sub)
        return sub

    def publish(self, topic: str, data: Any) -> int:
        """Invoke every subscription whose pattern matches the concrete ``topic``.

        Callbacks fire in global subscription order, each subscription at most
        once. Returns the number of callbacks invoked.
        """
        topic_segs = topic.split(".")
        count = 0
        # Snapshot so a handler that (un)subscribes mid-publish can't corrupt
        # the walk; delivery order is the registration order of this snapshot.
        for sub in list(self._subs):
            if _matches(sub._segs, topic_segs):
                sub.fn(topic, data)
                count += 1
        return count
