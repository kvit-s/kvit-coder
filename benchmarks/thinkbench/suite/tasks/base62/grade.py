#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_base62`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced `base62`
package against the BRIEF'S CONTRACT (a base-62 integer codec over the fixed
alphabet ``0-9A-Za-z`` whose ``encode(n) -> str`` / ``decode(s) -> int`` are
big-endian and round-trip for every non-negative ``n``), NOT against any
particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- ``encode(0)`` returns the empty string ``""`` (the ``while n > 0``
    loop never runs and there is no zero special-case) instead of ``"0"``.
  * BUG 2 -- digits are emitted LEAST-significant first and never reversed, so
    the output is little-endian: ``encode(62)`` yields ``"01"`` not ``"10"``.
    Single-digit values look fine, so coarse small-value spot checks pass.
  * BUG 3 -- ``decode`` looks characters up with ``str.find``, which returns
    ``-1`` for a character outside the alphabet instead of raising, silently
    producing a garbage (often negative) result.
  * BUG 4 -- ``decode("")`` returns ``0`` (the loop body never runs) instead of
    rejecting the empty string.

Because BUG 2 leaves single-digit encodings correct and the two encode bugs are
the ones that visibly break round-trips, a NAIVE fixer typically repairs digit
order + the zero case (making every encode/round-trip check pass) yet leaves
BOTH decode-validation gaps, landing well under 1.0. Only a careful fix that
also validates decode input reaches a perfect score.

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

# The grader computes every expected value ITSELF from this alphabet -- it does
# not trust (or import) the agent's constants.
ALPHABET = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
BASE = 62


def ref_encode(n):
    """Independent reference encoder used only to derive expected outputs."""
    if n == 0:
        return ALPHABET[0]
    out = []
    while n > 0:
        n, r = divmod(n, BASE)
        out.append(ALPHABET[r])
    return "".join(reversed(out))


def ref_decode(s):
    """Independent reference decoder used only to derive expected outputs."""
    n = 0
    for ch in s:
        n = n * BASE + ALPHABET.index(ch)
    return n


# --- fixed denominator: the full roster of checks, declared before any run ----
# (id, human description). Kept in lockstep with the checks registered below so
# that an import failure can still report a result line for every single check.
CHECK_SPECS = [
    ("alphabet_single_digits", "every single alphabet char encodes/decodes to its index value"),
    ("encode_zero", "encode(0) is the single digit '0', not the empty string"),
    ("decode_zero", "decode('0') is 0"),
    ("encode_small_multidigit", "encode(62) is '10' (big-endian, most-significant first)"),
    ("encode_boundary_values", "values around the base boundary encode big-endian correctly"),
    ("decode_known_value", "decode('10') is 62 and a few fixed strings decode correctly"),
    ("roundtrip_small", "decode(encode(n)) == n for every n in 0..1000"),
    ("roundtrip_powers", "round-trips hold at base powers and their neighbours"),
    ("roundtrip_large", "round-trips hold for large multi-digit integers"),
    ("encode_is_big_endian", "encode output matches an independent big-endian reference"),
    ("decode_leading_zero_ok", "decode tolerates non-canonical leading-zero strings ('0A' -> 10)"),
    ("no_empty_output", "no non-negative integer encodes to the empty string"),
    ("decode_rejects_unknown", "decode raises on a character outside the alphabet"),
    ("decode_rejects_unknown_midstring", "decode raises on a bad char even after valid digits"),
    ("decode_rejects_empty", "decode raises on the empty string (not silently 0)"),
    ("decode_no_silent_negative", "decode never yields a negative result for any string"),
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


def raises(fn):
    """True iff calling fn() raises any exception."""
    try:
        fn()
    except Exception:
        return True
    return False


# --- import the produced package (contract: base62.public, fallback pkg) ------
import_ok = True
import_detail = ""
encode = None
decode = None
try:
    try:
        mod = importlib.import_module("base62.public")
    except Exception:
        mod = importlib.import_module("base62")
    encode = getattr(mod, "encode")
    decode = getattr(mod, "decode")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. each single alphabet character round-trips to its own index value.
    def c_alphabet_single_digits():
        bad = []
        for i, ch in enumerate(ALPHABET):
            if encode(i) != ch or decode(ch) != i:
                bad.append((i, ch, encode(i), decode(ch)))
        return (not bad), f"mismatches={bad[:5]} ({len(bad)} total)"

    check("alphabet_single_digits", c_alphabet_single_digits)

    # 2. BUG 1: encode(0) must be the single zero digit, never "".
    def c_encode_zero():
        got = encode(0)
        return got == ALPHABET[0], f"encode(0)={got!r} (expected {ALPHABET[0]!r})"

    check("encode_zero", c_encode_zero)

    # 3. decode of the zero digit is 0.
    def c_decode_zero():
        got = decode(ALPHABET[0])
        return got == 0, f"decode('0')={got!r} (expected 0)"

    check("decode_zero", c_decode_zero)

    # 4. BUG 2: 62 must encode big-endian to '10' (peeled rem=0 then 1, MSB first).
    def c_encode_small_multidigit():
        got = encode(62)
        return got == "10", f"encode(62)={got!r} (expected '10')"

    check("encode_small_multidigit", c_encode_small_multidigit)

    # 5. BUG 2 sharper: a spread of two-digit values across the boundary.
    def c_encode_boundary_values():
        cases = [61, 62, 63, 123, 3843, 3844, 3845]  # z, 10, 11, ..., zz, 100, 101
        bad = [(n, encode(n), ref_encode(n)) for n in cases if encode(n) != ref_encode(n)]
        return (not bad), f"mismatches={bad}"

    check("encode_boundary_values", c_encode_boundary_values)

    # 6. decode of a few fixed known strings (computed by the reference decoder).
    def c_decode_known_value():
        cases = ["10", "zz", "A", "100", "ZZ", "aa"]
        bad = [(s, decode(s), ref_decode(s)) for s in cases if decode(s) != ref_decode(s)]
        return (not bad), f"mismatches={bad}"

    check("decode_known_value", c_decode_known_value)

    # 7. round-trip for every small n (catches order + zero together).
    def c_roundtrip_small():
        bad = [n for n in range(0, 1001) if decode(encode(n)) != n]
        return (not bad), f"{len(bad)} failures, first few={bad[:5]}"

    check("roundtrip_small", c_roundtrip_small)

    # 8. round-trip exactly at base powers and their +/-1 neighbours.
    def c_roundtrip_powers():
        ns = []
        for k in range(1, 8):
            p = BASE ** k
            ns += [p - 1, p, p + 1]
        bad = [n for n in ns if decode(encode(n)) != n]
        return (not bad), f"{len(bad)} failures, first few={bad[:5]}"

    check("roundtrip_powers", c_roundtrip_powers)

    # 9. round-trip for large integers (multi-digit, well past 64 bits).
    def c_roundtrip_large():
        ns = [123456789, 2 ** 32, 2 ** 53, 2 ** 64, 10 ** 20, 62 ** 10 + 12345]
        bad = [(n, encode(n), decode(encode(n))) for n in ns if decode(encode(n)) != n]
        return (not bad), f"mismatches={[(n, e) for n, e, _ in bad]}"

    check("roundtrip_large", c_roundtrip_large)

    # 10. encode output must match an independent big-endian reference exactly.
    def c_encode_is_big_endian():
        ns = [0, 1, 10, 35, 36, 61, 62, 999, 12345, 998877, 62 ** 5]
        bad = [(n, encode(n), ref_encode(n)) for n in ns if encode(n) != ref_encode(n)]
        return (not bad), f"mismatches={bad}"

    check("encode_is_big_endian", c_encode_is_big_endian)

    # 11. decode must tolerate non-canonical leading-zero strings (value-preserving).
    #     '0A' is 0*62 + 10 == 10; a fix that wrongly REJECTS leading zeros fails here.
    def c_decode_leading_zero_ok():
        cases = {"0A": 10, "00": 0, "0z": 61, "010": 62, "0000A": 10}
        bad = [(s, decode(s), v) for s, v in cases.items() if decode(s) != v]
        return (not bad), f"mismatches={bad}"

    check("decode_leading_zero_ok", c_decode_leading_zero_ok)

    # 12. BUG 1 corollary: NO non-negative integer may encode to the empty string.
    def c_no_empty_output():
        ns = list(range(0, 200)) + [62, 3844, 238328, 10 ** 12]
        bad = [n for n in ns if encode(n) == ""]
        return (not bad), f"empty-output for n in {bad[:5]} ({len(bad)} total)"

    check("no_empty_output", c_no_empty_output)

    # 13. BUG 3: a character outside the alphabet must RAISE, not return garbage.
    def c_decode_rejects_unknown():
        bads = ["!", "*", " ", "-", "+", "/", "\n"]
        survived = [s for s in bads if not raises(lambda s=s: decode(s))]
        return (not survived), f"did NOT raise for {survived}"

    check("decode_rejects_unknown", c_decode_rejects_unknown)

    # 14. BUG 3 sharper: a bad char AFTER valid digits must still raise.
    def c_decode_rejects_unknown_midstring():
        bads = ["1!", "A B", "zz#", "10-1"]
        survived = [s for s in bads if not raises(lambda s=s: decode(s))]
        return (not survived), f"did NOT raise for {survived}"

    check("decode_rejects_unknown_midstring", c_decode_rejects_unknown_midstring)

    # 15. BUG 4: the empty string must RAISE, not silently decode to 0.
    def c_decode_rejects_empty():
        return raises(lambda: decode("")), "decode('') did not raise"

    check("decode_rejects_empty", c_decode_rejects_empty)

    # 16. decode must never yield a NEGATIVE result for any input it accepts.
    #     (The buggy str.find(-1) path produces negatives; valid input never does.)
    def c_decode_no_silent_negative():
        # Valid strings: result must be >= 0. Invalid strings: must raise (not a
        # negative number). Either way, a negative return value is a failure.
        probes = ["0", "z", "10", "zzzz", "0A", "Az9", "!", "-1", "a b", "%%"]
        for s in probes:
            try:
                got = decode(s)
            except Exception:
                continue  # raising is acceptable for invalid input
            if got < 0:
                return False, f"decode({s!r}) returned negative {got!r}"
        return True, "no negative results"

    check("decode_no_silent_negative", c_decode_no_silent_negative)


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
    "task": "fix_base62",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
