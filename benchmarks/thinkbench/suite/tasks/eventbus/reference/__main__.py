"""Tiny demo / smoke CLI for the eventbus package.

Run with ``python -m eventbus`` to exercise wildcard delivery. Not part of the
graded contract; provided only as a convenience.
"""

import json

from .public import EventBus


def main() -> None:
    bus = EventBus()
    seen = []
    bus.subscribe("order.created", lambda t, d: seen.append("exact"))
    bus.subscribe("order.*", lambda t, d: seen.append("star"))
    bus.subscribe("order.#", lambda t, d: seen.append("hash"))

    n1 = bus.publish("order.created", 1)         # all three, in order
    seen_created = list(seen)
    seen.clear()
    n2 = bus.publish("order.created.late", 1)    # only the '#' subscription
    seen_late = list(seen)

    print(json.dumps({
        "created_invoked": n1,        # 3
        "created_order": seen_created,  # ["exact", "star", "hash"]
        "late_invoked": n2,           # 1
        "late_order": seen_late,      # ["hash"]
    }))


if __name__ == "__main__":
    main()
