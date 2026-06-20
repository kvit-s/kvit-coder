#!/usr/bin/env python3
"""Held-out behavior-level oracle for bug-fix task `fix_semvercmp`.

Dropped into the workspace ONLY after the agent stops -- the agent never sees
it, and it never imports the agent's own tests. It grades the produced
`semvercmp` package against the BRIEF'S CONTRACT (a SemVer 2.0 precedence
comparator whose `compare(a, b) -> -1 | 0 | 1` orders `major.minor.patch`
numerically, ranks a pre-release BELOW its release, compares pre-release
identifiers per SemVer rules, breaks ties by identifier count, and IGNORES
build metadata), NOT against any particular internal file layout.

The shipped (buggy) code has several SUBTLE defects:

  * BUG 1 -- a version WITH a pre-release is treated as EQUAL to the same
    version WITHOUT one (`1.0.0-rc.1` == `1.0.0`) instead of ranking below it.
  * BUG 2 -- pre-release identifiers are compared as plain text, so numeric
    identifiers do NOT compare numerically (`"2" > "10"`) and the
    numeric-ranks-below-alphanumeric rule is not enforced.
  * BUG 3 -- when one pre-release is a leading run of the other the comparison
    stops at the shorter list and returns 0, so the "more identifiers wins"
    tiebreak (`1.0.0-alpha` < `1.0.0-alpha.1`) is lost.
  * BUG 4 -- build metadata is folded into the comparison instead of being
    stripped, so `1.0.0+build.1` != `1.0.0+build.2` and a build tag can change
    the result.

Plain `x.y.z` ordering and two word-only pre-releases still compare correctly,
so a superficial fix can pass the easy checks while still failing the edges.

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
    ("core_numeric_order", "major/minor/patch compare numerically, most-significant first"),
    ("core_multidigit", "multi-digit fields compare as numbers, not as text (10 > 2)"),
    ("core_equal", "two identical plain versions are equal"),
    ("antisymmetry", "swapping the arguments negates the result for plain versions"),
    ("pre_word_order", "two word-only pre-releases order by ASCII (alpha < beta)"),
    ("pre_below_release", "a pre-release ranks BELOW its own release (1.0.0-alpha < 1.0.0)"),
    ("release_above_pre", "the release ranks ABOVE the pre-release (mirror, 1.0.0 > 1.0.0-alpha)"),
    ("pre_below_release_rc", "a late-stage pre-release still ranks below the release (rc.1 < 1.0.0)"),
    ("numeric_ident_value", "numeric identifiers compare by value, not text (2 < 10)"),
    ("numeric_ident_value_big", "multi-digit numeric identifiers compare by value (9 < 100)"),
    ("numeric_below_alpha", "a numeric identifier ranks below an alphanumeric one (1 < alpha)"),
    ("alpha_above_numeric", "an alphanumeric identifier ranks above a numeric one (mirror)"),
    ("more_identifiers_wins", "with shared identifiers equal, more identifiers wins (alpha < alpha.1)"),
    ("more_identifiers_mirror", "the longer pre-release ranks higher (mirror of the tiebreak)"),
    ("build_ignored_equal", "build metadata is ignored: +build.1 and +build.2 are equal"),
    ("build_ignored_vs_plain", "build metadata is ignored vs a plain version (1.0.0+x == 1.0.0)"),
    ("build_ignored_with_pre", "build metadata is ignored alongside a pre-release (-a+x == -a)"),
    ("spec_ordering_chain", "the full SemVer spec precedence chain orders strictly ascending"),
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


# --- import the produced package (contract: semvercmp.public, fallback pkg) ----
import_ok = True
import_detail = ""
compare = None
try:
    try:
        mod = importlib.import_module("semvercmp.public")
    except Exception:
        mod = importlib.import_module("semvercmp")
    compare = getattr(mod, "compare")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def sign(x):
    """Normalise a comparator result to -1 / 0 / 1 (tolerates any signed int)."""
    return (x > 0) - (x < 0)


if import_ok:
    # 1. core numeric ordering across major / minor / patch.
    def c_core_numeric_order():
        cases = [
            ("1.0.0", "2.0.0", -1),
            ("1.2.0", "1.1.0", 1),
            ("1.0.1", "1.0.0", 1),
            ("1.1.0", "1.1.0", 0),
        ]
        bad = [(a, b, sign(compare(a, b)), e) for a, b, e in cases if sign(compare(a, b)) != e]
        return (not bad), f"mismatches={bad}" if bad else "all 4 core orderings correct"

    check("core_numeric_order", c_core_numeric_order)

    # 2. multi-digit fields must compare as NUMBERS (10 > 2, never "10" < "2").
    def c_core_multidigit():
        r1 = sign(compare("1.0.10", "1.0.2"))
        r2 = sign(compare("1.10.0", "1.9.0"))
        return (r1 == 1 and r2 == 1), f"1.0.10 vs 1.0.2 -> {r1} ; 1.10.0 vs 1.9.0 -> {r2} (expect 1,1)"

    check("core_multidigit", c_core_multidigit)

    # 3. identical plain versions are equal.
    def c_core_equal():
        r = sign(compare("1.2.3", "1.2.3"))
        return (r == 0), f"compare(1.2.3,1.2.3) -> {r} (expect 0)"

    check("core_equal", c_core_equal)

    # 4. antisymmetry on plain versions: compare(a,b) == -compare(b,a).
    def c_antisymmetry():
        pairs = [("1.0.0", "1.0.1"), ("2.3.4", "2.3.4"), ("3.0.0", "1.9.9")]
        bad = [(a, b) for a, b in pairs if sign(compare(a, b)) != -sign(compare(b, a))]
        return (not bad), f"non-antisymmetric pairs={bad}" if bad else "antisymmetric on all 3 pairs"

    check("antisymmetry", c_antisymmetry)

    # 5. two word-only pre-releases order lexically (this already works in buggy).
    def c_pre_word_order():
        r1 = sign(compare("1.0.0-alpha", "1.0.0-beta"))
        r2 = sign(compare("1.0.0-beta", "1.0.0-alpha"))
        return (r1 == -1 and r2 == 1), f"alpha vs beta -> {r1} ; beta vs alpha -> {r2} (expect -1,1)"

    check("pre_word_order", c_pre_word_order)

    # 6. BUG 1: a pre-release ranks BELOW its release.
    def c_pre_below_release():
        r = sign(compare("1.0.0-alpha", "1.0.0"))
        return (r == -1), f"compare(1.0.0-alpha, 1.0.0) -> {r} (expect -1)"

    check("pre_below_release", c_pre_below_release)

    # 7. BUG 1 mirror: the release ranks above the pre-release.
    def c_release_above_pre():
        r = sign(compare("1.0.0", "1.0.0-alpha"))
        return (r == 1), f"compare(1.0.0, 1.0.0-alpha) -> {r} (expect 1)"

    check("release_above_pre", c_release_above_pre)

    # 8. BUG 1 again with a multi-identifier pre-release (rc.1 < 1.0.0).
    def c_pre_below_release_rc():
        r = sign(compare("1.0.0-rc.1", "1.0.0"))
        return (r == -1), f"compare(1.0.0-rc.1, 1.0.0) -> {r} (expect -1)"

    check("pre_below_release_rc", c_pre_below_release_rc)

    # 9. BUG 2: numeric identifiers compare by VALUE (2 < 10, not "2" > "10").
    def c_numeric_ident_value():
        r1 = sign(compare("1.0.0-2", "1.0.0-10"))
        r2 = sign(compare("1.0.0-10", "1.0.0-2"))
        return (r1 == -1 and r2 == 1), f"2 vs 10 -> {r1} ; 10 vs 2 -> {r2} (expect -1,1)"

    check("numeric_ident_value", c_numeric_ident_value)

    # 10. BUG 2 sharper: multi-digit numeric identifiers (9 < 100, not "9" > "100").
    def c_numeric_ident_value_big():
        r = sign(compare("1.0.0-alpha.9", "1.0.0-alpha.100"))
        return (r == -1), f"compare(alpha.9, alpha.100) -> {r} (expect -1)"

    check("numeric_ident_value_big", c_numeric_ident_value_big)

    # 11. BUG 2: a numeric identifier ranks BELOW an alphanumeric one. A digit
    #     that is numerically LARGE but textually small still loses to a letter.
    def c_numeric_below_alpha():
        r1 = sign(compare("1.0.0-1", "1.0.0-alpha"))
        r2 = sign(compare("1.0.0-999", "1.0.0-rc"))   # big number still < a word
        return (r1 == -1 and r2 == -1), f"1 vs alpha -> {r1} ; 999 vs rc -> {r2} (expect -1,-1)"

    check("numeric_below_alpha", c_numeric_below_alpha)

    # 12. BUG 2 mirror: an alphanumeric identifier ranks ABOVE a numeric one,
    #     including a later-position numeric identifier (alpha.beta > alpha.7).
    def c_alpha_above_numeric():
        r1 = sign(compare("1.0.0-alpha", "1.0.0-1"))
        r2 = sign(compare("1.0.0-alpha.beta", "1.0.0-alpha.7"))
        return (r1 == 1 and r2 == 1), f"alpha vs 1 -> {r1} ; alpha.beta vs alpha.7 -> {r2} (expect 1,1)"

    check("alpha_above_numeric", c_alpha_above_numeric)

    # 13. BUG 3: with all shared identifiers equal, MORE identifiers wins.
    def c_more_identifiers_wins():
        r = sign(compare("1.0.0-alpha", "1.0.0-alpha.1"))
        return (r == -1), f"compare(1.0.0-alpha, 1.0.0-alpha.1) -> {r} (expect -1)"

    check("more_identifiers_wins", c_more_identifiers_wins)

    # 14. BUG 3 mirror: the longer pre-release ranks higher.
    def c_more_identifiers_mirror():
        r1 = sign(compare("1.0.0-alpha.1", "1.0.0-alpha"))
        r2 = sign(compare("1.0.0-beta.2.3", "1.0.0-beta.2"))
        return (r1 == 1 and r2 == 1), f"alpha.1 vs alpha -> {r1} ; beta.2.3 vs beta.2 -> {r2} (expect 1,1)"

    check("more_identifiers_mirror", c_more_identifiers_mirror)

    # 15. BUG 4: build metadata is ignored -- two differing build tags are equal.
    def c_build_ignored_equal():
        r = sign(compare("1.0.0+build.1", "1.0.0+build.2"))
        return (r == 0), f"compare(1.0.0+build.1, 1.0.0+build.2) -> {r} (expect 0)"

    check("build_ignored_equal", c_build_ignored_equal)

    # 16. BUG 4: a build tag must not change a comparison vs the plain version.
    def c_build_ignored_vs_plain():
        r1 = sign(compare("1.0.0+exp.sha.5114f85", "1.0.0"))
        r2 = sign(compare("1.0.0", "1.0.0+20130313144700"))
        return (r1 == 0 and r2 == 0), f"+meta vs plain -> {r1} ; plain vs +meta -> {r2} (expect 0,0)"

    check("build_ignored_vs_plain", c_build_ignored_vs_plain)

    # 17. BUG 4: build metadata is ignored even alongside a pre-release.
    def c_build_ignored_with_pre():
        r1 = sign(compare("1.0.0-alpha+build.7", "1.0.0-alpha"))
        # And it must not leak so as to flip a pre-vs-release ordering:
        r2 = sign(compare("1.0.0-alpha+build.7", "1.0.0"))
        return (r1 == 0 and r2 == -1), f"-alpha+b vs -alpha -> {r1} ; -alpha+b vs 1.0.0 -> {r2} (expect 0,-1)"

    check("build_ignored_with_pre", c_build_ignored_with_pre)

    # 18. integration: the canonical SemVer 2.0 precedence chain must be strictly
    #     ascending, every adjacent pair comparing -1 (and its mirror +1). This is
    #     the example from the spec; any one of the four bugs breaks it.
    def c_spec_ordering_chain():
        chain = [
            "1.0.0-alpha",
            "1.0.0-alpha.1",
            "1.0.0-alpha.beta",
            "1.0.0-beta",
            "1.0.0-beta.2",
            "1.0.0-beta.11",
            "1.0.0-rc.1",
            "1.0.0",
        ]
        bad = []
        for lo, hi in zip(chain, chain[1:]):
            if sign(compare(lo, hi)) != -1 or sign(compare(hi, lo)) != 1:
                bad.append((lo, hi, sign(compare(lo, hi))))
        return (not bad), f"non-ascending adjacent pairs={bad}" if bad else "full spec chain strictly ascending"

    check("spec_ordering_chain", c_spec_ordering_chain)


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
    "task": "fix_semvercmp",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)
