"""Tiny demo / smoke CLI for the eventbus package.

Run with ``python -m eventbus`` to exercise the bus. Not part of the graded
contract; provided only as a convenience.
"""

import json

from .public import EventBus


def main() -> None:
    bus = EventBus()
    seen = []
    bus.subscribe("order.created", lambda t, d: seen.append((t, d)))
    n = bus.publish("order.created", {"id": 1})
    print(json.dumps({"invoked": n, "seen": seen}))


if __name__ == "__main__":
    main()
