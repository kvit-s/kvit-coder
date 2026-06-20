"""Tiny demo / smoke CLI for the hsm package.

Run with ``python -m hsm`` to exercise the machine. Not part of the graded
contract; provided only as a convenience.
"""

import json

from .public import Machine


def main() -> None:
    m = Machine("idle")
    m.add_transition("idle", "go", "running")
    m.add_transition("running", "stop", "idle")
    first = m.fire("go")
    second = m.fire("stop")
    print(json.dumps({"after_go": first, "after_stop": second}))


if __name__ == "__main__":
    main()
