#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `decimalfmt`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``decimalfmt`` package against the BRIEF'S CONTRACT (the ``decimalfmt.public``
``format_amount`` / ``parse_amount`` API), NOT against any particular internal
file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE sign / grouping / padding / places / round-trip edge cases, all with
expected values computed HERE (never read from the agent's tests). The FIXED
reference passes every check; the planted-bug starter fails a discriminating
subset, so a partial fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``decimalfmt.public``:
  1. sign placement — the minus is glued onto the fractional part (after the
     decimal separator) instead of prefixing the whole number;
  2. fractional zero-pad — ``str(frac)`` drops the leading zero, so 5 cents
     renders ``.5`` instead of ``.05`` (and a whole dollar renders ``.0``);
  3. thousands grouping direction — the integer part is chunked from the LEFT
     instead of the right, so ``1234567`` whole-units -> ``"123,456,7"`` not
     ``"1,234,567"``. A fourth, coupled defect: ``parse_amount`` does not strip
     the grouping separators, so even a correctly grouped string won't round-trip
     until both the formatter and the parser are fixed.

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
# Contract path is ``decimalfmt.public``; fall back to the package root
# ``decimalfmt`` so a submission that re-exports the API from ``__init__`` (but
# moved it off ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
format_amount = None
parse_amount = None
MoneyError = None
try:
    try:
        mod = importlib.import_module("decimalfmt.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("decimalfmt")
    format_amount = getattr(mod, "format_amount")
    parse_amount = getattr(mod, "parse_amount")
    MoneyError = getattr(mod, "MoneyError", None)
    if not (isinstance(MoneyError, type) and issubclass(MoneyError, BaseException)):
        MoneyError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
def ref_format(cents, places=2, sep=","):
    """Reference rendering, independent of the submission under test."""
    neg = cents < 0
    n = -cents if neg else cents
    scale = 10 ** places
    whole = n // scale
    frac = n % scale
    s = str(whole)
    parts = []
    while len(s) > 3:
        parts.append(s[-3:])
        s = s[:-3]
    parts.append(s)
    grouped = sep.join(reversed(parts))
    if places > 0:
        body = grouped + "." + str(frac).rjust(places, "0")
    else:
        body = grouped
    return ("-" + body) if neg else body


def fmt_eq(label, cents, places=2, sep=","):
    """Check ``format_amount`` matches the reference rendering."""

    def _fn():
        want = ref_format(cents, places, sep)
        got = format_amount(cents, places, sep) if (places != 2 or sep != ",") \
            else format_amount(cents)
        return (got == want), f"{label}: got {got!r}, expected {want!r}"

    return _fn


def round_trips(label, cents, places=2, sep=","):
    """Check ``parse_amount(format_amount(cents)) == cents``."""

    def _fn():
        if places != 2 or sep != ",":
            s = format_amount(cents, places, sep)
            back = parse_amount(s, places, sep)
        else:
            s = format_amount(cents)
            back = parse_amount(s)
        return (back == cents), f"{label}: format->{s!r}->parse {back!r}, expected {cents!r}"

    return _fn


def parses_to(label, s, want, places=2, sep=","):
    """Check ``parse_amount(s)`` returns ``want`` cents."""

    def _fn():
        got = parse_amount(s, places, sep) if (places != 2 or sep != ",") \
            else parse_amount(s)
        return (got == want), f"{label}: parse({s!r})={got!r}, expected {want!r}"

    return _fn


if import_ok:
    # --- baseline: small positive amounts (pass even buggy; guard regressions) -
    check("fmt_small_positive", "small positive amount renders correctly",
          fmt_eq("12345", 12345))
    check("fmt_under_thousand", "amount under $10 with two-digit frac is fine",
          fmt_eq("99999", 99999))
    check("fmt_zero", "zero renders as 0.00 with no sign", fmt_eq("0", 0))

    # --- BUG 1: sign placement -----------------------------------------------
    check("fmt_negative_lead_sign", "negative sign sits in front of the number",
          fmt_eq("-1234567", -1234567))
    check("fmt_small_negative", "small negative keeps a leading sign",
          fmt_eq("-5", -5))
    check("fmt_negative_round_thousands", "negative grouped amount, sign in front",
          fmt_eq("-1000000", -1000000))

    # --- BUG 2: fractional zero-padding --------------------------------------
    check("fmt_pad_single_cent", "5 cents -> .05 (leading zero kept)", fmt_eq("5", 5))
    check("fmt_pad_whole_dollar", "whole dollar -> .00", fmt_eq("100", 100))
    check("fmt_pad_nine_cents", "9 cents -> .09", fmt_eq("9", 9))
    check("fmt_no_pad_needed", "two-digit frac is unchanged", fmt_eq("99", 99))

    # --- BUG 3: grouping from the right --------------------------------------
    check("fmt_group_thousands", "four integer digits -> one separator",
          fmt_eq("1234567", 1234567))
    check("fmt_group_millions", "millions grouped from the right",
          fmt_eq("100000000", 100000000))
    check("fmt_group_ten_million", "an 8-digit integer part groups correctly",
          fmt_eq("1234567890", 1234567890))
    check("fmt_no_group_three_digits", "exactly three integer digits -> no separator",
          fmt_eq("99999", 99999))

    # --- places / sep variants -----------------------------------------------
    check("fmt_places0", "places=0 drops the fractional part",
          fmt_eq("places0", -1234500, places=0))
    check("fmt_places3", "places=3 keeps three padded fractional digits",
          fmt_eq("places3", 1234007, places=3))
    check("fmt_custom_sep", "a custom separator is honoured",
          fmt_eq("uspace", 1234567, sep=" "))

    # --- round trips (parse strips separators; sign + padding survive) -------
    check("rt_grouped_positive", "grouped positive round-trips",
          round_trips("1234567", 1234567))
    check("rt_grouped_negative", "grouped negative round-trips",
          round_trips("-1234567", -1234567))
    check("rt_single_cent", "single-cent amount round-trips", round_trips("5", 5))
    check("rt_zero", "zero round-trips", round_trips("0", 0))
    check("rt_places3", "places=3 round-trips", round_trips("p3", 1234007, places=3))

    check("parse_grouped_literal", "parse strips separators from a literal string",
          parses_to("12,345.67", "12,345.67", 1234567))
    check("parse_negative_literal", "parse honours a leading minus",
          parses_to("-1,000,000.05", "-1,000,000.05", -100000005))

    # --- the interaction (needs sign + grouping + padding + parse all fixed) -
    check("interaction_neg_group_pad_rt",
          "negative + grouping + single-cent pad + round-trip together",
          round_trips("interaction", -100000005))

    def c_interaction_exact():
        s = format_amount(-100000005)
        want = "-1,000,000.05"
        back = parse_amount(s) if s == want else None
        return (s == want and back == -100000005), \
            f"format->{s!r} (want {want!r}), parse->{back!r}"

    check("interaction_exact_string",
          "the interaction renders the exact contract string and parses back",
          c_interaction_exact)

    # --- validation ----------------------------------------------------------
    def c_bad_cents():
        try:
            format_amount("100")
        except MoneyError:
            return True, "raised MoneyError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected MoneyError"
        return False, "did not raise"

    check("bad_cents_raises", "non-int cents raises MoneyError", c_bad_cents)

    def c_bad_places():
        try:
            format_amount(100, places=-1)
        except MoneyError:
            return True, "raised MoneyError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected MoneyError"
        return False, "did not raise"

    check("bad_places_raises", "negative places raises MoneyError", c_bad_places)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 28

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "decimalfmt",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
