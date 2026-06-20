"""Tiny demo / smoke CLI for the hsm package.

Run with ``python -m hsm`` to exercise nested states with bubbling and hooks.
Not part of the graded contract; provided only as a convenience.
"""

import json

from .public import Machine


def main() -> None:
    m = Machine("a")
    m.add_state("top")
    m.add_state("a", parent="top")
    m.add_state("b", parent="top")
    m.add_transition("top", "go", "b")  # handled by the parent, via bubbling

    new_state = m.fire("go")
    print(json.dumps({"after_go": new_state, "trace": m.trace}))


if __name__ == "__main__":
    main()
