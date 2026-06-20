#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `middleware`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `middleware`
package against the BRIEF'S CONTRACT (onion-model before/after middleware with
short-circuiting, plus the unchanged core router API), NOT against any particular
internal file layout.

The defining behaviors under test are the ones a SUPERFICIAL implementation gets
wrong:

  * after-logic must unwind in REVERSE registration order (a naive "run all
    befores, call handler, run all afters in order" loop gets the unwind order
    backwards);
  * a SHORT-CIRCUIT (a middleware returning without calling ``next``) must skip
    the handler AND deeper layers, yet the already-entered OUTER layers must
    still run their after-logic and transform the response (a naive impl either
    can't short-circuit at all, or short-circuits by returning straight out of
    dispatch and so skips the outer after-logic);
  * ``next`` is LAZY: a short-circuit before the centre means the handler lookup
    never happens, so an unregistered path does NOT raise ``NotFound`` when some
    middleware short-circuits first;
  * the value ``dispatch`` returns is the OUTERMOST layer's (transformed) value,
    not the handler's raw response.

The shipped base has NO middleware, so it fails every middleware check while
passing the plain-router regression checks — that's what makes the task
discriminate (naive lands well under 1.0, a careful onion implementation lands
at 1.0).

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
    ("single_middleware_wraps", "one middleware runs before+after around the handler"),
    ("before_order_registration", "before-logic runs in registration order (outer first)"),
    ("after_order_reverse", "after-logic unwinds in REVERSE order (inner first, outer last)"),
    ("full_onion_trail", "full enter/exit trail is outer-in then inner-out"),
    ("response_transform_outer_wins", "dispatch returns the outermost layer's transformed value"),
    ("short_circuit_skips_handler", "a before-middleware returning without next skips the handler"),
    ("short_circuit_skips_deeper", "short-circuit skips middleware registered AFTER it (deeper)"),
    ("short_circuit_runs_outer_after", "short-circuit still runs the OUTER layers' after-logic"),
    ("short_circuit_outer_transforms", "outer after-logic still transforms the short-circuit response"),
    ("lazy_next_no_notfound", "short-circuit before centre: unregistered path does NOT raise"),
    ("notfound_when_centre_reached", "centre reached for unregistered path still raises NotFound"),
    ("three_layer_order", "3 middleware: before 0,1,2 then after 2,1,0"),
    ("regression_plain_dispatch", "REGRESSION: dispatch with no middleware calls the handler"),
    ("regression_plain_notfound", "REGRESSION: unregistered path with no middleware raises NotFound"),
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


# --- import the produced package (contract: middleware.public, fallback middleware) ---
import_ok = True
import_detail = ""
Router = None
NotFound = None
try:
    try:
        mod = importlib.import_module("middleware.public")
    except Exception:
        mod = importlib.import_module("middleware")
    Router = getattr(mod, "Router")
    # NotFound is part of the contract; fall back to KeyError so the error checks
    # still grade something sensible if the name is missing.
    try:
        pkg = importlib.import_module("middleware")
        NotFound = getattr(pkg, "NotFound", None)
    except Exception:
        NotFound = None
    if NotFound is None:
        NotFound = getattr(mod, "NotFound", KeyError)
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. a single middleware wraps the handler: before + after both run, and the
    #    handler's response is visible to the middleware.
    def c_single_middleware_wraps():
        r = Router()
        r.add("/x", lambda req: "H")
        seen = {}

        def mw(request, next):
            seen["before"] = True
            resp = next()
            seen["after"] = resp
            return resp + "+"

        r.use(mw)
        out = r.dispatch("/x")
        ok = (seen.get("before") is True and seen.get("after") == "H" and out == "H+")
        return ok, f"before={seen.get('before')!r} after={seen.get('after')!r} out={out!r} (expected True/'H'/'H+')"

    check("single_middleware_wraps", c_single_middleware_wraps)

    # 2. before-logic runs in registration order: first-registered runs first.
    def c_before_order_registration():
        r = Router()
        r.add("/x", lambda req: "H")
        trail = []

        def a(request, next):
            trail.append("a")
            return next()

        def b(request, next):
            trail.append("b")
            return next()

        r.use(a)
        r.use(b)
        r.dispatch("/x")
        return trail == ["a", "b"], f"before trail={trail!r} (expected ['a','b'])"

    check("before_order_registration", c_before_order_registration)

    # 3. after-logic unwinds in REVERSE order: last-registered finishes first.
    def c_after_order_reverse():
        r = Router()
        r.add("/x", lambda req: "H")
        trail = []

        def a(request, next):
            resp = next()
            trail.append("a")
            return resp

        def b(request, next):
            resp = next()
            trail.append("b")
            return resp

        r.use(a)
        r.use(b)
        r.dispatch("/x")
        # b registered last -> innermost -> its after runs first.
        return trail == ["b", "a"], f"after trail={trail!r} (expected ['b','a'])"

    check("after_order_reverse", c_after_order_reverse)

    # 4. the FULL onion trail: enter outer..inner, exit inner..outer.
    def c_full_onion_trail():
        r = Router()
        r.add("/x", lambda req: "H")
        trail = []

        def outer(request, next):
            trail.append("outer-in")
            resp = next()
            trail.append("outer-out")
            return resp

        def inner(request, next):
            trail.append("inner-in")
            resp = next()
            trail.append("inner-out")
            return resp

        r.use(outer)
        r.use(inner)
        r.dispatch("/x")
        expected = ["outer-in", "inner-in", "inner-out", "outer-out"]
        return trail == expected, f"trail={trail!r} (expected {expected!r})"

    check("full_onion_trail", c_full_onion_trail)

    # 5. dispatch returns the OUTERMOST layer's transformed value, not the
    #    handler's raw response (each layer transforms on the way out).
    def c_response_transform_outer_wins():
        r = Router()
        r.add("/x", lambda req: "h")

        def outer(request, next):
            return next() + "-outer"

        def inner(request, next):
            return next().upper() + "-inner"

        r.use(outer)
        r.use(inner)
        out = r.dispatch("/x")
        # handler -> "h"; inner -> "H-inner"; outer -> "H-inner-outer"
        return out == "H-inner-outer", f"out={out!r} (expected 'H-inner-outer')"

    check("response_transform_outer_wins", c_response_transform_outer_wins)

    # 6. a before-middleware that returns without calling next skips the handler.
    def c_short_circuit_skips_handler():
        r = Router()
        ran = {"handler": False}

        def handler(req):
            ran["handler"] = True
            return "H"

        r.add("/x", handler)

        def guard(request, next):
            return "BLOCKED"  # never calls next

        r.use(guard)
        out = r.dispatch("/x")
        ok = (out == "BLOCKED" and ran["handler"] is False)
        return ok, f"out={out!r} handler_ran={ran['handler']!r} (expected 'BLOCKED'/False)"

    check("short_circuit_skips_handler", c_short_circuit_skips_handler)

    # 7. a short-circuit skips middleware registered AFTER it (deeper layers).
    def c_short_circuit_skips_deeper():
        r = Router()
        r.add("/x", lambda req: "H")
        ran = {"deeper": False}

        def guard(request, next):
            return "BLOCKED"  # short-circuits before reaching `deeper`

        def deeper(request, next):
            ran["deeper"] = True
            return next()

        r.use(guard)
        r.use(deeper)
        out = r.dispatch("/x")
        ok = (out == "BLOCKED" and ran["deeper"] is False)
        return ok, f"out={out!r} deeper_ran={ran['deeper']!r} (expected 'BLOCKED'/False)"

    check("short_circuit_skips_deeper", c_short_circuit_skips_deeper)

    # 8. a short-circuit STILL runs the outer layers' after-logic.
    def c_short_circuit_runs_outer_after():
        r = Router()
        r.add("/x", lambda req: "H")
        trail = []

        def outer(request, next):
            trail.append("outer-in")
            resp = next()
            trail.append("outer-out")
            return resp

        def guard(request, next):
            trail.append("guard")
            return "BLOCKED"  # short-circuits; outer is already inside its next()

        r.use(outer)
        r.use(guard)
        r.dispatch("/x")
        expected = ["outer-in", "guard", "outer-out"]
        return trail == expected, f"trail={trail!r} (expected {expected!r})"

    check("short_circuit_runs_outer_after", c_short_circuit_runs_outer_after)

    # 9. the outer after-logic can still TRANSFORM the short-circuit response.
    def c_short_circuit_outer_transforms():
        r = Router()
        r.add("/x", lambda req: "H")

        def audit(request, next):
            return f"[audited] {next()}"

        def guard(request, next):
            return "403"  # short-circuit

        r.use(audit)
        r.use(guard)
        out = r.dispatch("/x")
        return out == "[audited] 403", f"out={out!r} (expected '[audited] 403')"

    check("short_circuit_outer_transforms", c_short_circuit_outer_transforms)

    # 10. LAZY next: a short-circuit before the centre means no handler lookup,
    #     so an UNREGISTERED path does NOT raise NotFound.
    def c_lazy_next_no_notfound():
        r = Router()  # nothing registered for "/missing"

        def guard(request, next):
            return "EARLY"  # returns before ever calling next -> centre unreached

        r.use(guard)
        try:
            out = r.dispatch("/missing")
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__} but should have returned 'EARLY'"
        return out == "EARLY", f"out={out!r} (expected 'EARLY', no NotFound)"

    check("lazy_next_no_notfound", c_lazy_next_no_notfound)

    # 11. but if the centre IS reached for an unregistered path, NotFound raises.
    def c_notfound_when_centre_reached():
        r = Router()  # nothing registered for "/missing"

        def passthru(request, next):
            return next()  # descends to the centre -> lookup fails

        r.use(passthru)
        try:
            r.dispatch("/missing")
            return False, "dispatch of unregistered path did not raise"
        except Exception as e:  # noqa: BLE001
            ok = isinstance(e, NotFound) and isinstance(e, KeyError)
            return ok, f"raised {type(e).__name__} (want NotFound(KeyError))"

    check("notfound_when_centre_reached", c_notfound_when_centre_reached)

    # 12. three layers: before 0,1,2 (registration order); after 2,1,0 (reverse).
    def c_three_layer_order():
        r = Router()
        r.add("/x", lambda req: "H")
        trail = []

        def make(name):
            def mw(request, next):
                trail.append(f"in:{name}")
                resp = next()
                trail.append(f"out:{name}")
                return resp
            return mw

        r.use(make("0"))
        r.use(make("1"))
        r.use(make("2"))
        r.dispatch("/x")
        expected = ["in:0", "in:1", "in:2", "out:2", "out:1", "out:0"]
        return trail == expected, f"trail={trail!r} (expected {expected!r})"

    check("three_layer_order", c_three_layer_order)

    # 13. REGRESSION: dispatch with no middleware calls the handler directly.
    def c_regression_plain_dispatch():
        r = Router()
        r.add("/a", lambda req: ("A", req))
        r.add("/b", lambda req: "B")
        a = r.dispatch("/a")
        b = r.dispatch("/b")
        ok = (a == ("A", "/a") and b == "B")
        return ok, f"a={a!r} b={b!r} (expected ('A','/a') and 'B')"

    check("regression_plain_dispatch", c_regression_plain_dispatch)

    # 14. REGRESSION: unregistered path with no middleware raises NotFound(KeyError).
    def c_regression_plain_notfound():
        r = Router()
        r.add("/a", lambda req: "A")
        try:
            r.dispatch("/nope")
            return False, "dispatch of unregistered path did not raise"
        except Exception as e:  # noqa: BLE001
            ok = isinstance(e, NotFound) and isinstance(e, KeyError)
            return ok, f"raised {type(e).__name__} (want NotFound(KeyError))"

    check("regression_plain_notfound", c_regression_plain_notfound)


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
    "task": "middleware",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
