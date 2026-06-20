#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `routerwild`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `routerwild`
package against the BRIEF'S CONTRACT (the `Router` API: `add(path, handler)` and
`match(path) -> (handler, params)` or `(None, {})`, with static > param >
wildcard precedence), NOT against any particular internal file layout.

The added capability under test: WILDCARD / catch-all segments written
`{name:*}` that capture the REMAINING path (slashes included) at the LOWEST
precedence. The shipped (setup) code supports static + param routes but has NO
wildcard, so the NEW checks below FAIL on the shipped code and PASS once the
feature is added — that's what makes the task discriminate. The REGRESSION checks
(static + param matching, no-match) pass on BOTH, guarding against a fix that
breaks what already worked.

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

# --- fixed denominator: the full roster of checks, declared before any run ----
# (id, human description). Kept in lockstep with the checks registered below so
# that an import failure can still report a result line for every single check.
#
# NEW   -> exercises the added wildcard capability (fails on shipped setup code).
# REGRESSION -> existing static/param/no-match behaviour (must keep passing).
CHECK_SPECS = [
    # --- NEW: wildcard / catch-all behaviour --------------------------------
    ("wild_captures_rest", "NEW: {path:*} matches /files/a/b/c capturing 'a/b/c'"),
    ("wild_single_segment", "NEW: {path:*} also captures a single remaining segment"),
    ("wild_includes_slashes", "NEW: wildcard capture keeps embedded slashes verbatim"),
    ("wild_after_param", "NEW: a wildcard can follow a param, capturing the tail"),
    ("static_beats_wildcard", "NEW: an exact static route beats a wildcard at the same prefix"),
    ("param_beats_wildcard", "NEW: a param segment beats a wildcard at the same position"),
    ("wild_no_false_match", "NEW: a wildcard route does not match an unrelated prefix"),
    # --- REGRESSION: existing static + param behaviour ----------------------
    ("static_match", "REGRESSION: a static route matches and returns no params"),
    ("param_capture", "REGRESSION: a {name} param matches one segment and captures it"),
    ("static_beats_param", "REGRESSION: a static route beats a param at the same position"),
    ("param_is_single_segment", "REGRESSION: a param matches exactly one segment, not more"),
    ("no_match_returns_none", "REGRESSION: an unmatched path returns (None, {})"),
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


def _ismatch(result):
    """A match result must be a 2-tuple/list (handler, params-dict)."""
    return (
        isinstance(result, (tuple, list))
        and len(result) == 2
        and isinstance(result[1], dict)
    )


def _miss(result):
    """A miss is (None, {}) — handler None and empty params (tolerant of list)."""
    return _ismatch(result) and result[0] is None and result[1] == {}


# --- import the produced package (contract: routerwild.public, fallback routerwild)
import_ok = True
import_detail = ""
Router = None
try:
    try:
        mod = importlib.import_module("routerwild.public")
    except Exception:
        mod = importlib.import_module("routerwild")
    Router = getattr(mod, "Router")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # ========================= NEW: wildcard checks =========================

    # 1. THE feature: {path:*} captures the whole remaining path incl. slashes.
    def c_wild_captures_rest():
        r = Router()
        r.add("/files/{path:*}", "serve")
        res = r.match("/files/a/b/c")
        if not _ismatch(res):
            return False, f"result={res!r} (expected (handler, params))"
        h, p = res[0], res[1]
        return (h == "serve" and p.get("path") == "a/b/c"), f"handler={h!r} params={p!r}"

    check("wild_captures_rest", c_wild_captures_rest)

    # 2. a wildcard with only one segment left still captures it.
    def c_wild_single_segment():
        r = Router()
        r.add("/files/{path:*}", "serve")
        res = r.match("/files/readme")
        h, p = (res[0], res[1]) if _ismatch(res) else (None, {})
        return (h == "serve" and p.get("path") == "readme"), f"handler={h!r} params={p!r}"

    check("wild_single_segment", c_wild_single_segment)

    # 3. the capture preserves embedded slashes exactly (not split / re-joined oddly).
    def c_wild_includes_slashes():
        r = Router()
        r.add("/assets/{rest:*}", "asset")
        res = r.match("/assets/css/site/main.css")
        h, p = (res[0], res[1]) if _ismatch(res) else (None, {})
        return (h == "asset" and p.get("rest") == "css/site/main.css"), \
            f"handler={h!r} params={p!r}"

    check("wild_includes_slashes", c_wild_includes_slashes)

    # 4. a wildcard can sit after a param: both captures are present.
    def c_wild_after_param():
        r = Router()
        r.add("/u/{id}/files/{path:*}", "uf")
        res = r.match("/u/7/files/a/b")
        h, p = (res[0], res[1]) if _ismatch(res) else (None, {})
        return (h == "uf" and p.get("id") == "7" and p.get("path") == "a/b"), \
            f"handler={h!r} params={p!r}"

    check("wild_after_param", c_wild_after_param)

    # 5. precedence: an exact STATIC route beats a wildcard sharing the prefix.
    def c_static_beats_wildcard():
        r = Router()
        r.add("/files/{path:*}", "wild")
        r.add("/files/readme", "exact")
        res = r.match("/files/readme")
        h, p = (res[0], res[1]) if _ismatch(res) else (None, {})
        # static wins -> handler "exact", and no wildcard capture leaks in.
        return (h == "exact"), f"handler={h!r} params={p!r} (expected 'exact')"

    check("static_beats_wildcard", c_static_beats_wildcard)

    # 6. precedence: a PARAM beats a wildcard at the same position (param > wildcard).
    def c_param_beats_wildcard():
        r = Router()
        r.add("/files/{rest:*}", "wild")
        r.add("/files/{name}", "one")
        res = r.match("/files/report")  # single segment: both could match
        h, p = (res[0], res[1]) if _ismatch(res) else (None, {})
        return (h == "one" and p.get("name") == "report"), \
            f"handler={h!r} params={p!r} (expected param 'one')"

    check("param_beats_wildcard", c_param_beats_wildcard)

    # 7. a wildcard route does not bleed into an unrelated prefix.
    def c_wild_no_false_match():
        r = Router()
        r.add("/files/{path:*}", "serve")
        res = r.match("/other/a/b")
        return _miss(res), f"result={res!r} (expected (None, {{}}))"

    check("wild_no_false_match", c_wild_no_false_match)

    # ===================== REGRESSION: existing behaviour ===================

    # 8. a purely static route matches and returns empty params.
    def c_static_match():
        r = Router()
        r.add("/health", "ok")
        res = r.match("/health")
        h, p = (res[0], res[1]) if _ismatch(res) else (object(), None)
        return (h == "ok" and p == {}), f"handler={h!r} params={p!r}"

    check("static_match", c_static_match)

    # 9. a {name} param matches exactly one segment and captures it.
    def c_param_capture():
        r = Router()
        r.add("/users/{id}", "user")
        res = r.match("/users/42")
        h, p = (res[0], res[1]) if _ismatch(res) else (None, {})
        return (h == "user" and p.get("id") == "42"), f"handler={h!r} params={p!r}"

    check("param_capture", c_param_capture)

    # 10. static beats param at the same position (pre-existing precedence).
    def c_static_beats_param():
        r = Router()
        r.add("/users/{id}", "by_id")
        r.add("/users/me", "me")
        res = r.match("/users/me")
        h, p = (res[0], res[1]) if _ismatch(res) else (None, {})
        return (h == "me" and p == {}), f"handler={h!r} params={p!r} (expected 'me')"

    check("static_beats_param", c_static_beats_param)

    # 11. a param matches exactly ONE segment — an extra segment is a miss.
    def c_param_is_single_segment():
        r = Router()
        r.add("/users/{id}", "user")
        res = r.match("/users/42/extra")
        return _miss(res), f"result={res!r} (expected (None, {{}}))"

    check("param_is_single_segment", c_param_is_single_segment)

    # 12. an unmatched path returns the (None, {}) miss shape.
    def c_no_match_returns_none():
        r = Router()
        r.add("/users/{id}", "user")
        res = r.match("/nope")
        return _miss(res), f"result={res!r} (expected (None, {{}}))"

    check("no_match_returns_none", c_no_match_returns_none)


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
    "task": "routerwild",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
