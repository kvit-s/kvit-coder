"""A hierarchical finite state machine (statechart-style).

A ``Machine`` has states that may be NESTED: each state can declare a parent,
forming an ancestor chain up to a root. On top of the flat behavior (wire up
transitions with ``add_transition``, drive with ``fire``) this adds:

* **Event bubbling.** ``fire(event)`` looks for a transition on the current
  (leaf) state; if that state defines none for the event, the event BUBBLES up
  the parent chain and the first ancestor that defines a transition for it wins.
  The machine still moves to that transition's ``target`` (and ``current``
  becomes the target leaf).

* **Entry / exit hooks.** ``on_enter(state, fn)`` / ``on_exit(state, fn)``
  register callbacks. On a transition from ``source`` to ``target`` the machine
  exits states from the old leaf UP TO (but not including) the least common
  ancestor of source and target, then enters states from below the common
  ancestor DOWN TO the target. The common ancestor is neither exited nor
  entered. Exit hooks fire deepest-first; entry hooks fire shallowest-first.

The firing order is recorded on ``Machine.trace`` for testing — a list of
``("exit", state)`` / ``("enter", state)`` tuples, in the exact order callbacks
fired, accumulated across every ``fire`` call.

Subtleties worth stating precisely:

* The set of states exited/entered is computed from the actual current LEAF
  state and the transition's target, NOT from the ancestor where the matching
  transition happened to be found while bubbling.
* A transition whose source and target are the SAME state is an *external*
  self-transition: that state is exited and re-entered (its common ancestor with
  itself is its parent, so it is below the LCA).
* If source and target share no ancestor (separate trees) every state from the
  source up to its root is exited and every state from the target's root down is
  entered.

Example
-------
    >>> m = Machine("a")
    >>> m.add_state("top")
    >>> m.add_state("a", parent="top")
    >>> m.add_state("b", parent="top")
    >>> m.add_transition("top", "go", "b")   # 'go' handled by the PARENT
    >>> m.fire("go")          # bubbles a -> top, moves to b
    'b'
    >>> m.trace
    [('exit', 'a'), ('enter', 'b')]
"""

from __future__ import annotations

from typing import Callable, Dict, List, Optional, Tuple


class UnknownEvent(Exception):
    """Raised by ``fire`` when no state in the chain can handle the event."""


class Machine:
    """A hierarchical finite state machine."""

    def __init__(self, initial: str) -> None:
        self._initial = initial
        self.current = initial
        # (state, event) -> target
        self._transitions: Dict[Tuple[str, str], str] = {}
        # state -> parent state (or None for a root / unregistered state)
        self._parent: Dict[str, Optional[str]] = {}
        # state -> list of enter/exit callbacks
        self._on_enter: Dict[str, List[Callable[[], None]]] = {}
        self._on_exit: Dict[str, List[Callable[[], None]]] = {}
        # ordered log of ("enter"/"exit", state) as callbacks fire
        self.trace: List[Tuple[str, str]] = []

    # -- wiring ------------------------------------------------------------

    def add_state(self, name: str, parent: Optional[str] = None) -> None:
        """Declare a state, optionally nested inside ``parent``."""
        self._parent[name] = parent

    def add_transition(self, state: str, event: str, target: str) -> None:
        """Register: while in ``state`` (or a descendant), ``event`` -> ``target``."""
        self._transitions[(state, event)] = target

    def on_enter(self, state: str, fn: Callable[[], None]) -> None:
        """Register a callback fired when ``state`` is entered."""
        self._on_enter.setdefault(state, []).append(fn)

    def on_exit(self, state: str, fn: Callable[[], None]) -> None:
        """Register a callback fired when ``state`` is exited."""
        self._on_exit.setdefault(state, []).append(fn)

    # -- driving -----------------------------------------------------------

    def fire(self, event: str) -> str:
        """Handle ``event`` from the current state (with bubbling) and transition.

        Walk from the current leaf up the parent chain; the first state that has
        a transition registered for ``event`` provides the target. Then run the
        exit/enter hooks for the move from the current leaf to that target and
        update ``current``. Raise ``UnknownEvent`` if no ancestor handles it.
        """
        source = self.current
        node: Optional[str] = source
        while node is not None:
            target = self._transitions.get((node, event))
            if target is not None:
                self._transition(source, target)
                return self.current
            node = self._parent.get(node)
        raise UnknownEvent(f"no transition for {event!r} from {source!r}")

    def reset(self) -> str:
        """Return the machine to its initial state (no hooks fired)."""
        self.current = self._initial
        return self.current

    # -- internals ---------------------------------------------------------

    def _ancestors(self, state: str) -> List[str]:
        """``[state, parent, grandparent, ...]`` up to the root."""
        chain: List[str] = []
        node: Optional[str] = state
        seen = set()
        while node is not None and node not in seen:
            seen.add(node)
            chain.append(node)
            node = self._parent.get(node)
        return chain

    def _transition(self, source: str, target: str) -> None:
        """Fire exit hooks from source up to the LCA, then enter hooks down to target."""
        src_chain = self._ancestors(source)        # leaf -> root
        tgt_chain = self._ancestors(target)        # leaf -> root
        tgt_set = set(tgt_chain)

        # Least common ancestor: the FIRST state on source's chain that is also an
        # ancestor of target. For an external self-transition (source == target)
        # the LCA is the parent, so the state itself is exited and re-entered.
        lca: Optional[str] = None
        for anc in src_chain:
            if anc == source and target == source:
                # self-transition: skip source so we don't treat it as the LCA
                continue
            if anc in tgt_set:
                lca = anc
                break

        # Exit: from the source leaf up to (not including) the LCA, deepest first.
        for state in src_chain:
            if state == lca:
                break
            self._fire_hooks(self._on_exit, "exit", state)

        # Enter: from just below the LCA down to the target leaf, shallowest first.
        entry_path = []
        for state in tgt_chain:
            if state == lca:
                break
            entry_path.append(state)
        for state in reversed(entry_path):
            self._fire_hooks(self._on_enter, "enter", state)

        self.current = target

    def _fire_hooks(self, table: Dict[str, List[Callable[[], None]]],
                    kind: str, state: str) -> None:
        self.trace.append((kind, state))
        for fn in table.get(state, []):
            fn()
