#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield task `confstack`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `confstack.public`
API and the `python -m confstack` CLI), NOT against the model's own tests and NOT
against any particular internal file layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. The denominator is FIXED: if the
package fails to import, every check is still recorded as failed and the score is
forced to 0.0. Exit code is 0 whenever grading ran to completion (even score 0.0);
nonzero only on a grader-internal failure.

Tolerance: the oracle derives behavior from the public contract and never demands
more than brief + Contract pin. It accepts any nested-dict representation and
checks BEHAVIOR (precedence, expansion, inference), not incidental shape. Spots
where it leans on a convention the brief leaves open are pinned in brief.txt's
`## Contract`; the few residual assumptions are marked `# ASSUMES`.
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

# Fixed registry of every check this oracle performs. Used so the denominator is
# constant whether or not the import succeeds (on import failure each id is
# recorded as failed, never silently dropped).
CHECK_SPECS = [
    ("infer_bool_true", "string 'true' is inferred as boolean True"),
    ("infer_bool_false", "string 'false' is inferred as boolean False"),
    ("infer_int", "an integer-looking string is inferred as int"),
    ("infer_str", "a non-bool non-int string stays a string (e.g. '1.5')"),
    ("expand_dotted_cli", "a --nested.key flag expands to a nested dict"),
    ("nested_dict_preserved", "nested dict values are returned as nested dicts"),
    ("prec_cli_over_env", "CLI flags override environment variables"),
    ("prec_env_over_config", "environment variables override the config file"),
    ("prec_config_over_default", "config file overrides defaults"),
    ("bool_false_overrides_true", "a higher-precedence false overrides a lower true"),
    ("deep_merge_siblings", "deep merge keeps non-overlapping sibling keys"),
    ("missing_config_no_error", "a missing config file path does not raise"),
    ("none_config_ok", "config_file=None contributes nothing and does not raise"),
    ("no_flag_false", "--no-flag sets the flag to boolean false"),
    ("bare_flag_true", "a bare --flag sets it to boolean true"),
    ("env_prefix_filter", "only APP_-prefixed env vars are consumed"),
    ("unknown_flag_ok", "an unknown --flag is accepted, not rejected"),
    ("deterministic", "load_config is deterministic across repeated calls"),
    ("cli_show_json", "`python -m confstack show` emits JSON and exits 0"),
    ("cli_precedence", "the CLI applies CLI > config > defaults precedence"),
]

checks = []


def record(cid, desc, ok, detail):
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --- import the produced package (contract: confstack.public + python -m confstack)
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("confstack.public")
    if not hasattr(pub, "load_config"):
        raise AttributeError("confstack.public has no load_config")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def leaf(tree, dotted, default="__MISSING__"):
    """Pull a leaf out of a nested dict by dot path; default if any hop is absent."""
    cur = tree
    for part in dotted.split("."):
        if isinstance(cur, dict) and part in cur:
            cur = cur[part]
        else:
            return default
    return cur


if import_ok:
    load = pub.load_config

    def run_check(cid, desc, fn):
        try:
            ok, detail = fn()
        except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
            ok, detail = False, f"{type(e).__name__}: {e}"
        record(cid, desc, ok, detail)

    # --- type inference (driven through CLI string values; pinned in Contract) ---
    def c_infer_bool_true():
        r = load({}, None, {}, ["--x", "true"])
        return leaf(r, "x") is True, f"x={leaf(r, 'x')!r}"

    def c_infer_bool_false():
        r = load({}, None, {}, ["--x", "false"])
        return leaf(r, "x") is False, f"x={leaf(r, 'x')!r}"

    def c_infer_int():
        r = load({}, None, {}, ["--port", "9000"])
        v = leaf(r, "port")
        return (v == 9000 and isinstance(v, int) and not isinstance(v, bool)), f"port={v!r}"

    def c_infer_str():
        # 1.5 is not an integer-looking string -> stays a string (Contract pins floats as str).
        r = load({}, None, {}, ["--ratio", "1.5"])
        v = leaf(r, "ratio")
        return v == "1.5", f"ratio={v!r}"

    run_check("infer_bool_true", dict(CHECK_SPECS)["infer_bool_true"], c_infer_bool_true)
    run_check("infer_bool_false", dict(CHECK_SPECS)["infer_bool_false"], c_infer_bool_false)
    run_check("infer_int", dict(CHECK_SPECS)["infer_int"], c_infer_int)
    run_check("infer_str", dict(CHECK_SPECS)["infer_str"], c_infer_str)

    # --- dot-notation expansion -------------------------------------------------
    def c_expand_dotted_cli():
        r = load({}, None, {}, ["--database.host", "localhost"])
        # The dotted string must NOT survive as a flat top-level key.
        flat_present = "database.host" in r
        return (leaf(r, "database.host") == "localhost" and not flat_present), f"r={r!r}"

    def c_nested_dict_preserved():
        r = load({"server": {"port": 80}}, None, {}, [])
        return leaf(r, "server.port") == 80, f"r={r!r}"

    run_check("expand_dotted_cli", dict(CHECK_SPECS)["expand_dotted_cli"], c_expand_dotted_cli)
    run_check("nested_dict_preserved", dict(CHECK_SPECS)["nested_dict_preserved"], c_nested_dict_preserved)

    # --- precedence (each pair isolates ONE boundary) ---------------------------
    def c_prec_cli_over_env():
        r = load({}, None, {"APP_PORT": "1"}, ["--port", "2"])
        return leaf(r, "port") == 2, f"port={leaf(r, 'port')!r}"

    def c_prec_env_over_config():
        fd, path = tempfile.mkstemp(suffix=".json", dir=ROOT)
        try:
            with os.fdopen(fd, "w") as f:
                json.dump({"port": 1}, f)
            r = load({}, path, {"APP_PORT": "2"}, [])
            return leaf(r, "port") == 2, f"port={leaf(r, 'port')!r}"
        finally:
            try:
                os.remove(path)
            except OSError:
                pass

    def c_prec_config_over_default():
        fd, path = tempfile.mkstemp(suffix=".json", dir=ROOT)
        try:
            with os.fdopen(fd, "w") as f:
                json.dump({"port": 2}, f)
            r = load({"port": 1}, path, {}, [])
            return leaf(r, "port") == 2, f"port={leaf(r, 'port')!r}"
        finally:
            try:
                os.remove(path)
            except OSError:
                pass

    run_check("prec_cli_over_env", dict(CHECK_SPECS)["prec_cli_over_env"], c_prec_cli_over_env)
    run_check("prec_env_over_config", dict(CHECK_SPECS)["prec_env_over_config"], c_prec_env_over_config)
    run_check("prec_config_over_default", dict(CHECK_SPECS)["prec_config_over_default"], c_prec_config_over_default)

    # --- boolean false overriding true (the headline fairness case) -------------
    def c_bool_false_overrides_true():
        # defaults say debug=true; CLI --no-debug must flip it to false.
        r = load({"debug": True}, None, {}, ["--no-debug"])
        return leaf(r, "debug") is False, f"debug={leaf(r, 'debug')!r}"

    run_check("bool_false_overrides_true", dict(CHECK_SPECS)["bool_false_overrides_true"], c_bool_false_overrides_true)

    # --- deep merge keeps siblings ---------------------------------------------
    def c_deep_merge_siblings():
        # defaults define a.x and a.y; CLI overrides only a.x -> a.y must survive.
        r = load({"a": {"x": 1, "y": 2}}, None, {}, ["--a.x", "9"])
        return (leaf(r, "a.x") == 9 and leaf(r, "a.y") == 2), f"a={leaf(r, 'a')!r}"

    run_check("deep_merge_siblings", dict(CHECK_SPECS)["deep_merge_siblings"], c_deep_merge_siblings)

    # --- robustness: missing / None config -------------------------------------
    def c_missing_config_no_error():
        nope = os.path.join(ROOT, "does-not-exist-12345.json")
        r = load({"port": 5}, nope, {}, [])  # must not raise
        return leaf(r, "port") == 5, f"r={r!r}"

    def c_none_config_ok():
        r = load({"port": 5}, None, {}, [])  # must not raise
        return leaf(r, "port") == 5, f"r={r!r}"

    run_check("missing_config_no_error", dict(CHECK_SPECS)["missing_config_no_error"], c_missing_config_no_error)
    run_check("none_config_ok", dict(CHECK_SPECS)["none_config_ok"], c_none_config_ok)

    # --- boolean flag forms -----------------------------------------------------
    def c_no_flag_false():
        r = load({}, None, {}, ["--no-feature"])
        return leaf(r, "feature") is False, f"feature={leaf(r, 'feature')!r}"

    def c_bare_flag_true():
        r = load({}, None, {}, ["--verbose"])
        return leaf(r, "verbose") is True, f"verbose={leaf(r, 'verbose')!r}"

    run_check("no_flag_false", dict(CHECK_SPECS)["no_flag_false"], c_no_flag_false)
    run_check("bare_flag_true", dict(CHECK_SPECS)["bare_flag_true"], c_bare_flag_true)

    # --- env prefix filtering ---------------------------------------------------
    def c_env_prefix_filter():
        r = load({}, None, {"APP_PORT": "8080", "PATH": "/usr/bin", "HOME": "/root"}, [])
        # APP_PORT consumed; PATH/HOME ignored (not surfaced as keys).
        consumed = leaf(r, "port") == 8080
        # ASSUMES non-prefixed vars are not surfaced under any lowercased key.
        leaked = ("path" in r) or ("home" in r)
        return (consumed and not leaked), f"r={r!r}"

    run_check("env_prefix_filter", dict(CHECK_SPECS)["env_prefix_filter"], c_env_prefix_filter)

    # --- unknown flags accepted (no schema) ------------------------------------
    def c_unknown_flag_ok():
        r = load({}, None, {}, ["--totally-unknown", "v"])  # must not raise/reject
        return leaf(r, "totally-unknown") == "v", f"r={r!r}"

    run_check("unknown_flag_ok", dict(CHECK_SPECS)["unknown_flag_ok"], c_unknown_flag_ok)

    # --- determinism ------------------------------------------------------------
    def c_deterministic():
        fd, path = tempfile.mkstemp(suffix=".json", dir=ROOT)
        try:
            with os.fdopen(fd, "w") as f:
                json.dump({"a": {"y": 2}}, f)
            args = (["--a.x", "1"],)
            r1 = json.dumps(load({"d": 1}, path, {"APP_E": "3"}, list(args[0])), sort_keys=True, default=str)
            r2 = json.dumps(load({"d": 1}, path, {"APP_E": "3"}, list(args[0])), sort_keys=True, default=str)
            return r1 == r2, "stable" if r1 == r2 else "differs across runs"
        finally:
            try:
                os.remove(path)
            except OSError:
                pass

    run_check("deterministic", dict(CHECK_SPECS)["deterministic"], c_deterministic)


# --- CLI: `python -m confstack show` ----------------------------------------
# These run regardless of the in-process import (a package can have a broken
# public.py but a working __main__, or vice-versa); each is its own check. They
# are skipped only when the import already failed, since the fixed denominator
# then records them as failed below.
def _cli_show_json():
    """`show` with no extra args must emit JSON and exit 0."""
    proc = subprocess.run(
        [sys.executable, "-m", "confstack", "show"],
        capture_output=True, text=True, timeout=60, cwd=ROOT,
        env={**os.environ},
    )
    parsed = json.loads(proc.stdout)  # raises if not JSON
    return (proc.returncode == 0 and isinstance(parsed, dict)), f"rc={proc.returncode}"


def _cli_precedence():
    """CLI > config > defaults end-to-end through the real CLI process.

    Writes a defaults JSON and a config JSON, then passes a post-`--` flag that
    must win. ASSUMES the `--defaults`/`--config`/`--` surface pinned in Contract.
    """
    dfd, dpath = tempfile.mkstemp(suffix=".json", dir=ROOT)
    cfd, cpath = tempfile.mkstemp(suffix=".json", dir=ROOT)
    try:
        with os.fdopen(dfd, "w") as f:
            json.dump({"port": 1, "keep": "yes"}, f)
        with os.fdopen(cfd, "w") as f:
            json.dump({"port": 2}, f)
        proc = subprocess.run(
            [sys.executable, "-m", "confstack", "show",
             "--defaults", dpath, "--config", cpath, "--", "--port", "9000"],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
            env={**os.environ},
        )
        parsed = json.loads(proc.stdout)
        port_ok = leaf(parsed, "port") == 9000          # CLI beats config beats defaults
        keep_ok = leaf(parsed, "keep") == "yes"          # untouched default survives
        return (proc.returncode == 0 and port_ok and keep_ok), f"rc={proc.returncode} parsed={parsed!r}"
    finally:
        for p in (dpath, cpath):
            try:
                os.remove(p)
            except OSError:
                pass


if import_ok:
    try:
        ok, detail = _cli_show_json()
    except Exception as e:  # noqa: BLE001
        ok, detail = False, f"{type(e).__name__}: {e}"
    record("cli_show_json", dict(CHECK_SPECS)["cli_show_json"], ok, detail)

    try:
        ok, detail = _cli_precedence()
    except Exception as e:  # noqa: BLE001
        ok, detail = False, f"{type(e).__name__}: {e}"
    record("cli_precedence", dict(CHECK_SPECS)["cli_precedence"], ok, detail)


# --- fixed denominator: on import failure, record EVERY spec as failed -------
if not import_ok:
    for cid, desc in CHECK_SPECS:
        record(cid, desc, False, f"import failed: {import_detail}")

# Safety net: if any spec did not get recorded (e.g. an unexpected early exit in
# the import_ok branch), backfill it as failed so total is always len(CHECK_SPECS).
_recorded = {c["id"] for c in checks}
for cid, desc in CHECK_SPECS:
    if cid not in _recorded:
        record(cid, desc, False, "not run")


passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "confstack",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
