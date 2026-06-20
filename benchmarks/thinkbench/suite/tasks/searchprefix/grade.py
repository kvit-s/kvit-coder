#!/usr/bin/env python3
"""Held-out behavior-level oracle for the feature-add task `searchprefix`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never reads the model's own tests. It grades the produced `searchprefix`
package against the BRIEF'S CONTRACT (the `searchprefix.public.SearchIndex` API),
not against any particular internal layout.

The task: the starter package supports case-insensitive EXACT-term search ranked
by term frequency. The model must ADD prefix-query support — a query token ending
in ``*`` matches any document term starting with that prefix — WITHOUT breaking the
existing exact-search / ranking behavior.

  - NEW checks (prefix): the feature works (``pay*`` matches ``payment``/``payable``
    but not unrelated docs; mid-word non-prefix does NOT match). These FAIL on the
    starter (no prefix support) and PASS on the reference.
  - REGRESSION checks (existing): exact-term search and frequency ranking still
    work. These PASS on BOTH the starter and the reference — they guard against an
    "add the feature, break the base" change.

Output: ONE JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is 0
whenever grading ran to completion (even score 0.0); a failed import forces score
0.0. This grader writes no files and leaves nothing behind.
"""
import importlib
import json
import os
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# FIXED roster of check ids — the denominator never depends on which checks ran, so
# an import crash or a half-built package scores against the same total as a full one.
CHECK_IDS = [
    # --- NEW: prefix-query feature (FAIL on starter, PASS on reference) ---------
    "prefix_matches_multiple",     # pay* matches payment AND payable
    "prefix_excludes_unrelated",   # pay* does not match an unrelated doc
    "prefix_other_term",           # ship* matches shipping
    "prefix_not_midword",          # ment* does NOT match payment (mid-word)
    "prefix_ranks_by_frequency",   # prefix results ranked by total term frequency
    "mixed_plain_and_prefix",      # a query mixing plain + prefix tokens
    # --- REGRESSION: existing exact-search behavior (PASS on starter too) -------
    "exact_term_match",            # plain term matches the exact term only
    "exact_not_prefix",            # plain 'pay' (no *) still matches nothing
    "rank_by_term_frequency",      # exact-search ranking by frequency
    "and_semantics_multi_term",    # multi-term query requires every term
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


# --- import the produced package (contract: searchprefix.public.SearchIndex) ----
# Brief pins `searchprefix.public`; fall back to top-level `searchprefix` for a
# package that re-exports SearchIndex but did not keep the `.public` submodule.
import_ok = True
import_detail = ""
SearchIndex = None
try:
    try:
        mod = importlib.import_module("searchprefix.public")
    except Exception:  # noqa: BLE001 - fall back to the top-level package
        mod = importlib.import_module("searchprefix")
    SearchIndex = getattr(mod, "SearchIndex")
    if not callable(SearchIndex):
        raise TypeError("searchprefix SearchIndex is not callable")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def _index(docs):
    """Build a fresh SearchIndex from an iterable of (doc_id, text) pairs."""
    idx = SearchIndex()
    for doc_id, text in docs:
        idx.add_document(doc_id, text)
    return idx


# A small corpus reused across checks. Designed so that:
#   - 'payment'/'payable' share the prefix 'pay' but are distinct exact terms;
#   - 'pay' itself is NOT an exact term in any document;
#   - d3 is unrelated (no 'pay'/'ship' overlap with the prefix queries below it).
CORPUS = [
    ("d1", "Payment received and payment processed"),  # 'payment' x2
    ("d2", "Refund payable next week"),                 # 'payable' x1
    ("d3", "Shipping label printed"),                   # 'shipping' x1
]


if import_ok:
    # ============================ NEW FEATURE: prefix ==========================

    # 1. pay* matches BOTH docs whose terms start with 'pay' (payment, payable).
    #    (Starter: FAILS — no prefix support, returns [].)
    def c_prefix_matches_multiple():
        idx = _index(CORPUS)
        got = set(idx.search("pay*"))
        return got == {"d1", "d2"}, f"search('pay*')={sorted(map(str, got))!r}"

    check("prefix_matches_multiple", "pay* matches payment and payable", c_prefix_matches_multiple)

    # 2. pay* must NOT match the unrelated doc (d3 has no 'pay'-prefixed term).
    def c_prefix_excludes_unrelated():
        idx = _index(CORPUS)
        got = set(idx.search("pay*"))
        return "d3" not in got, f"search('pay*')={sorted(map(str, got))!r}"

    check("prefix_excludes_unrelated", "pay* excludes documents with no matching prefix", c_prefix_excludes_unrelated)

    # 3. A different prefix resolves to its own document (ship* -> shipping/d3).
    def c_prefix_other_term():
        idx = _index(CORPUS)
        got = set(idx.search("ship*"))
        return got == {"d3"}, f"search('ship*')={sorted(map(str, got))!r}"

    check("prefix_other_term", "ship* matches shipping", c_prefix_other_term)

    # 4. A prefix only matches at the START of a term — 'ment*' must NOT match
    #    'payment' (mid-word substring is not a prefix). (Starter: vacuously passes
    #    via empty result; reference must also return []. This pins prefix==start.)
    def c_prefix_not_midword():
        idx = _index(CORPUS)
        got = set(idx.search("ment*"))
        return got == set(), f"search('ment*')={sorted(map(str, got))!r}"

    check("prefix_not_midword", "a prefix matches only the start of a term, not mid-word", c_prefix_not_midword)

    # 5. Prefix results are ranked by total term frequency (d1 has 'payment' x2 = 2,
    #    d2 has 'payable' x1 = 1), so d1 ranks before d2. (Starter: FAILS — [].)
    def c_prefix_ranks_by_frequency():
        idx = _index(CORPUS)
        got = idx.search("pay*")
        return got == ["d1", "d2"], f"search('pay*')={got!r}"

    check("prefix_ranks_by_frequency", "prefix results are ranked by total term frequency", c_prefix_ranks_by_frequency)

    # 6. A query mixing a plain token and a prefix token requires BOTH; only d1
    #    has both 'received' (exact) and a 'pay'-prefixed term. (Starter: FAILS —
    #    the 'pay*' token has no exact term to match, so it returns [].)
    def c_mixed_plain_and_prefix():
        idx = _index(CORPUS)
        got = set(idx.search("received pay*"))
        return got == {"d1"}, f"search('received pay*')={sorted(map(str, got))!r}"

    check("mixed_plain_and_prefix", "a query mixing a plain and a prefix token requires both", c_mixed_plain_and_prefix)

    # ====================== REGRESSION: existing behavior ======================

    # 7. A plain term matches the EXACT term only — 'payment' -> d1 (not d2/d3).
    #    (Starter AND reference both PASS.)
    def c_exact_term_match():
        idx = _index(CORPUS)
        got = idx.search("payment")
        return got == ["d1"], f"search('payment')={got!r}"

    check("exact_term_match", "a plain term matches the exact term only", c_exact_term_match)

    # 8. A plain token that is itself only a PREFIX of stored terms (no trailing *)
    #    still matches NOTHING — exact-match semantics are preserved. 'pay' is not
    #    an exact term in any doc. (Both PASS — guards the feature from leaking into
    #    plain-token behavior.)
    def c_exact_not_prefix():
        idx = _index(CORPUS)
        got = idx.search("pay")
        return got == [], f"search('pay')={got!r}"

    check("exact_not_prefix", "a plain token is exact-match, not an implicit prefix", c_exact_not_prefix)

    # 9. Exact-search ranking by term frequency is unchanged. Two docs both contain
    #    'alpha'; the one with more occurrences ranks first; tie-break is asc doc id.
    #    (Both PASS.)
    def c_rank_by_term_frequency():
        idx = _index([
            ("r1", "alpha beta alpha gamma"),  # alpha x2
            ("r2", "alpha delta"),             # alpha x1
            ("r3", "nothing here"),
        ])
        got = idx.search("alpha")
        return got == ["r1", "r2"], f"search('alpha')={got!r}"

    check("rank_by_term_frequency", "exact search ranks by term frequency, descending", c_rank_by_term_frequency)

    # 10. Multi-term (plain) query keeps AND semantics — every term must be present.
    #     Only the doc with both 'quick' and 'fox' matches. (Both PASS.)
    def c_and_semantics_multi_term():
        idx = _index([
            ("m1", "the quick brown fox"),   # has quick AND fox
            ("m2", "quick start guide"),     # quick, no fox
            ("m3", "silver fox stole"),      # fox, no quick
        ])
        got = idx.search("quick fox")
        return got == ["m1"], f"search('quick fox')={got!r}"

    check("and_semantics_multi_term", "a multi-term query requires every term (AND semantics)", c_and_semantics_multi_term)


# --- assemble the scorecard over the FIXED denominator ------------------------
checks = []
for cid in CHECK_IDS:
    passed, detail = results.get(cid, (False, "not run (import failed)"))
    checks.append({"id": cid, "passed": passed, "detail": detail})

passed = sum(1 for c in checks if c["passed"])
card = {
    "task": "searchprefix",
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
