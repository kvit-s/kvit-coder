#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_luhn`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced `luhn`
package against the BRIEF'S CONTRACT (the Luhn mod-10 checksum: double every
second digit FROM THE RIGHT, fold doubled values over 9 by subtracting 9, sum,
valid iff the total is a multiple of 10; `is_valid` ignores spaces and rejects
empty/non-digit input; `check_digit` returns the 0..9 digit that, appended,
validates -- 0, never 10, when the total is already a multiple of 10), NOT
against any particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- it doubles from the WRONG END (every second digit counting from the
    LEFT instead of the right), so validity flips with the number's length
    parity: odd-length numbers double the right set, even-length ones the wrong
    set. Hand-picked even-length examples can still look correct.
  * BUG 2 -- it folds a doubled value over 9 by subtracting 10 instead of 9
    (e.g. 8 -> 16 -> 6 instead of 7), so any number whose checksum involves a
    doubled digit >= 5 is mis-scored.
  * BUG 3 -- `check_digit` returns `10 - total % 10` without the final `% 10`,
    so when the correct check digit is 0 it returns 10.
  * BUG 4 -- `is_valid` never strips spaces and never guards empty / non-digit
    input: spaced (grouped) numbers are rejected, "" scores as valid (sum 0),
    and non-digit input raises instead of returning False.

The textbook valid number and an obviously-wrong one still validate/reject the
way a couple of hand checks expect, so a superficial fix can pass the easy
checks while still failing the edge cases.

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


# --- independent reference: the grader computes every expected value itself ---
def ref_sum(digits):
    """Correct Luhn total: double odd positions FROM THE RIGHT, fold by -9."""
    total = 0
    for pos, ch in enumerate(reversed(digits)):
        d = ord(ch) - 48
        if pos % 2 == 1:
            d *= 2
            if d > 9:
                d -= 9
        total += d
    return total


def ref_is_valid(number):
    s = number.replace(" ", "")
    if not s or not s.isdigit():
        return False
    return ref_sum(s) % 10 == 0


def ref_check_digit(partial):
    s = partial.replace(" ", "")
    if not s.isdigit():
        s = "" if s == "" else s  # only digit/empty partials are used below
    total = ref_sum((s if s.isdigit() else "") + "0")
    return (10 - total % 10) % 10


# --- fixed denominator: the full roster of checks, declared before any run -----
CHECK_SPECS = [
    ("textbook_valid", "the classic 11-digit Luhn number validates"),
    ("textbook_invalid", "a number with one wrong digit is rejected"),
    ("valid_even_length", "a known-valid EVEN-length (16-digit) card validates"),
    ("valid_odd_length", "a known-valid ODD-length number validates"),
    ("parity_invalid_even", "a one-digit-off even-length number is rejected"),
    ("parity_invalid_odd", "a one-digit-off odd-length number is rejected"),
    ("parity_nofold_valid", "valid numbers with NO doubled-digit>9 validate (isolates the doubling end)"),
    ("parity_nofold_invalid", "a non-doubled-digit error in a no-fold number is rejected"),
    ("fold_over_nine", "a number exercising doubled digits >9 scores correctly"),
    ("fold_distinguishes", "-9 folding (not -10) is what makes the valid one valid"),
    ("spaces_ignored", "a spaced/grouped number validates like its compact form"),
    ("spaces_invalid", "a spaced number with a bad digit is still rejected"),
    ("empty_invalid", "the empty string is not valid"),
    ("nondigit_invalid", "non-digit input returns False (does not raise)"),
    ("single_zero_valid", "the single digit '0' is valid (total 0)"),
    ("check_digit_basic", "check_digit on a partial returns the validating digit"),
    ("check_digit_zero", "check_digit returns 0 (not 10) when the total is a multiple of 10"),
    ("check_digit_roundtrip", "appending check_digit makes every sampled partial valid"),
    ("validity_sweep", "is_valid matches the reference across mixed-length samples"),
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


# --- import the produced package (contract: luhn.public, fallback pkg) ---------
import_ok = True
import_detail = ""
is_valid = None
check_digit = None
try:
    try:
        mod = importlib.import_module("luhn.public")
    except Exception:
        mod = importlib.import_module("luhn")
    is_valid = getattr(mod, "is_valid")
    check_digit = getattr(mod, "check_digit")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# A spread of known-valid numbers (verified by the reference at module load) of
# both length parities, used across several checks.
VALID_NUMBERS = [
    "79927398713",        # classic 11-digit example (odd length)
    "4539148803436467",   # Visa test number (16, even length)
    "4111111111111111",   # Visa (16, even length)
    "5500005555555559",   # MasterCard test (16, even)
    "6011000990139424",   # Discover test (16, even)
    "371449635398431",    # Amex test (15, odd length)
    "0",                  # single zero (odd length 1)
    "18",                 # tiny even-length valid (1 doubled ->2, +8 =10)
]

if import_ok:
    # 1. textbook valid number (odd length) validates.
    def c_textbook_valid():
        got = is_valid("79927398713")
        return got is True, f"is_valid('79927398713') -> {got!r} (expected True)"

    check("textbook_valid", c_textbook_valid)

    # 2. one wrong digit -> rejected.
    def c_textbook_invalid():
        got = is_valid("79927398710")
        return got is False, f"is_valid('79927398710') -> {got!r} (expected False)"

    check("textbook_invalid", c_textbook_invalid)

    # 3. a known-valid EVEN-length card validates (BUG 1 trips here when the
    #    doubling is measured from the wrong end).
    def c_valid_even_length():
        n = "4539148803436467"
        got = is_valid(n)
        return got is True, f"is_valid({n!r}) -> {got!r} (expected True, ref={ref_is_valid(n)})"

    check("valid_even_length", c_valid_even_length)

    # 4. a known-valid ODD-length number validates.
    def c_valid_odd_length():
        n = "371449635398431"  # Amex, 15 digits
        got = is_valid(n)
        return got is True, f"is_valid({n!r}) -> {got!r} (expected True, ref={ref_is_valid(n)})"

    check("valid_odd_length", c_valid_odd_length)

    # 5. BUG 1: a single-digit error in an EVEN-length number must be rejected.
    def c_parity_invalid_even():
        n = "4539148803436460"  # last digit altered from the valid ...467
        got = is_valid(n)
        return (got is False and ref_is_valid(n) is False), \
            f"is_valid({n!r}) -> {got!r} (expected False, ref={ref_is_valid(n)})"

    check("parity_invalid_even", c_parity_invalid_even)

    # 6. BUG 1: a single-digit error in an ODD-length number must be rejected.
    def c_parity_invalid_odd():
        n = "79927398714"  # classic example with last digit bumped
        got = is_valid(n)
        return (got is False and ref_is_valid(n) is False), \
            f"is_valid({n!r}) -> {got!r} (expected False, ref={ref_is_valid(n)})"

    check("parity_invalid_odd", c_parity_invalid_odd)

    # 6b. BUG 1 in ISOLATION: valid numbers chosen so NO doubled digit exceeds 9
    #     (every doubled position holds 0..4). The fold (BUG 2) is irrelevant
    #     here, so getting the doubling END right is sufficient and necessary --
    #     these pass once the doubling parity is fixed, even if folding is still
    #     wrong. Covers both length parities.
    def c_parity_nofold_valid():
        nums = ["1107", "04341", "081414", "2461416"]  # even & odd lengths, no fold
        bad = [(n, is_valid(n)) for n in nums if is_valid(n) is not True or ref_is_valid(n) is not True]
        return (not bad), f"should all be valid; offenders {bad}" if bad else "all no-fold valids pass"

    check("parity_nofold_valid", c_parity_nofold_valid)

    # 6c. BUG 1 isolation, negative side: bump a NON-doubled (right-position-even)
    #     digit so the only thing that can catch it is correct doubling, not the
    #     fold. The altered numbers must be rejected.
    def c_parity_nofold_invalid():
        # '1107'->'1108' (rightmost, undoubled): 1108 -> 8+0+2+1=11, invalid.
        # '2461416'->'2461417' (rightmost): invalid.
        nums = ["1108", "2461417"]
        bad = [(n, is_valid(n)) for n in nums if is_valid(n) is not False or ref_is_valid(n) is not False]
        return (not bad), f"should all be invalid; offenders {bad}" if bad else "all no-fold perturbations rejected"

    check("parity_nofold_invalid", c_parity_nofold_invalid)

    # 7. BUG 2: numbers whose checksum involves doubled digits > 9 must score
    #    right. Compare is_valid against the reference on several such numbers.
    def c_fold_over_nine():
        # these all contain digits 5..9 in doubled positions -> exercise folding
        cases = ["6011000990139424", "5500005555555559", "371449635398431", "18"]
        bad = [(n, is_valid(n), ref_is_valid(n)) for n in cases
               if is_valid(n) != ref_is_valid(n)]
        return (not bad), f"mismatches (got != ref): {bad}" if bad else "all fold cases match ref"

    check("fold_over_nine", c_fold_over_nine)

    # 8. BUG 2 sharper: a number that is valid under -9 folding but NOT under -10
    #    folding. "91" -> from right: 1 (undoubled) + 9*2=18->9 = 10, valid.
    #    Under the buggy -10 fold, 18->8, total 9, invalid. So is_valid('91')
    #    must be True, and the off-by-one neighbour '90'/'92' must be False.
    def c_fold_distinguishes():
        assert ref_is_valid("91") is True and ref_sum("91") % 10 == 0
        v = is_valid("91")
        n1 = is_valid("90")
        n2 = is_valid("92")
        return (v is True and n1 is False and n2 is False), \
            f"is_valid('91')->{v!r} (expected True); '90'->{n1!r} '92'->{n2!r} (both False)"

    check("fold_distinguishes", c_fold_distinguishes)

    # 9. BUG 4: spaces are stripped before validating.
    def c_spaces_ignored():
        spaced = is_valid("4539 1488 0343 6467")
        compact = is_valid("4539148803436467")
        return (spaced is True and compact is True), \
            f"spaced->{spaced!r} compact->{compact!r} (both expected True)"

    check("spaces_ignored", c_spaces_ignored)

    # 10. spaces don't paper over a real error.
    def c_spaces_invalid():
        got = is_valid("4539 1488 0343 6460")  # last group digit wrong
        return got is False, f"is_valid(spaced-bad) -> {got!r} (expected False)"

    check("spaces_invalid", c_spaces_invalid)

    # 11. BUG 4: empty string is not valid (buggy code sums to 0 -> True).
    def c_empty_invalid():
        got = is_valid("")
        return got is False, f"is_valid('') -> {got!r} (expected False)"

    check("empty_invalid", c_empty_invalid)

    # 12. BUG 4: non-digit input returns False, never raises.
    def c_nondigit_invalid():
        a = is_valid("12 34a")
        b = is_valid("4539-1488")
        c = is_valid("abc")
        return (a is False and b is False and c is False), \
            f"'12 34a'->{a!r} '4539-1488'->{b!r} 'abc'->{c!r} (all expected False, no raise)"

    check("nondigit_invalid", c_nondigit_invalid)

    # 13. the single digit '0' is valid (total 0, a multiple of 10).
    def c_single_zero_valid():
        got = is_valid("0")
        return got is True, f"is_valid('0') -> {got!r} (expected True)"

    check("single_zero_valid", c_single_zero_valid)

    # 14. check_digit on a known partial returns the validating digit.
    def c_check_digit_basic():
        got = check_digit("7992739871")
        exp = ref_check_digit("7992739871")  # 3
        return got == exp, f"check_digit('7992739871') -> {got!r} (expected {exp})"

    check("check_digit_basic", c_check_digit_basic)

    # 15. BUG 3: check_digit must return 0 (NOT 10) when the total is already a
    #     multiple of 10. '123456781234567' is such a partial.
    def c_check_digit_zero():
        partial = "123456781234567"
        exp = ref_check_digit(partial)  # 0
        assert exp == 0, "fixture sanity: expected a 0 check digit"
        got = check_digit(partial)
        return got == 0, f"check_digit({partial!r}) -> {got!r} (expected 0, buggy yields 10)"

    check("check_digit_zero", c_check_digit_zero)

    # 16. round-trip: appending check_digit makes each sampled partial valid,
    #     including a partial whose check digit is 0. Uses the produced is_valid.
    def c_check_digit_roundtrip():
        partials = [
            "7992739871",        # cd 3
            "123456781234567",   # cd 0  (the BUG 3 case)
            "453914880343646",   # cd 7
            "411111111111111",   # cd 1
            "37144963539843",    # cd 1  (Amex, odd-length completion)
        ]
        bad = []
        for p in partials:
            cd = check_digit(p)
            full = p + str(cd)
            if not (isinstance(cd, int) and 0 <= cd <= 9 and ref_is_valid(full)):
                bad.append((p, cd, ref_is_valid(full) if isinstance(cd, int) else None))
        return (not bad), f"non-validating completions (partial, cd, ref_valid): {bad}" if bad \
            else "every partial+check_digit validates"

    check("check_digit_roundtrip", c_check_digit_roundtrip)

    # 17. broad sweep: is_valid must agree with the reference on a mixed batch of
    #     valid numbers AND single-digit perturbations of them, across lengths.
    def c_validity_sweep():
        samples = list(VALID_NUMBERS)
        # add a perturbed (last-digit +1 mod 10) variant of each to get invalids
        for n in VALID_NUMBERS:
            last = (int(n[-1]) + 1) % 10
            samples.append(n[:-1] + str(last))
        mism = [(s, is_valid(s), ref_is_valid(s)) for s in samples
                if is_valid(s) != ref_is_valid(s)]
        return (not mism), (f"{len(mism)} mismatch(es) vs ref, e.g. {mism[:4]}" if mism
                            else f"all {len(samples)} samples match the reference")

    check("validity_sweep", c_validity_sweep)


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
    "task": "fix_luhn",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
