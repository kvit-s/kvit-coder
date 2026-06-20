#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield task `migrato` (SQLite migration runner).

Dropped into the workspace ONLY after the agent stops — the agent never sees it. Grades
the produced package against the BRIEF'S CONTRACT (the `migrato.public` API and the
`python -m migrato` CLI), NOT against the model's own tests and NOT against any particular
internal file layout or bookkeeping-table schema.

Output: ONE JSON scorecard on stdout. Each check is independent, so the score is continuous
(passed / total), never binary. The denominator is FIXED: the same set of checks is recorded
whether or not the package imports — on an import failure every check is recorded as failed
and the score is forced to 0.0. Exit code is 0 whenever grading ran to completion (even score
0.0); nonzero is reserved for a grader-internal failure.

Tolerance: the brief under-specifies bookkeeping-table schema, timestamp format, CLI output
shape, and the exact representation of an error. This oracle DERIVES outcomes (it never
REQUIRES a private key), accepts any contract-conformant representation, and detects the
checksum-mismatch error tolerantly. It is never stricter than brief + Contract. Spots where
it leans on a convention beyond what the Contract pins are marked `# ASSUMES`.
"""
import importlib
import json
import os
import shutil
import subprocess
import sys
import tempfile

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# Every check id the oracle KNOWS ABOUT, declared up front so the denominator is fixed
# regardless of import success (fairness: import failure must not shrink the denominator).
CHECK_IDS = [
    ("discover_order", "discover_migrations returns migrations in ascending NUMERIC order"),
    ("discover_checksum", "discover_migrations exposes a stable sha256-hex checksum per file"),
    ("discover_empty", "an empty migrations directory discovers no migrations"),
    ("apply_ordered", "apply_migrations applies all pending migrations in numeric order"),
    ("apply_effect", "applied up-SQL actually runs against the db (tables created)"),
    ("apply_idempotent", "re-running apply_migrations applies nothing the second time"),
    ("status_shape", "migration_status returns per-file {filename, applied: bool}"),
    ("status_reflects_apply", "migration_status flips applied False->True after apply"),
    ("status_no_table", "migration_status on a fresh db reports all-unapplied, no crash"),
    ("checksum_mismatch", "a changed checksum on an applied migration is signalled (no crash)"),
    ("mismatch_no_partial", "on checksum mismatch, later migrations are NOT applied"),
    ("up_marker", "the `-- migrate:up` section is the SQL that gets executed"),
    ("cli_status_rc0", "`python -m migrato status` runs and exits 0"),
    ("cli_up_rc0", "`python -m migrato up` runs, exits 0, and applies migrations"),
]

results_by_id = {}


def record(cid, ok, detail=""):
    results_by_id[cid] = {"passed": bool(ok), "detail": str(detail or "")}


def check(cid, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


# --- tolerant readers (DERIVE; never REQUIRE a private key) -------------------
def status_entry(entry):
    """Pull (filename, applied_bool) from a migration_status entry tolerantly.

    Contract pins keys `filename` and `applied`; we accept a couple of obvious synonyms so
    a contract-conformant-but-differently-named impl is not unfairly failed on key names.
    """
    if not isinstance(entry, dict):
        return None, None
    fn = entry.get("filename")
    if fn is None:
        fn = entry.get("name") or entry.get("file")
    applied = entry.get("applied")
    if applied is None:
        for k in ("is_applied", "applied?", "done", "ran"):
            if k in entry:
                applied = entry[k]
                break
    return fn, applied


def applied_list(result):
    """Pull the list of filenames applied during an apply_migrations call, tolerantly.

    Contract pins key `applied` as a list of filenames. We also accept a count or a list of
    dicts carrying a filename, so we measure BEHAVIOR (what got applied) not the key shape.
    """
    if not isinstance(result, dict):
        return None
    val = result.get("applied")
    if val is None:
        for k in ("applied_migrations", "migrations_applied", "ran"):
            if k in result:
                val = result[k]
                break
    if val is None:
        return None
    if isinstance(val, int):
        return val  # a count is acceptable behavior evidence
    if isinstance(val, list):
        names = []
        for item in val:
            if isinstance(item, str):
                names.append(item)
            elif isinstance(item, dict):
                names.append(item.get("filename") or item.get("name") or item.get("file"))
        return names
    return None


def signals_error(result):
    """Tolerant detection that an apply_migrations RESULT signals a checksum mismatch.

    The Contract pins this as an ERROR RESULT (not an exception): `error` truthy. We accept
    any top-level key whose name names an error/mismatch/conflict with a truthy value, so a
    conformant impl that calls the key `error`/`errors`/`mismatch`/`conflict`/`ok=False`
    all pass. This is a KEY-level check, not a substring scan of arbitrary content (which
    could false-pass on, e.g., a filename containing the word 'error')."""
    if not isinstance(result, dict):
        return False
    for k, v in result.items():
        kl = str(k).lower()
        if any(tok in kl for tok in ("error", "mismatch", "conflict", "fail")) and v:
            return True
        # an explicit ok/success flag set False is also an error signal
        if kl in ("ok", "success", "succeeded") and v is False:
            return True
    return False


# --- workspace builders ------------------------------------------------------
UP1 = "-- migrate:up\nCREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);\n-- migrate:down\nDROP TABLE users;\n"
UP2 = "-- migrate:up\nALTER TABLE users ADD COLUMN email TEXT;\n-- migrate:down\n"
UP3 = "-- migrate:up\nCREATE TABLE logs (id INTEGER PRIMARY KEY);\n"


def make_migrations(dirpath, files):
    os.makedirs(dirpath, exist_ok=True)
    for name, body in files:
        with open(os.path.join(dirpath, name), "w", encoding="utf-8") as f:
            f.write(body)


# track every temp dir we create so we can guarantee cleanup
_TEMP_DIRS = []


def fresh_dir(prefix):
    d = tempfile.mkdtemp(prefix=prefix, dir=ROOT)
    _TEMP_DIRS.append(d)
    return d


def db_in(d, name="app.db"):
    return os.path.join(d, name)


# --- import the produced package (contract: migrato.public) ------------------
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("migrato.public")
    for fn_name in ("init_migration_table", "discover_migrations", "apply_migrations", "migration_status"):
        if not callable(getattr(pub, fn_name, None)):
            import_ok = False
            import_detail = f"missing callable: migrato.public.{fn_name}"
            break
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    STD = [
        ("002_add_email.sql", UP2),   # deliberately out of file-listing order on disk
        ("001_create_users.sql", UP1),
        ("010_add_logs.sql", UP3),    # 010 must sort AFTER 002 numerically, not lexically vs "002"
    ]
    ORDER_EXPECTED = ["001_create_users.sql", "002_add_email.sql", "010_add_logs.sql"]

    def c_discover_order():
        # Use names where NUMERIC order != lexicographic order (Contract pins numeric:
        # `2_x.sql` < `10_x.sql`). A lexical-sort impl would order 10 before 2 and fail.
        d = fresh_dir("disc_")
        mdir = os.path.join(d, "migrations")
        make_migrations(mdir, [
            ("10_add_logs.sql", UP3),
            ("2_add_email.sql", UP2),
            ("1_create_users.sql", UP1),
        ])
        migs = pub.discover_migrations(mdir)
        names = [(m.get("filename") if isinstance(m, dict) else m) for m in migs]
        want = ["1_create_users.sql", "2_add_email.sql", "10_add_logs.sql"]
        return names == want, f"order={names!r}"

    check("discover_order", c_discover_order)

    def c_discover_checksum():
        import hashlib
        d = fresh_dir("disc2_")
        mdir = os.path.join(d, "migrations")
        make_migrations(mdir, STD)
        migs = pub.discover_migrations(mdir)
        # ASSUMES the Contract-pinned `checksum` key. Verify it is the sha256 hex of the
        # raw file bytes for at least one file (the pinned algorithm).
        target = None
        for m in migs:
            if isinstance(m, dict) and m.get("filename") == "001_create_users.sql":
                target = m
        if target is None:
            return False, f"no checksum-bearing entry: {migs!r}"
        cs = target.get("checksum")
        want = hashlib.sha256(UP1.encode("utf-8")).hexdigest()
        return cs == want, f"checksum={cs!r} want={want!r}"

    check("discover_checksum", c_discover_checksum)

    def c_discover_empty():
        d = fresh_dir("empty_")
        mdir = os.path.join(d, "migrations")
        os.makedirs(mdir, exist_ok=True)
        migs = pub.discover_migrations(mdir)
        return (isinstance(migs, list) and len(migs) == 0), f"discovered={migs!r}"

    check("discover_empty", c_discover_empty)

    def c_apply_ordered():
        d = fresh_dir("apply_")
        mdir = os.path.join(d, "migrations")
        make_migrations(mdir, STD)
        res = pub.apply_migrations(db_in(d), mdir)
        applied = applied_list(res)
        if isinstance(applied, int):
            return applied == 3, f"applied count={applied}"
        return applied == ORDER_EXPECTED, f"applied={applied!r}"

    check("apply_ordered", c_apply_ordered)

    def c_apply_effect():
        import sqlite3
        d = fresh_dir("effect_")
        mdir = os.path.join(d, "migrations")
        make_migrations(mdir, STD)
        dbp = db_in(d)
        pub.apply_migrations(dbp, mdir)
        # DERIVE the effect from the db itself: the up-SQL created `users` with an `email`
        # column and a `logs` table. We never read the impl's bookkeeping table.
        conn = sqlite3.connect(dbp)
        try:
            cols = [r[1] for r in conn.execute("PRAGMA table_info(users)").fetchall()]
            logs = conn.execute(
                "SELECT name FROM sqlite_master WHERE type='table' AND name='logs'"
            ).fetchone()
        finally:
            conn.close()
        ok = ("id" in cols and "email" in cols and logs is not None)
        return ok, f"users_cols={cols!r} logs={logs!r}"

    check("apply_effect", c_apply_effect)

    def c_apply_idempotent():
        d = fresh_dir("idem_")
        mdir = os.path.join(d, "migrations")
        make_migrations(mdir, STD)
        dbp = db_in(d)
        res1 = pub.apply_migrations(dbp, mdir)
        # non-vacuous: the FIRST apply must actually have applied all 3, else idempotency
        # is meaningless (guards against a no-op stub passing this for free).
        applied1 = applied_list(res1)
        first_ok = (applied1 == 3) if isinstance(applied1, int) else (applied1 == ORDER_EXPECTED)
        if not first_ok:
            return False, f"first_apply did not apply all 3: {res1!r}"
        res2 = pub.apply_migrations(dbp, mdir)
        applied2 = applied_list(res2)
        if isinstance(applied2, int):
            ok = applied2 == 0
        else:
            ok = applied2 == []
        # the second call must also not error
        return (ok and not signals_error(res2)), f"second_apply={res2!r}"

    check("apply_idempotent", c_apply_idempotent)

    def c_status_shape():
        d = fresh_dir("sshape_")
        mdir = os.path.join(d, "migrations")
        make_migrations(mdir, STD)
        st = pub.migration_status(db_in(d), mdir)
        if not (isinstance(st, list) and len(st) == 3):
            return False, f"status={st!r}"
        names = []
        for e in st:
            fn, applied = status_entry(e)
            if fn is None or not isinstance(applied, bool):
                return False, f"bad entry={e!r}"
            names.append(fn)
        return names == ORDER_EXPECTED, f"names={names!r}"

    check("status_shape", c_status_shape)

    def c_status_reflects_apply():
        d = fresh_dir("sreflect_")
        mdir = os.path.join(d, "migrations")
        make_migrations(mdir, STD)
        dbp = db_in(d)
        before = pub.migration_status(dbp, mdir)
        pub.apply_migrations(dbp, mdir)
        after = pub.migration_status(dbp, mdir)
        b = {fn: ap for fn, ap in (status_entry(e) for e in before)}
        a = {fn: ap for fn, ap in (status_entry(e) for e in after)}
        # non-vacuous: there must BE 3 migrations whose flags flip False -> True
        # (all([]) is vacuously True, so pin the count to defeat a no-op stub).
        ok = (
            len(b) == 3 and len(a) == 3
            and all(v is False for v in b.values())
            and all(v is True for v in a.values())
        )
        return ok, f"before={b!r} after={a!r}"

    check("status_reflects_apply", c_status_reflects_apply)

    def c_status_no_table():
        # fresh db file that has never been initialized: status must not crash and must
        # report everything unapplied.
        d = fresh_dir("notable_")
        mdir = os.path.join(d, "migrations")
        make_migrations(mdir, STD)
        st = pub.migration_status(db_in(d, "never_init.db"), mdir)
        applieds = [status_entry(e)[1] for e in st]
        return (len(st) == 3 and all(v is False for v in applieds)), f"status={st!r}"

    check("status_no_table", c_status_no_table)

    def c_checksum_mismatch():
        d = fresh_dir("mismatch_")
        mdir = os.path.join(d, "migrations")
        make_migrations(mdir, STD)
        dbp = db_in(d)
        pub.apply_migrations(dbp, mdir)  # apply all 3
        # mutate an already-applied migration on disk -> checksum changes
        with open(os.path.join(mdir, "001_create_users.sql"), "w", encoding="utf-8") as f:
            f.write(UP1 + "\n-- tampered comment changes the bytes\n")
        # Contract: must NOT raise; must signal an error result.
        try:
            res = pub.apply_migrations(dbp, mdir)
        except Exception as e:  # noqa: BLE001
            return False, f"raised instead of error-result: {type(e).__name__}: {e}"
        return signals_error(res), f"result={res!r}"

    check("checksum_mismatch", c_checksum_mismatch)

    def c_mismatch_no_partial():
        # A db with only the FIRST migration applied; then tamper that first file AND add a
        # brand-new later migration. The mismatch on the applied file must stop the run so
        # the new later migration is NOT applied.
        d = fresh_dir("partial_")
        mdir = os.path.join(d, "migrations")
        import sqlite3
        make_migrations(mdir, [("001_create_users.sql", UP1)])
        dbp = db_in(d)
        pub.apply_migrations(dbp, mdir)  # applies 001 only
        # non-vacuous: 001 must genuinely be applied (users table exists), else there is
        # nothing to mismatch and a no-op stub would pass this for free.
        conn = sqlite3.connect(dbp)
        try:
            pre = conn.execute(
                "SELECT name FROM sqlite_master WHERE type='table' AND name='users'"
            ).fetchone()
        finally:
            conn.close()
        if pre is None:
            return False, "001 was not actually applied (no users table); precondition unmet"
        # tamper 001 and introduce a pending 002
        with open(os.path.join(mdir, "001_create_users.sql"), "w", encoding="utf-8") as f:
            f.write(UP1 + "\n-- tampered\n")
        make_migrations(mdir, [("002_add_email.sql", UP2)])
        try:
            res = pub.apply_migrations(dbp, mdir)
        except Exception as e:  # noqa: BLE001
            return False, f"raised: {type(e).__name__}: {e}"
        # behavior: 002 must NOT appear as applied. Derive from db: users.email absent.
        conn = sqlite3.connect(dbp)
        try:
            cols = [r[1] for r in conn.execute("PRAGMA table_info(users)").fetchall()]
        finally:
            conn.close()
        applied = applied_list(res)
        applied_names = applied if isinstance(applied, list) else []
        ok = ("email" not in cols) and ("002_add_email.sql" not in applied_names)
        return ok, f"users_cols={cols!r} applied={applied!r}"

    check("mismatch_no_partial", c_mismatch_no_partial)

    def c_up_marker():
        # A file whose `-- migrate:down` section contains SQL that would ERROR if executed.
        # If the impl correctly runs only the up-section, apply succeeds; if it runs the
        # whole file (incl. down), it errors. Either the up-table exists (down not run) and
        # no error, proving the up-section is what executes.
        import sqlite3
        d = fresh_dir("upmark_")
        mdir = os.path.join(d, "migrations")
        body = (
            "-- migrate:up\n"
            "CREATE TABLE widgets (id INTEGER PRIMARY KEY);\n"
            "-- migrate:down\n"
            "DROP TABLE this_table_does_not_exist;\n"  # would error if down were executed
        )
        make_migrations(mdir, [("001_widgets.sql", body)])
        dbp = db_in(d)
        res = pub.apply_migrations(dbp, mdir)
        if signals_error(res):
            return False, f"down-section appears to have run: {res!r}"
        conn = sqlite3.connect(dbp)
        try:
            row = conn.execute(
                "SELECT name FROM sqlite_master WHERE type='table' AND name='widgets'"
            ).fetchone()
        finally:
            conn.close()
        return (row is not None), f"widgets_table={row!r} res={res!r}"

    check("up_marker", c_up_marker)


# --- CLI checks (contract: python -m migrato status|up --db --migrations) -----
def run_cli(subcmd):
    """Run the CLI in a fresh temp workspace. Returns (ok, detail). Always cleans up."""
    d = fresh_dir("cli_")
    mdir = os.path.join(d, "migrations")
    make_migrations(mdir, [("001_create_users.sql", UP1), ("002_add_email.sql", UP2)])
    dbp = db_in(d, "cli.db")
    proc = subprocess.run(
        [sys.executable, "-m", "migrato", subcmd, "--db", dbp, "--migrations", mdir],
        capture_output=True, text=True, timeout=60, cwd=ROOT,
    )
    return proc, dbp


def c_cli_status():
    proc, _ = run_cli("status")
    return (proc.returncode == 0), f"rc={proc.returncode} err={proc.stderr[-200:]!r}"


def c_cli_up():
    import sqlite3
    proc, dbp = run_cli("up")
    if proc.returncode != 0:
        return False, f"rc={proc.returncode} err={proc.stderr[-200:]!r}"
    # DERIVE the effect: the up command must have created the users table with email.
    if not os.path.exists(dbp):
        return False, f"no db created; rc={proc.returncode}"
    conn = sqlite3.connect(dbp)
    try:
        cols = [r[1] for r in conn.execute("PRAGMA table_info(users)").fetchall()]
    finally:
        conn.close()
    return ("email" in cols), f"users_cols={cols!r}"


if import_ok:
    check("cli_status_rc0", c_cli_status)
    check("cli_up_rc0", c_cli_up)


# --- assemble scorecard with a FIXED denominator -----------------------------
# Every known check id is present. Any id not run (because import failed, or a CLI check was
# skipped) is recorded as a FAILED check so the denominator never shrinks.
checks = []
for cid, desc in CHECK_IDS:
    r = results_by_id.get(cid, {"passed": False, "detail": "not run (import failure)" if not import_ok else "not run"})
    checks.append({"id": cid, "desc": desc, "passed": bool(r["passed"]), "detail": r["detail"]})

# --- cleanup: remove every temp dir (incl. sqlite .db/-wal/-shm + __pycache__) ----
for d in _TEMP_DIRS:
    shutil.rmtree(d, ignore_errors=True)
# defensive: the package may have written __pycache__ next to itself during import; that is
# the harness's job to clean, but our OWN temp dirs are gone above.

passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "migrato",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
