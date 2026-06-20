"""eventbus — a tiny in-memory publish/subscribe message bus with wildcards.

Public API is re-exported here for convenience; the implementation lives in
``eventbus.public``.

    >>> from eventbus import EventBus
    >>> bus = EventBus()
    >>> got = []
    >>> _ = bus.subscribe("a.#", lambda t, d: got.append(t))
    >>> bus.publish("a.b.c", 1)
    1
    >>> got
    ['a.b.c']
"""

from .public import EventBus, Subscription

__all__ = ["EventBus", "Subscription"]
