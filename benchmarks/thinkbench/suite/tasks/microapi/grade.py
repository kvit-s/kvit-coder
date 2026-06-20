#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield task `microapi`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `microapi.public` `App`
and its `handle_request` API), NOT against the model's own tests and NOT against any
particular internal file layout. The `serve` CLI is explicitly OUT of scope: the oracle
drives everything in-process through `handle_request` and never opens a socket.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. The denominator is FIXED: the full check
list is defined up front, so an import failure records EVERY check as failed (score
0.0) rather than shrinking the total. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

Tolerance: the brief under-specifies some shapes. This oracle accepts any
contract-conformant representation and checks BEHAVIOR (status codes + parsed JSON
bodies + path-param delivery), not incidental key names. Spots where it assumes a
convention the brief does not pin are marked `# ASSUMES` — those are pinned in the
brief's "## Contract" section, so we never grade a guess.
"""
import importlib
import json
import os
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)


# --- fixed check registry ----------------------------------------------------
# Declared BEFORE the import attempt so the denominator never depends on whether the
# package imports. On import failure every id below is recorded failed -> score 0.0.
CHECK_IDS = [
    ("static_route", "a static route returns 200 and its JSON body"),
    ("path_param", "a named path param is delivered to the handler as the decoded value"),
    ("wildcard", "a trailing wildcard captures the rest of the path, slashes intact"),
    ("precedence_static_over_param", "an all-static match beats a param match for the same path"),
    ("precedence_param_over_wildcard", "a param match beats a wildcard match for the same path"),
    ("query_parse", "query-string params are parsed and exposed to the handler"),
    ("json_body", "a JSON request body is parsed and exposed to the handler"),
    ("json_response", "responses are JSON-encoded bytes with a JSON content-type"),
    ("status_passthrough", "a handler-chosen status code is returned verbatim"),
    ("not_found_404", "an unmatched path returns status 404"),
    ("method_not_allowed_405", "a matched path with a wrong method returns 405, not 404"),
    ("error_body_shape", "404/405 bodies are JSON objects carrying an 'error' signal"),
    ("middleware_order", "middlewares run in registration order, unwinding in reverse"),
    ("middleware_short_circuit", "a middleware can short-circuit before the handler runs"),
]

checks = []
_results = {}


def record(cid, ok, detail):
    _results[cid] = (bool(ok), str(detail or ""))


def finish():
    """Emit checks in the fixed registry order; unrun ids are failures."""
    for cid, desc in CHECK_IDS:
        ok, detail = _results.get(cid, (False, "not evaluated (import failed)"))
        checks.append({"id": cid, "desc": desc, "passed": ok, "detail": detail})


def run(cid, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


# --- tolerant helpers --------------------------------------------------------
def call(app, method, path, headers=None, body=b""):
    """Invoke handle_request and normalize the 3-tuple. Returns (status, headers, raw_body)."""
    status, hdrs, raw = app.handle_request(method, path, headers or {}, body)
    return status, hdrs, raw


def as_json(raw):
    """Decode a response body (bytes or str) to a parsed JSON value, or MISSING."""
    if isinstance(raw, (bytes, bytearray)):
        try:
            return json.loads(raw.decode("utf-8"))
        except Exception:  # noqa: BLE001
            return MISSING
    if isinstance(raw, str):
        try:
            return json.loads(raw)
        except Exception:  # noqa: BLE001
            return MISSING
    return MISSING


MISSING = object()


def has_error_signal(blob):
    """Structured error body: a JSON object with a top-level key naming the error and a
    non-empty value. Not a substring scan (which could false-pass on arbitrary content)."""
    if not isinstance(blob, dict):
        return False
    for k, v in blob.items():
        kl = str(k).lower()
        if ("error" in kl or "message" in kl or "detail" in kl) and v:
            return True
    return False


def ct_is_json(hdrs):
    """Content-Type header (case-insensitively keyed) advertises JSON."""
    if not isinstance(hdrs, dict):
        return False
    for k, v in hdrs.items():
        if str(k).lower() == "content-type" and "json" in str(v).lower():
            return True
    return False


# --- import the produced package (contract: microapi.public) -----------------
import_ok = True
import_detail = ""
App = None
try:
    pub = importlib.import_module("microapi.public")
    App = getattr(pub, "App")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def build_app():
    """Construct one App wired with the routes the behavior checks exercise.

    ASSUMES the Contract's registration API: App() with no required args, `route` as a
    decorator factory returning a decorator, handler called as handler(request, **params),
    handler returns `value` (->200) or `(status, value)`. All pinned in ## Contract.
    """
    app = App()

    @app.route("GET", "/users")
    def list_users(request):  # noqa: ANN001
        return {"users": ["alice", "bob"], "q": request.query.get("limit")}

    # Register the LESS-specific (param) route FIRST, so a registration-order router
    # can't pass the precedence check — only one that prefers the static route wins.
    @app.route("GET", "/users/{user_id}")
    def get_user(request, user_id):  # noqa: ANN001
        return {"user_id": user_id}

    @app.route("GET", "/users/me")
    def me(request):  # noqa: ANN001 - static, must beat the param route registered above
        return {"who": "me"}

    @app.route("POST", "/users")
    def make_user(request):  # noqa: ANN001
        return (201, {"created": (request.json or {}).get("name")})

    @app.route("GET", "/files/{path:*}")
    def get_file(request, path):  # noqa: ANN001
        return {"path": path}

    # Register the LESS-specific (wildcard) route FIRST, so a registration-order router
    # can't pass the precedence check — only one that prefers the param route wins.
    @app.route("GET", "/a/{rest:*}")
    def a_wild(request, rest):  # noqa: ANN001
        return {"matched": "wildcard", "rest": rest}

    @app.route("GET", "/a/{x}")
    def a_param(request, x):  # noqa: ANN001
        return {"matched": "param", "x": x}

    return app


if import_ok:
    # 1. static route -> 200 + JSON body
    def c_static():
        app = build_app()
        st, hd, raw = call(app, "GET", "/users")
        body = as_json(raw)
        return (st == 200 and isinstance(body, dict) and body.get("users") == ["alice", "bob"]), \
            f"status={st} body={body!r}"

    run("static_route", c_static)

    # 2. named path param delivered (and percent-decoded)
    def c_path_param():
        app = build_app()
        st, hd, raw = call(app, "GET", "/users/42")
        body = as_json(raw)
        # also exercise decoding
        st2, _, raw2 = call(app, "GET", "/users/a%20b")
        body2 = as_json(raw2)
        ok = (st == 200 and isinstance(body, dict) and body.get("user_id") == "42"
              and st2 == 200 and isinstance(body2, dict) and body2.get("user_id") == "a b")
        return ok, f"id={body!r} decoded={body2!r}"

    run("path_param", c_path_param)

    # 3. wildcard captures the rest, slashes intact
    def c_wildcard():
        app = build_app()
        st, hd, raw = call(app, "GET", "/files/a/b/c.txt")
        body = as_json(raw)
        return (st == 200 and isinstance(body, dict) and body.get("path") == "a/b/c.txt"), \
            f"status={st} body={body!r}"

    run("wildcard", c_wildcard)

    # 4. static beats param: /users/me must hit the static handler
    def c_prec_static():
        app = build_app()
        st, hd, raw = call(app, "GET", "/users/me")
        body = as_json(raw)
        return (st == 200 and isinstance(body, dict) and body.get("who") == "me"), \
            f"status={st} body={body!r}"

    run("precedence_static_over_param", c_prec_static)

    # 5. param beats wildcard: /a/x (single segment) must hit the param handler
    def c_prec_param():
        app = build_app()
        st, hd, raw = call(app, "GET", "/a/x")
        body = as_json(raw)
        return (st == 200 and isinstance(body, dict) and body.get("matched") == "param"
                and body.get("x") == "x"), f"status={st} body={body!r}"

    run("precedence_param_over_wildcard", c_prec_param)

    # 6. query string parsed and exposed
    def c_query():
        app = build_app()
        st, hd, raw = call(app, "GET", "/users?limit=5")
        body = as_json(raw)
        return (st == 200 and isinstance(body, dict) and body.get("q") == "5"), \
            f"status={st} body={body!r}"

    run("query_parse", c_query)

    # 7. JSON request body parsed and exposed
    def c_json_body():
        app = build_app()
        st, hd, raw = call(app, "POST", "/users", {"Content-Type": "application/json"},
                           json.dumps({"name": "carol"}).encode("utf-8"))
        body = as_json(raw)
        return (st == 201 and isinstance(body, dict) and body.get("created") == "carol"), \
            f"status={st} body={body!r}"

    run("json_body", c_json_body)

    # 8. responses are JSON-encoded bytes with a JSON content-type
    def c_json_resp():
        app = build_app()
        st, hd, raw = call(app, "GET", "/users")
        body = as_json(raw)
        return (isinstance(raw, (bytes, bytearray)) and body is not MISSING and ct_is_json(hd)), \
            f"raw_type={type(raw).__name__} ct_json={ct_is_json(hd)} headers={hd!r}"

    run("json_response", c_json_resp)

    # 9. handler-chosen status returned verbatim (201 on POST /users)
    def c_status_passthrough():
        app = build_app()
        st, hd, raw = call(app, "POST", "/users", {}, json.dumps({"name": "z"}).encode("utf-8"))
        return (st == 201), f"status={st}"

    run("status_passthrough", c_status_passthrough)

    # 10. unmatched path -> 404
    def c_404():
        app = build_app()
        st, hd, raw = call(app, "GET", "/nope/nothing")
        return (st == 404), f"status={st}"

    run("not_found_404", c_404)

    # 11. matched path, wrong method -> 405 (NOT 404)
    def c_405():
        app = build_app()
        # /users is registered for GET and POST; DELETE matches the path but no method
        st, hd, raw = call(app, "DELETE", "/users")
        return (st == 405), f"status={st}"

    run("method_not_allowed_405", c_405)

    # 12. error bodies (404 & 405) are JSON objects carrying an error signal
    def c_error_body():
        app = build_app()
        s404, _, r404 = call(app, "GET", "/totally/missing")
        s405, _, r405 = call(app, "DELETE", "/users")
        b404, b405 = as_json(r404), as_json(r405)
        ok = has_error_signal(b404) and has_error_signal(b405)
        return ok, f"404body={b404!r} 405body={b405!r}"

    run("error_body_shape", c_error_body)

    # 13. middleware ordering: first-registered runs outermost (first in, last out)
    def c_mw_order():
        app = App()
        trail = []

        def mw1(request, call_next):
            trail.append("in1")
            resp = call_next(request)
            trail.append("out1")
            return resp

        def mw2(request, call_next):
            trail.append("in2")
            resp = call_next(request)
            trail.append("out2")
            return resp

        app.use(mw1)
        app.use(mw2)

        @app.route("GET", "/ping")
        def ping(request):  # noqa: ANN001
            trail.append("handler")
            return {"ok": True}

        st, hd, raw = call(app, "GET", "/ping")
        body = as_json(raw)
        ok = (st == 200 and isinstance(body, dict) and body.get("ok") is True
              and trail == ["in1", "in2", "handler", "out2", "out1"])
        return ok, f"trail={trail!r} status={st}"

    run("middleware_order", c_mw_order)

    # 14. a middleware can short-circuit before the handler
    def c_mw_short():
        app = App()
        reached = {"handler": False}

        def gate(request, call_next):
            return (401, {"error": "blocked"})  # never calls call_next

        app.use(gate)

        @app.route("GET", "/secret")
        def secret(request):  # noqa: ANN001
            reached["handler"] = True
            return {"secret": 1}

        st, hd, raw = call(app, "GET", "/secret")
        body = as_json(raw)
        ok = (st == 401 and reached["handler"] is False
              and isinstance(body, dict) and has_error_signal(body))
        return ok, f"status={st} reached_handler={reached['handler']} body={body!r}"

    run("middleware_short_circuit", c_mw_short)


# --- scorecard ---------------------------------------------------------------
finish()
passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "microapi",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
