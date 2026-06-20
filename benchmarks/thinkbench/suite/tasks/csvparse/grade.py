#!/usr/bin/env python3
"""Held-out behavior-level oracle for the bug-fix task `fix_csvparse`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never reads the model's own tests. It grades the produced `csvparse`
package against the BRIEF'S CONTRACT (the `csvparse.public.parse_csv` API), not
against any particular internal layout.

The planted bug: the starter parser splits each record on `,` naively, so a comma
INSIDE a quoted field is wrongly treated as a separator (and `""` escapes are not
unescaped). A correct fix parses RFC-4180-style quoted fields. The fixed reference
passes every check; the buggy starter passes the PLAIN-row checks but fails the
QUOTING checks.

Output: ONE JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is 0
whenever grading ran to completion (even score 0.0); a failed import forces score
0.0. This grader writes its own temp CSV inputs and cleans them up; it never leaves
files behind.
"""
import importlib
import json
import os
import sys
import tempfile

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# FIXED roster of check ids — the denominator never depends on which checks ran, so
# an import crash or a half-built package scores against the same total as a full one.
CHECK_IDS = [
    "header_plain",
    "plain_multi_row",
    "quoted_comma_single",
    "quoted_comma_alignment",
    "doubled_quote_escape",
    "quoted_comma_from_file",
    "multiple_quoted_fields",
    "plain_unquoted_unchanged",
]
TOTAL = len(CHECK_IDS)

results = {}  # cid -> (passed: bool, detail: str)


def record(cid, passed, detail=""):
    results[cid] = (bool(passed), str(detail or ""))


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


# --- import the produced package (contract: csvparse.public.parse_csv) --------
import_ok = True
import_detail = ""
parse_csv = None
try:
    pub = importlib.import_module("csvparse.public")
    parse_csv = getattr(pub, "parse_csv")
    if not callable(parse_csv):
        raise TypeError("csvparse.public.parse_csv is not callable")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


Q = '"'  # one literal double-quote, kept out of the f-strings for clarity


if import_ok:
    # 1. PLAIN header mapping — a single plain row maps header -> value in order.
    #    (Passes for the buggy starter too: it guards against "always fails".)
    def c_header_plain():
        rows = parse_csv("name,role,city\nAda,Engineer,London\n")
        return (rows == [{"name": "Ada", "role": "Engineer", "city": "London"}]), f"rows={rows!r}"

    check("header_plain", "plain row maps header columns to values in order", c_header_plain)

    # 2. PLAIN multi-row — several plain rows, all correct. (Passes for buggy too.)
    def c_plain_multi_row():
        rows = parse_csv("a,b\n1,2\n3,4\n5,6\n")
        expect = [{"a": "1", "b": "2"}, {"a": "3", "b": "4"}, {"a": "5", "b": "6"}]
        return (rows == expect), f"rows={rows!r}"

    check("plain_multi_row", "multiple plain rows all parse correctly", c_plain_multi_row)

    # 3. QUOTED COMMA — a comma inside a quoted field stays in ONE field. (Buggy: FAILS.)
    def c_quoted_comma_single():
        text = "name,role,city\nAda," + Q + "Smith, Jr." + Q + ",London\n"
        rows = parse_csv(text)
        ok = rows == [{"name": "Ada", "role": "Smith, Jr.", "city": "London"}]
        return ok, f"rows={rows!r}"

    check("quoted_comma_single", "comma inside a quoted field is not split into extra columns", c_quoted_comma_single)

    # 4. QUOTED COMMA ALIGNMENT — the column AFTER a quoted-comma field is not shifted,
    #    and a plain row in the same input is still correct. (Buggy: FAILS — column shift.)
    def c_quoted_comma_alignment():
        text = ("name,role,city\n"
                "Ada," + Q + "Smith, Jr." + Q + ",London\n"
                "Bob,Engineer,Paris\n")
        rows = parse_csv(text)
        if len(rows) != 2:
            return False, f"expected 2 rows, got {len(rows)}: {rows!r}"
        ok = (rows[0] == {"name": "Ada", "role": "Smith, Jr.", "city": "London"}
              and rows[1] == {"name": "Bob", "role": "Engineer", "city": "Paris"})
        return ok, f"rows={rows!r}"

    check("quoted_comma_alignment", "trailing columns stay aligned after a quoted-comma field", c_quoted_comma_alignment)

    # 5. DOUBLED-QUOTE ESCAPE — `""` inside a quoted field is one literal quote. (Buggy: FAILS.)
    def c_doubled_quote_escape():
        # field source: "She said ""hi"""  -> value: She said "hi"
        text = "name,quote\nAda," + Q + "She said " + Q + Q + "hi" + Q + Q + Q + "\n"
        rows = parse_csv(text)
        ok = rows == [{"name": "Ada", "quote": 'She said "hi"'}]
        return ok, f"rows={rows!r}"

    check("doubled_quote_escape", "doubled quotes inside a quoted field unescape to one literal quote", c_doubled_quote_escape)

    # 6. QUOTED COMMA read from a real FILE on disk (grader writes + cleans up its own
    #    temp CSV). Exercises the same contract via file-sourced text. (Buggy: FAILS.)
    def c_quoted_comma_from_file():
        fd, path = tempfile.mkstemp(suffix=".csv", dir=ROOT)
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                f.write("product,note,price\n")
                f.write("Widget," + Q + "red, large, sturdy" + Q + ",9.99\n")
            with open(path, "r", encoding="utf-8") as f:
                text = f.read()
        finally:
            try:
                os.remove(path)
            except OSError:
                pass
        rows = parse_csv(text)
        ok = rows == [{"product": "Widget", "note": "red, large, sturdy", "price": "9.99"}]
        return ok, f"rows={rows!r}"

    check("quoted_comma_from_file", "quoted-comma field parses correctly from file-sourced CSV text", c_quoted_comma_from_file)

    # 7. MULTIPLE QUOTED FIELDS — two quoted-comma fields in one row stay separate
    #    and correctly placed. (Buggy: FAILS.)
    def c_multiple_quoted_fields():
        text = ("a,b,c\n"
                + Q + "x, y" + Q + "," + Q + "p, q, r" + Q + ",z\n")
        rows = parse_csv(text)
        ok = rows == [{"a": "x, y", "b": "p, q, r", "c": "z"}]
        return ok, f"rows={rows!r}"

    check("multiple_quoted_fields", "multiple quoted-comma fields in one row parse to the right columns", c_multiple_quoted_fields)

    # 8. PLAIN UNQUOTED UNCHANGED — an unquoted field with no special chars is returned
    #    verbatim, and a fix must not corrupt plain rows. (Passes for buggy too.)
    def c_plain_unquoted_unchanged():
        rows = parse_csv("id,label\n42,hello world\n")
        return (rows == [{"id": "42", "label": "hello world"}]), f"rows={rows!r}"

    check("plain_unquoted_unchanged", "plain unquoted fields are returned verbatim", c_plain_unquoted_unchanged)


# --- assemble the scorecard over the FIXED denominator ------------------------
checks = []
for cid in CHECK_IDS:
    passed, detail = results.get(cid, (False, "not run (import failed)"))
    checks.append({"id": cid, "passed": passed, "detail": detail})

passed = sum(1 for c in checks if c["passed"])
card = {
    "task": "fix_csvparse",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": TOTAL,
    # An unimportable package scores a hard 0.0, regardless of any partial credit.
    "score": 0.0 if not import_ok else round(passed / TOTAL, 4),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
