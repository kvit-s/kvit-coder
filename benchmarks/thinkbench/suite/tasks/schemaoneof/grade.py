#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `schemaoneof`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `schemaoneof`
package against the BRIEF'S CONTRACT (the `validate(instance, schema)` function:
its existing keywords PLUS the newly-added `oneOf` and `not` combinators), NOT
against any particular internal file layout.

The defining capability under test: the SHIPPED code supports `type`, `enum`,
`required`, `properties` but lacks `oneOf` and `not`. A validator that simply
IGNORES an unknown keyword (as the shipped code does) treats every `oneOf`/`not`
schema as vacuously valid — so the NEW checks below FAIL on the shipped code and
PASS once the combinators are correctly added. The EXISTING-keyword checks pass
on both, guarding against a regression. That split is what makes the task
discriminate the feature-add from doing nothing.

Output: a single JSON scorecard on stdout. Each check runs in isolation, so the
score is continuous (passed / total), never all-or-nothing. FIXED DENOMINATOR:
the full check list is registered up front, so an import failure records every
check as failed and forces score 0.0. Exit code is 0 whenever grading ran to
completion (even score 0.0); the process never raises out.

Tolerance: the brief leaves the error-dict SHAPE up to the implementer, so this
oracle judges errors by PRESENCE only — an empty list is "valid", any non-empty
list is "has errors". It never inspects key names or messages.
"""
import importlib
import json
import os
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# --- fixed denominator: the full roster of checks, declared before any run ----
# (id, human description). Kept in lockstep with the checks registered below so
# that an import failure can still report a result line for every single check.
#
# Two families:
#   NEW (the added capability): oneOf exactly-one semantics + not must-not-match.
#   EXISTING (regression guard): type / required / properties / enum unchanged.
CHECK_SPECS = [
    # --- NEW: oneOf -------------------------------------------------------
    ("oneof_exactly_one_valid", "oneOf with exactly one matching subschema is VALID"),
    ("oneof_zero_matches_error", "oneOf with zero matching subschemas is an ERROR"),
    ("oneof_two_matches_error", "oneOf with two matching subschemas is an ERROR"),
    ("oneof_nested_object", "oneOf discriminates between two object shapes"),
    # --- NEW: not ---------------------------------------------------------
    ("not_match_is_error", "not is an ERROR when the subschema MATCHES the instance"),
    ("not_nomatch_is_valid", "not is VALID when the subschema does NOT match"),
    # --- NEW: composition -------------------------------------------------
    ("combinator_composes", "oneOf/not compose with sibling keywords on one schema"),
    # --- EXISTING (regression): type -------------------------------------
    ("type_ok", "a value of the declared type validates clean"),
    ("type_mismatch", "a value of the wrong type is an error"),
    ("type_number_accepts_int", "type 'number' accepts an integer"),
    # --- EXISTING (regression): required ---------------------------------
    ("required_present_ok", "an object with all required props is valid"),
    ("required_missing_error", "a missing required property is an error"),
    # --- EXISTING (regression): properties -------------------------------
    ("properties_ok", "a property whose value fits its subschema is valid"),
    ("properties_nested_error", "a property whose value violates its subschema errors"),
    # --- EXISTING (regression): enum -------------------------------------
    ("enum_member_ok", "a value in the enum is valid"),
    ("enum_nonmember_error", "a value not in the enum is an error"),
    ("valid_empty_list", "a fully-valid instance yields an EMPTY error list"),
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


def has_errors(result):
    """Tolerant 'did validation report a problem?' — judges by presence only.

    Contract: validate returns a LIST; empty == valid, non-empty == errors. We
    deliberately do NOT inspect the error-dict shape (the brief leaves it open).
    A non-list return is treated as a failure to honor the list contract.
    """
    if not isinstance(result, list):
        raise AssertionError(f"validate did not return a list: {result!r}")
    return len(result) > 0


def is_valid(result):
    return not has_errors(result)


# --- import the produced package (contract: schemaoneof.public, fallback pkg) --
import_ok = True
import_detail = ""
validate = None
try:
    try:
        mod = importlib.import_module("schemaoneof.public")
    except Exception:
        mod = importlib.import_module("schemaoneof")
    validate = getattr(mod, "validate")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # =====================================================================
    # NEW capability: oneOf  (must match EXACTLY ONE subschema)
    # =====================================================================

    # 1. exactly one subschema matches -> VALID.
    def c_oneof_exactly_one_valid():
        schema = {"oneOf": [{"type": "integer"}, {"type": "string"}]}
        r = validate(5, schema)  # 5 is integer only -> exactly one
        return is_valid(r), f"validate(5, oneOf[int,str]) -> {r!r}"

    check("oneof_exactly_one_valid", c_oneof_exactly_one_valid)

    # 2. zero subschemas match -> ERROR. (Shipped code ignores oneOf -> valid.)
    def c_oneof_zero_matches_error():
        schema = {"oneOf": [{"type": "integer"}, {"type": "string"}]}
        r = validate(True, schema)  # bool is neither integer nor string
        return has_errors(r), f"validate(True, oneOf[int,str]) -> {r!r}"

    check("oneof_zero_matches_error", c_oneof_zero_matches_error)

    # 3. two subschemas match -> ERROR. (THE exactly-one distinction.)
    def c_oneof_two_matches_error():
        schema = {"oneOf": [{"type": "integer"}, {"type": "number"}]}
        r = validate(5, schema)  # 5 matches BOTH integer and number
        return has_errors(r), f"validate(5, oneOf[int,number]) -> {r!r}"

    check("oneof_two_matches_error", c_oneof_two_matches_error)

    # 4. oneOf discriminating between two distinct object shapes.
    def c_oneof_nested_object():
        cat = {"type": "object", "required": ["meow"]}
        dog = {"type": "object", "required": ["bark"]}
        schema = {"oneOf": [cat, dog]}
        one = validate({"meow": True}, schema)        # matches cat only -> valid
        both = validate({"meow": 1, "bark": 1}, schema)  # matches both -> error
        neither = validate({"x": 1}, schema)          # matches neither -> error
        ok = is_valid(one) and has_errors(both) and has_errors(neither)
        return ok, f"one={one!r} both={both!r} neither={neither!r}"

    check("oneof_nested_object", c_oneof_nested_object)

    # =====================================================================
    # NEW capability: not  (instance must NOT match the subschema)
    # =====================================================================

    # 5. subschema MATCHES the instance -> ERROR.
    def c_not_match_is_error():
        schema = {"not": {"type": "string"}}
        r = validate("x", schema)  # "x" IS a string -> not violated -> error
        return has_errors(r), f"validate('x', not[string]) -> {r!r}"

    check("not_match_is_error", c_not_match_is_error)

    # 6. subschema does NOT match the instance -> VALID.
    def c_not_nomatch_is_valid():
        schema = {"not": {"type": "string"}}
        r = validate(5, schema)  # 5 is not a string -> ok
        return is_valid(r), f"validate(5, not[string]) -> {r!r}"

    check("not_nomatch_is_valid", c_not_nomatch_is_valid)

    # 7. combinators compose with sibling keywords on the SAME schema.
    def c_combinator_composes():
        # type passes, but oneOf fails (5 matches both int and number) -> error.
        compose_fail = validate(
            5, {"type": "integer", "oneOf": [{"type": "integer"}, {"type": "number"}]}
        )
        # type passes AND not passes (5 is not a string) -> valid.
        compose_ok = validate(5, {"type": "integer", "not": {"type": "string"}})
        ok = has_errors(compose_fail) and is_valid(compose_ok)
        return ok, f"fail={compose_fail!r} ok={compose_ok!r}"

    check("combinator_composes", c_combinator_composes)

    # =====================================================================
    # EXISTING keywords (regression guard): type / required / properties / enum
    # =====================================================================

    def c_type_ok():
        r = validate(5, {"type": "integer"})
        return is_valid(r), f"validate(5, type=integer) -> {r!r}"

    check("type_ok", c_type_ok)

    def c_type_mismatch():
        r = validate("x", {"type": "integer"})
        return has_errors(r), f"validate('x', type=integer) -> {r!r}"

    check("type_mismatch", c_type_mismatch)

    def c_type_number_accepts_int():
        r = validate(7, {"type": "number"})
        return is_valid(r), f"validate(7, type=number) -> {r!r}"

    check("type_number_accepts_int", c_type_number_accepts_int)

    def c_required_present_ok():
        r = validate({"a": 1, "b": 2}, {"type": "object", "required": ["a", "b"]})
        return is_valid(r), f"validate(ab, required=[a,b]) -> {r!r}"

    check("required_present_ok", c_required_present_ok)

    def c_required_missing_error():
        r = validate({"a": 1}, {"type": "object", "required": ["a", "b"]})
        return has_errors(r), f"validate(a, required=[a,b]) -> {r!r}"

    check("required_missing_error", c_required_missing_error)

    def c_properties_ok():
        schema = {"type": "object", "properties": {"age": {"type": "integer"}}}
        r = validate({"age": 30}, schema)
        return is_valid(r), f"validate(age=30, props age:int) -> {r!r}"

    check("properties_ok", c_properties_ok)

    def c_properties_nested_error():
        schema = {"type": "object", "properties": {"age": {"type": "integer"}}}
        r = validate({"age": "old"}, schema)  # age must be integer
        return has_errors(r), f"validate(age='old', props age:int) -> {r!r}"

    check("properties_nested_error", c_properties_nested_error)

    def c_enum_member_ok():
        r = validate("green", {"enum": ["red", "green", "blue"]})
        return is_valid(r), f"validate('green', enum) -> {r!r}"

    check("enum_member_ok", c_enum_member_ok)

    def c_enum_nonmember_error():
        r = validate("purple", {"enum": ["red", "green", "blue"]})
        return has_errors(r), f"validate('purple', enum) -> {r!r}"

    check("enum_nonmember_error", c_enum_nonmember_error)

    # A larger fully-valid instance must yield an EMPTY list (guards "always errors").
    def c_valid_empty_list():
        schema = {
            "type": "object",
            "required": ["name", "role"],
            "properties": {
                "name": {"type": "string"},
                "role": {"enum": ["admin", "user"]},
            },
        }
        r = validate({"name": "ada", "role": "admin"}, schema)
        return (isinstance(r, list) and len(r) == 0), f"valid instance -> {r!r}"

    check("valid_empty_list", c_valid_empty_list)


# --- assemble the scorecard with a FIXED denominator -------------------------
checks_out = []
for cid in CHECK_IDS:
    r = results.get(cid)
    if r is None:
        # Not run (e.g. import failed): record as a failed check, keep denominator.
        r = {"passed": False, "detail": "not run (import failed)" if not import_ok else "not run"}
    checks_out.append({"id": cid, "desc": DESC[cid], "passed": r["passed"], "detail": r["detail"]})

passed = sum(1 for c in checks_out if c["passed"])
total = len(checks_out)  # always len(CHECK_SPECS): fixed denominator
card = {
    "task": "schemaoneof",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
