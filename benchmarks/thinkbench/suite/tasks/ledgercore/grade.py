#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield Task (ledgercore).

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `ledgercore.public`
API and the `python -m ledgercore` CLI), NOT against the model's own tests and NOT
against any particular internal file layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

Tolerance: the brief under-specifies some return SHAPES (balance dict keys, the
statement-entry shape, how rejection is signalled, the transfer-destination key).
This oracle accepts any contract-conformant representation and checks BEHAVIOR, not
incidental key names. Spots where it assumes a convention the brief does not pin are
marked `# ASSUMES` — those get pinned in the brief when the full suite is built, so
we never grade a guess.
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
_tmp_dbs = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


def fresh_db():
    """A brand-new, initialized sqlite db path owned by this grader."""
    fd, path = tempfile.mkstemp(suffix=".db", dir=ROOT)
    os.close(fd)
    os.remove(path)  # let the impl create it; some impls reject an existing file
    _tmp_dbs.append(path)
    pub.init_db(path)
    return path


def cleanup():
    for path in _tmp_dbs:
        for p in (path, path + "-wal", path + "-shm", path + "-journal"):
            try:
                os.remove(p)
            except OSError:
                pass


# --- tolerant extractors -----------------------------------------------------
MISSING = object()


def balance_of(blob):
    """Pull an integer cents balance out of any conformant balance representation.

    Accepts {"balance_cents": N}, {"balance": N}, {"amount_cents": N}, a bare int,
    or a nested {"account": {...}} / {"data": {...}} wrapper. Behavior over keys.
    """
    if isinstance(blob, bool):
        return MISSING
    if isinstance(blob, int):
        return blob
    if isinstance(blob, dict):
        for k in ("balance_cents", "balance", "amount_cents", "cents", "value"):
            v = blob.get(k)
            if isinstance(v, int) and not isinstance(v, bool):
                return v
        for wrap in ("account", "data", "result"):
            if isinstance(blob.get(wrap), dict):
                inner = balance_of(blob[wrap])
                if inner is not MISSING:
                    return inner
    return MISSING


def get_balance(db, account):
    return balance_of(pub.get_account_balance(db, account))


def was_rejected(callable_):
    """True if an operation was rejected — by raising OR by a returned error blob.

    The brief says debits/transfers/conflicts "must reject" but does not pin HOW.
    We accept either an exception or a result that structurally signals refusal.
    """
    try:
        r = callable_()
    except Exception:  # noqa: BLE001 - a raise is a valid rejection
        return True, "raised"
    # Returned-value rejection: an error/ok=False/rejected marker anywhere.
    if isinstance(r, dict):
        if r.get("ok") is False or r.get("rejected") is True or r.get("accepted") is False:
            return True, f"returned {r!r}"
        for k in ("error", "errors", "rejected", "reason"):
            if r.get(k):
                return True, f"returned {r!r}"
    return False, f"accepted: returned {r!r}"


def opened(db, account, *, overdraft=False, opening=0, eid=None, at="2026-01-01T00:00:00Z"):
    ev = {
        "event_id": eid or f"open_{account}",
        "type": "account_opened",
        "account_id": account,
        "occurred_at": at,
    }
    if overdraft:
        ev["overdraft"] = True
    if opening:
        ev["amount_cents"] = opening
    pub.append_event(db, ev)


def deposit(db, account, cents, eid, at="2026-01-01T01:00:00Z"):
    pub.append_event(
        db,
        {
            "event_id": eid,
            "type": "deposit_posted",
            "account_id": account,
            "occurred_at": at,
            "amount_cents": cents,
        },
    )


# --- import the produced package (contract: ledgercore.public) ---------------
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("ledgercore.public")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # 1. happy path — open + deposit + withdraw lands a correct balance.
    def c_basic_balance():
        db = fresh_db()
        opened(db, "acct_cash")
        deposit(db, "acct_cash", 10000, "d1")
        pub.append_event(
            db,
            {
                "event_id": "w1",
                "type": "withdrawal_posted",
                "account_id": "acct_cash",
                "occurred_at": "2026-01-01T02:00:00Z",
                "amount_cents": 3000,
            },
        )
        bal = get_balance(db, "acct_cash")
        return (bal == 7000), f"balance={bal!r} (want 7000)"

    check("basic_balance", "open+deposit+withdraw yields the correct projected balance", c_basic_balance)

    # 2. idempotency — exact-duplicate replay of an event_id does NOT double-apply.
    def c_idempotent_replay():
        db = fresh_db()
        opened(db, "acct_cash")
        ev = {
            "event_id": "dep_once",
            "type": "deposit_posted",
            "account_id": "acct_cash",
            "occurred_at": "2026-01-01T01:00:00Z",
            "amount_cents": 5000,
        }
        pub.append_event(db, ev)
        pub.append_event(db, dict(ev))  # exact duplicate replay — must be a no-op
        pub.append_event(db, dict(ev))
        bal = get_balance(db, "acct_cash")
        return (bal == 5000), f"balance={bal!r} after 3x same event (want 5000)"

    check("idempotent_replay", "exact-duplicate event_id replays do not double-apply", c_idempotent_replay)

    # 3. idempotency conflict — same event_id, different payload, must reject.
    def c_idempotent_conflict():
        db = fresh_db()
        opened(db, "acct_cash")
        deposit(db, "acct_cash", 5000, "evt_dup")
        rejected, how = was_rejected(
            lambda: pub.append_event(
                db,
                {
                    "event_id": "evt_dup",  # same id...
                    "type": "deposit_posted",
                    "account_id": "acct_cash",
                    "occurred_at": "2026-01-01T01:00:00Z",
                    "amount_cents": 9999,  # ...different payload
                },
            )
        )
        bal = get_balance(db, "acct_cash")
        # Must reject AND must not have applied the conflicting amount.
        return (rejected and bal == 5000), f"{how}; balance={bal!r} (want 5000)"

    check("idempotent_conflict", "a conflicting payload under a used event_id is rejected", c_idempotent_conflict)

    # 4. overdraft rejection — withdrawal beyond balance rejected without overdraft.
    def c_overdraft_reject():
        db = fresh_db()
        opened(db, "acct_cash")  # overdraft OFF
        deposit(db, "acct_cash", 1000, "d1")
        rejected, how = was_rejected(
            lambda: pub.append_event(
                db,
                {
                    "event_id": "over_w",
                    "type": "withdrawal_posted",
                    "account_id": "acct_cash",
                    "occurred_at": "2026-01-01T02:00:00Z",
                    "amount_cents": 5000,  # > 1000
                },
            )
        )
        bal = get_balance(db, "acct_cash")
        return (rejected and bal == 1000), f"{how}; balance={bal!r} (want 1000, unchanged)"

    check("overdraft_reject", "an overdrawing withdrawal is rejected when overdraft is off", c_overdraft_reject)

    # 5. overdraft allowed — same withdrawal succeeds when overdraft is enabled.
    def c_overdraft_allow():
        db = fresh_db()
        opened(db, "acct_od", overdraft=True)
        deposit(db, "acct_od", 1000, "d1")
        pub.append_event(
            db,
            {
                "event_id": "ov_ok",
                "type": "withdrawal_posted",
                "account_id": "acct_od",
                "occurred_at": "2026-01-01T02:00:00Z",
                "amount_cents": 5000,
            },
        )
        bal = get_balance(db, "acct_od")
        return (bal == -4000), f"balance={bal!r} (want -4000 with overdraft on)"

    check("overdraft_allow", "an overdrawing withdrawal succeeds when overdraft is enabled", c_overdraft_allow)

    # 6. atomic transfer success — source debited and dest credited together.
    def c_transfer_atomic_ok():
        db = fresh_db()
        opened(db, "src")
        opened(db, "dst")
        deposit(db, "src", 8000, "d1")
        # ASSUMES the destination of a transfer is named by one of these keys; the
        # brief shows only a single-account event shape, so we probe the common
        # conventions and accept whichever the impl honors.
        ev = {
            "event_id": "xfer1",
            "type": "transfer_posted",
            "account_id": "src",
            "occurred_at": "2026-01-01T03:00:00Z",
            "amount_cents": 3000,
            "counterparty_account_id": "dst",
            "to_account_id": "dst",
            "dest_account_id": "dst",
            "destination_account_id": "dst",
        }
        pub.append_event(db, ev)
        s, d = get_balance(db, "src"), get_balance(db, "dst")
        return (s == 5000 and d == 3000), f"src={s!r} (want 5000), dst={d!r} (want 3000)"

    check("transfer_atomic_ok", "a valid transfer debits source and credits dest", c_transfer_atomic_ok)

    # 7. atomic transfer failure — an underfunded transfer posts NEITHER leg.
    def c_transfer_atomic_fail():
        db = fresh_db()
        opened(db, "src")  # overdraft OFF
        opened(db, "dst")
        deposit(db, "src", 1000, "d1")
        rejected, how = was_rejected(
            lambda: pub.append_event(
                db,
                {
                    "event_id": "xfer_bad",
                    "type": "transfer_posted",
                    "account_id": "src",
                    "occurred_at": "2026-01-01T03:00:00Z",
                    "amount_cents": 9000,  # > 1000, no overdraft
                    "counterparty_account_id": "dst",
                    "to_account_id": "dst",
                    "dest_account_id": "dst",
                    "destination_account_id": "dst",
                },
            )
        )
        s, d = get_balance(db, "src"), get_balance(db, "dst")
        # Atomic: neither side moved.
        return (rejected and s == 1000 and d == 0), f"{how}; src={s!r} (want 1000), dst={d!r} (want 0)"

    check("transfer_atomic_fail", "an underfunded transfer posts neither leg (atomicity)", c_transfer_atomic_fail)

    # 8. replay consistency — replay reconstructs the stored projected balance.
    def c_replay_consistency():
        db = fresh_db()
        opened(db, "a", opening=0)
        opened(db, "b")
        deposit(db, "a", 7000, "d1")
        pub.append_event(
            db,
            {
                "event_id": "f1",
                "type": "fee_charged",
                "account_id": "a",
                "occurred_at": "2026-01-01T02:00:00Z",
                "amount_cents": 500,
            },
        )
        pub.append_event(
            db,
            {
                "event_id": "x1",
                "type": "transfer_posted",
                "account_id": "a",
                "occurred_at": "2026-01-01T03:00:00Z",
                "amount_cents": 1500,
                "counterparty_account_id": "b",
                "to_account_id": "b",
                "dest_account_id": "b",
                "destination_account_id": "b",
            },
        )
        stored = get_balance(db, "a")
        replayed = balance_of(pub.replay_account(db, "a"))
        return (stored == replayed == 5000), f"stored={stored!r} replay={replayed!r} (want 5000)"

    check("replay_consistency", "replay_account reproduces the stored projected balance", c_replay_consistency)

    # 9. event ordering — statement ordered by occurred_at, then insertion order.
    def c_event_ordering():
        db = fresh_db()
        opened(db, "acct", at="2026-01-01T00:00:00Z")
        # Insert out of occurred_at order; also two ties to test insertion-order.
        deposit(db, "acct", 100, "later", at="2026-03-01T00:00:00Z")
        deposit(db, "acct", 200, "earlier", at="2026-02-01T00:00:00Z")
        deposit(db, "acct", 300, "tie_a", at="2026-02-15T00:00:00Z")
        deposit(db, "acct", 400, "tie_b", at="2026-02-15T00:00:00Z")  # same ts, later insert
        stmt = pub.get_account_statement(db, "acct")
        if not isinstance(stmt, list):
            return False, f"statement type={type(stmt).__name__}"
        times, ids = [], []
        for entry in stmt:
            # ASSUMES each entry exposes occurred_at + event_id at top level or in a
            # nested 'event'; accept either.
            src = entry if isinstance(entry, dict) else {}
            nested = src.get("event") if isinstance(src.get("event"), dict) else {}
            times.append(src.get("occurred_at") or nested.get("occurred_at"))
            ids.append(src.get("event_id") or nested.get("event_id"))
        sorted_by_time = times == sorted(times)
        # tie_a (earlier insert) must precede tie_b (later insert) at equal ts.
        tie_ok = ("tie_a" in ids and "tie_b" in ids and ids.index("tie_a") < ids.index("tie_b"))
        return (sorted_by_time and tie_ok), f"order={ids!r} times={times!r}"

    check("event_ordering", "statement is ordered by occurred_at then insertion order", c_event_ordering)

    # 10. trial balance correctness — sums match and reconcile across the ledger.
    def c_trial_balance():
        db = fresh_db()
        opened(db, "x")
        opened(db, "y")
        deposit(db, "x", 10000, "d1")
        pub.append_event(
            db,
            {
                "event_id": "tx",
                "type": "transfer_posted",
                "account_id": "x",
                "occurred_at": "2026-01-01T03:00:00Z",
                "amount_cents": 4000,
                "counterparty_account_id": "y",
                "to_account_id": "y",
                "dest_account_id": "y",
                "destination_account_id": "y",
            },
        )
        tb = pub.export_trial_balance(db)
        # ASSUMES the trial balance exposes per-account balances somewhere and/or a
        # total. We verify the per-account figures against direct balance queries
        # and that the internal transfer nets to zero across the ledger total.
        bx, by = get_balance(db, "x"), get_balance(db, "y")
        if bx != 6000 or by != 4000:
            return False, f"per-account: x={bx!r} y={by!r} (want 6000/4000)"
        # Find the figures inside the trial balance, shape-tolerantly.
        blob = json.dumps(tb, default=str)
        total = None
        if isinstance(tb, dict):
            for k in ("total_cents", "total", "sum_cents", "sum"):
                if isinstance(tb.get(k), int) and not isinstance(tb.get(k), bool):
                    total = tb[k]
                    break
        # The deposit of 10000 is the only external inflow; transfer is internal →
        # the ledger total must be 10000 (= 6000 + 4000).
        total_ok = (total == 10000) if total is not None else ("10000" in blob)
        # Per-account figures should also be discoverable in the trial balance.
        mentions = "6000" in blob and "4000" in blob
        return (total_ok and mentions), f"trial_balance={tb!r}"

    check("trial_balance", "trial balance reconciles per-account sums and ledger total", c_trial_balance)


# --- CLI: grade the CONTRACT (commands + JSON), not the file layout ----------
def run_cli(args, db, stdin_event=None):
    proc = subprocess.run(
        [sys.executable, "-m", "ledgercore", *args],
        capture_output=True, text=True, timeout=60, cwd=ROOT,
    )
    return proc


def c_cli_roundtrip():
    db = tempfile.mkstemp(suffix=".db", dir=ROOT)
    os.close(db[0])
    db = db[1]
    os.remove(db)
    _tmp_dbs.append(db)
    # init-db
    p = run_cli(["init-db", "--db", db], db)
    if p.returncode != 0:
        return False, f"init-db rc={p.returncode} err={p.stderr.strip()[:200]}"
    # append an account_opened then a deposit via --event files
    ev_dir = tempfile.mkdtemp(dir=ROOT)
    open_path = os.path.join(ev_dir, "open.json")
    dep_path = os.path.join(ev_dir, "dep.json")
    with open(open_path, "w") as f:
        json.dump({"event_id": "o1", "type": "account_opened", "account_id": "cli_acct",
                   "occurred_at": "2026-01-01T00:00:00Z"}, f)
    with open(dep_path, "w") as f:
        json.dump({"event_id": "c_d1", "type": "deposit_posted", "account_id": "cli_acct",
                   "occurred_at": "2026-01-01T01:00:00Z", "amount_cents": 4200}, f)
    try:
        p1 = run_cli(["append", "--db", db, "--event", open_path], db)
        p2 = run_cli(["append", "--db", db, "--event", dep_path], db)
        pb = run_cli(["balance", "--db", db, "--account", "cli_acct"], db)
        ps = run_cli(["statement", "--db", db, "--account", "cli_acct"], db)
        pt = run_cli(["trial-balance", "--db", db], db)
        for name, p in (("append-open", p1), ("append-dep", p2), ("balance", pb),
                        ("statement", ps), ("trial-balance", pt)):
            json.loads(p.stdout)  # every command must emit JSON
        bal_blob = json.loads(pb.stdout)
        bal = balance_of(bal_blob)
        stmt = json.loads(ps.stdout)
        stmt_ok = isinstance(stmt, list) and len(stmt) >= 1
        return (bal == 4200 and stmt_ok), f"cli balance={bal!r} (want 4200), statement_len_ok={stmt_ok}"
    finally:
        for p in (open_path, dep_path):
            try:
                os.remove(p)
            except OSError:
                pass
        try:
            os.rmdir(ev_dir)
        except OSError:
            pass


check("cli_contract", "the `python -m ledgercore` CLI runs the commands and emits JSON", c_cli_roundtrip)


cleanup()

passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "ledgercore",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
