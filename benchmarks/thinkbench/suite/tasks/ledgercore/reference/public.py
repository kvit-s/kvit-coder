"""Reference ledgercore.public — an event-sourced ledger over SQLite.

Standard library only. The ledger stores an append-only event log and a
materialized per-account balance projection. Both are kept in lock-step inside a
single transaction per appended event, so the stored projection always equals a
fresh replay of the log.

Design notes (contract-level, not pinned by the brief — see grader `# ASSUMES`):

* Money is integer ``amount_cents``; balances are integer cents.
* ``account_opened`` may carry ``overdraft`` (bool) and an optional opening
  ``amount_cents``. Overdraft permission is a per-account flag.
* ``transfer_posted`` moves ``amount_cents`` from ``account_id`` to a destination
  account named by ``counterparty_account_id`` (alias: ``to_account_id`` /
  ``dest_account_id``). It debits the source and credits the destination in one
  transaction — both sides or neither.
* ``deposit_posted`` / ``adjustment_posted`` credit; ``withdrawal_posted`` /
  ``fee_charged`` debit. ``adjustment_posted`` may carry a signed ``amount_cents``.
* Insufficient-funds is rejected for ``withdrawal_posted``, ``fee_charged`` and the
  source leg of ``transfer_posted`` unless the debited account has overdraft on.
"""
import json
import sqlite3


# --- errors -----------------------------------------------------------------
class LedgerError(Exception):
    """Base class for all rejected events."""


class IdempotencyConflict(LedgerError):
    """Same event_id replayed with a different payload."""


class InsufficientFunds(LedgerError):
    """A debit would overdraw an account without overdraft enabled."""


class UnknownAccount(LedgerError):
    """An event references an account that was never opened."""


class InvalidEvent(LedgerError):
    """An event is structurally malformed."""


CREDIT_TYPES = {"deposit_posted"}
DEBIT_TYPES = {"withdrawal_posted", "fee_charged"}
KNOWN_TYPES = {
    "account_opened",
    "deposit_posted",
    "withdrawal_posted",
    "transfer_posted",
    "fee_charged",
    "adjustment_posted",
}


def _connect(db_path):
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


def init_db(db_path):
    conn = _connect(db_path)
    try:
        with conn:
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS events (
                    seq         INTEGER PRIMARY KEY AUTOINCREMENT,
                    event_id    TEXT NOT NULL UNIQUE,
                    type        TEXT NOT NULL,
                    account_id  TEXT NOT NULL,
                    occurred_at TEXT NOT NULL,
                    amount_cents INTEGER,
                    payload     TEXT NOT NULL
                )
                """
            )
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS accounts (
                    account_id    TEXT PRIMARY KEY,
                    balance_cents INTEGER NOT NULL DEFAULT 0,
                    overdraft     INTEGER NOT NULL DEFAULT 0
                )
                """
            )
    finally:
        conn.close()
    return None


def _destination_of(event):
    for k in ("counterparty_account_id", "to_account_id", "dest_account_id", "destination_account_id"):
        if event.get(k):
            return event[k]
    return None


def _canonical(event):
    """A stable JSON serialization of an event for idempotency comparison."""
    return json.dumps(event, sort_keys=True, separators=(",", ":"))


def _require(event, key):
    if key not in event or event[key] is None:
        raise InvalidEvent(f"event missing required field {key!r}")
    return event[key]


def _get_account(conn, account_id):
    return conn.execute(
        "SELECT account_id, balance_cents, overdraft FROM accounts WHERE account_id = ?",
        (account_id,),
    ).fetchone()


def _apply_delta(conn, account_id, delta, allow_overdraft_event=False):
    """Apply a signed cents delta to an account's projection, enforcing overdraft."""
    row = _get_account(conn, account_id)
    if row is None:
        raise UnknownAccount(f"account {account_id!r} was never opened")
    new_balance = row["balance_cents"] + delta
    if delta < 0 and new_balance < 0 and not row["overdraft"]:
        raise InsufficientFunds(
            f"account {account_id!r} balance {row['balance_cents']} cannot absorb {delta}"
        )
    conn.execute(
        "UPDATE accounts SET balance_cents = ? WHERE account_id = ?",
        (new_balance, account_id),
    )


def append_event(db_path, event):
    if not isinstance(event, dict):
        raise InvalidEvent("event must be a dict")
    event_id = _require(event, "event_id")
    etype = _require(event, "type")
    account_id = _require(event, "account_id")
    _require(event, "occurred_at")
    if etype not in KNOWN_TYPES:
        raise InvalidEvent(f"unknown event type {etype!r}")

    conn = _connect(db_path)
    try:
        # Idempotency: an existing event_id with an identical payload is a no-op
        # replay; with a different payload it is a conflict.
        prior = conn.execute(
            "SELECT payload FROM events WHERE event_id = ?", (event_id,)
        ).fetchone()
        if prior is not None:
            if prior["payload"] == _canonical(event):
                return get_account_balance(db_path, account_id)
            raise IdempotencyConflict(
                f"event_id {event_id!r} already recorded with a different payload"
            )

        with conn:  # single atomic transaction for log + projection
            if etype == "account_opened":
                if _get_account(conn, account_id) is not None:
                    # Re-opening with the same event_id was handled above; a *new*
                    # event_id re-opening an existing account is a conflict.
                    raise IdempotencyConflict(f"account {account_id!r} already opened")
                overdraft = 1 if event.get("overdraft") else 0
                opening = int(event.get("amount_cents") or 0)
                conn.execute(
                    "INSERT INTO accounts (account_id, balance_cents, overdraft) VALUES (?, ?, ?)",
                    (account_id, opening, overdraft),
                )

            elif etype in CREDIT_TYPES:
                amount = int(_require(event, "amount_cents"))
                if amount < 0:
                    raise InvalidEvent("deposit amount_cents must be non-negative")
                _apply_delta(conn, account_id, +amount)

            elif etype in DEBIT_TYPES:
                amount = int(_require(event, "amount_cents"))
                if amount < 0:
                    raise InvalidEvent("debit amount_cents must be non-negative")
                _apply_delta(conn, account_id, -amount)

            elif etype == "adjustment_posted":
                # Signed adjustment: positive credits, negative debits.
                amount = int(_require(event, "amount_cents"))
                _apply_delta(conn, account_id, amount)

            elif etype == "transfer_posted":
                amount = int(_require(event, "amount_cents"))
                if amount < 0:
                    raise InvalidEvent("transfer amount_cents must be non-negative")
                dest = _destination_of(event)
                if not dest:
                    raise InvalidEvent("transfer requires a counterparty account")
                # Both legs inside the same transaction: debit source first (this
                # raises InsufficientFunds before any credit lands), then credit
                # dest. Any raise rolls back the whole `with conn` block.
                _apply_delta(conn, account_id, -amount)
                _apply_delta(conn, dest, +amount)

            else:  # pragma: no cover - guarded by KNOWN_TYPES above
                raise InvalidEvent(f"unhandled event type {etype!r}")

            conn.execute(
                "INSERT INTO events (event_id, type, account_id, occurred_at, amount_cents, payload)"
                " VALUES (?, ?, ?, ?, ?, ?)",
                (
                    event_id,
                    etype,
                    account_id,
                    event["occurred_at"],
                    event.get("amount_cents"),
                    _canonical(event),
                ),
            )
    finally:
        conn.close()

    return get_account_balance(db_path, account_id)


def get_account_balance(db_path, account_id):
    conn = _connect(db_path)
    try:
        row = _get_account(conn, account_id)
        if row is None:
            raise UnknownAccount(f"account {account_id!r} was never opened")
        return {
            "account_id": account_id,
            "balance_cents": row["balance_cents"],
            "overdraft": bool(row["overdraft"]),
        }
    finally:
        conn.close()


def _statement_rows(conn, account_id):
    # An account participates in its own events AND the credit leg of inbound
    # transfers. Order by occurred_at, then insertion order (seq).
    return conn.execute(
        """
        SELECT seq, event_id, type, account_id, occurred_at, amount_cents, payload
        FROM events
        WHERE account_id = ?
           OR (type = 'transfer_posted' AND (
                   json_extract(payload, '$.counterparty_account_id') = ?
                OR json_extract(payload, '$.to_account_id') = ?
                OR json_extract(payload, '$.dest_account_id') = ?
                OR json_extract(payload, '$.destination_account_id') = ?
           ))
        ORDER BY occurred_at ASC, seq ASC
        """,
        (account_id, account_id, account_id, account_id, account_id),
    ).fetchall()


def get_account_statement(db_path, account_id):
    conn = _connect(db_path)
    try:
        if _get_account(conn, account_id) is None:
            raise UnknownAccount(f"account {account_id!r} was never opened")
        out = []
        for r in _statement_rows(conn, account_id):
            event = json.loads(r["payload"])
            out.append(
                {
                    "event_id": r["event_id"],
                    "type": r["type"],
                    "account_id": r["account_id"],
                    "occurred_at": r["occurred_at"],
                    "amount_cents": r["amount_cents"],
                    "event": event,
                }
            )
        return out
    finally:
        conn.close()


def _signed_delta_for(account_id, event):
    """The signed cents effect of one event on `account_id`, by replay rules."""
    etype = event.get("type")
    amount = int(event.get("amount_cents") or 0)
    if etype == "account_opened" and event.get("account_id") == account_id:
        return int(event.get("amount_cents") or 0)
    if etype in CREDIT_TYPES and event.get("account_id") == account_id:
        return +amount
    if etype in DEBIT_TYPES and event.get("account_id") == account_id:
        return -amount
    if etype == "adjustment_posted" and event.get("account_id") == account_id:
        return amount
    if etype == "transfer_posted":
        if event.get("account_id") == account_id:
            return -amount
        if _destination_of(event) == account_id:
            return +amount
    return 0


def replay_account(db_path, account_id):
    conn = _connect(db_path)
    try:
        if _get_account(conn, account_id) is None:
            raise UnknownAccount(f"account {account_id!r} was never opened")
        rows = _statement_rows(conn, account_id)
    finally:
        conn.close()

    balance = 0
    count = 0
    for r in rows:
        event = json.loads(r["payload"])
        balance += _signed_delta_for(account_id, event)
        count += 1
    return {
        "account_id": account_id,
        "balance_cents": balance,
        "event_count": count,
    }


def export_trial_balance(db_path):
    conn = _connect(db_path)
    try:
        rows = conn.execute(
            "SELECT account_id, balance_cents FROM accounts ORDER BY account_id"
        ).fetchall()
    finally:
        conn.close()

    accounts = {r["account_id"]: r["balance_cents"] for r in rows}
    total = sum(accounts.values())
    return {
        "accounts": accounts,
        "total_cents": total,
        "balanced": total == _expected_external_total(db_path),
    }


def _expected_external_total(db_path):
    """Sum of net external inflows: deposits + opening balances + adjustments
    minus withdrawals/fees. Transfers are internal and net to zero across the
    ledger, so this must equal the sum of account balances when the projection is
    consistent — that equality is what `balanced` asserts.
    """
    conn = _connect(db_path)
    try:
        rows = conn.execute("SELECT payload FROM events").fetchall()
    finally:
        conn.close()
    total = 0
    for r in rows:
        event = json.loads(r["payload"])
        etype = event.get("type")
        amount = int(event.get("amount_cents") or 0)
        if etype in ("account_opened", "deposit_posted", "adjustment_posted"):
            total += amount
        elif etype in DEBIT_TYPES:
            total -= amount
        # transfer_posted nets to zero across the two legs → ignored
    return total
