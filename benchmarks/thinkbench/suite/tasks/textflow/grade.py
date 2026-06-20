#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `textflow`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``textflow`` package against the BRIEF'S CONTRACT (the ``textflow.public``
``justify`` API), NOT against any particular internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE spacing / single-word / last-line / width edge cases, all with expected
values computed HERE by an independent reference (never read from the agent's
tests). The FIXED reference passes every check; the planted-bug starter fails a
discriminating subset, so a partial fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``textflow.public``:
  1. uneven space distribution -- when the leftover spaces do not divide evenly
     across the gaps, the EXTRA spaces are pushed to the RIGHT-most gaps instead
     of the LEFT-most ones (lines that divide evenly are unaffected);
  2. single-word lines -- a lone word on an interior line is emitted as-is and
     comes out SHORTER than ``width`` instead of being left-justified and padded;
  3. last-line special case -- the final line is fully justified (its single
     spaces stretched into wide gaps) instead of left-justified and padded.

These interact: a paragraph that contains an uneven interior line, a single-word
interior line, AND a ragged last line exercises all three at once, and only a
fix that addresses every bug reproduces the reference layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs). It
does NOT import or execute the agent's ``test_*.py``.
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
# Contract path is ``textflow.public``; fall back to the package root
# ``textflow`` so a submission that re-exports ``justify`` from ``__init__`` (but
# moved it off ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
justify = None
JustifyError = None
try:
    try:
        mod = importlib.import_module("textflow.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("textflow")
    justify = getattr(mod, "justify")
    JustifyError = getattr(mod, "JustifyError", None)
    if not (isinstance(JustifyError, type) and issubclass(JustifyError, BaseException)):
        JustifyError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
def _pack(words, width):
    """Greedy line packing (one space assumed between adjacent words)."""
    lines = []
    cur = []
    cur_len = 0
    for w in words:
        if cur and cur_len + len(cur) + len(w) > width:
            lines.append(cur)
            cur = [w]
            cur_len = len(w)
        else:
            cur.append(w)
            cur_len += len(w)
    if cur:
        lines.append(cur)
    return lines


def oracle(words, width):
    """Reference full justification, independent of the submission under test."""
    if not words:
        return []
    lines = _pack(words, width)
    out = []
    for idx, line in enumerate(lines):
        is_last = idx == len(lines) - 1
        if is_last or len(line) == 1:
            s = " ".join(line)
            out.append(s + " " * (width - len(s)))
        else:
            word_chars = sum(len(w) for w in line)
            gaps = len(line) - 1
            base, extra = divmod(width - word_chars, gaps)
            parts = []
            for i, w in enumerate(line[:-1]):
                parts.append(w)
                parts.append(" " * (base + (1 if i < extra else 0)))
            parts.append(line[-1])
            out.append("".join(parts))
    return out


def expect(label, words, width):
    """Check ``justify(words, width)`` equals the oracle's layout."""

    def _fn():
        want = oracle(list(words), width)
        got = justify(list(words), width)
        return (list(got) == want), f"{label}: got {got!r}, expected {want!r}"

    return _fn


def expect_width(label, words, width):
    """Check every produced line is exactly ``width`` characters (a weaker,
    independent invariant that the single-word / last-line bugs also break)."""

    def _fn():
        got = justify(list(words), width)
        bad = [ln for ln in got if len(ln) != width]
        return (len(bad) == 0), f"{label}: lines not width {width}: {bad!r} (full {got!r})"

    return _fn


if import_ok:
    # --- baseline: even-fit / no-remainder lines (pass even buggy) -----------
    check("single_line_exact", "words filling a line exactly pass through",
          expect("exact", ["a", "b", "c"], 5))
    check("even_distribution", "evenly divisible spaces look right under any rule",
          expect("even", ["aa", "bb", "cc"], 8))  # 6 chars, 2 spaces, 2 gaps -> even
    check("even_three_gaps", "three gaps that divide evenly look right",
          expect("even3", ["a", "b", "c", "d"], 7))  # 4 chars, 3 spaces, 3 gaps -> even
    check("two_word_full_width", "two words that exactly fill a line pass through",
          expect("two", ["ab", "cd"], 5))  # 4 chars + 1 space == width 5, even
    check("even_multi_line", "consecutive even-fit lines pack and justify cleanly",
          expect("evenml", ["ab", "cd", "ef", "gh"], 5))  # two full-width lines
    check("greedy_pack_widths", "greedy packing yields exactly-width lines",
          expect_width("pack", ["the", "quick", "brown", "fox", "jumps"], 11))

    # --- BUG 1: extra spaces go to the LEFT gaps -----------------------------
    check("uneven_extra_left_2gaps", "1 extra space goes to the left gap",
          expect("u2", ["a", "b", "c"], 8))  # 5 spaces / 2 gaps -> "a   b  c"
    check("uneven_extra_left_3gaps", "2 extra spaces go to the first two gaps",
          expect("u3", ["aa", "bb", "cc", "dd"], 13))  # 5/3 -> "aa  bb  cc dd"
    check("uneven_big_remainder", "remainder front-loaded across many gaps",
          expect("ubig", ["x", "y", "z", "w", "v"], 14))  # 5 words, 10 spaces/4 gaps
    check("uneven_width_invariant", "uneven lines are still exactly width wide",
          expect_width("uw", ["a", "bb", "c", "dd", "e"], 16))

    # --- BUG 2: single-word interior line is left-justified + padded ---------
    check("single_word_interior_padded", "lone interior word padded out to width",
          expect("sw", ["longword", "tail"], 12))
    check("single_word_interior_width", "lone interior word line is exactly width",
          expect_width("sww", ["enormoustoken", "z"], 13))
    check("single_word_then_pack", "single-word line then a justified line",
          expect("swp", ["alpha", "to", "be", "or", "not"], 6))

    # --- BUG 3: the last line is left-justified, not fully justified ---------
    check("last_line_left_justified", "final line left-justified + padded, not stretched",
          expect("ll", ["alpha", "beta", "gamma"], 14))
    check("last_line_multiword_single_spaced", "final line keeps single spaces then pads",
          expect("llm", ["one", "two", "three", "tiny", "end"], 9))
    check("last_line_is_width", "the last line is still exactly width wide",
          expect_width("llw", ["pack", "these", "words", "here"], 12))

    # --- the three-way interaction (needs ALL three bugs fixed) --------------
    check("interaction_all_three", "uneven + single-word + last-line together",
          expect("inter", ["practical", "no", "gap", "x", "the", "final", "row"], 10))
    check("interaction_long_paragraph", "a longer paragraph exercises every path",
          expect("para",
                 ["this", "is", "an", "example", "of", "text", "justification",
                  "done", "right"],
                 16))
    check("interaction_widths", "every line of the interaction paragraph is width",
          expect_width("interw",
                       ["practical", "no", "gap", "x", "the", "final", "row"], 10))

    # --- validation ----------------------------------------------------------
    def c_empty():
        got = justify([], 10)
        return (list(got) == []), f"got {got!r}, expected []"

    check("empty_words_empty", "empty word list yields empty result", c_empty)

    def c_long_word():
        try:
            justify(["toolongword"], 4)
        except JustifyError:
            return True, "raised JustifyError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected JustifyError"
        return False, "did not raise"

    check("long_word_raises", "a word longer than width raises JustifyError", c_long_word)

    def c_bad_width():
        try:
            justify(["a"], 0)
        except JustifyError:
            return True, "raised JustifyError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected JustifyError"
        return False, "did not raise"

    check("bad_width_raises", "a non-positive width raises JustifyError", c_bad_width)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 22

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "textflow",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
