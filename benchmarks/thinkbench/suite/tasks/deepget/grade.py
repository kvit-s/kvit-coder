#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_deepget`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced `deepget`
package against the BRIEF'S CONTRACT (dotted-path `get`/`set_` over nested dicts
and lists), NOT against any particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- `get` treats a PRESENT but ``None`` value as a miss and returns
    ``default`` instead of the real stored ``None``.
  * BUG 2 -- a numeric-looking segment is ALWAYS used as a list index, so a dict
    keyed by digit-strings (e.g. ``{"2024": ...}``) is unreachable and the
    lookup raises / returns default.
  * BUG 3 -- `set_` only ever creates dicts for missing intermediates, so a path
    that needs a LIST in the middle builds the wrong shape (a dict keyed by the
    string index instead of a one-element list); it also can't pad a list.
  * BUG 4 -- list indexing on `get` uses raw Python semantics: an out-of-range
    index raises (rather than falling back to default) and a NEGATIVE index
    quietly wraps to grab from the end instead of failing.

Plain nested-dict reads/writes and the common in-range list index still work, so
a superficial fix can pass the easy checks while still failing the edge cases.

Output: a single JSON scorecard on stdout. Each check runs in isolation, so the
score is continuous (passed / total), never all-or-nothing. FIXED DENOMINATOR:
the full check list is registered up front, so an import failure records every
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

# A unique sentinel so "returned default" is distinguishable from "returned a
# real value that happens to equal the default". Each get-check passes this in
# as the default and asserts on identity.
SENT = object()

# --- fixed denominator: the full roster of checks, declared before any run ----
# (id, human description). Kept in lockstep with the checks registered below so
# that an import failure can still report a result line for every single check.
CHECK_SPECS = [
    ("get_nested_dict", "get walks plain nested dict keys"),
    ("get_list_index", "get indexes a list with a numeric segment"),
    ("get_mixed", "get walks a mixed dict/list path to a leaf"),
    ("get_missing_key", "a missing dict key returns default"),
    ("get_missing_returns_exact_default", "the exact default object is returned on a miss"),
    ("get_empty_path", "an empty path resolves to obj itself"),
    ("get_present_none", "a present None value is returned, not the default"),
    ("get_none_deep", "a present None deep in the tree is returned, not default"),
    ("get_none_vs_missing", "present-None and missing-key are distinguished"),
    ("get_digit_string_dict_key", "a digit-string dict key is reachable (not forced to a list index)"),
    ("get_int_dict_key", "an int dict key is reachable via the digit segment"),
    ("get_string_key_wins_over_int", "the string key is tried before the int key"),
    ("get_index_out_of_range", "an out-of-range list index returns default (no raise)"),
    ("get_negative_index", "a negative list index returns default (no end-wrap)"),
    ("get_descend_non_container", "descending into a non-container returns default"),
    ("get_no_raise_on_bad_path", "an unresolvable path never raises"),
    ("set_nested_dict", "set_ creates nested dicts and stores the value"),
    ("set_into_existing", "set_ mutates an existing nested dict in place"),
    ("set_creates_list_intermediate", "a numeric next segment makes set_ create a list"),
    ("set_pads_list", "set_ pads a list with None past its end"),
    ("set_preserves_sibling", "set_ does not clobber a sibling container it descends through"),
    ("set_roundtrip", "a value written by set_ is readable by get along the same path"),
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


# --- import the produced package (contract: deepget.public, fallback pkg) ------
import_ok = True
import_detail = ""
get = None
set_ = None
try:
    try:
        mod = importlib.import_module("deepget.public")
    except Exception:
        mod = importlib.import_module("deepget")
    get = getattr(mod, "get")
    set_ = getattr(mod, "set_")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # ---- baseline get: the happy paths a naive fix should keep working --------
    def c_get_nested_dict():
        obj = {"a": {"b": {"c": 7}}}
        v = get(obj, "a.b.c", SENT)
        return v == 7, f"a.b.c -> {v!r} (expected 7)"

    check("get_nested_dict", c_get_nested_dict)

    def c_get_list_index():
        obj = {"xs": [10, 20, 30]}
        v = get(obj, "xs.1", SENT)
        return v == 20, f"xs.1 -> {v!r} (expected 20)"

    check("get_list_index", c_get_list_index)

    def c_get_mixed():
        obj = {"a": {"b": [{"c": 1}, {"c": 2}]}}
        v = get(obj, "a.b.1.c", SENT)
        return v == 2, f"a.b.1.c -> {v!r} (expected 2)"

    check("get_mixed", c_get_mixed)

    def c_get_missing_key():
        obj = {"a": {"b": 1}}
        v = get(obj, "a.z", SENT)
        return v is SENT, f"a.z -> {v!r} (expected the default)"

    check("get_missing_key", c_get_missing_key)

    def c_get_missing_returns_exact_default():
        # The default must be returned by identity, not some coerced/None value.
        obj = {"a": 1}
        d = ["my-default"]
        v = get(obj, "nope", d)
        return v is d, f"miss returned {v!r} (expected the exact default object)"

    check("get_missing_returns_exact_default", c_get_missing_returns_exact_default)

    def c_get_empty_path():
        obj = {"a": 1}
        v = get(obj, "", SENT)
        return v is obj, f"'' -> {v!r} (expected obj itself)"

    check("get_empty_path", c_get_empty_path)

    # ---- BUG 1: a present None must come back as None, never as default -------
    def c_get_present_none():
        obj = {"x": None}
        v = get(obj, "x", SENT)
        return v is None, f"x -> {v!r} (expected None, the present value)"

    check("get_present_none", c_get_present_none)

    def c_get_none_deep():
        obj = {"a": {"b": {"c": None}}}
        v = get(obj, "a.b.c", SENT)
        return v is None, f"a.b.c -> {v!r} (expected None, the present value)"

    check("get_none_deep", c_get_none_deep)

    def c_get_none_vs_missing():
        # Present-None must be the stored None; a truly missing key must be the
        # default. The two must NOT be conflated.
        present = get({"k": None}, "k", SENT)
        missing = get({"k": None}, "absent", SENT)
        ok = (present is None) and (missing is SENT)
        return ok, f"present={present!r} missing={missing!r} (expected None / default)"

    check("get_none_vs_missing", c_get_none_vs_missing)

    # ---- BUG 2: a digit segment is a dict KEY when the container is a dict -----
    def c_get_digit_string_dict_key():
        obj = {"2024": {"q": 4}}
        v = get(obj, "2024.q", SENT)
        return v == 4, f"2024.q -> {v!r} (expected 4 via string dict key)"

    check("get_digit_string_dict_key", c_get_digit_string_dict_key)

    def c_get_int_dict_key():
        # A dict keyed by an int is reachable via the digit segment (int fallback).
        obj = {0: "zero", 1: "one"}
        v = get(obj, "1", SENT)
        return v == "one", f"'1' on int-keyed dict -> {v!r} (expected 'one')"

    check("get_int_dict_key", c_get_int_dict_key)

    def c_get_string_key_wins_over_int():
        # Both "0" (str) and 0 (int) present: the string key is tried first.
        obj = {"0": "str", 0: "int"}
        v = get(obj, "0", SENT)
        return v == "str", f"'0' -> {v!r} (expected 'str', string key first)"

    check("get_string_key_wins_over_int", c_get_string_key_wins_over_int)

    # ---- BUG 4: list index bounds + no negative wrap --------------------------
    def c_get_index_out_of_range():
        obj = {"xs": [1, 2, 3]}
        v = get(obj, "xs.9", SENT)
        return v is SENT, f"xs.9 -> {v!r} (expected default, in range only)"

    check("get_index_out_of_range", c_get_index_out_of_range)

    def c_get_negative_index():
        # A negative index must NOT wrap to the end; it simply fails to resolve.
        obj = {"xs": [10, 20, 30]}
        v = get(obj, "xs.-1", SENT)
        return v is SENT, f"xs.-1 -> {v!r} (expected default, not 30)"

    check("get_negative_index", c_get_negative_index)

    def c_get_descend_non_container():
        # Path tries to descend into an int -> unresolvable -> default.
        obj = {"a": 5}
        v = get(obj, "a.b", SENT)
        return v is SENT, f"a.b (a is int) -> {v!r} (expected default)"

    check("get_descend_non_container", c_get_descend_non_container)

    def c_get_no_raise_on_bad_path():
        # A grab-bag of unresolvable paths: none may raise, all -> default.
        cases = [
            ({"xs": [1]}, "xs.5"),       # off the end
            ({"xs": [1]}, "xs.foo"),     # non-numeric into a list
            ({"a": 1}, "a.b.c.d"),       # descend into an int
            ({}, "anything.here"),       # empty root
            ([1, 2], "x"),               # non-numeric into a list root
        ]
        bad = []
        for obj, path in cases:
            try:
                v = get(obj, path, SENT)
            except Exception as e:  # noqa: BLE001
                bad.append(f"{path!r} raised {type(e).__name__}")
                continue
            if v is not SENT:
                bad.append(f"{path!r} -> {v!r} (expected default)")
        return not bad, ("ok" if not bad else "; ".join(bad))

    check("get_no_raise_on_bad_path", c_get_no_raise_on_bad_path)

    # ---- set_: baseline + the list-intermediate / padding subtleties ----------
    def c_set_nested_dict():
        root = set_({}, "a.b.c", 9)
        ok = isinstance(root, dict) and root == {"a": {"b": {"c": 9}}}
        return ok, f"set_ {{}} a.b.c=9 -> {root!r}"

    check("set_nested_dict", c_set_nested_dict)

    def c_set_into_existing():
        obj = {"a": {"keep": 1}}
        root = set_(obj, "a.b", 2)
        ok = root is obj and obj == {"a": {"keep": 1, "b": 2}}
        return ok, f"set_ existing a.b=2 -> {obj!r} (sibling 'keep' must survive)"

    check("set_into_existing", c_set_into_existing)

    def c_set_creates_list_intermediate():
        # A numeric NEXT segment must create a LIST, not a dict keyed by "0".
        root = set_({}, "items.0.name", "hi")
        ok = root == {"items": [{"name": "hi"}]}
        return ok, f"set_ {{}} items.0.name -> {root!r} (expected list intermediate)"

    check("set_creates_list_intermediate", c_set_creates_list_intermediate)

    def c_set_pads_list():
        # Assigning past the end pads with None.
        root = set_([], "2", "z")
        ok = root == [None, None, "z"]
        return ok, f"set_ [] '2'=z -> {root!r} (expected [None, None, 'z'])"

    check("set_pads_list", c_set_pads_list)

    def c_set_preserves_sibling():
        # Descending through an existing list must not replace it; the existing
        # element and its fields must survive.
        obj = {"servers": [{"host": "a"}, {"host": "b"}]}
        root = set_(obj, "servers.1.port", 8080)
        ok = (root is obj
              and obj == {"servers": [{"host": "a"}, {"host": "b", "port": 8080}]})
        return ok, f"set_ servers.1.port -> {obj!r}"

    check("set_preserves_sibling", c_set_preserves_sibling)

    def c_set_roundtrip():
        # What set_ writes, get must read back along the SAME path -- the shapes
        # have to agree (this catches a set_ that builds dicts where get expects
        # lists).
        root = set_({}, "a.0.b.1", "deep")
        v = get(root, "a.0.b.1", SENT)
        ok = v == "deep"
        return ok, f"roundtrip a.0.b.1 -> built {root!r}, get -> {v!r}"

    check("set_roundtrip", c_set_roundtrip)


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
    "task": "fix_deepget",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
