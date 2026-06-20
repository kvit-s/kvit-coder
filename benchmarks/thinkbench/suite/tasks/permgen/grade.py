#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `permgen`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``permgen`` package against the BRIEF'S CONTRACT (the ``permgen.public``
``nth_permutation`` / ``permutation_rank`` API), NOT against any particular
internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE ranks / list sizes / round-trips, all with expected values computed HERE
(never read from the agent's tests) by an independent factoradic oracle. The
FIXED reference passes every check; the planted-bug starter fails a
discriminating subset, so a partial fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``permgen.public``:
  1. factoradic place value — ``nth_permutation`` uses ``factorial(i)`` where the
     step with ``i`` remaining items needs place value ``factorial(i-1)``, so the
     digits are mis-scaled and the wrong items are chosen;
  2. removing already-used elements — ``permutation_rank`` never removes the
     chosen item from the remaining list, so each ``index()`` is taken against
     the full original list and the rank is inflated;
  3. rank/unrank off-by-one — ``nth_permutation`` decodes ``n`` as if the
     permutations were numbered from 1, shifting every non-identity result by one
     rank, so it is no longer the exact inverse of ``permutation_rank``.

Bug 1 and bug 3 both live in ``nth_permutation`` and interact: ``nth`` is only
correct once BOTH are fixed. Bug 2 lives in ``permutation_rank``. The two
functions are exact inverses only when all three are fixed. The identity
(``nth_permutation(items, 0)``) is correct even in the buggy starter and guards
against regressions.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs).
"""
import importlib
import itertools
import json
import sys
from math import factorial

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --- import the produced package ---------------------------------------------
# Contract path is ``permgen.public``; fall back to the package root ``permgen``
# so a submission that re-exports the API from ``__init__`` (but moved it off
# ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
nth_permutation = None
permutation_rank = None
PermError = None
try:
    try:
        mod = importlib.import_module("permgen.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("permgen")
    nth_permutation = getattr(mod, "nth_permutation")
    permutation_rank = getattr(mod, "permutation_rank")
    PermError = getattr(mod, "PermError", None)
    if not (isinstance(PermError, type) and issubclass(PermError, BaseException)):
        PermError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
def oracle_nth(items, n):
    """Reference factoradic decode, independent of the submission under test."""
    items = list(items)
    k = len(items)
    if k == 0:
        return []
    avail = list(items)
    out = []
    rest = n
    for i in range(k, 0, -1):
        place = factorial(i - 1)
        digit = rest // place
        rest = rest % place
        out.append(avail.pop(digit))
    return out


def oracle_rank(perm, items):
    """Reference factoradic encode, the exact inverse of ``oracle_nth``."""
    perm = list(perm)
    items = list(items)
    k = len(items)
    avail = list(items)
    rank = 0
    for i in range(k, 0, -1):
        cur = perm[k - i]
        digit = avail.index(cur)
        rank += digit * factorial(i - 1)
        avail.pop(digit)
    return rank


def eq_nth(label, items, n):
    """Check ``nth_permutation(items, n)`` equals the oracle (and does not mutate
    the caller's ``items``)."""
    def _fn():
        want = oracle_nth(items, n)
        passed_in = list(items)
        got = nth_permutation(list(items), n)
        ok = list(got) == want
        return ok, f"{label}: nth({items!r},{n}) -> {got!r}, expected {want!r}"

    return _fn


def eq_rank(label, perm, items):
    """Check ``permutation_rank(perm, items)`` equals the oracle."""
    def _fn():
        want = oracle_rank(perm, items)
        got = permutation_rank(list(perm), list(items))
        return got == want, f"{label}: rank({perm!r}) -> {got!r}, expected {want!r}"

    return _fn


I3 = [1, 2, 3]
I4 = ["a", "b", "c", "d"]
I5 = ["p", "q", "r", "s", "t"]

if import_ok:
    # --- baseline identity (passes even buggy; guards regressions) -----------
    check("identity_nth_4", "nth(items, 0) is the identity (4 items)",
          eq_nth("identity4", I4, 0))
    check("identity_nth_3", "nth(items, 0) is the identity (3 items)",
          eq_nth("identity3", I3, 0))
    check("identity_nth_5", "nth(items, 0) is the identity (5 items)",
          eq_nth("identity5", I5, 0))

    # --- nth_permutation values: need BUG 1 (factoradic) and BUG 3 (off-by-one)
    check("nth_4_r1", "nth rank 1 of 4 items",
          eq_nth("nth4_1", I4, 1))
    check("nth_4_r5", "nth rank 5 of 4 items",
          eq_nth("nth4_5", I4, 5))
    check("nth_4_r11", "nth rank 11 of 4 items",
          eq_nth("nth4_11", I4, 11))
    check("nth_4_r23_last", "nth last rank (fully reversed) of 4 items",
          eq_nth("nth4_23", I4, 23))
    check("nth_4_r2", "nth rank 2 of 4 items",
          eq_nth("nth4_2", I4, 2))
    check("nth_4_r17", "nth rank 17 of 4 items",
          eq_nth("nth4_17", I4, 17))

    # --- nth on other sizes --------------------------------------------------
    check("nth_3_r4", "nth rank 4 of 3 items",
          eq_nth("nth3_4", I3, 4))
    check("nth_5_r50", "nth rank 50 of 5 items",
          eq_nth("nth5_50", I5, 50))
    check("nth_5_r119_last", "nth last rank of 5 items",
          eq_nth("nth5_119", I5, 119))

    # --- nth full enumeration (every rank of 4 items) ------------------------
    def c_nth_enum():
        bad = [n for n in range(24) if list(nth_permutation(list(I4), n)) != oracle_nth(I4, n)]
        return (not bad), f"ranks decoding wrong: {bad[:6]}{'...' if len(bad) > 6 else ''}"

    check("nth_enumerate_4", "nth decodes every rank of 4 items correctly", c_nth_enum)

    # --- permutation_rank values: need BUG 2 (remaining-list removal) --------
    check("rank_identity_4", "rank of the identity is 0 (4 items)",
          eq_rank("rank_id4", I4, I4))
    check("rank_4_a", "rank of ['a','b','d','c']",
          eq_rank("rank4_a", ["a", "b", "d", "c"], I4))
    check("rank_4_b", "rank of ['b','a','c','d']",
          eq_rank("rank4_b", ["b", "a", "c", "d"], I4))
    check("rank_4_c", "rank of ['c','a','b','d']",
          eq_rank("rank4_c", ["c", "a", "b", "d"], I4))
    check("rank_4_last", "rank of the fully reversed 4-item list",
          eq_rank("rank4_last", ["d", "c", "b", "a"], I4))
    check("rank_3", "rank of [3,1,2] over [1,2,3]",
          eq_rank("rank3", [3, 1, 2], I3))

    # --- rank full enumeration ----------------------------------------------
    def c_rank_enum():
        bad = [n for n, p in enumerate(itertools.permutations(I4))
               if permutation_rank(list(p), list(I4)) != n]
        return (not bad), f"ranks computed wrong for: {bad[:6]}{'...' if len(bad) > 6 else ''}"

    check("rank_enumerate_4", "rank assigns the right index to every permutation of 4 items",
          c_rank_enum)

    def c_rank_5_identity():
        got = permutation_rank(list(I5), list(I5))
        return got == 0, f"rank of 5-item identity -> {got!r}, expected 0"

    check("rank_identity_5", "rank of the identity is 0 (5 items)", c_rank_5_identity)

    # --- the two are exact inverses (needs ALL three bugs fixed) -------------
    def c_round_trip_nr():
        bad = [n for n in range(24)
               if permutation_rank(nth_permutation(list(I4), n), list(I4)) != n]
        return (not bad), f"rank(nth(n)) != n for: {bad[:6]}{'...' if len(bad) > 6 else ''}"

    check("round_trip_nth_then_rank", "rank(nth(items, n)) == n for all n (4 items)",
          c_round_trip_nr)

    def c_round_trip_rn():
        bad = []
        for p in itertools.permutations(I4):
            r = permutation_rank(list(p), list(I4))
            if nth_permutation(list(I4), r) != list(p):
                bad.append("".join(map(str, p)))
        return (not bad), f"nth(rank(perm)) != perm for: {bad[:6]}{'...' if len(bad) > 6 else ''}"

    check("round_trip_rank_then_nth", "nth(items, rank(perm)) == perm for all perms (4 items)",
          c_round_trip_rn)

    def c_round_trip_5():
        got = permutation_rank(nth_permutation(list(I5), 77), list(I5))
        return got == 77, f"rank(nth(77)) -> {got!r}, expected 77 (5 items)"

    check("round_trip_5", "rank(nth(items, 77)) == 77 (5 items)", c_round_trip_5)

    # --- validation / edge cases --------------------------------------------
    def c_nth_out_of_range():
        try:
            nth_permutation(list(I4), 999)
        except PermError:
            return True, "raised PermError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected PermError"
        return False, "did not raise"

    check("nth_out_of_range_raises", "n >= len(items)! raises PermError", c_nth_out_of_range)

    def c_nth_empty():
        got = nth_permutation([], 0)
        return list(got) == [], f"nth([], 0) -> {got!r}, expected []"

    check("nth_empty_list", "nth([], 0) returns []", c_nth_empty)

    def c_rank_bad_length():
        try:
            permutation_rank(["a", "b"], list(I4))
        except PermError:
            return True, "raised PermError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected PermError"
        return False, "did not raise"

    check("rank_length_mismatch_raises", "perm length != items raises PermError", c_rank_bad_length)


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 27

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "permgen",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
