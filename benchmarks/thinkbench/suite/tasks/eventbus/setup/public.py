"""In-memory publish/subscribe message bus.

A tiny event bus: register callbacks against topics with ``subscribe`` and fan
out messages with ``publish``. Topics are dot-delimited strings of one or more
non-empty segments, e.g. ``"order"`` or ``"sensor.kitchen.temp"``.

The core EXACT-topic delivery works today: ``publish(topic, data)`` invokes
every callback subscribed to that exact ``topic``, passing ``(topic, data)``,
in the order the subscriptions were registered.

There is a FIRST attempt at wildcard subscriptions (a subscription topic may
contain ``*`` or ``#`` segments), but it does NOT actually work: wildcard
subscriptions are stored, yet matching still compares topics by plain string
equality, so a wildcard subscription only ever fires if a publisher publishes to
the literal wildcard string. See ``brief.txt`` for the intended semantics.

Example
-------
    >>> bus = EventBus()
    >>> got = []
    >>> _ = bus.subscribe("order.created", lambda t, d: got.append((t, d)))
    >>> bus.publish("order.created", 7)
    1
    >>> got
    [('order.created', 7)]
"""

from __future__ import annotations

from typing import Any, Callable, List


class Subscription:
    """An opaque handle returned by :meth:`EventBus.subscribe`."""

    __slots__ = ("topic", "fn", "order")

    def __init__(self, topic: str, fn: Callable[[str, Any], Any], order: int) -> None:
        self.topic = topic
        self.fn = fn
        self.order = order


class EventBus:
    """A minimal in-memory pub/sub bus (exact topics only, so far)."""

    def __init__(self) -> None:
        self._subs: List[Subscription] = []
        self._counter = 0

    def subscribe(self, topic: str, fn: Callable[[str, Any], Any]) -> Subscription:
        """Register ``fn`` to be called when ``topic`` is published.

        The ``topic`` may contain ``*`` / ``#`` wildcard segments, but the
        current matching logic ignores that and compares topics literally.
        """
        sub = Subscription(topic, fn, self._counter)
        self._counter += 1
        self._subs.append(sub)
        return sub

    def publish(self, topic: str, data: Any) -> int:
        """Invoke every subscription matching ``topic``; return how many fired.

        Matching is plain string equality for now, so wildcard subscriptions do
        not actually fire on concrete topics.
        """
        count = 0
        for sub in self._subs:
            if sub.topic == topic:
                sub.fn(topic, data)
                count += 1
        return count
