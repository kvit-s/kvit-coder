"""A small finite state machine.

A ``Machine`` has a set of states and a ``current`` state. You wire up
transitions with ``add_transition(state, event, target)`` and drive the machine
with ``fire(event)``: if the current state has a transition registered for that
event, the machine moves to the target state and returns it.

This is a FLAT machine: states have no structure, an event is handled only if
the *current* state defines a transition for it, and changing state does nothing
beyond updating ``current``.

The task (see ``brief.txt``) is to add HIERARCHICAL (nested) states with event
bubbling and entry/exit hooks, without breaking the flat behavior below.

Example
-------
    >>> m = Machine("idle")
    >>> m.add_transition("idle", "go", "running")
    >>> m.add_transition("running", "stop", "idle")
    >>> m.fire("go")
    'running'
    >>> m.current
    'running'
    >>> m.fire("stop")
    'idle'
"""

from __future__ import annotations

from typing import Dict, Optional, Tuple


class UnknownEvent(Exception):
    """Raised by ``fire`` when no state can handle the event."""


class Machine:
    """A flat finite state machine."""

    def __init__(self, initial: str) -> None:
        self._initial = initial
        self.current = initial
        # (state, event) -> target
        self._transitions: Dict[Tuple[str, str], str] = {}

    # -- wiring ------------------------------------------------------------

    def add_transition(self, state: str, event: str, target: str) -> None:
        """Register: while in ``state``, ``event`` moves the machine to ``target``."""
        self._transitions[(state, event)] = target

    # -- driving -----------------------------------------------------------

    def fire(self, event: str) -> str:
        """Handle ``event`` from the current state and return the new current state.

        If the current state has no transition for ``event``, raise
        ``UnknownEvent``.
        """
        target = self._transitions.get((self.current, event))
        if target is None:
            raise UnknownEvent(f"no transition for {event!r} from {self.current!r}")
        self.current = target
        return self.current

    def reset(self) -> str:
        """Return the machine to its initial state."""
        self.current = self._initial
        return self.current
