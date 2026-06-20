#!/usr/bin/env python3
"""Held-out behavior-level oracle for the greenfield `patchwise` task.

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `patchwise.public`
API and the `python -m patchwise` CLI), NOT against the model's own tests and NOT
against any particular internal file layout or diff TEXT.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. The denominator is FIXED: if the
package fails to import, every behavior check is still recorded as FAILED (never
skipped), so a non-importable submission scores 0.0 over the full denominator.
Exit code is 0 whenever grading ran to completion (even score 0.0); nonzero only
on a grader-internal failure.

FAIRNESS — the central tolerance for this task: a correct implementation's exact
diff TEXT may differ from the reference's (context-line count, hunk headers,
ordering of equal hunks, the trailing-newline marker spelling, etc.). This oracle
therefore grades ROUND-TRIP BEHAVIOR —

    apply_patch(old, unified_diff(old, new)) == new   (byte-for-byte)

— and verifies that a HAND-WRITTEN, standard unified diff applies correctly. It
NEVER demands string-equality against any particular diff text, and it never reads
the model's own tests. Spots where it assumes a convention the brief does not pin
are marked `# ASSUMES`.
"""
import importlib
import json
import os
import subprocess
import sys
import tempfile

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# The full set of check (id, description) pairs is declared up front so the
# denominator is FIXED regardless of whether the import succeeds.
CHECK_SPECS = [
    ("roundtrip_addition", "round-trip holds when lines are added"),
    ("roundtrip_deletion", "round-trip holds when lines are deleted"),
    ("roundtrip_replace", "round-trip holds when lines are replaced"),
    ("roundtrip_multi_hunk", "round-trip holds across multiple separated hunks"),
    ("roundtrip_no_trailing_nl", "round-trip preserves a missing final newline"),
    ("roundtrip_add_trailing_nl", "round-trip preserves added/kept trailing newline"),
    ("roundtrip_empty_old", "round-trip holds when old is the empty string"),
    ("roundtrip_empty_new", "round-trip holds when new is the empty string"),
    ("roundtrip_identical", "diff of identical content round-trips unchanged"),
    ("apply_handwritten_diff", "a hand-written standard unified diff applies correctly"),
    ("diff_is_string", "unified_diff returns a string in unified-diff shape"),
    ("context_mismatch_raises_patch_exc", "failed patch raises a 'Patch'-named exception from patchwise.public"),
    ("context_mismatch_not_silent", "failed patch never silently returns wrong content"),
    ("cli_diff_emits_diff", "`python -m patchwise diff` writes a unified diff to stdout"),
    ("cli_roundtrip", "CLI diff then apply --out reproduces the new file"),
]

results_by_id = {}


def record(cid, ok, detail):
    results_by_id[cid] = {"passed": bool(ok), "detail": str(detail or "")}


def check(cid, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


# --- import the produced package (contract: patchwise.public) ----------------
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("patchwise.public")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def roundtrip(old, new):
    """The core contract: apply(old, diff(old, new)) must equal new byte-for-byte."""
    patch = pub.unified_diff(old, new)
    if not isinstance(patch, str):
        return False, f"unified_diff returned {type(patch).__name__}, not str"
    got = pub.apply_patch(old, patch)
    if got == new:
        return True, "round-trip exact"
    return False, f"round-trip mismatch: got {got!r} expected {new!r}"


if import_ok:
    # 1. addition
    check("roundtrip_addition", lambda: roundtrip(
        "line1\nline2\nline3\n",
        "line1\nline2\nINSERTED\nline3\n",
    ))

    # 2. deletion
    check("roundtrip_deletion", lambda: roundtrip(
        "a\nb\nc\nd\ne\n",
        "a\nb\nd\ne\n",
    ))

    # 3. replacement
    check("roundtrip_replace", lambda: roundtrip(
        "alpha\nbeta\ngamma\n",
        "alpha\nBETA\ngamma\n",
    ))

    # 4. multiple separated hunks (far apart so they form distinct hunks)
    def c_multi_hunk():
        old = "".join(f"line{i}\n" for i in range(1, 21))
        new_lines = [f"line{i}\n" for i in range(1, 21)]
        new_lines[1] = "CHANGED-2\n"      # near the top
        new_lines[17] = "CHANGED-18\n"    # near the bottom -> separate hunk
        return roundtrip(old, "".join(new_lines))

    check("roundtrip_multi_hunk", c_multi_hunk)

    # 5. old lacks a trailing newline; new also lacks one (must survive round-trip)
    check("roundtrip_no_trailing_nl", lambda: roundtrip(
        "first\nsecond\nthird",   # no final newline
        "first\nSECOND\nthird",   # still no final newline
    ))

    # 6. old lacks trailing newline, new adds one
    check("roundtrip_add_trailing_nl", lambda: roundtrip(
        "only line",       # no final newline
        "only line\n",     # final newline added
    ))

    # 7. empty old -> non-empty new (pure creation)
    check("roundtrip_empty_old", lambda: roundtrip(
        "",
        "brand\nnew\ncontent\n",
    ))

    # 8. non-empty old -> empty new (full deletion)
    check("roundtrip_empty_new", lambda: roundtrip(
        "remove\nall\nof\nthis\n",
        "",
    ))

    # 9. identical content: diff applied to old yields old unchanged
    def c_identical():
        same = "unchanged\ncontent\nhere\n"
        patch = pub.unified_diff(same, same)
        got = pub.apply_patch(same, patch)
        return (got == same), f"got {got!r}"

    check("roundtrip_identical", c_identical)

    # 10. a HAND-WRITTEN standard unified diff (not produced by the model) must apply.
    #     This pins that apply_patch understands the real unified-diff format, while
    #     staying text-tolerant about what the model's OWN diff looks like.
    def c_handwritten():
        old = "one\ntwo\nthree\nfour\n"
        expected = "one\nTWO\nthree\nfour\nfive\n"
        # ASSUMES the standard unified-diff grammar: `@@ -l,s +l,s @@` headers,
        # ' '/'-'/'+' line prefixes. Generous context (whole file) so a parser that
        # ignores the exact @@ line numbers and matches on context still succeeds.
        patch = (
            "--- old\n"
            "+++ new\n"
            "@@ -1,4 +1,5 @@\n"
            " one\n"
            "-two\n"
            "+TWO\n"
            " three\n"
            " four\n"
            "+five\n"
        )
        got = pub.apply_patch(old, patch)
        return (got == expected), f"got {got!r}"

    check("apply_handwritten_diff", c_handwritten)

    # 11. unified_diff yields a string that looks like a unified diff for a real change
    def c_diff_shape():
        patch = pub.unified_diff("x\ny\n", "x\nz\n")
        if not isinstance(patch, str):
            return False, f"type {type(patch).__name__}"
        # A real change must produce at least one hunk header. Text-tolerant beyond that.
        return ("@@" in patch and ("-y" in patch or "+z" in patch)), f"patch={patch!r}"

    check("diff_is_string", c_diff_shape)

    # 12. a context mismatch must raise an exception whose type name contains "Patch",
    #     and that type must be exported from patchwise.public (the pinned mechanism).
    def c_mismatch_raises():
        # Build a valid patch against `base`, then apply it to MISMATCHED content.
        base = "aaa\nbbb\nccc\n"
        changed = "aaa\nBBB\nccc\n"
        patch = pub.unified_diff(base, changed)
        mismatched = "xxx\nyyy\nzzz\n"  # context lines won't match
        try:
            out = pub.apply_patch(mismatched, patch)
        except Exception as e:  # noqa: BLE001
            name = type(e).__name__
            # A meaningful CUSTOM signal exported from patchwise.public. We do NOT gate
            # on the exception NAME — a correct `DiffError` is as valid as `PatchError`.
            if type(e).__module__ == "builtins":
                return False, f"raised a bare builtin {name}; expected a custom patch error"
            exported = getattr(pub, name, None) is type(e) or any(
                getattr(pub, a, None) is type(e) for a in dir(pub)
            )
            if not exported:
                return False, f"{name} not exported from patchwise.public"
            return True, f"raised {name}"
        return False, f"no exception raised; returned {out!r}"

    check("context_mismatch_raises_patch_exc", c_mismatch_raises)

    # 13. independent of HOW it signals, a failed patch must NOT silently return the
    #     correctly-patched-elsewhere content nor the raw mismatched input as if ok.
    def c_mismatch_not_silent():
        base = "aaa\nbbb\nccc\n"
        changed = "aaa\nBBB\nccc\n"
        patch = pub.unified_diff(base, changed)
        mismatched = "xxx\nyyy\nzzz\n"
        try:
            out = pub.apply_patch(mismatched, patch)
        except Exception:  # noqa: BLE001 - raising is the pinned, acceptable signal
            return True, "signalled via exception"
        # If it didn't raise, the only acceptable non-raising outcome would be a
        # structured error object — never a plausible-but-wrong string. Since the
        # contract pins raising, any returned string here is a silent failure.
        return False, f"silently returned {out!r} on a context mismatch"

    check("context_mismatch_not_silent", c_mismatch_not_silent)


# --- CLI: python -m patchwise diff / apply ------------------------------------
def _write_file(text, suffix):
    fd, path = tempfile.mkstemp(suffix=suffix, dir=ROOT)
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        f.write(text)
    return path


def c_cli_diff_emits():
    old_p = _write_file("p\nq\nr\n", "_old.txt")
    new_p = _write_file("p\nQ\nr\n", "_new.txt")
    try:
        proc = subprocess.run(
            [sys.executable, "-m", "patchwise", "diff", old_p, new_p],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        out = proc.stdout
        return ("@@" in out), f"rc={proc.returncode} stdout={out[:120]!r}"
    finally:
        for p in (old_p, new_p):
            try:
                os.remove(p)
            except OSError:
                pass


def c_cli_roundtrip():
    old_text = "k1\nk2\nk3\nk4\n"
    new_text = "k1\nK2\nk3\nk4\nk5\n"
    old_p = _write_file(old_text, "_old.txt")
    new_p = _write_file(new_text, "_new.txt")
    # ASSUMES `--out` names the file to write; the brief pins this CLI form.
    patch_p = os.path.join(ROOT, os.path.basename(tempfile.mktemp(suffix="_patch.diff")))
    out_p = os.path.join(ROOT, os.path.basename(tempfile.mktemp(suffix="_result.txt")))
    try:
        dproc = subprocess.run(
            [sys.executable, "-m", "patchwise", "diff", old_p, new_p],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        with open(patch_p, "w", encoding="utf-8") as f:
            f.write(dproc.stdout)
        aproc = subprocess.run(
            [sys.executable, "-m", "patchwise", "apply", old_p, patch_p, "--out", out_p],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        if not os.path.exists(out_p):
            return False, f"no --out file; apply rc={aproc.returncode} stderr={aproc.stderr[:120]!r}"
        with open(out_p, encoding="utf-8") as f:
            got = f.read()
        return (got == new_text), f"got {got!r} expected {new_text!r}"
    finally:
        for p in (old_p, new_p, patch_p, out_p):
            try:
                os.remove(p)
            except OSError:
                pass


check("cli_diff_emits_diff", c_cli_diff_emits)
check("cli_roundtrip", c_cli_roundtrip)


# --- assemble the scorecard over the FIXED denominator ------------------------
checks = []
for cid, desc in CHECK_SPECS:
    res = results_by_id.get(cid)
    if res is None:
        # Never skip: a missing result (e.g. import failed) counts as FAILED.
        res = {"passed": False, "detail": "not run (import failed)" if not import_ok else "not run"}
    checks.append({"id": cid, "desc": desc, "passed": res["passed"], "detail": res["detail"]})

passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "patchwise",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": round(passed / total, 4) if total else 0.0,
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
