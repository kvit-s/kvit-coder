#!/usr/bin/env python3
"""Held-out behavior-level oracle for the greenfield `budgetrules` task.

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `budgetrules.public`
API and the `python -m budgetrules` CLI), NOT against the model's own tests and NOT
against any particular internal file layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

FAIRNESS / FIXED DENOMINATOR: the list of checks (hence `total`) is identical whether
or not the package imports — every check is registered, and if import fails they are
all recorded as FAILED and the score is forced to 0.0. The denominator never shrinks
to flatter a broken submission.

Tolerance: the brief under-specifies some shapes. This oracle DERIVES facts from the
returned structure rather than REQUIRING incidental key names, and is never stricter
than the brief + Contract. Spots where it leans on a convention the Contract pins are
marked `# ASSUMES`.
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

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


def fail_all(reason):
    """FIXED DENOMINATOR: when the package can't import, every check is recorded as a
    failure with the same ids/total it would have had on success."""
    for cid, desc in CHECK_REGISTRY:
        checks.append({"id": cid, "desc": desc, "passed": False, "detail": reason})


# Tolerant accessors --------------------------------------------------------------

def field(txn, name):
    """Pull a field from a categorized txn dict; tolerate non-dict gracefully."""
    return txn.get(name) if isinstance(txn, dict) else None


def get_bucket(summary, *candidates):
    """Pull a sub-dict (by_category / by_month) from a summarize() result, accepting
    any of the candidate key names. Behavior over incidental key naming, but the
    Contract pins `by_category`/`by_month` so those lead."""
    if not isinstance(summary, dict):
        return None
    for key in candidates:
        if key in summary and isinstance(summary[key], dict):
            return summary[key]
    return None


# --- import the produced package (contract: budgetrules.public) ------------------
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("budgetrules.public")
    if not hasattr(pub, "categorize") or not hasattr(pub, "summarize"):
        raise ImportError("budgetrules.public missing categorize/summarize")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# The registry fixes the denominator: same (id, desc) set on success or import failure.
CHECK_REGISTRY = [
    ("categorize_basic_match", "a matching rule sets category + tags on the txn"),
    ("categorize_preserves_order_len", "categorize returns same length & order as input, fields preserved"),
    ("categorize_uncategorized", "no matching rule -> category 'uncategorized', tags []"),
    ("tags_type_is_list", "tags is always a list (and [] when the winner sets none)"),
    ("priority_highest_wins", "highest-priority matching rule wins over a lower one"),
    ("priority_tie_rule_order", "equal priority is broken by rule order (earliest wins)"),
    ("priority_default_zero", "absent priority defaults to 0 (a priority>0 rule beats it)"),
    ("regex_match", "description_regex matches via re.search"),
    ("amount_range_match", "amount_min_cents / amount_max_cents bound matching"),
    ("merchant_equals_match", "merchant_equals does exact-equality matching"),
    ("malformed_rule_no_crash", "a malformed rule (bad regex / non-dict) never raises"),
    ("summarize_by_category", "summarize totals spending per category"),
    ("summarize_by_month", "summarize totals spending per YYYY-MM month"),
    ("summarize_refunds_reduce", "refunds (positive amount_cents) REDUCE spending"),
    ("no_input_mutation", "categorize does not mutate the input transactions"),
    ("cli_categorize_json", "`python -m budgetrules categorize` emits JSON"),
    ("cli_summarize_json", "`python -m budgetrules summarize` emits JSON"),
]


if not import_ok:
    fail_all(import_detail)
else:
    # 1. basic match: a description_contains rule sets category + tags.
    def c_basic():
        txns = [{"id": "t1", "date": "2026-01-01", "description": "ACME GROCERY", "amount_cents": -5234}]
        rules = [{"description_contains": "grocery", "set_category": "food", "set_tags": ["essential"]}]
        out = pub.categorize(txns, rules)
        ok = (
            isinstance(out, list) and len(out) == 1
            and field(out[0], "category") == "food"
            and field(out[0], "tags") == ["essential"]
        )
        return ok, f"out={out!r}"

    check("categorize_basic_match", "a matching rule sets category + tags on the txn", c_basic)

    # 2. order + length + original fields preserved.
    def c_order():
        txns = [
            {"id": "a", "date": "2026-01-01", "description": "FIRST", "amount_cents": -100},
            {"id": "b", "date": "2026-01-02", "description": "SECOND", "amount_cents": -200},
            {"id": "c", "date": "2026-01-03", "description": "THIRD", "amount_cents": -300},
        ]
        out = pub.categorize(txns, [])
        if not (isinstance(out, list) and len(out) == 3):
            return False, f"len/type wrong: {out!r}"
        ids = [field(o, "id") for o in out]
        amts = [field(o, "amount_cents") for o in out]
        ok = ids == ["a", "b", "c"] and amts == [-100, -200, -300]
        return ok, f"ids={ids!r} amts={amts!r}"

    check("categorize_preserves_order_len", "categorize returns same length & order as input, fields preserved", c_order)

    # 3. no rule matches -> uncategorized + empty tags.
    def c_uncat():
        txns = [{"id": "t1", "date": "2026-01-01", "description": "MYSTERY", "amount_cents": -42}]
        out = pub.categorize(txns, [{"description_contains": "nomatch", "set_category": "x"}])
        ok = field(out[0], "category") == "uncategorized" and field(out[0], "tags") == []
        return ok, f"out={out!r}"

    check("categorize_uncategorized", "no matching rule -> category 'uncategorized', tags []", c_uncat)

    # 4. tags type: a matching rule WITHOUT set_tags yields [] (a list, never None).
    def c_tags_type():
        txns = [{"id": "t1", "date": "2026-01-01", "description": "COFFEE", "amount_cents": -500}]
        out = pub.categorize(txns, [{"description_contains": "coffee", "set_category": "drink"}])
        tags = field(out[0], "tags")
        ok = isinstance(tags, list) and tags == []
        return ok, f"tags={tags!r}"

    check("tags_type_is_list", "tags is always a list (and [] when the winner sets none)", c_tags_type)

    # 5. highest priority wins. 3 matching rules with the highest-priority (10) in the
    #    MIDDLE, so neither a first-match nor a last-match heuristic can win incidentally
    #    — only a true priority comparison selects "winner".
    def c_pri_high():
        txns = [{"id": "t1", "date": "2026-01-01", "description": "ACME GROCERY", "amount_cents": -5234}]
        rules = [
            {"description_contains": "acme", "set_category": "lowfirst", "priority": 1},
            {"description_contains": "grocery", "set_category": "winner", "priority": 10},
            {"description_contains": "acme", "set_category": "lowlast", "priority": 5},
        ]
        out = pub.categorize(txns, rules)
        ok = field(out[0], "category") == "winner"
        return ok, f"category={field(out[0], 'category')!r}"

    check("priority_highest_wins", "highest-priority matching rule wins over a lower one", c_pri_high)

    # 6. tie broken by rule order: two equal-priority matches -> earliest wins.
    def c_pri_tie():
        txns = [{"id": "t1", "date": "2026-01-01", "description": "ACME GROCERY", "amount_cents": -5234}]
        rules = [
            {"description_contains": "acme", "set_category": "first", "priority": 5},
            {"description_contains": "grocery", "set_category": "second", "priority": 5},
        ]
        out = pub.categorize(txns, rules)
        ok = field(out[0], "category") == "first"
        return ok, f"category={field(out[0], 'category')!r}"

    check("priority_tie_rule_order", "equal priority is broken by rule order (earliest wins)", c_pri_tie)

    # 7. absent priority defaults to 0: a priority>0 rule beats a no-priority rule
    #    even when the no-priority rule is listed FIRST (so it can't win on order).
    def c_pri_default():
        txns = [{"id": "t1", "date": "2026-01-01", "description": "ACME GROCERY", "amount_cents": -5234}]
        rules = [
            {"description_contains": "acme", "set_category": "nopri"},          # priority defaults 0
            {"description_contains": "grocery", "set_category": "withpri", "priority": 3},
        ]
        out = pub.categorize(txns, rules)
        ok = field(out[0], "category") == "withpri"
        return ok, f"category={field(out[0], 'category')!r}"

    check("priority_default_zero", "absent priority defaults to 0 (a priority>0 rule beats it)", c_pri_default)

    # 8. regex matching via re.search.
    def c_regex():
        txns = [
            {"id": "t1", "date": "2026-01-01", "description": "UBER TRIP 1234", "amount_cents": -1500},
            {"id": "t2", "date": "2026-01-01", "description": "GROCERY STORE", "amount_cents": -900},
        ]
        rules = [{"description_regex": r"UBER.*\d+", "set_category": "transport"}]
        out = pub.categorize(txns, rules)
        ok = field(out[0], "category") == "transport" and field(out[1], "category") == "uncategorized"
        return ok, f"cats={[field(o, 'category') for o in out]!r}"

    check("regex_match", "description_regex matches via re.search", c_regex)

    # 9. amount range bounds: min and max each constrain matching.
    def c_amount():
        txns = [
            {"id": "small", "date": "2026-01-01", "description": "X", "amount_cents": -50},
            {"id": "mid", "date": "2026-01-01", "description": "X", "amount_cents": -500},
            {"id": "big", "date": "2026-01-01", "description": "X", "amount_cents": -5000},
        ]
        # match only the mid txn: -1000 <= amount <= -100
        rules = [{"amount_min_cents": -1000, "amount_max_cents": -100, "set_category": "midband"}]
        out = pub.categorize(txns, rules)
        cats = [field(o, "category") for o in out]
        ok = cats == ["uncategorized", "midband", "uncategorized"]
        return ok, f"cats={cats!r}"

    check("amount_range_match", "amount_min_cents / amount_max_cents bound matching", c_amount)

    # 10. merchant_equals exact equality (a near-miss must NOT match).
    def c_merchant():
        txns = [
            {"id": "t1", "date": "2026-01-01", "description": "STARBUCKS", "amount_cents": -450},
            {"id": "t2", "date": "2026-01-01", "description": "STARBUCKS #42", "amount_cents": -450},
        ]
        rules = [{"merchant_equals": "STARBUCKS", "set_category": "coffee"}]
        out = pub.categorize(txns, rules)
        ok = field(out[0], "category") == "coffee" and field(out[1], "category") == "uncategorized"
        return ok, f"cats={[field(o, 'category') for o in out]!r}"

    check("merchant_equals_match", "merchant_equals does exact-equality matching", c_merchant)

    # 11. malformed rules never crash; valid rules still apply around them.
    def c_malformed():
        txns = [{"id": "t1", "date": "2026-01-01", "description": "ACME GROCERY", "amount_cents": -5234}]
        rules = [
            {"description_regex": "(((", "set_category": "bad_regex"},  # uncompilable -> just no match
            "not a dict",                                              # malformed -> skipped
            {"description_contains": "grocery", "set_category": "food"},
        ]
        out = pub.categorize(txns, rules)  # must not raise
        ok = field(out[0], "category") == "food"
        return ok, f"category={field(out[0], 'category')!r}"

    check("malformed_rule_no_crash", "a malformed rule (bad regex / non-dict) never raises", c_malformed)

    # Shared categorized fixture for the summarize checks.
    def _summary_fixture():
        txns = [
            {"id": "g1", "date": "2026-01-05", "description": "GROCERY", "amount_cents": -3000},
            {"id": "g2", "date": "2026-01-20", "description": "GROCERY", "amount_cents": -2000},
            {"id": "c1", "date": "2026-02-10", "description": "COFFEE", "amount_cents": -500},
        ]
        rules = [
            {"description_contains": "grocery", "set_category": "food"},
            {"description_contains": "coffee", "set_category": "drink"},
        ]
        return pub.categorize(txns, rules)

    # 12. totals by category. -amount_cents is the spending convention (Contract).
    def c_by_cat():
        summ = pub.summarize(_summary_fixture())
        bucket = get_bucket(summ, "by_category", "categories", "by_cat")
        if bucket is None:
            return False, f"no category bucket in {summ!r}"
        # food: 3000 + 2000 = 5000 ; drink: 500   (spending = -amount_cents)
        ok = bucket.get("food") == 5000 and bucket.get("drink") == 500
        return ok, f"bucket={bucket!r}"

    check("summarize_by_category", "summarize totals spending per category", c_by_cat)

    # 13. totals by month (YYYY-MM).
    def c_by_month():
        summ = pub.summarize(_summary_fixture())
        bucket = get_bucket(summ, "by_month", "months", "by_mon")
        if bucket is None:
            return False, f"no month bucket in {summ!r}"
        # 2026-01: 3000 + 2000 = 5000 ; 2026-02: 500
        ok = bucket.get("2026-01") == 5000 and bucket.get("2026-02") == 500
        return ok, f"bucket={bucket!r}"

    check("summarize_by_month", "summarize totals spending per YYYY-MM month", c_by_month)

    # 14. refunds reduce spending: a positive amount_cents in the same category/month
    #     lowers both the category and the month total.
    def c_refunds():
        txns = [
            {"id": "buy", "date": "2026-03-01", "description": "STORE", "amount_cents": -10000},
            {"id": "refund", "date": "2026-03-15", "description": "STORE", "amount_cents": 4000},
        ]
        rules = [{"description_contains": "store", "set_category": "shopping"}]
        cat = pub.categorize(txns, rules)
        summ = pub.summarize(cat)
        cbucket = get_bucket(summ, "by_category", "categories", "by_cat")
        mbucket = get_bucket(summ, "by_month", "months", "by_mon")
        if cbucket is None or mbucket is None:
            return False, f"missing buckets in {summ!r}"
        # net spending = 10000 - 4000 = 6000 (refund reduced it)
        ok = cbucket.get("shopping") == 6000 and mbucket.get("2026-03") == 6000
        return ok, f"cat={cbucket!r} month={mbucket!r}"

    check("summarize_refunds_reduce", "refunds (positive amount_cents) REDUCE spending", c_refunds)

    # 15. categorize must not mutate the caller's input transactions.
    def c_no_mutation():
        txns = [{"id": "t1", "date": "2026-01-01", "description": "ACME GROCERY", "amount_cents": -5234}]
        snapshot = json.dumps(txns, sort_keys=True)
        pub.categorize(txns, [{"description_contains": "grocery", "set_category": "food", "set_tags": ["x"]}])
        ok = json.dumps(txns, sort_keys=True) == snapshot
        return ok, "unchanged" if ok else f"input mutated -> {txns!r}"

    check("no_input_mutation", "categorize does not mutate the input transactions", c_no_mutation)


# --- CLI: all output must be JSON ------------------------------------------------
def run_cli_categorize():
    txns = [{"id": "t1", "date": "2026-01-01", "description": "ACME GROCERY", "amount_cents": -5234}]
    rules = [{"description_contains": "grocery", "set_category": "food", "set_tags": ["x"]}]
    tdir = tempfile.mkdtemp(dir=ROOT)
    try:
        tp = os.path.join(tdir, "txns.json")
        rp = os.path.join(tdir, "rules.json")
        with open(tp, "w") as f:
            json.dump(txns, f)
        with open(rp, "w") as f:
            json.dump(rules, f)
        proc = subprocess.run(
            [sys.executable, "-m", "budgetrules", "categorize", "--transactions", tp, "--rules", rp],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        parsed = json.loads(proc.stdout)  # raises if not JSON
        return isinstance(parsed, list), f"rc={proc.returncode} type={type(parsed).__name__}"
    finally:
        import shutil
        shutil.rmtree(tdir, ignore_errors=True)


def run_cli_summarize():
    categorized = [
        {"id": "t1", "date": "2026-01-01", "description": "ACME GROCERY",
         "amount_cents": -5234, "category": "food", "tags": []},
    ]
    tdir = tempfile.mkdtemp(dir=ROOT)
    try:
        cp = os.path.join(tdir, "categorized.json")
        with open(cp, "w") as f:
            json.dump(categorized, f)
        proc = subprocess.run(
            [sys.executable, "-m", "budgetrules", "summarize", cp],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        parsed = json.loads(proc.stdout)  # raises if not JSON
        return isinstance(parsed, dict), f"rc={proc.returncode} type={type(parsed).__name__}"
    finally:
        import shutil
        shutil.rmtree(tdir, ignore_errors=True)


if import_ok:
    check("cli_categorize_json", "`python -m budgetrules categorize` emits JSON", run_cli_categorize)
    check("cli_summarize_json", "`python -m budgetrules summarize` emits JSON", run_cli_summarize)


passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "budgetrules",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
