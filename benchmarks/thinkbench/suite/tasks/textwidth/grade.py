#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_textwidth`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced
`textwidth` package against the BRIEF'S CONTRACT (a greedy word-wrap
`wrap(text, width) -> list[str]` that packs whole words up to an INCLUSIVE
`width`, single-space-joined, collapses any run of whitespace to one boundary,
hard-breaks a word longer than `width` into width-sized pieces, emits no
trailing empty line, and returns `[]` for empty/whitespace input or width<=0),
NOT against any particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- the fit test is strict (`len(cur)+1+len(word) < width`) instead of
    inclusive (`<= width`), so a word that would land the line exactly at
    `width` is wrongly bumped to the next line: lines come out too narrow.
  * BUG 2 -- a word longer than `width` is never hard-broken; it is emitted on
    its own line and overflows past `width`.
  * BUG 3 -- the text is split with `str.split(" ")`, so runs of whitespace,
    tabs and newlines produce empty / wrong word tokens instead of collapsing
    to a single boundary.
  * BUG 4 -- the final line is flushed unconditionally, so empty / whitespace
    or trailing-whitespace input leaves a spurious trailing `""`. The buggy
    code also has no `width <= 0` guard.

Tidy, single-spaced sentences at a roomy width still wrap correctly, so a
superficial fix can pass the easy checks while still failing the edge cases.

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
CHECK_SPECS = [
    ("basic_wrap", "an ordinary sentence wraps greedily into the expected lines"),
    ("single_word", "a lone word shorter than width is returned on one line"),
    ("all_fit_one_line", "words that all fit return a single line, no extra splits"),
    ("exact_fit_packs", "a word landing the line exactly at width stays on the line"),
    ("exact_fit_two_words", "two words whose joined length equals width pack together"),
    ("no_line_exceeds_width", "no emitted line is ever wider than width (ordinary text)"),
    ("greedy_fills", "packing is greedy -- each line is filled as full as it can be"),
    ("collapse_multi_space", "runs of multiple spaces collapse to a single boundary"),
    ("collapse_tabs_newlines", "tabs and newlines are treated as whitespace boundaries"),
    ("leading_trailing_ws", "leading and trailing whitespace is dropped, no blank words"),
    ("no_empty_words", "the result never contains an empty string"),
    ("empty_input", "empty input returns [] (not ['']), no trailing empty line"),
    ("whitespace_only_input", "all-whitespace input returns [] (no blank line)"),
    ("trailing_ws_no_dangle", "trailing whitespace does not leave a dangling '' line"),
    ("overlong_hard_break", "a word longer than width is hard-broken into width pieces"),
    ("overlong_exact_multiple", "a word an exact multiple of width breaks with no empty tail"),
    ("overlong_no_overflow", "hard-broken pieces never exceed width; chars are preserved"),
    ("overlong_flushes_current", "an overlong word flushes the in-progress line first"),
    ("overlong_tail_joins", "the short tail of a broken word can take following words"),
    ("width_zero", "width == 0 returns [] for any input"),
    ("width_negative", "a negative width returns [] for any input"),
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


def _is_list_of_str(x):
    return isinstance(x, list) and all(isinstance(s, str) for s in x)


# --- import the produced package (contract: textwidth.public, fallback pkg) ----
import_ok = True
import_detail = ""
wrap = None
try:
    try:
        mod = importlib.import_module("textwidth.public")
    except Exception:
        mod = importlib.import_module("textwidth")
    wrap = getattr(mod, "wrap")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. baseline: an ordinary sentence wraps greedily.
    def c_basic_wrap():
        got = wrap("the quick brown fox", 9)
        exp = ["the quick", "brown fox"]
        return (got == exp), f"got={got!r} expected={exp!r}"

    check("basic_wrap", c_basic_wrap)

    # 2. a single short word is returned verbatim on one line.
    def c_single_word():
        got = wrap("hello", 10)
        return (got == ["hello"]), f"got={got!r} expected={['hello']!r}"

    check("single_word", c_single_word)

    # 3. words that all comfortably fit return one line (no spurious splitting).
    def c_all_fit_one_line():
        got = wrap("a b c d", 80)
        return (got == ["a b c d"]), f"got={got!r} expected={['a b c d']!r}"

    check("all_fit_one_line", c_all_fit_one_line)

    # 4. BUG 1: a word that lands the line EXACTLY at width must stay on it.
    #    'the quick' is 9 chars; at width 9 it must NOT be split to two lines.
    def c_exact_fit_packs():
        got = wrap("the quick brown", 9)
        exp = ["the quick", "brown"]
        return (got == exp), f"got={got!r} expected={exp!r} (exact-9 line must pack)"

    check("exact_fit_packs", c_exact_fit_packs)

    # 5. BUG 1 sharper: 'aa bb' is exactly 5 chars -> the two words pack together.
    def c_exact_fit_two_words():
        got = wrap("aa bb cc", 5)
        exp = ["aa bb", "cc"]
        return (got == exp), f"got={got!r} expected={exp!r}"

    check("exact_fit_two_words", c_exact_fit_two_words)

    # 6. no ordinary-text line is ever wider than width.
    def c_no_line_exceeds_width():
        w = 12
        got = wrap("alpha beta gamma delta epsilon zeta eta", w)
        bad = [ln for ln in got if len(ln) > w]
        return (_is_list_of_str(got) and not bad), f"got={got!r} over-width lines={bad!r}"

    check("no_line_exceeds_width", c_no_line_exceeds_width)

    # 7. packing is greedy: with width 11, 'aaa bbb ccc' (3+1+3+1+3=11) is one line.
    def c_greedy_fills():
        got = wrap("aaa bbb ccc ddd", 11)
        exp = ["aaa bbb ccc", "ddd"]
        return (got == exp), f"got={got!r} expected={exp!r}"

    check("greedy_fills", c_greedy_fills)

    # 8. BUG 3: runs of multiple spaces collapse to a single boundary.
    def c_collapse_multi_space():
        got = wrap("foo     bar", 80)
        return (got == ["foo bar"]), f"got={got!r} expected={['foo bar']!r}"

    check("collapse_multi_space", c_collapse_multi_space)

    # 9. BUG 3: tabs and newlines are whitespace boundaries, not characters.
    def c_collapse_tabs_newlines():
        got = wrap("foo\tbar\nbaz", 80)
        return (got == ["foo bar baz"]), f"got={got!r} expected={['foo bar baz']!r}"

    check("collapse_tabs_newlines", c_collapse_tabs_newlines)

    # 10. BUG 3/4: leading & trailing whitespace dropped, no empty words emitted.
    def c_leading_trailing_ws():
        got = wrap("   hi there   ", 80)
        return (got == ["hi there"]), f"got={got!r} expected={['hi there']!r}"

    check("leading_trailing_ws", c_leading_trailing_ws)

    # 11. the result must never contain an empty string (general invariant).
    def c_no_empty_words():
        got = wrap("a  b   c    d", 3)
        return (_is_list_of_str(got) and "" not in got), f"got={got!r} (no '' allowed)"

    check("no_empty_words", c_no_empty_words)

    # 12. BUG 4: empty input -> [] (NOT ['']).
    def c_empty_input():
        got = wrap("", 5)
        return (got == []), f"got={got!r} expected=[]"

    check("empty_input", c_empty_input)

    # 13. BUG 3/4: all-whitespace input -> [] (no blank line).
    def c_whitespace_only_input():
        got = wrap("   \t\n  ", 5)
        return (got == []), f"got={got!r} expected=[]"

    check("whitespace_only_input", c_whitespace_only_input)

    # 14. BUG 4: trailing whitespace must not leave a dangling '' final line.
    def c_trailing_ws_no_dangle():
        got = wrap("hello world   ", 11)
        exp = ["hello world"]
        return (got == exp), f"got={got!r} expected={exp!r} (no trailing '')"

    check("trailing_ws_no_dangle", c_trailing_ws_no_dangle)

    # 15. BUG 2: a word longer than width is hard-broken into width-sized pieces.
    def c_overlong_hard_break():
        got = wrap("supercalifragilistic", 7)  # len 20 -> 7,7,6
        exp = ["superca", "lifragi", "listic"]
        return (got == exp), f"got={got!r} expected={exp!r}"

    check("overlong_hard_break", c_overlong_hard_break)

    # 16. BUG 2 sharper: a word an exact multiple of width breaks with no empty tail.
    def c_overlong_exact_multiple():
        got = wrap("abcdefgh", 4)  # len 8 -> 'abcd','efgh', NOT a trailing ''
        exp = ["abcd", "efgh"]
        return (got == exp), f"got={got!r} expected={exp!r} (no empty tail piece)"

    check("overlong_exact_multiple", c_overlong_exact_multiple)

    # 17. BUG 2: every hard-broken piece is <= width and characters are preserved.
    def c_overlong_no_overflow():
        w = 5
        word = "abcdefghijklmnopqr"  # len 18
        got = wrap(word, w)
        widths_ok = _is_list_of_str(got) and all(len(p) <= w for p in got)
        rejoined = "".join(got)
        return (widths_ok and rejoined == word), \
            f"got={got!r} rejoined={rejoined!r} (each<=5, chars preserved in order)"

    check("overlong_no_overflow", c_overlong_no_overflow)

    # 18. BUG 2: an overlong word flushes the line already in progress first.
    def c_overlong_flushes_current():
        # 'hi' fits; then a 9-char word at width 4 must break, and 'hi' stays its
        # own line (not merged into the break and not lost).
        got = wrap("hi abcdefghi", 4)  # 'hi' | 'abcd' 'efgh' 'i'
        exp = ["hi", "abcd", "efgh", "i"]
        return (got == exp), f"got={got!r} expected={exp!r}"

    check("overlong_flushes_current", c_overlong_flushes_current)

    # 19. BUG 2 corollary: the short tail of a broken word may take following words.
    def c_overlong_tail_joins():
        # width 6: 'abcdefgh' -> 'abcdef' + tail 'gh'; then 'ij' joins -> 'gh ij' (5).
        got = wrap("abcdefgh ij", 6)
        exp = ["abcdef", "gh ij"]
        return (got == exp), f"got={got!r} expected={exp!r} (tail packs next word)"

    check("overlong_tail_joins", c_overlong_tail_joins)

    # 20. width == 0 -> [] for any input.
    def c_width_zero():
        got = wrap("hello world", 0)
        return (got == []), f"got={got!r} expected=[]"

    check("width_zero", c_width_zero)

    # 21. negative width -> [] for any input.
    def c_width_negative():
        got = wrap("hello world", -3)
        return (got == []), f"got={got!r} expected=[]"

    check("width_negative", c_width_negative)


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
    "task": "fix_textwidth",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
