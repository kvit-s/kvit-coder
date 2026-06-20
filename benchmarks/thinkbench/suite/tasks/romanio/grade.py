#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `romanio`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``romanio`` package against the BRIEF'S CONTRACT (the ``romanio.public``
``to_roman`` / ``from_roman`` API), NOT against any particular internal file
layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE subtractive / round-trip / boundary cases, all with expected values
computed HERE (never read from the agent's tests). The FIXED reference passes
every check; the planted-bug starter fails a discriminating subset, so a partial
fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``romanio.public``:
  1. ``to_roman`` uses ADDITIVE notation only — its value table omits the six
     subtractive pairs, so 4 -> "IIII", 9 -> "VIIII", 40 -> "XXXX", 900 ->
     "DCCCC", instead of IV / IX / XL / CM;
  2. ``from_roman`` SUMS every symbol, never subtracting a smaller symbol that
     precedes a larger one, so "IV" -> 6 and "MCMXCIV" parses wrong;
  3. ``to_roman`` skips the range guard — n < 1 or n > 3999 should raise
     ``RomanError`` but instead returns "" (for n <= 0) or a run of leading Ms.

Bugs 1 and 2 interact through the round-trip property
``from_roman(to_roman(n)) == n``: it only holds on subtractive values once BOTH
directions agree on subtractive notation. Bug 3 is what makes the 1..3999
boundary checks pass.

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
# Contract path is ``romanio.public``; fall back to the package root ``romanio``
# so a submission that re-exports the API from ``__init__`` (but moved it off
# ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
to_roman = None
from_roman = None
RomanError = None
try:
    try:
        mod = importlib.import_module("romanio.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("romanio")
    to_roman = getattr(mod, "to_roman")
    from_roman = getattr(mod, "from_roman")
    RomanError = getattr(mod, "RomanError", None)
    if not (isinstance(RomanError, type) and issubclass(RomanError, BaseException)):
        RomanError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
_REF_VALUES = [
    (1000, "M"), (900, "CM"), (500, "D"), (400, "CD"),
    (100, "C"), (90, "XC"), (50, "L"), (40, "XL"),
    (10, "X"), (9, "IX"), (5, "V"), (4, "IV"), (1, "I"),
]


def ref_to_roman(n):
    """Reference encoder, independent of the submission under test."""
    out = []
    for value, symbol in _REF_VALUES:
        while n >= value:
            out.append(symbol)
            n -= value
    return "".join(out)


def expect_to(label, n, want):
    def _fn():
        got = to_roman(n)
        return (got == want), f"{label}: to_roman({n}) = {got!r}, expected {want!r}"
    return _fn


def expect_from(label, s, want):
    def _fn():
        got = from_roman(s)
        return (got == want), f"{label}: from_roman({s!r}) = {got!r}, expected {want!r}"
    return _fn


if import_ok:
    # --- baseline: simple additive numerals (pass even buggy; guards regressions)
    check("to_simple_i_to_iii", "to_roman renders 1,2,3 as I,II,III",
          lambda: (
              (to_roman(1), to_roman(2), to_roman(3)) == ("I", "II", "III"),
              f"got {(to_roman(1), to_roman(2), to_roman(3))!r}",
          ))
    check("to_additive_vi_viii", "to_roman renders 6 -> VI, 8 -> VIII",
          lambda: ((to_roman(6), to_roman(8)) == ("VI", "VIII"),
                   f"got {(to_roman(6), to_roman(8))!r}"))
    check("to_additive_thirtyeight", "to_roman renders 38 -> XXXVIII",
          expect_to("add38", 38, "XXXVIII"))
    check("from_additive_basics", "from_roman parses III,VI,XXX additively",
          lambda: (
              (from_roman("III"), from_roman("VI"), from_roman("XXX")) == (3, 6, 30),
              f"got {(from_roman('III'), from_roman('VI'), from_roman('XXX'))!r}",
          ))

    # --- BUG 1: to_roman subtractive notation --------------------------------
    check("to_sub_four", "to_roman(4) -> IV (not IIII)", expect_to("sub4", 4, "IV"))
    check("to_sub_nine", "to_roman(9) -> IX (not VIIII)", expect_to("sub9", 9, "IX"))
    check("to_sub_forty", "to_roman(40) -> XL", expect_to("sub40", 40, "XL"))
    check("to_sub_ninety", "to_roman(90) -> XC", expect_to("sub90", 90, "XC"))
    check("to_sub_four_hundred", "to_roman(400) -> CD", expect_to("sub400", 400, "CD"))
    check("to_sub_nine_hundred", "to_roman(900) -> CM", expect_to("sub900", 900, "CM"))
    check("to_sub_composite", "to_roman(1994) -> MCMXCIV (many subtractive pairs)",
          expect_to("sub1994", 1994, "MCMXCIV"))
    check("to_sub_2949", "to_roman(2949) -> MMCMXLIX", expect_to("sub2949", 2949, "MMCMXLIX"))

    # --- BUG 2: from_roman subtractive parsing -------------------------------
    check("from_sub_four", "from_roman('IV') -> 4 (not 6)", expect_from("sub4", "IV", 4))
    check("from_sub_nine", "from_roman('IX') -> 9", expect_from("sub9", "IX", 9))
    check("from_sub_forty_ninety", "from_roman('XL'),('XC') -> 40,90",
          lambda: ((from_roman("XL"), from_roman("XC")) == (40, 90),
                   f"got {(from_roman('XL'), from_roman('XC'))!r}"))
    check("from_sub_cd_cm", "from_roman('CD'),('CM') -> 400,900",
          lambda: ((from_roman("CD"), from_roman("CM")) == (400, 900),
                   f"got {(from_roman('CD'), from_roman('CM'))!r}"))
    check("from_sub_composite", "from_roman('MCMXCIV') -> 1994",
          expect_from("sub1994", "MCMXCIV", 1994))
    check("from_case_insensitive", "from_roman lowercases input ('mcmxciv' -> 1994)",
          expect_from("lower", "mcmxciv", 1994))

    # --- round trip: BUG 1 and BUG 2 must BOTH be fixed ----------------------
    def c_round_trip():
        bad = []
        for n in range(1, 4000):
            want = ref_to_roman(n)
            enc = to_roman(n)
            if enc != want:
                bad.append((n, enc, want))
                if len(bad) >= 3:
                    break
            dec = from_roman(enc)
            if dec != n:
                bad.append((n, enc, dec))
                if len(bad) >= 3:
                    break
        return (not bad), ("all 1..3999 round-trip" if not bad else f"failures: {bad}")

    check("round_trip_full_range", "from_roman(to_roman(n)) == n for all 1..3999", c_round_trip)

    # --- BUG 3: out-of-range raises -----------------------------------------
    def c_to_zero():
        try:
            to_roman(0)
        except RomanError:
            return True, "raised RomanError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected RomanError"
        return False, "did not raise"

    check("to_zero_raises", "to_roman(0) raises RomanError", c_to_zero)

    def c_to_negative():
        try:
            to_roman(-5)
        except RomanError:
            return True, "raised RomanError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected RomanError"
        return False, "did not raise"

    check("to_negative_raises", "to_roman(-5) raises RomanError", c_to_negative)

    def c_to_too_big():
        try:
            to_roman(4000)
        except RomanError:
            return True, "raised RomanError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected RomanError"
        return False, "did not raise"

    check("to_too_big_raises", "to_roman(4000) raises RomanError", c_to_too_big)

    def c_to_way_too_big():
        try:
            to_roman(10000)
        except RomanError:
            return True, "raised RomanError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected RomanError"
        return False, "did not raise"

    check("to_way_too_big_raises", "to_roman(10000) raises RomanError", c_to_way_too_big)

    # --- boundary values inside the range still encode ----------------------
    check("to_boundary_one", "to_roman(1) -> I (lower boundary stays valid)",
          expect_to("one", 1, "I"))
    check("to_boundary_3999", "to_roman(3999) -> MMMCMXCIX (upper boundary stays valid)",
          expect_to("max", 3999, "MMMCMXCIX"))

    # --- from_roman validation ----------------------------------------------
    def c_from_bad_symbol():
        try:
            from_roman("IZ")
        except RomanError:
            return True, "raised RomanError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected RomanError"
        return False, "did not raise"

    check("from_bad_symbol_raises", "from_roman('IZ') raises RomanError", c_from_bad_symbol)

    def c_from_empty():
        try:
            from_roman("")
        except RomanError:
            return True, "raised RomanError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected RomanError"
        return False, "did not raise"

    check("from_empty_raises", "from_roman('') raises RomanError", c_from_empty)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 27

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "romanio",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
