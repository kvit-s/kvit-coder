#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `serialhook`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `serialhook`
package against the BRIEF'S CONTRACT (custom-type hooks via tagged forms, plus
circular-reference detection, plus the unchanged basic-type behavior), NOT
against any particular internal file layout.

The defining behaviors under test are the ones a SUPERFICIAL implementation gets
wrong:

  * a registered type NESTED inside lists / dict values / another registered
    type's payload must round-trip (a hook that only fires at the top level
    fails these);
  * the encoded payload is serialized RECURSIVELY, so a registered type inside
    another registered type's payload round-trips (a hook that json.dumps()es
    the payload directly fails);
  * decode dispatch is BY TAG, and an UNKNOWN tag raises (a hook that leaks the
    raw tagged dict fails);
  * a plain dict that merely contains a "__type__" key is data, not a tag (an
    over-eager decoder corrupts it);
  * circular detection is by ANCESTRY along the path, so a SHARED / diamond
    reference is fine while a true cycle raises (a flat "seen" set false-positives
    on shared refs; no guard at all recurses forever);
  * basic-type output stays byte-identical to json.dumps defaults.

The shipped base handles only basic types via a json wrapper, so it fails every
hook and cycle check while passing the basic-type regression checks — that's
what makes the task discriminate (a naive hook lands well under 1.0, a careful
implementation lands at 1.0).

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
from datetime import datetime
from decimal import Decimal

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# --- fixed denominator: the full roster of checks, declared before any run ----
CHECK_SPECS = [
    ("register_top_level", "a registered type round-trips at the top level via the tagged form"),
    ("tagged_wire_shape", "a registered value serializes to {'__type__': tag, 'value': <encoded>}"),
    ("registered_in_list", "a registered type nested inside a list round-trips"),
    ("registered_in_dict_value", "a registered type nested as a dict value round-trips"),
    ("registered_in_payload", "a registered type inside another registered type's payload round-trips"),
    ("two_types_mixed", "two registered types in one structure each decode via their own tag"),
    ("unknown_tag_raises", "loads of a tagged form with an unregistered tag raises UnknownTagError"),
    ("plain_type_key_is_data", "a plain dict with a '__type__' key among others is data, not a tag"),
    ("exact_two_key_required", "a dict with __type__/value plus a third key is data, not a tag"),
    ("cycle_self_list", "dumps of a list that contains itself raises CircularReferenceError"),
    ("cycle_self_dict", "dumps of a dict that contains itself raises CircularReferenceError"),
    ("cycle_indirect", "dumps of an indirect (a->b->a) cycle raises CircularReferenceError"),
    ("shared_ref_ok", "a shared (diamond) reference is NOT a cycle and serializes fine"),
    ("shared_ref_reused_after", "the same list reused in a sibling AND deeper position serializes fine"),
    ("regression_basic_roundtrip", "basic types round-trip: loads(dumps(x)) == x"),
    ("regression_byte_identical", "dumps of basic values is byte-identical to json.dumps defaults"),
    ("regression_bool_int_distinct", "bool/int/float/None survive the round-trip with the right types"),
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


# --- import the produced package (contract: serialhook.public, fallback serialhook)
import_ok = True
import_detail = ""
dumps = None
loads = None
register = None
CircErr = None
UnknownErr = None
try:
    try:
        mod = importlib.import_module("serialhook.public")
    except Exception:
        mod = importlib.import_module("serialhook")
    # dumps/loads are the irreducible core: without them there is nothing to
    # grade, so their absence is a hard import failure (forces score 0.0).
    dumps = getattr(mod, "dumps")
    loads = getattr(mod, "loads")
    # register and the error classes are part of the FEATURE under test. A base
    # that lacks them must still grade — and PASS the basic-type regression
    # checks — instead of collapsing to 0.0. So look them up softly: a missing
    # name leaves a sentinel that the relevant checks treat as a failure.
    try:
        pkg = importlib.import_module("serialhook")
    except Exception:
        pkg = mod

    def _soft(name):
        return getattr(pkg, name, None) or getattr(mod, name, None)

    register = _soft("register")
    # For the error checks, fall back to a private sentinel that no real error
    # can be an instance of, so "missing error class" reads as a failed check
    # (not an accidental pass against bare Exception).
    class _NeverMatches(Exception):
        pass

    CircErr = _soft("CircularReferenceError") or _NeverMatches
    UnknownErr = _soft("UnknownTagError") or _NeverMatches
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def _reg_datetime():
    register(
        datetime, "datetime",
        lambda dt: dt.isoformat(),
        lambda s: datetime.fromisoformat(s),
    )


def _reg_decimal():
    register(Decimal, "decimal", str, Decimal)


if import_ok:
    # 1. a registered type round-trips at the top level.
    def c_register_top_level():
        _reg_datetime()
        dt = datetime(2020, 1, 2, 3, 4, 5)
        back = loads(dumps(dt))
        return back == dt, f"round-tripped datetime -> {back!r} (expected {dt!r})"

    check("register_top_level", c_register_top_level)

    # 2. the on-the-wire shape is the tagged form (we parse the JSON ourselves
    #    rather than string-match, so formatting differences don't matter).
    def c_tagged_wire_shape():
        _reg_datetime()
        dt = datetime(2020, 1, 2, 3, 4, 5)
        wire = json.loads(dumps(dt))
        ok = (
            isinstance(wire, dict)
            and wire.get("__type__") == "datetime"
            and wire.get("value") == dt.isoformat()
            and set(wire.keys()) == {"__type__", "value"}
        )
        return ok, f"wire form -> {wire!r}"

    check("tagged_wire_shape", c_tagged_wire_shape)

    # 3. registered type nested inside a list.
    def c_registered_in_list():
        _reg_datetime()
        dt = datetime(1999, 12, 31, 23, 59, 58)
        src = ["a", dt, "b"]
        back = loads(dumps(src))
        return back == src, f"round-tripped list -> {back!r} (expected {src!r})"

    check("registered_in_list", c_registered_in_list)

    # 4. registered type nested as a dict value.
    def c_registered_in_dict_value():
        _reg_datetime()
        dt = datetime(2021, 6, 7, 8, 9, 10)
        src = {"when": dt, "tags": ["x", "y"]}
        back = loads(dumps(src))
        return back == src, f"round-tripped dict -> {back!r} (expected {src!r})"

    check("registered_in_dict_value", c_registered_in_dict_value)

    # 5. a registered type INSIDE another registered type's payload. encode for
    #    'box' returns a dict that itself holds a Decimal, so the payload must be
    #    serialized recursively.
    def c_registered_in_payload():
        _reg_decimal()

        class Box:
            def __init__(self, amount):
                self.amount = amount  # a Decimal

            def __eq__(self, other):
                return isinstance(other, Box) and other.amount == self.amount

        register(
            Box, "box",
            lambda b: {"amount": b.amount},   # payload contains a Decimal
            lambda d: Box(d["amount"]),
        )
        src = Box(Decimal("1.50"))
        back = loads(dumps(src))
        ok = back == src and isinstance(getattr(back, "amount", None), Decimal)
        return ok, f"round-tripped Box -> {getattr(back, 'amount', back)!r} (expected Decimal('1.50'))"

    check("registered_in_payload", c_registered_in_payload)

    # 6. two distinct registered types in one structure, each via its own tag.
    def c_two_types_mixed():
        _reg_datetime()
        _reg_decimal()
        dt = datetime(2000, 1, 1, 0, 0, 0)
        src = {"t": dt, "amounts": [Decimal("1"), Decimal("2.25")]}
        back = loads(dumps(src))
        ok = (
            back == src
            and isinstance(back["t"], datetime)
            and all(isinstance(a, Decimal) for a in back["amounts"])
        )
        return ok, f"round-tripped -> {back!r} (expected {src!r})"

    check("two_types_mixed", c_two_types_mixed)

    # 7. a tagged form naming an UNREGISTERED tag must raise UnknownTagError.
    def c_unknown_tag_raises():
        wire = json.dumps({"__type__": "definitely_not_registered_xyz", "value": 1})
        try:
            out = loads(wire)
            return False, f"loads of unknown tag did not raise -> {out!r}"
        except Exception as e:  # noqa: BLE001
            ok = isinstance(e, UnknownErr)
            return ok, f"raised {type(e).__name__} (want UnknownTagError)"

    check("unknown_tag_raises", c_unknown_tag_raises)

    # 8. a plain dict carrying a '__type__' key AMONG OTHERS is data, not a tag.
    def c_plain_type_key_is_data():
        src = {"__type__": "x", "n": 1, "ok": True}
        back = loads(dumps(src))
        return back == src, f"round-tripped -> {back!r} (expected {src!r})"

    check("plain_type_key_is_data", c_plain_type_key_is_data)

    # 9. a dict with __type__ + value PLUS a third key is NOT a tagged form: it
    #    has the wrong arity, so it must survive as ordinary data even though the
    #    tag would be registered.
    def c_exact_two_key_required():
        _reg_datetime()
        src = {"__type__": "datetime", "value": "2020-01-01T00:00:00", "extra": 9}
        back = loads(dumps(src))
        return back == src, f"round-tripped -> {back!r} (expected {src!r})"

    check("exact_two_key_required", c_exact_two_key_required)

    # 10. a list that contains itself raises CircularReferenceError.
    def c_cycle_self_list():
        a = [1, 2]
        a.append(a)
        try:
            out = dumps(a)
            return False, f"dumps of self-list did not raise -> {out[:40]!r}"
        except Exception as e:  # noqa: BLE001
            ok = isinstance(e, CircErr)
            return ok, f"raised {type(e).__name__} (want CircularReferenceError)"

    check("cycle_self_list", c_cycle_self_list)

    # 11. a dict that contains itself raises CircularReferenceError.
    def c_cycle_self_dict():
        d = {"k": 1}
        d["self"] = d
        try:
            out = dumps(d)
            return False, f"dumps of self-dict did not raise -> {out[:40]!r}"
        except Exception as e:  # noqa: BLE001
            ok = isinstance(e, CircErr)
            return ok, f"raised {type(e).__name__} (want CircularReferenceError)"

    check("cycle_self_dict", c_cycle_self_dict)

    # 12. an indirect cycle a -> b -> a raises CircularReferenceError.
    def c_cycle_indirect():
        a = {}
        b = {"a": a}
        a["b"] = b
        try:
            out = dumps({"root": a})
            return False, f"dumps of indirect cycle did not raise -> {out[:40]!r}"
        except Exception as e:  # noqa: BLE001
            ok = isinstance(e, CircErr)
            return ok, f"raised {type(e).__name__} (want CircularReferenceError)"

    check("cycle_indirect", c_cycle_indirect)

    # 13. a SHARED (diamond) reference is NOT a cycle: same inner list under two
    #     sibling keys must serialize without error and round-trip.
    def c_shared_ref_ok():
        inner = [1, 2, 3]
        src = {"x": inner, "y": inner}
        try:
            back = loads(dumps(src))
        except Exception as e:  # noqa: BLE001
            return False, f"shared ref wrongly raised {type(e).__name__}: {e}"
        return back == {"x": [1, 2, 3], "y": [1, 2, 3]}, f"round-tripped -> {back!r}"

    check("shared_ref_ok", c_shared_ref_ok)

    # 14. same list reused as a sibling AND nested deeper — still not a cycle.
    def c_shared_ref_reused_after():
        inner = ["s"]
        src = [inner, {"again": inner}, inner]
        try:
            back = loads(dumps(src))
        except Exception as e:  # noqa: BLE001
            return False, f"reused ref wrongly raised {type(e).__name__}: {e}"
        return back == [["s"], {"again": ["s"]}, ["s"]], f"round-tripped -> {back!r}"

    check("shared_ref_reused_after", c_shared_ref_reused_after)

    # 15. REGRESSION: basic types round-trip to equal values.
    def c_regression_basic_roundtrip():
        src = {"b": 1, "a": [True, None, 1.5, "x"], "n": {"deep": [1, 2]}}
        back = loads(dumps(src))
        return back == src, f"round-tripped -> {back!r} (expected {src!r})"

    check("regression_basic_roundtrip", c_regression_basic_roundtrip)

    # 16. REGRESSION: basic output is byte-identical to json.dumps defaults. The
    #     grader computes the expected string itself.
    def c_regression_byte_identical():
        cases = [
            {"b": 1, "a": [True, None, 1.5]},
            [1, 2, 3],
            "hello",
            1.0,
            True,
            None,
            {"nested": {"k": [False, "v"]}},
        ]
        bad = []
        for x in cases:
            expected = json.dumps(x)
            got = dumps(x)
            if got != expected:
                bad.append(f"{x!r}: got {got!r} != {expected!r}")
        return (not bad), ("; ".join(bad) if bad else "all byte-identical")

    check("regression_byte_identical", c_regression_byte_identical)

    # 17. REGRESSION: bool/int/float/None keep their exact types (bool is not int).
    def c_regression_bool_int_distinct():
        back = loads(dumps([True, 1, 1.0, False, 0, None]))
        ok = (
            back[0] is True and isinstance(back[0], bool)
            and back[1] == 1 and isinstance(back[1], int) and back[1] is not True
            and isinstance(back[2], float)
            and back[3] is False and isinstance(back[3], bool)
            and back[4] == 0 and isinstance(back[4], int) and back[4] is not False
            and back[5] is None
        )
        return ok, f"round-tripped -> {back!r} (expected [True,1,1.0,False,0,None] with types)"

    check("regression_bool_int_distinct", c_regression_bool_int_distinct)


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
    "task": "serialhook",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
