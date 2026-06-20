#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `hsm`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `hsm` package
against the BRIEF'S CONTRACT (hierarchical states with event bubbling and
ordered entry/exit hooks, plus the unchanged flat API), NOT against any
particular internal file layout.

The defining behaviors under test are the ones a SUPERFICIAL implementation gets
wrong:

  * the exit/enter set is bounded by the LEAST COMMON ANCESTOR — a naive impl
    that exits to the root and re-enters from the root fires extra hooks for the
    shared ancestors (wrong trace);
  * the exited/entered states are computed from the actual current LEAF and the
    target, NOT from the ancestor where a bubbled transition was found;
  * an external self-transition exits AND re-enters its own state;
  * exit hooks fire deepest-first while entry hooks fire shallowest-first.

The shipped flat machine handles only same-state transitions and keeps no
hierarchy/hooks, so it fails every nesting/hook check while passing the flat
regression checks — that's what makes the task discriminate (a naive nested
attempt lands well under 1.0; a careful LCA-aware implementation lands at 1.0).

Output: a single JSON scorecard on stdout. Each check runs in isolation, so the
score is continuous (passed / total), never all-or-nothing. FIXED DENOMINATOR:
the full check roster is declared up front, so an import failure records every
check as failed and forces score 0.0. Exit code is 0 whenever grading ran to
completion (even score 0.0); the process never raises out.
"""
import importlib
import json
import os
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# --- fixed denominator: the full roster of checks, declared before any run ----
CHECK_SPECS = [
    ("flat_move", "flat fire moves current to the target"),
    ("flat_unknown_event", "flat fire of an unhandled event raises UnknownEvent"),
    ("flat_trace", "flat move traces exit(source) then enter(target)"),
    ("bubble_to_parent", "event unhandled by leaf bubbles to a parent that handles it"),
    ("bubble_target_from_leaf", "bubbled transition exits the actual LEAF, not the handling ancestor"),
    ("lca_not_exited_or_entered", "least common ancestor is neither exited nor entered"),
    ("exit_order_deepest_first", "exit hooks fire deepest-first up to the LCA"),
    ("enter_order_shallowest_first", "enter hooks fire shallowest-first down to the target"),
    ("self_transition_reenters", "external self-transition exits and re-enters the state"),
    ("hook_callbacks_fire_in_order", "registered enter/exit callbacks run in trace order"),
    ("separate_trees_full_chains", "transition across trees exits/enters full chains"),
    ("trace_accumulates", "trace accumulates across multiple fire calls"),
    ("unknown_event_after_bubble", "event no ancestor handles raises UnknownEvent"),
    ("regression_flat_reset", "reset returns to initial; flat wiring still works"),
]
CHECK_IDS = [cid for cid, _ in CHECK_SPECS]
DESC = dict(CHECK_SPECS)

results = {}  # cid -> {"passed": bool, "detail": str}


def record(cid, passed, detail=""):
    results[cid] = {"passed": bool(passed), "detail": str(detail or "")}


def check(cid, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


# --- import the produced package (contract: hsm.public, fallback hsm) ----------
import_ok = True
import_detail = ""
Machine = None
UnknownEvent = None
try:
    try:
        mod = importlib.import_module("hsm.public")
    except Exception:
        mod = importlib.import_module("hsm")
    Machine = getattr(mod, "Machine")
    # UnknownEvent is part of the contract; fall back to a broad type so the
    # error checks still grade something sensible if the name is missing.
    try:
        pkg = importlib.import_module("hsm")
        UnknownEvent = getattr(pkg, "UnknownEvent", None)
    except Exception:
        UnknownEvent = None
    if UnknownEvent is None:
        UnknownEvent = getattr(mod, "UnknownEvent", Exception)
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def _build_nested():
    """A two-deep tree shared by several checks.

        root
        ├── A ── a1
        └── B ── b1
    """
    m = Machine("a1")
    m.add_state("root")
    m.add_state("A", parent="root")
    m.add_state("B", parent="root")
    m.add_state("a1", parent="A")
    m.add_state("b1", parent="B")
    return m


if import_ok:
    # 1. flat machine: fire moves to the target.
    def c_flat_move():
        m = Machine("idle")
        m.add_transition("idle", "go", "running")
        out = m.fire("go")
        return (out == "running" and m.current == "running"), \
            f"fire('go') -> {out!r}, current={m.current!r} (expected 'running')"

    check("flat_move", c_flat_move)

    # 2. flat machine: unhandled event raises UnknownEvent.
    def c_flat_unknown_event():
        m = Machine("idle")
        m.add_transition("idle", "go", "running")
        try:
            m.fire("nope")
            return False, "fire('nope') did not raise"
        except Exception as e:  # noqa: BLE001
            return isinstance(e, UnknownEvent), f"raised {type(e).__name__} (want UnknownEvent)"

    check("flat_unknown_event", c_flat_unknown_event)

    # 3. flat move traces exit(source) then enter(target).
    def c_flat_trace():
        m = Machine("idle")
        m.add_transition("idle", "go", "running")
        m.fire("go")
        expected = [("exit", "idle"), ("enter", "running")]
        got = list(m.trace)
        return got == expected, f"trace={got!r} (expected {expected!r})"

    check("flat_trace", c_flat_trace)

    # 4. event unhandled by the leaf bubbles to a parent that handles it.
    def c_bubble_to_parent():
        m = Machine("a")
        m.add_state("top")
        m.add_state("a", parent="top")
        m.add_state("b", parent="top")
        m.add_transition("top", "go", "b")  # only the PARENT handles 'go'
        out = m.fire("go")
        return (out == "b" and m.current == "b"), \
            f"fire('go') -> {out!r}, current={m.current!r} (expected 'b')"

    check("bubble_to_parent", c_bubble_to_parent)

    # 5. a bubbled transition exits the actual LEAF, not the handling ancestor.
    def c_bubble_target_from_leaf():
        m = Machine("a")
        m.add_state("top")
        m.add_state("a", parent="top")
        m.add_state("b", parent="top")
        m.add_transition("top", "go", "b")
        m.fire("go")
        # LCA of a and b is 'top'; exit 'a' (not 'top'), enter 'b'.
        expected = [("exit", "a"), ("enter", "b")]
        got = list(m.trace)
        return got == expected, f"trace={got!r} (expected {expected!r})"

    check("bubble_target_from_leaf", c_bubble_target_from_leaf)

    # 6. the least common ancestor is neither exited nor entered.
    def c_lca_not_exited_or_entered():
        m = _build_nested()
        m.add_transition("a1", "x", "b1")
        m.fire("x")
        states_touched = [s for _, s in m.trace]
        # 'root' is the LCA of a1 and b1 -> must NOT appear.
        return ("root" not in states_touched), \
            f"trace touched={states_touched!r} (must not include 'root')"

    check("lca_not_exited_or_entered", c_lca_not_exited_or_entered)

    # 7. exit hooks fire deepest-first up to (excluding) the LCA.
    def c_exit_order_deepest_first():
        m = _build_nested()
        m.add_transition("a1", "x", "b1")
        m.fire("x")
        exits = [s for kind, s in m.trace if kind == "exit"]
        expected = ["a1", "A"]
        return exits == expected, f"exits={exits!r} (expected {expected!r})"

    check("exit_order_deepest_first", c_exit_order_deepest_first)

    # 8. enter hooks fire shallowest-first down to the target.
    def c_enter_order_shallowest_first():
        m = _build_nested()
        m.add_transition("a1", "x", "b1")
        m.fire("x")
        enters = [s for kind, s in m.trace if kind == "enter"]
        expected = ["B", "b1"]
        return enters == expected, f"enters={enters!r} (expected {expected!r})"

    check("enter_order_shallowest_first", c_enter_order_shallowest_first)

    # 9. an external self-transition exits AND re-enters its own state.
    def c_self_transition_reenters():
        m = Machine("s")
        m.add_state("p")
        m.add_state("s", parent="p")
        m.add_transition("s", "loop", "s")
        m.fire("loop")
        expected = [("exit", "s"), ("enter", "s")]
        got = list(m.trace)
        return (got == expected and m.current == "s"), \
            f"trace={got!r}, current={m.current!r} (expected {expected!r} / 's')"

    check("self_transition_reenters", c_self_transition_reenters)

    # 10. registered enter/exit callbacks actually run, in trace order.
    def c_hook_callbacks_fire_in_order():
        m = _build_nested()
        m.add_transition("a1", "x", "b1")
        log = []
        m.on_exit("a1", lambda: log.append("exit:a1"))
        m.on_exit("A", lambda: log.append("exit:A"))
        m.on_enter("B", lambda: log.append("enter:B"))
        m.on_enter("b1", lambda: log.append("enter:b1"))
        m.fire("x")
        expected = ["exit:a1", "exit:A", "enter:B", "enter:b1"]
        return log == expected, f"callbacks={log!r} (expected {expected!r})"

    check("hook_callbacks_fire_in_order", c_hook_callbacks_fire_in_order)

    # 11. a transition across SEPARATE trees exits/enters the full chains.
    def c_separate_trees_full_chains():
        m = Machine("a1")
        m.add_state("ra")
        m.add_state("a0", parent="ra")
        m.add_state("a1", parent="a0")
        m.add_state("rb")
        m.add_state("b0", parent="rb")
        m.add_transition("a1", "jump", "b0")
        m.fire("jump")
        # No common ancestor: exit a1, a0, ra; enter rb, b0.
        expected = [("exit", "a1"), ("exit", "a0"), ("exit", "ra"),
                    ("enter", "rb"), ("enter", "b0")]
        got = list(m.trace)
        return got == expected, f"trace={got!r} (expected {expected!r})"

    check("separate_trees_full_chains", c_separate_trees_full_chains)

    # 12. trace accumulates across multiple fire calls.
    def c_trace_accumulates():
        m = Machine("a")
        m.add_state("top")
        m.add_state("a", parent="top")
        m.add_state("b", parent="top")
        m.add_transition("a", "go", "b")
        m.add_transition("b", "back", "a")
        m.fire("go")
        m.fire("back")
        expected = [("exit", "a"), ("enter", "b"), ("exit", "b"), ("enter", "a")]
        got = list(m.trace)
        return got == expected, f"trace={got!r} (expected {expected!r})"

    check("trace_accumulates", c_trace_accumulates)

    # 13. an event no ancestor in the chain handles raises UnknownEvent.
    def c_unknown_event_after_bubble():
        m = _build_nested()
        m.add_transition("A", "x", "b1")  # handled in the A subtree only
        try:
            m.fire("zzz")  # nothing on a1 -> A -> root handles it
            return False, "fire('zzz') did not raise"
        except Exception as e:  # noqa: BLE001
            return isinstance(e, UnknownEvent), f"raised {type(e).__name__} (want UnknownEvent)"

    check("unknown_event_after_bubble", c_unknown_event_after_bubble)

    # 14. REGRESSION: reset returns to initial; flat wiring keeps working.
    def c_regression_flat_reset():
        m = Machine("idle")
        m.add_transition("idle", "go", "running")
        m.add_transition("running", "stop", "idle")
        a = m.fire("go")
        b = m.fire("stop")
        c = m.reset()
        # drive again after reset to confirm wiring is intact
        d = m.fire("go")
        ok = (a == "running" and b == "idle" and c == "idle" and d == "running")
        return ok, f"go={a!r} stop={b!r} reset={c!r} go2={d!r} (expected running/idle/idle/running)"

    check("regression_flat_reset", c_regression_flat_reset)


# --- assemble the scorecard with a FIXED denominator -------------------------
checks_out = []
for cid in CHECK_IDS:
    r = results.get(cid)
    if r is None:
        r = {"passed": False, "detail": "not run (import failed)" if not import_ok else "not run"}
    checks_out.append({"id": cid, "desc": DESC[cid], "passed": r["passed"], "detail": r["detail"]})

passed = sum(1 for c in checks_out if c["passed"])
total = len(checks_out)  # always len(CHECK_SPECS): fixed denominator
card = {
    "task": "hsm",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
