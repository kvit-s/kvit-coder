"""Tiny demo / smoke CLI for the kvtxn package.

Run with ``python -m kvtxn`` to exercise nested transactions. Not part of the
graded contract; provided only as a convenience.
"""

import json

from .public import Store


def main() -> None:
    s = Store()
    s.set("a", 1)

    s.begin()                 # outer
    s.set("a", 2)
    s.begin()                 # inner
    s.set("a", 3)
    s.commit()                # fold inner into outer: a == 3, still in outer txn
    inner_committed = s.get("a")
    s.rollback()              # undo outer (which includes the inner's commit)
    after_outer_rollback = s.get("a")

    print(json.dumps({
        "inner_committed": inner_committed,      # 3
        "after_outer_rollback": after_outer_rollback,  # 1
    }))


if __name__ == "__main__":
    main()
