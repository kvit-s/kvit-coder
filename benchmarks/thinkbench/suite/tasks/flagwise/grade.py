#!/usr/bin/env python3
"""Held-out behavior-level oracle for the greenfield `flagwise` task.

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `flagwise.public`
API and the `python -m flagwise` CLI), NOT against the model's own tests and NOT
against any particular internal file layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

FIXED DENOMINATOR: the full behavior-check list is built up front. If the package
fails to import, EVERY behavior check is recorded as FAILED (never skipped), so a
broken import scores ~0 and can never masquerade as a passing run.

Tolerance: the brief under-specifies the exact `reason` spellings and accepts any
served value type. This oracle derives the reason mapping from the reference-style
distinctions rather than pinning literal strings, and checks BEHAVIOR (the pinned
{"key","value","reason","matched_rule_index"} contract), not incidental key names.
Spots where it assumes a convention the brief does not pin are marked `# ASSUMES`.
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


def record(cid, desc, ok, detail=""):
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, desc, ok, detail)


# --- the FULL, FIXED check list (id -> human description) ---------------------
# Declared up front so the denominator is constant regardless of import success.
CHECK_SPECS = [
    ("result_shape", "evaluate_flag returns a dict with key/value/reason/matched_rule_index"),
    ("rule_ordering", "the FIRST matching rule decides; matched_rule_index points at it"),
    ("disabled_flag", "a disabled flag serves its default and matches no rule"),
    ("default_no_match", "when no rule matches, the flag default is served"),
    ("serve_value", "a matched rule serves its configured value (not just True)"),
    ("op_equals", "equals condition matches on equality and only then"),
    ("op_in", "in / not_in conditions match set membership"),
    ("op_exists", "exists condition tests context-field presence"),
    ("op_numeric", "greater_than / less_than compare numerically"),
    ("nested_boolean", "nested and / or / not combinators evaluate correctly"),
    ("missing_field", "a missing context field makes a leaf False without raising"),
    ("unknown_flag", "an unknown flag_key yields a structured (non-raising) result"),
    ("determinism_same", "same (flag, context) yields an equal result across repeated calls"),
    ("rollout_stable_user", "a rollout decision is stable per user across many calls"),
    ("rollout_hash_inputs", "rollout depends on flag_key + user_id (a stored hash, not hash())"),
    ("rollout_distribution", "rollout N serves ~N% of many synthetic users (tolerant band)"),
    ("evaluate_all_keys", "evaluate_all returns one contract result per configured flag"),
    ("cli_eval_json", "`python -m flagwise eval` emits a JSON result dict"),
    ("cli_eval_all_json", "`python -m flagwise eval-all` emits JSON keyed by flag"),
]
CHECK_DESC = dict(CHECK_SPECS)


# --- tolerant accessors on the pinned result shape ---------------------------
def res_field(result, *names):
    """Pull a contract field tolerantly. Pinned names are key/value/reason/
    matched_rule_index; we also accept a couple of obvious synonyms so we grade
    behavior, not spelling. Returns MISSING when absent."""
    if not isinstance(result, dict):
        return MISSING
    for n in names:
        if n in result:
            return result[n]
    return MISSING


MISSING = object()


def is_result_dict(r):
    if not isinstance(r, dict):
        return False
    return res_field(r, "value") is not MISSING and res_field(r, "reason") is not MISSING


def matched_index(r):
    return res_field(r, "matched_rule_index", "matched_rule", "rule_index", "index")


# --- config builders ---------------------------------------------------------
def cfg(flags):
    return {"flags": flags}


def flag(rules, enabled=True, default=False):
    return {"enabled": enabled, "default": default, "rules": rules}


# Import the produced package (contract: flagwise.public) ----------------------
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("flagwise.public")
    for name in ("evaluate_flag", "evaluate_all"):
        if not hasattr(pub, name):
            import_ok = False
            import_detail = f"flagwise.public missing {name}"
            break
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if not import_ok:
    # FIXED DENOMINATOR: every behavior check fails, none are skipped.
    for cid, desc in CHECK_SPECS:
        record(cid, desc, False, f"import failed: {import_detail}")
else:
    # 1. result shape — the pinned contract dict
    def c_shape():
        c = cfg({"f": flag([{"if": {"country": {"equals": "US"}}, "serve": True}])})
        r = pub.evaluate_flag(c, "f", {"country": "US", "user_id": "u1"})
        if not is_result_dict(r):
            return False, f"not a result dict: {r!r}"
        key = res_field(r, "key", "flag", "flag_key")
        return (key == "f"), f"key={key!r} result={r!r}"

    # 2. rule ordering — first matching rule wins; index points at it
    def c_ordering():
        # both rules match country=US; the FIRST (index 0) must decide.
        c = cfg({"f": flag([
            {"if": {"country": {"equals": "US"}}, "serve": "first"},
            {"if": {"country": {"equals": "US"}}, "serve": "second"},
        ])})
        r = pub.evaluate_flag(c, "f", {"country": "US", "user_id": "u1"})
        val = res_field(r, "value")
        idx = matched_index(r)
        return (val == "first" and idx == 0), f"value={val!r} idx={idx!r}"

    # 3. disabled flag -> default, no rule matched
    def c_disabled():
        c = cfg({"f": flag(
            [{"if": {"country": {"equals": "US"}}, "serve": True}],
            enabled=False, default=False,
        )})
        r = pub.evaluate_flag(c, "f", {"country": "US", "user_id": "u1"})
        val = res_field(r, "value")
        idx = matched_index(r)
        # disabled serves default (False) even though the rule WOULD match.
        return (val is False and (idx is None or idx is MISSING)), f"value={val!r} idx={idx!r}"

    # 4. no rule matches -> default served
    def c_default():
        c = cfg({"f": flag(
            [{"if": {"country": {"equals": "US"}}, "serve": True}],
            default="DEF",
        )})
        r = pub.evaluate_flag(c, "f", {"country": "CA", "user_id": "u1"})
        val = res_field(r, "value")
        idx = matched_index(r)
        return (val == "DEF" and (idx is None or idx is MISSING)), f"value={val!r} idx={idx!r}"

    # 5. a matched rule serves its configured value (not hard-coded True)
    def c_serve_value():
        c = cfg({"f": flag([{"if": {"country": {"equals": "US"}}, "serve": "variant_b"}])})
        r = pub.evaluate_flag(c, "f", {"country": "US", "user_id": "u1"})
        val = res_field(r, "value")
        return (val == "variant_b"), f"value={val!r}"

    # 6. equals — matches on equality and ONLY on equality
    def c_equals():
        c = cfg({"f": flag([{"if": {"tier": {"equals": "gold"}}, "serve": True}], default=False)})
        hit = res_field(pub.evaluate_flag(c, "f", {"tier": "gold", "user_id": "u"}), "value")
        miss = res_field(pub.evaluate_flag(c, "f", {"tier": "silver", "user_id": "u"}), "value")
        return (hit is True and miss is False), f"hit={hit!r} miss={miss!r}"

    # 7. in / not_in — set membership
    def c_in():
        c_in_ = cfg({"f": flag([{"if": {"plan": {"in": ["pro", "ent"]}}, "serve": True}], default=False)})
        c_notin = cfg({"f": flag([{"if": {"plan": {"not_in": ["pro", "ent"]}}, "serve": True}], default=False)})
        in_hit = res_field(pub.evaluate_flag(c_in_, "f", {"plan": "pro", "user_id": "u"}), "value")
        in_miss = res_field(pub.evaluate_flag(c_in_, "f", {"plan": "free", "user_id": "u"}), "value")
        notin_hit = res_field(pub.evaluate_flag(c_notin, "f", {"plan": "free", "user_id": "u"}), "value")
        return (in_hit is True and in_miss is False and notin_hit is True), \
            f"in_hit={in_hit!r} in_miss={in_miss!r} notin_hit={notin_hit!r}"

    # 8. exists — presence test (and its falsey form)
    def c_exists():
        c_yes = cfg({"f": flag([{"if": {"email": {"exists": True}}, "serve": True}], default=False)})
        c_no = cfg({"f": flag([{"if": {"email": {"exists": False}}, "serve": True}], default=False)})
        present = res_field(pub.evaluate_flag(c_yes, "f", {"email": "a@b.c", "user_id": "u"}), "value")
        absent = res_field(pub.evaluate_flag(c_yes, "f", {"user_id": "u"}), "value")
        not_present = res_field(pub.evaluate_flag(c_no, "f", {"user_id": "u"}), "value")
        return (present is True and absent is False and not_present is True), \
            f"present={present!r} absent={absent!r} exists_false={not_present!r}"

    # 9. greater_than / less_than — numeric comparison
    def c_numeric():
        c_gt = cfg({"f": flag([{"if": {"age": {"greater_than": 18}}, "serve": True}], default=False)})
        c_lt = cfg({"f": flag([{"if": {"age": {"less_than": 18}}, "serve": True}], default=False)})
        gt_hit = res_field(pub.evaluate_flag(c_gt, "f", {"age": 21, "user_id": "u"}), "value")
        gt_miss = res_field(pub.evaluate_flag(c_gt, "f", {"age": 16, "user_id": "u"}), "value")
        lt_hit = res_field(pub.evaluate_flag(c_lt, "f", {"age": 10, "user_id": "u"}), "value")
        return (gt_hit is True and gt_miss is False and lt_hit is True), \
            f"gt_hit={gt_hit!r} gt_miss={gt_miss!r} lt_hit={lt_hit!r}"

    # 10. nested boolean logic — and / or / not
    def c_nested():
        # (country == US AND plan in [pro,ent]) OR (NOT beta)
        cond = {"or": [
            {"and": [{"country": {"equals": "US"}}, {"plan": {"in": ["pro", "ent"]}}]},
            {"not": {"beta": {"equals": True}}},
        ]}
        c = cfg({"f": flag([{"if": cond, "serve": True}], default=False)})
        # left branch true
        a = res_field(pub.evaluate_flag(c, "f", {"country": "US", "plan": "pro", "beta": True, "user_id": "u"}), "value")
        # left false (plan), right true (not beta) -> beta absent -> not(False) True
        b = res_field(pub.evaluate_flag(c, "f", {"country": "US", "plan": "free", "user_id": "u"}), "value")
        # left false, right false (beta True) -> overall False
        c2 = res_field(pub.evaluate_flag(c, "f", {"country": "CA", "plan": "free", "beta": True, "user_id": "u"}), "value")
        return (a is True and b is True and c2 is False), f"a={a!r} b={b!r} c={c2!r}"

    # 11. missing context field -> leaf False, never raises
    def c_missing():
        c = cfg({"f": flag([{"if": {"country": {"equals": "US"}}, "serve": True}], default="D")})
        r = pub.evaluate_flag(c, "f", {"user_id": "u"})  # no country at all
        val = res_field(r, "value")
        return (val == "D"), f"value={val!r}"

    # 12. unknown flag -> structured result, no raise
    def c_unknown():
        c = cfg({"f": flag([{"if": {"country": {"equals": "US"}}, "serve": True}])})
        r = pub.evaluate_flag(c, "nonexistent", {"user_id": "u"})  # must not raise
        return (isinstance(r, dict)), f"result={r!r}"

    # 13. determinism — identical result across repeated calls
    def c_determinism():
        c = cfg({"f": flag([{"if": {"plan": {"in": ["pro"]}}, "rollout": 50}], default=False)})
        ctx = {"plan": "pro", "user_id": "stable-user-xyz"}
        r1 = json.dumps(pub.evaluate_flag(c, "f", ctx), sort_keys=True, default=str)
        r2 = json.dumps(pub.evaluate_flag(c, "f", ctx), sort_keys=True, default=str)
        r3 = json.dumps(pub.evaluate_flag(c, "f", ctx), sort_keys=True, default=str)
        return (r1 == r2 == r3), "stable" if r1 == r2 == r3 else f"{r1} vs {r2} vs {r3}"

    # 14. rollout stable per user — same user, same decision every time
    def c_rollout_stable():
        c = cfg({"f": flag([{"if": {"plan": {"in": ["pro"]}}, "rollout": 50}], default=False)})
        for uid in ("alice", "bob", "carol", "dave"):
            ctx = {"plan": "pro", "user_id": uid}
            vals = {res_field(pub.evaluate_flag(c, "f", ctx), "value") for _ in range(8)}
            if len(vals) != 1:
                return False, f"user {uid} unstable: {vals}"
        return True, "all users stable"

    # 15. rollout keyed on flag_key + user_id (stored hash, not salted hash())
    def c_rollout_hash_inputs():
        # Same user, two DIFFERENT flag_keys with the same 50% rollout: a hash of
        # (flag_key, user_id) makes the two buckets independent, so across many
        # users the two flags must NOT produce identical decisions for everyone.
        # ASSUMES the implementation buckets on flag_key+user_id (the pinned
        # contract); this distinguishes that from bucketing on user_id alone.
        c = cfg({
            "fa": flag([{"if": {"user_id": {"exists": True}}, "rollout": 50}], default=False),
            "fb": flag([{"if": {"user_id": {"exists": True}}, "rollout": 50}], default=False),
        })
        diffs = 0
        for i in range(200):
            ctx = {"user_id": f"user-{i}"}
            va = res_field(pub.evaluate_flag(c, "fa", ctx), "value")
            vb = res_field(pub.evaluate_flag(c, "fb", ctx), "value")
            if va != vb:
                diffs += 1
        # If flag_key were ignored, diffs would be 0. Independent buckets differ
        # ~50% of the time; require a healthy minimum.
        return (diffs >= 40), f"diffs={diffs}/200 (need >=40)"

    # 16. rollout distribution — ~N% over many synthetic users, tolerant band
    def c_distribution():
        c = cfg({"f": flag([{"if": {"user_id": {"exists": True}}, "rollout": 25}], default=False)})
        n, hits = 1000, 0
        for i in range(n):
            v = res_field(pub.evaluate_flag(c, "f", {"user_id": f"synthetic-user-{i}"}), "value")
            if v is True:
                hits += 1
        pct = 100.0 * hits / n
        # tolerant band: 25% +/- 10pp. Do NOT demand an exact count.
        return (15.0 <= pct <= 35.0), f"served true to {pct:.1f}% (want 25% +/-10pp)"

    # 17. evaluate_all — one contract result per configured flag
    def c_eval_all():
        c = cfg({
            "a": flag([{"if": {"country": {"equals": "US"}}, "serve": True}], default=False),
            "b": flag([], default="bb"),
        })
        allr = pub.evaluate_all(c, {"country": "US", "user_id": "u"})
        if not isinstance(allr, dict) or set(allr) != {"a", "b"}:
            return False, f"keys={list(allr) if isinstance(allr, dict) else allr!r}"
        good = is_result_dict(allr["a"]) and is_result_dict(allr["b"])
        va = res_field(allr["a"], "value")
        vb = res_field(allr["b"], "value")
        return (good and va is True and vb == "bb"), f"a={allr['a']!r} b={allr['b']!r}"

    check("result_shape", CHECK_DESC["result_shape"], c_shape)
    check("rule_ordering", CHECK_DESC["rule_ordering"], c_ordering)
    check("disabled_flag", CHECK_DESC["disabled_flag"], c_disabled)
    check("default_no_match", CHECK_DESC["default_no_match"], c_default)
    check("serve_value", CHECK_DESC["serve_value"], c_serve_value)
    check("op_equals", CHECK_DESC["op_equals"], c_equals)
    check("op_in", CHECK_DESC["op_in"], c_in)
    check("op_exists", CHECK_DESC["op_exists"], c_exists)
    check("op_numeric", CHECK_DESC["op_numeric"], c_numeric)
    check("nested_boolean", CHECK_DESC["nested_boolean"], c_nested)
    check("missing_field", CHECK_DESC["missing_field"], c_missing)
    check("unknown_flag", CHECK_DESC["unknown_flag"], c_unknown)
    check("determinism_same", CHECK_DESC["determinism_same"], c_determinism)
    check("rollout_stable_user", CHECK_DESC["rollout_stable_user"], c_rollout_stable)
    check("rollout_hash_inputs", CHECK_DESC["rollout_hash_inputs"], c_rollout_hash_inputs)
    check("rollout_distribution", CHECK_DESC["rollout_distribution"], c_distribution)
    check("evaluate_all_keys", CHECK_DESC["evaluate_all_keys"], c_eval_all)


# --- CLI: output must be JSON ------------------------------------------------
def _write_json(obj):
    fd, path = tempfile.mkstemp(suffix=".json", dir=ROOT)
    with os.fdopen(fd, "w") as f:
        json.dump(obj, f)
    return path


def run_cli_eval():
    config = cfg({"new_checkout": flag([{"if": {"country": {"equals": "US"}}, "serve": True}], default=False)})
    cpath = _write_json(config)
    upath = _write_json({"country": "US", "user_id": "u1"})
    try:
        proc = subprocess.run(
            [sys.executable, "-m", "flagwise", "eval",
             "--config", cpath, "--flag", "new_checkout", "--context", upath],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        out = json.loads(proc.stdout)  # raises if not JSON
        ok = isinstance(out, dict) and res_field(out, "value") is not MISSING
        return ok, f"rc={proc.returncode} out={out!r}"
    finally:
        for p in (cpath, upath):
            try:
                os.remove(p)
            except OSError:
                pass


def run_cli_eval_all():
    config = cfg({
        "a": flag([{"if": {"country": {"equals": "US"}}, "serve": True}], default=False),
        "b": flag([], default=True),
    })
    cpath = _write_json(config)
    upath = _write_json({"country": "US", "user_id": "u1"})
    try:
        proc = subprocess.run(
            [sys.executable, "-m", "flagwise", "eval-all",
             "--config", cpath, "--context", upath],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        out = json.loads(proc.stdout)  # raises if not JSON
        # ASSUMES eval-all emits a JSON object keyed by flag key (the pinned
        # evaluate_all shape). Accept any object that contains both flag keys.
        ok = isinstance(out, dict) and "a" in out and "b" in out
        return ok, f"rc={proc.returncode} keys={list(out) if isinstance(out, dict) else out!r}"
    finally:
        for p in (cpath, upath):
            try:
                os.remove(p)
            except OSError:
                pass


if import_ok:
    check("cli_eval_json", CHECK_DESC["cli_eval_json"], run_cli_eval)
    check("cli_eval_all_json", CHECK_DESC["cli_eval_all_json"], run_cli_eval_all)
# On import failure the up-front CHECK_SPECS loop already records these as failed —
# don't record them again here (that would double-count the denominator).


passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "flagwise",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": round(passed / total, 4) if total else 0.0,
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
