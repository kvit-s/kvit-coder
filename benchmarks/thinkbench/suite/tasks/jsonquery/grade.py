#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `jsonquery`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``jsonquery`` package against the BRIEF'S CONTRACT (the ``jsonquery.public``
``select`` API), NOT against any particular internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE wildcard / recursive-descent / missing-step / ordering edge cases, all with
expected values computed HERE (never read from the agent's tests). The FIXED
reference passes every check; the planted-bug starter fails a discriminating
subset, so a partial fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``jsonquery.public``:
  1. ``[*]`` does not fan out — it appends the list itself as a single nested
     value instead of extending the frontier with each element in order, so a
     terminal ``[*]`` returns ``[[...]]`` and ``.users[*].name`` never reaches
     the individual elements;
  2. ``..key`` recursive descent is wrong twice over — it records a node's own
     match AFTER recursing (post-order, wrong order) and it never traverses LIST
     elements, so every match nested inside a list is silently missed;
  3. a missing key / out-of-range index is silently skipped instead of raising
     ``SelectError`` (only the ``..key`` scan is allowed to find nothing).

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs).
"""
import importlib
import json
import sys

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --- import the produced package ---------------------------------------------
# Contract path is ``jsonquery.public``; fall back to the package root
# ``jsonquery`` so a submission that re-exports ``select`` from ``__init__`` (but
# moved it off ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
select = None
SelectError = None
try:
    try:
        mod = importlib.import_module("jsonquery.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("jsonquery")
    select = getattr(mod, "select")
    SelectError = getattr(mod, "SelectError", None)
    if not (isinstance(SelectError, type) and issubclass(SelectError, BaseException)):
        SelectError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
class _OracleError(Exception):
    """The oracle's own "structural mismatch" signal (distinct from the
    submission's ``SelectError`` so the two can never be confused)."""


def _otokenize(path):
    """Tokenize a path into steps, mirroring the contract grammar."""
    steps = []
    i, n = 0, len(path)
    while i < n:
        c = path[i]
        if c == ".":
            if i + 1 < n and path[i + 1] == ".":  # ..key
                i += 2
                j = i
                while j < n and (path[j].isalnum() or path[j] == "_"):
                    j += 1
                if j == i:
                    raise _OracleError("bad ..key")
                steps.append(("descend", path[i:j]))
                i = j
            else:  # .key
                i += 1
                j = i
                while j < n and (path[j].isalnum() or path[j] == "_"):
                    j += 1
                if j == i:
                    raise _OracleError("bad .key")
                steps.append(("key", path[i:j]))
                i = j
        elif c == "[":
            close = path.find("]", i)
            if close < 0:
                raise _OracleError("unterminated [")
            inner = path[i + 1:close]
            if inner == "*":
                steps.append(("wild", None))
            elif inner.isdigit():
                steps.append(("index", int(inner)))
            else:
                raise _OracleError(f"bad index {inner!r}")
            i = close + 1
        else:  # leading bare key
            j = i
            while j < n and (path[j].isalnum() or path[j] == "_"):
                j += 1
            if j == i:
                raise _OracleError(f"bad token at {path[i:]!r}")
            steps.append(("key", path[i:j]))
            i = j
    return steps


def _odescend(node, name, out):
    """Pre-order recursive descent through dicts AND lists; never raises."""
    if isinstance(node, dict):
        if name in node:
            out.append(node[name])
        for v in node.values():
            _odescend(v, name, out)
    elif isinstance(node, list):
        for item in node:
            _odescend(item, name, out)


def oracle(obj, path):
    """Reference evaluation, independent of the submission under test. Raises
    ``_OracleError`` exactly where the contract says ``select`` must raise."""
    values = [obj]
    for kind, arg in _otokenize(path):
        nxt = []
        if kind == "descend":
            for v in values:
                _odescend(v, arg, nxt)
        else:
            for v in values:
                if kind == "key":
                    if not isinstance(v, dict):
                        raise _OracleError("key of non-mapping")
                    if arg not in v:
                        raise _OracleError(f"missing key {arg!r}")
                    nxt.append(v[arg])
                elif kind == "index":
                    if not isinstance(v, list) or isinstance(v, bool):
                        raise _OracleError("index of non-list")
                    if arg >= len(v):
                        raise _OracleError("index out of range")
                    nxt.append(v[arg])
                else:  # wild
                    if not isinstance(v, list):
                        raise _OracleError("[*] on non-list")
                    nxt.extend(v)
        values = nxt
    return values


# Documents used by the checks (built HERE; expected values computed HERE).
DOC = {
    "users": [
        {"name": "ada", "id": 1, "roles": [{"id": 10}, {"id": 11}]},
        {"name": "linus", "id": 2, "roles": [{"id": 20}]},
    ],
    "owner": {"name": "grace", "id": 3},
}
NESTED = {
    "a": {"id": 1, "b": {"id": 2, "c": {"id": 3}}},
    "list": [{"id": 4}, {"id": 5, "kids": [{"id": 6}]}],
}
SCALARS = {"xs": [10, 20, 30], "ys": [[1, 2], [3]], "n": 7}


def expect_eq(label, obj, path):
    """``select(obj, path)`` must equal the oracle's value (and be a list)."""
    def _fn():
        want = oracle(obj, path)
        got = select(obj, path)
        if not isinstance(got, list):
            return False, f"{label}: select returned {type(got).__name__}, expected list"
        return (list(got) == want), f"{label}: got {got!r}, expected {want!r}"

    return _fn


def expect_raises(label, obj, path):
    """``select(obj, path)`` must raise ``SelectError`` (the oracle agrees)."""
    def _fn():
        # sanity: the oracle itself treats this path as a structural mismatch
        try:
            oracle(obj, path)
            oracle_raised = False
        except _OracleError:
            oracle_raised = True
        if not oracle_raised:
            return False, f"{label}: GRADER BUG — oracle did not consider this a mismatch"
        try:
            got = select(obj, path)
        except SelectError:
            return True, f"{label}: raised SelectError"
        except Exception as e:  # noqa: BLE001
            return False, f"{label}: raised {type(e).__name__}, expected SelectError"
        return False, f"{label}: returned {got!r}, expected SelectError"

    return _fn


if import_ok:
    # --- baseline: simple .a.b chains (pass even buggy; guards regressions) ---
    check("simple_key_chain", "single match for a plain key chain",
          expect_eq("owner.name", DOC, ".owner.name"))
    check("simple_returns_list", "a single match is still wrapped in a list",
          expect_eq("owner.id", DOC, ".owner.id"))
    check("index_then_key", "index a list then descend a key",
          expect_eq("users[0].name", DOC, ".users[0].name"))
    check("leading_bare_key", "a leading bare key is sugar for .key",
          expect_eq("owner.id bare", DOC, "owner.id"))
    check("deep_key_chain", "a deep plain key chain resolves",
          expect_eq("a.b.c.id", NESTED, ".a.b.c.id"))

    # --- [*] fan-out / flatten / order (BUG 1) -------------------------------
    check("wildcard_terminal", "terminal [*] returns the elements flat, in order",
          expect_eq("xs[*]", SCALARS, ".xs[*]"))
    check("wildcard_then_key", "[*] fans out, then .key maps over each element",
          expect_eq("users[*].name", DOC, ".users[*].name"))
    check("wildcard_then_key_ids", "[*] then .id over each element",
          expect_eq("users[*].id", DOC, ".users[*].id"))
    check("wildcard_then_index", "[*] fans out, then [index] into each sub-list",
          expect_eq("ys[*][0]", SCALARS, ".ys[*][0]"))
    check("wildcard_double", "[*] after [*] fans out two levels, flat",
          expect_eq("ys[*][*]", SCALARS, ".ys[*][*]"))

    # --- ..key recursive descent: order + lists (BUG 2) ----------------------
    check("descend_ids_preorder", "..id is pre-order over dicts AND lists",
          expect_eq("..id DOC", DOC, "..id"))
    check("descend_names", "..name collects names top-down",
          expect_eq("..name DOC", DOC, "..name"))
    check("descend_nested_preorder", "..id pre-order through nested dicts + lists",
          expect_eq("..id NESTED", NESTED, "..id"))
    check("descend_from_subtree", ".key then ..id scans only that subtree",
          expect_eq("users..id", DOC, ".users..id"))
    check("descend_no_match_empty", "..key with no match returns [] (never raises)",
          expect_eq("..nope", DOC, "..nope"))

    # --- missing key / index raises (BUG 3) ----------------------------------
    check("missing_key_raises", "a missing mapping key raises SelectError",
          expect_raises("owner.missing", DOC, ".owner.missing"))
    check("index_out_of_range_raises", "an out-of-range index raises SelectError",
          expect_raises("users[5]", DOC, ".users[5]"))
    check("key_of_non_mapping_raises", "keying a non-mapping raises SelectError",
          expect_raises("owner.name.x", DOC, ".owner.name.x"))
    check("index_non_list_raises", "indexing a non-list raises SelectError",
          expect_raises("n[0]", SCALARS, ".n[0]"))

    # --- interactions: needs more than one bug fixed -------------------------
    # [*] fan-out THEN a missing key on each branch -> must raise (BUG 1 + 3).
    check("wildcard_then_missing_raises",
          "[*] fan-out then a missing key surfaces SelectError",
          expect_raises("users[*].missing", DOC, ".users[*].missing"))
    # [*] fan-out THEN recursive descent into each branch (BUG 1 + 2).
    check("wildcard_then_descend",
          "[*] fan-out then ..id pre-order into each element",
          expect_eq("users[*]..id", DOC, ".users[*]..id"))
    # descend collects role dicts, then [*] would need a list — here .roles[*].id
    # exercises fan-out under a real subtree (BUG 1, ordering).
    check("key_wildcard_key",
          ".roles[*].id over a fanned-out user",
          expect_eq("users[0].roles[*].id", DOC, ".users[0].roles[*].id"))


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 22

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "jsonquery",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
