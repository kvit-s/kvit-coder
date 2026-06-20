"""eventbus — a tiny in-memory publish/subscribe message bus.

Public API is re-exported here for convenience; the implementation lives in
``eventbus.public``.

    >>> from eventbus import EventBus
    >>> bus = EventBus()
    >>> got = []
    >>> _ = bus.subscribe("hello", lambda t, d: got.append(d))
    >>> bus.publish("hello", 1)
    1
    >>> got
    [1]
"""

from .public import EventBus, Subscription

__all__ = ["EventBus", "Subscription"]
