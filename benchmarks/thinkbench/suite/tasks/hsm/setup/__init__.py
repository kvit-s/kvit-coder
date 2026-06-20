"""hsm — a tiny finite state machine.

Public API is re-exported here for convenience; the implementation lives in
``hsm.public``.

    >>> from hsm import Machine
    >>> m = Machine("idle")
    >>> m.add_transition("idle", "go", "running")
    >>> m.fire("go")
    'running'
"""

from .public import Machine, UnknownEvent

__all__ = ["Machine", "UnknownEvent"]
