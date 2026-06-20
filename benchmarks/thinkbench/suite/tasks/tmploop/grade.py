#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `tmploop`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `tmploop`
package against the BRIEF'S CONTRACT (the `{{#each}}` / `{{#if}}` block tags with
their nesting + scoping rules, plus the UNCHANGED plain-variable rendering), NOT
against any particular internal file layout.

The defining behaviors under test are the ones a SUPERFICIAL block implementation
gets wrong:

  * `{{ @index }}` / `{{ @first }}` / `{{ @last }}` are exposed inside `each`
    (a naive "iterate the body" loop forgets the loop metadata);
  * an EMPTY or MISSING `each` collection iterates ZERO times, and an empty
    collection is FALSY for `if` (a naive impl that only checks "is the name
    present" renders the body / the then-arm anyway);
  * a dict element's keys shadow the outer scope for that one iteration, with a
    fall-back to the outer context for names the element lacks;
  * blocks NEST in any combination and the right closer pairs with the right
    opener — a regex/non-stack matcher mispairs `{{/each}}` vs `{{/if}}`;
  * literal whitespace around tags is preserved BYTE FOR BYTE (no trimming);
  * a malformed template RAISES rather than silently mis-rendering;
  * and plain `{{ var }}` / dotted lookups / missing -> "" still work (regression).

The shipped base understands variable placeholders ONLY, so it fails every block
check while passing the regression checks — that's what makes the task
discriminate (naive lands well under 1.0, a careful nested implementation lands
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
    ("each_basic", "each iterates a list, exposing {{ this }}"),
    ("each_index", "each exposes {{ @index }} (0-based)"),
    ("each_first_last", "each exposes {{ @first }}/{{ @last }} usable as #if conditions"),
    ("each_empty_zero", "an empty each collection iterates ZERO times"),
    ("each_missing_zero", "a missing each collection iterates ZERO times"),
    ("each_dotted_this", "{{ this.field }} walks into a dict element"),
    ("each_dict_shadow", "a dict element's keys shadow outer scope, with fallback"),
    ("each_string_not_iterated", "a string value is NOT iterated char-by-char by each"),
    ("if_then_truthy", "#if renders the then-arm when cond is truthy"),
    ("if_else_falsy", "#if renders the else-arm when cond is falsy"),
    ("if_empty_collection_falsy", "an empty list/dict/0/'' is FALSY for #if"),
    ("if_no_else_arm", "#if with no {{else}} emits nothing when cond is falsy"),
    ("nest_each_in_if", "an each block nested inside an if block"),
    ("nest_if_in_each", "an if block nested inside an each block (per-element)"),
    ("nest_each_in_each", "each-in-each nests and closers pair correctly"),
    ("whitespace_preserved", "literal whitespace around tags is preserved byte-for-byte"),
    ("malformed_unclosed_raises", "an unclosed block raises rather than mis-rendering"),
    ("malformed_mismatch_raises", "a mismatched closer ({{#each}}..{{/if}}) raises"),
    ("regression_plain_var", "plain {{ var }} substitution still works"),
    ("regression_dotted_and_missing", "dotted lookup works; missing renders as ''"),
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


# --- import the produced package (contract: tmploop.public, fallback tmploop) --
import_ok = True
import_detail = ""
render = None
try:
    try:
        mod = importlib.import_module("tmploop.public")
    except Exception:
        mod = importlib.import_module("tmploop")
    render = getattr(mod, "render")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def _eq(got, want):
    return got == want, f"got {got!r} (expected {want!r})"


if import_ok:
    # 1. each iterates, exposing {{ this }}.
    def c_each_basic():
        out = render("{{#each xs}}<{{ this }}>{{/each}}", {"xs": ["a", "b", "c"]})
        return _eq(out, "<a><b><c>")

    check("each_basic", c_each_basic)

    # 2. {{ @index }} is exposed and 0-based.
    def c_each_index():
        out = render("{{#each xs}}{{ @index }}:{{ this }};{{/each}}", {"xs": ["a", "b", "c"]})
        return _eq(out, "0:a;1:b;2:c;")

    check("each_index", c_each_index)

    # 3. {{ @first }}/{{ @last }} are exposed and work as #if conditions.
    def c_each_first_last():
        tmpl = "{{#each xs}}{{#if @first}}[{{/if}}{{ this }}{{#if @last}}]{{else}},{{/if}}{{/each}}"
        out = render(tmpl, {"xs": ["a", "b", "c"]})
        return _eq(out, "[a,b,c]")

    check("each_first_last", c_each_first_last)

    # 4. an EMPTY collection iterates zero times.
    def c_each_empty_zero():
        out = render("start{{#each xs}}BODY{{/each}}end", {"xs": []})
        return _eq(out, "startend")

    check("each_empty_zero", c_each_empty_zero)

    # 5. a MISSING collection iterates zero times.
    def c_each_missing_zero():
        out = render("start{{#each nope}}BODY{{/each}}end", {})
        return _eq(out, "startend")

    check("each_missing_zero", c_each_missing_zero)

    # 6. {{ this.field }} walks into a dict element.
    def c_each_dotted_this():
        out = render("{{#each us}}{{ this.name }}/{{/each}}",
                     {"us": [{"name": "Ada"}, {"name": "Bo"}]})
        return _eq(out, "Ada/Bo/")

    check("each_dotted_this", c_each_dotted_this)

    # 7. dict-element keys shadow the outer scope; missing keys fall back to outer.
    def c_each_dict_shadow():
        out = render("{{#each rows}}{{ x }}-{{ y }};{{/each}}",
                     {"x": "OUT", "y": "OY", "rows": [{"x": "A"}, {"x": "B", "y": "BY"}]})
        # row0: x=A (own), y=OY (fallback). row1: x=B, y=BY (both own).
        return _eq(out, "A-OY;B-BY;")

    check("each_dict_shadow", c_each_dict_shadow)

    # 8. a string value is NOT iterated char-by-char.
    def c_each_string_not_iterated():
        out = render("[{{#each s}}{{ this }}{{/each}}]", {"s": "abc"})
        return _eq(out, "[]")

    check("each_string_not_iterated", c_each_string_not_iterated)

    # 9. #if then-arm when truthy.
    def c_if_then_truthy():
        out = render("{{#if ok}}YES{{else}}NO{{/if}}", {"ok": "x"})
        return _eq(out, "YES")

    check("if_then_truthy", c_if_then_truthy)

    # 10. #if else-arm when falsy.
    def c_if_else_falsy():
        out = render("{{#if ok}}YES{{else}}NO{{/if}}", {"ok": False})
        return _eq(out, "NO")

    check("if_else_falsy", c_if_else_falsy)

    # 11. empty collection / 0 / '' are FALSY for #if (not merely "present").
    def c_if_empty_collection_falsy():
        a = render("{{#if v}}T{{else}}F{{/if}}", {"v": []})
        b = render("{{#if v}}T{{else}}F{{/if}}", {"v": {}})
        c = render("{{#if v}}T{{else}}F{{/if}}", {"v": 0})
        d = render("{{#if v}}T{{else}}F{{/if}}", {"v": ""})
        e = render("{{#if v}}T{{else}}F{{/if}}", {"v": [0]})  # non-empty -> truthy
        ok = (a == "F" and b == "F" and c == "F" and d == "F" and e == "T")
        return ok, f"list={a!r} dict={b!r} zero={c!r} empty={d!r} nonempty={e!r} (want F/F/F/F/T)"

    check("if_empty_collection_falsy", c_if_empty_collection_falsy)

    # 12. #if with no else emits nothing when falsy (and the then-arm when truthy).
    def c_if_no_else_arm():
        off = render("a{{#if v}}B{{/if}}c", {"v": False})
        on = render("a{{#if v}}B{{/if}}c", {"v": True})
        ok = (off == "ac" and on == "aBc")
        return ok, f"off={off!r} on={on!r} (want 'ac'/'aBc')"

    check("if_no_else_arm", c_if_no_else_arm)

    # 13. each nested inside if.
    def c_nest_each_in_if():
        tmpl = "{{#if show}}<{{#each xs}}{{ this }}{{/each}}>{{else}}hidden{{/if}}"
        on = render(tmpl, {"show": True, "xs": [1, 2, 3]})
        off = render(tmpl, {"show": False, "xs": [1, 2, 3]})
        ok = (on == "<123>" and off == "hidden")
        return ok, f"on={on!r} off={off!r} (want '<123>'/'hidden')"

    check("nest_each_in_if", c_nest_each_in_if)

    # 14. if nested inside each, branching per element.
    def c_nest_if_in_each():
        tmpl = "{{#each xs}}{{#if this}}+{{else}}-{{/if}}{{/each}}"
        out = render(tmpl, {"xs": [1, 0, 2, 0]})
        return _eq(out, "+-+-")

    check("nest_if_in_each", c_nest_if_in_each)

    # 15. each nested inside each: closers must pair with the right opener.
    def c_nest_each_in_each():
        tmpl = "{{#each rows}}({{#each this}}{{ this }}{{/each}}){{/each}}"
        out = render(tmpl, {"rows": [[1, 2], [3], [4, 5, 6]]})
        return _eq(out, "(12)(3)(456)")

    check("nest_each_in_each", c_nest_each_in_each)

    # 16. literal whitespace around tags is preserved byte for byte.
    def c_whitespace_preserved():
        out = render("a {{#if t}} b {{/if}} c", {"t": True})
        # the spaces that flanked the removed tags survive: 'a ' + ' b ' + ' c'
        return _eq(out, "a  b  c")

    check("whitespace_preserved", c_whitespace_preserved)

    # 17. an unclosed block raises.
    def c_malformed_unclosed_raises():
        try:
            out = render("{{#each xs}}{{ this }}", {"xs": [1]})
            return False, f"unclosed each did not raise; returned {out!r}"
        except Exception as e:  # noqa: BLE001
            return True, f"raised {type(e).__name__}"

    check("malformed_unclosed_raises", c_malformed_unclosed_raises)

    # 18. a mismatched closer raises.
    def c_malformed_mismatch_raises():
        try:
            out = render("{{#each xs}}{{ this }}{{/if}}", {"xs": [1]})
            return False, f"mismatched closer did not raise; returned {out!r}"
        except Exception as e:  # noqa: BLE001
            return True, f"raised {type(e).__name__}"

    check("malformed_mismatch_raises", c_malformed_mismatch_raises)

    # 19. REGRESSION: plain variable substitution still works.
    def c_regression_plain_var():
        out = render("Hi {{ name }}, you have {{ count }} msgs", {"name": "Ada", "count": 3})
        return _eq(out, "Hi Ada, you have 3 msgs")

    check("regression_plain_var", c_regression_plain_var)

    # 20. REGRESSION: dotted lookup works; a missing variable renders as ''.
    def c_regression_dotted_and_missing():
        out = render("{{ user.name }}|{{ user.absent }}|{{ gone }}",
                     {"user": {"name": "Ada"}})
        return _eq(out, "Ada||")

    check("regression_dotted_and_missing", c_regression_dotted_and_missing)


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
    "task": "tmploop",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
