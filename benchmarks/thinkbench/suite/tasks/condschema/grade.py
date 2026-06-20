#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `condschema`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `condschema`
package against the BRIEF'S CONTRACT (nested object/list schemas with dotted
error paths, plus conditional `requiredIf` requirements, plus the unchanged flat
validation), NOT against any particular internal file layout.

The defining behaviors under test are the ones a SUPERFICIAL implementation gets
wrong:

  * a NESTED error must carry a DOTTED path (``address.zip``, ``items.2.sku``),
    not a bare field name and not a crash — a flat-only validator never recurses;
  * a `requiredIf` field is required ONLY when the sibling EXACTLY equals the
    trigger value — absent sibling, different value, or a bool/number mix-up all
    leave it optional (a naive "requiredIf == required" or truthy-compare fails);
  * a required-but-absent field yields ONE "required" error and NO "type" error;
  * a field present with the wrong type is reported AND not recursed into;
  * ALL errors are reported, in a STABLE pre-order (schema order, then ascending
    list index) — first-only or order-shuffled implementations fail.

The shipped flat attempt validates top-level type/required only: it passes the
flat regression checks but fails every nesting and conditional check — that is
what makes the task discriminate (a naive land well under 1.0, a careful nested
+ conditional implementation lands at 1.0).

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
    # --- NEW: nested objects ---------------------------------------------
    ("nested_object_dotted_path", "a missing field in a nested object reports a DOTTED path"),
    ("nested_object_valid", "a satisfied nested object yields no errors"),
    ("nested_object_type_no_recurse", "a non-dict where an object is expected: one type error, no recursion"),
    ("nested_object_inner_type", "a wrong-typed value inside a nested object reports at the dotted path"),
    ("deep_nesting_path", "two levels of object nesting build the full dotted path"),
    # --- NEW: lists of items ---------------------------------------------
    ("list_item_index_path", "a bad element reports field.<index>.<inner> as its path"),
    ("list_all_valid", "a list whose every element fits yields no errors"),
    ("list_type_no_recurse", "a non-list where a list is expected: one type error, no recursion"),
    ("list_scalar_items", "an items spec of a scalar type checks each element by index"),
    ("list_order_by_index", "multiple bad elements are reported in ascending index order"),
    # --- NEW: conditional requirements -----------------------------------
    ("requiredif_trigger_present", "requiredIf fires when the sibling equals the trigger value"),
    ("requiredif_other_value", "requiredIf does NOT fire when the sibling has a different value"),
    ("requiredif_sibling_absent", "requiredIf does NOT fire when the sibling is absent"),
    ("requiredif_present_ok", "a satisfied requiredIf with the field present is valid"),
    ("requiredif_bool_not_number", "requiredIf does not conflate True with 1 (exact match)"),
    ("requiredif_present_is_type_checked", "a present conditional field is still type-checked"),
    ("required_absent_no_type_error", "an absent required field gives ONE required error, no type error"),
    # --- composition / ordering ------------------------------------------
    ("all_errors_reported", "every violation is reported, not just the first"),
    ("stable_pre_order", "errors come in schema order then ascending list index"),
    # --- EXISTING (regression): flat validation --------------------------
    ("regression_flat_valid", "a valid flat instance yields an empty list"),
    ("regression_flat_required_missing", "a missing flat required field errors at its bare name"),
    ("regression_flat_type_mismatch", "a wrong-typed flat field errors with code 'type'"),
    ("regression_number_accepts_int", "type 'number' accepts an integer; 'integer' rejects a float"),
    ("regression_bool_not_integer", "a bool does not satisfy type 'integer'"),
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


# --- helpers over the returned error list (shape: dicts with path + code) -----
def _norm(errors):
    """Normalize the returned error list to (path, code) tuples in order.

    Tolerates either dict errors ({"path","code",...}) or, defensively, errors
    exposing .path/.code attributes. Anything unparseable becomes (repr, "").
    """
    out = []
    for e in errors:
        if isinstance(e, dict):
            out.append((e.get("path"), e.get("code")))
        else:
            out.append((getattr(e, "path", repr(e)), getattr(e, "code", "")))
    return out


def _paths(errors):
    return [p for (p, _c) in _norm(errors)]


def _has(errors, path, code):
    return (path, code) in _norm(errors)


# --- import the produced package (contract: condschema.public, fallback) ------
import_ok = True
import_detail = ""
validate = None
try:
    try:
        mod = importlib.import_module("condschema.public")
    except Exception:
        mod = importlib.import_module("condschema")
    validate = getattr(mod, "validate")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # === NESTED OBJECTS ===================================================

    # 1. a missing field inside a nested object reports a DOTTED path.
    def c_nested_object_dotted_path():
        schema = {"address": {"type": "object", "fields": {
            "zip": {"type": "string", "required": True}}}}
        errs = validate({"address": {}}, schema)
        n = _norm(errs)
        ok = n == [("address.zip", "required")]
        return ok, f"errors -> {n!r} (expected [('address.zip','required')])"

    check("nested_object_dotted_path", c_nested_object_dotted_path)

    # 2. a satisfied nested object yields no errors.
    def c_nested_object_valid():
        schema = {"address": {"type": "object", "fields": {
            "zip": {"type": "string", "required": True}}}}
        errs = validate({"address": {"zip": "94110"}}, schema)
        return _norm(errs) == [], f"errors -> {_norm(errs)!r} (expected [])"

    check("nested_object_valid", c_nested_object_valid)

    # 3. a non-dict where an object is expected: ONE type error, no recursion.
    def c_nested_object_type_no_recurse():
        schema = {"address": {"type": "object", "fields": {
            "zip": {"type": "string", "required": True}}}}
        errs = validate({"address": "nope"}, schema)
        n = _norm(errs)
        ok = n == [("address", "type")]  # NOT also address.zip/required
        return ok, f"errors -> {n!r} (expected exactly [('address','type')])"

    check("nested_object_type_no_recurse", c_nested_object_type_no_recurse)

    # 4. a wrong-typed value inside a nested object reports at the dotted path.
    def c_nested_object_inner_type():
        schema = {"address": {"type": "object", "fields": {
            "zip": {"type": "string"}}}}
        errs = validate({"address": {"zip": 94110}}, schema)
        n = _norm(errs)
        ok = n == [("address.zip", "type")]
        return ok, f"errors -> {n!r} (expected [('address.zip','type')])"

    check("nested_object_inner_type", c_nested_object_inner_type)

    # 5. two levels of object nesting build the full dotted path.
    def c_deep_nesting_path():
        schema = {"a": {"type": "object", "fields": {
            "b": {"type": "object", "fields": {
                "c": {"type": "string", "required": True}}}}}}
        errs = validate({"a": {"b": {}}}, schema)
        n = _norm(errs)
        ok = n == [("a.b.c", "required")]
        return ok, f"errors -> {n!r} (expected [('a.b.c','required')])"

    check("deep_nesting_path", c_deep_nesting_path)

    # === LISTS OF ITEMS ===================================================

    # 6. a bad element reports field.<index>.<inner> as its path.
    def c_list_item_index_path():
        schema = {"items": {"type": "list", "items": {
            "type": "object", "fields": {
                "sku": {"type": "string", "required": True}}}}}
        errs = validate({"items": [{"sku": "A1"}, {}, {"sku": 5}]}, schema)
        n = _norm(errs)
        ok = n == [("items.1.sku", "required"), ("items.2.sku", "type")]
        return ok, f"errors -> {n!r} (expected items.1.sku/required, items.2.sku/type)"

    check("list_item_index_path", c_list_item_index_path)

    # 7. a list whose every element fits yields no errors.
    def c_list_all_valid():
        schema = {"items": {"type": "list", "items": {
            "type": "object", "fields": {
                "sku": {"type": "string", "required": True}}}}}
        errs = validate({"items": [{"sku": "A1"}, {"sku": "B2"}]}, schema)
        return _norm(errs) == [], f"errors -> {_norm(errs)!r} (expected [])"

    check("list_all_valid", c_list_all_valid)

    # 8. a non-list where a list is expected: ONE type error, no recursion.
    def c_list_type_no_recurse():
        schema = {"items": {"type": "list", "items": {
            "type": "object", "fields": {
                "sku": {"type": "string", "required": True}}}}}
        errs = validate({"items": {"sku": "A1"}}, schema)
        n = _norm(errs)
        ok = n == [("items", "type")]
        return ok, f"errors -> {n!r} (expected exactly [('items','type')])"

    check("list_type_no_recurse", c_list_type_no_recurse)

    # 9. an items spec of a SCALAR type checks each element by index.
    def c_list_scalar_items():
        schema = {"tags": {"type": "list", "items": {"type": "string"}}}
        errs = validate({"tags": ["a", 2, "c", 4]}, schema)
        n = _norm(errs)
        ok = n == [("tags.1", "type"), ("tags.3", "type")]
        return ok, f"errors -> {n!r} (expected tags.1/type, tags.3/type)"

    check("list_scalar_items", c_list_scalar_items)

    # 10. multiple bad elements are reported in ascending index order.
    def c_list_order_by_index():
        schema = {"xs": {"type": "list", "items": {"type": "integer"}}}
        errs = validate({"xs": ["z", "y", 3, "w"]}, schema)
        paths = _paths(errs)
        ok = paths == ["xs.0", "xs.1", "xs.3"]
        return ok, f"paths -> {paths!r} (expected ['xs.0','xs.1','xs.3'])"

    check("list_order_by_index", c_list_order_by_index)

    # === CONDITIONAL REQUIREMENTS ========================================

    COND = {
        "country": {"type": "string"},
        "state": {"type": "string", "requiredIf": ["country", "US"]},
    }

    # 11. requiredIf fires when the sibling equals the trigger value.
    def c_requiredif_trigger_present():
        errs = validate({"country": "US"}, COND)
        ok = _has(errs, "state", "required") and _norm(errs) == [("state", "required")]
        return ok, f"errors -> {_norm(errs)!r} (expected [('state','required')])"

    check("requiredif_trigger_present", c_requiredif_trigger_present)

    # 12. requiredIf does NOT fire when the sibling has a different value.
    def c_requiredif_other_value():
        errs = validate({"country": "CA"}, COND)
        return _norm(errs) == [], f"errors -> {_norm(errs)!r} (expected [] for country=CA)"

    check("requiredif_other_value", c_requiredif_other_value)

    # 13. requiredIf does NOT fire when the sibling is absent.
    def c_requiredif_sibling_absent():
        errs = validate({}, COND)
        return _norm(errs) == [], f"errors -> {_norm(errs)!r} (expected [] when sibling absent)"

    check("requiredif_sibling_absent", c_requiredif_sibling_absent)

    # 14. a satisfied requiredIf with the field present is valid.
    def c_requiredif_present_ok():
        errs = validate({"country": "US", "state": "CA"}, COND)
        return _norm(errs) == [], f"errors -> {_norm(errs)!r} (expected [] when state supplied)"

    check("requiredif_present_ok", c_requiredif_present_ok)

    # 15. requiredIf must not conflate True with 1 (exact match, no bool/number mix).
    def c_requiredif_bool_not_number():
        schema = {
            "flag": {"type": "bool"},
            "reason": {"type": "string", "requiredIf": ["flag", 1]},
        }
        # flag is True, trigger value is 1: True != 1, so reason stays optional.
        errs = validate({"flag": True}, schema)
        return _norm(errs) == [], f"errors -> {_norm(errs)!r} (True must NOT equal trigger 1)"

    check("requiredif_bool_not_number", c_requiredif_bool_not_number)

    # 16. a PRESENT conditional field is still type-checked.
    def c_requiredif_present_is_type_checked():
        errs = validate({"country": "US", "state": 5}, COND)
        n = _norm(errs)
        ok = n == [("state", "type")]  # present, satisfies requirement, but wrong type
        return ok, f"errors -> {n!r} (expected [('state','type')])"

    check("requiredif_present_is_type_checked", c_requiredif_present_is_type_checked)

    # 17. an absent required field gives ONE required error and NO type error.
    def c_required_absent_no_type_error():
        schema = {"name": {"type": "string", "required": True}}
        errs = validate({}, schema)
        n = _norm(errs)
        ok = n == [("name", "required")]  # never also ("name","type")
        return ok, f"errors -> {n!r} (expected exactly [('name','required')])"

    check("required_absent_no_type_error", c_required_absent_no_type_error)

    # === COMPOSITION / ORDERING ==========================================

    # 18. every violation is reported, not just the first.
    def c_all_errors_reported():
        schema = {
            "a": {"type": "string", "required": True},
            "b": {"type": "integer", "required": True},
            "c": {"type": "string"},
        }
        errs = validate({"c": 7}, schema)  # a missing, b missing, c wrong type
        codes = sorted(_norm(errs))
        want = sorted([("a", "required"), ("b", "required"), ("c", "type")])
        return codes == want, f"errors -> {sorted(_norm(errs))!r} (expected 3: a/req,b/req,c/type)"

    check("all_errors_reported", c_all_errors_reported)

    # 19. errors come in schema order then ascending list index (stable pre-order).
    def c_stable_pre_order():
        schema = {
            "first": {"type": "string", "required": True},
            "items": {"type": "list", "items": {
                "type": "object", "fields": {
                    "id": {"type": "integer", "required": True}}}},
            "last": {"type": "string", "required": True},
        }
        data = {"items": [{"id": 1}, {}, {"id": "x"}]}  # first & last missing
        paths = _paths(validate(data, schema))
        want = ["first", "items.1.id", "items.2.id", "last"]
        return paths == want, f"paths -> {paths!r} (expected {want!r})"

    check("stable_pre_order", c_stable_pre_order)

    # === REGRESSION: flat validation still works =========================

    # 20. a valid flat instance yields an empty list.
    def c_regression_flat_valid():
        schema = {"name": {"type": "string", "required": True}, "age": {"type": "integer"}}
        errs = validate({"name": "Ada", "age": 36}, schema)
        return _norm(errs) == [], f"errors -> {_norm(errs)!r} (expected [])"

    check("regression_flat_valid", c_regression_flat_valid)

    # 21. a missing flat required field errors at its bare name.
    def c_regression_flat_required_missing():
        schema = {"name": {"type": "string", "required": True}}
        errs = validate({}, schema)
        return _norm(errs) == [("name", "required")], \
            f"errors -> {_norm(errs)!r} (expected [('name','required')])"

    check("regression_flat_required_missing", c_regression_flat_required_missing)

    # 22. a wrong-typed flat field errors with code 'type'.
    def c_regression_flat_type_mismatch():
        schema = {"age": {"type": "integer"}}
        errs = validate({"age": "old"}, schema)
        return _norm(errs) == [("age", "type")], \
            f"errors -> {_norm(errs)!r} (expected [('age','type')])"

    check("regression_flat_type_mismatch", c_regression_flat_type_mismatch)

    # 23. type 'number' accepts an integer; 'integer' rejects a float.
    def c_regression_number_accepts_int():
        s_num = {"x": {"type": "number"}}
        s_int = {"x": {"type": "integer"}}
        num_ok = _norm(validate({"x": 5}, s_num)) == []        # int is a number
        int_bad = _norm(validate({"x": 5.0}, s_int)) == [("x", "type")]  # float is not an integer
        return (num_ok and int_bad), f"number<-int empty={num_ok}, integer<-float errors={int_bad}"

    check("regression_number_accepts_int", c_regression_number_accepts_int)

    # 24. a bool does not satisfy type 'integer'.
    def c_regression_bool_not_integer():
        schema = {"flag": {"type": "integer"}}
        errs = validate({"flag": True}, schema)
        return _norm(errs) == [("flag", "type")], \
            f"errors -> {_norm(errs)!r} (a bool must NOT pass type integer)"

    check("regression_bool_not_integer", c_regression_bool_not_integer)


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
    "task": "condschema",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
