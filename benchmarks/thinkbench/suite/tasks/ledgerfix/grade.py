#!/usr/bin/env python3
"""Held-out behavior-level oracle for the bug-fix task `fix_ledger`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never reads the agent's own tests. It grades the produced `ledgerfix`
package against the BRIEF'S CONTRACT (the `ledgerfix` / `ledgerfix.public` API),
NOT against any particular internal file layout.

The planted bug: a NON-ATOMIC transfer. When the source account is underfunded
the debit must be rejected and BOTH balances must stay untouched; the buggy
package instead credits the destination anyway, creating money and unbalancing
the books. The atomicity/conservation checks below fail on the buggy package and
pass on a correct fix.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total over a FIXED denominator), never binary. Exit code is
0 whenever grading ran to completion (even score 0.0); nonzero only on a
grader-internal failure. On import failure the score is forced to 0.0.

This grader creates only in-memory state (no files, no temp dirs, no DBs).
"""
import importlib
import json
import sys

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --- import the produced package (contract: ledgerfix.public) ----------------
import_ok = True
import_detail = ""
pub = None
InsufficientFunds = None
try:
    pub = importlib.import_module("ledgerfix.public")
    # InsufficientFunds is part of the pinned public API; fall back to the base
    # LedgerError, then to any Exception, so a renamed-but-present rejection
    # still lets the atomicity checks run rather than mis-scoring them.
    InsufficientFunds = (
        getattr(pub, "InsufficientFunds", None)
        or getattr(pub, "LedgerError", None)
        or Exception
    )
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def fresh(*accounts):
    """A Ledger with the given (name, opening_cents) accounts opened."""
    led = pub.Ledger()
    for name, opening in accounts:
        led.open_account(name, opening)
    return led


if import_ok:
    # 1. deposit credits the account and returns/reports the new balance.
    def c_deposit():
        led = fresh(("a", 0))
        led.deposit("a", 250)
        return led.balance("a") == 250, f"balance={led.balance('a')!r}"

    check("deposit_basic", "deposit credits the account by the given cents", c_deposit)

    # 2. withdraw debits a sufficiently funded account.
    def c_withdraw():
        led = fresh(("a", 300))
        led.withdraw("a", 120)
        return led.balance("a") == 180, f"balance={led.balance('a')!r}"

    check("withdraw_basic", "withdraw debits a funded account by the given cents", c_withdraw)

    # 3. withdraw on an underfunded account is REJECTED and leaves the balance intact.
    def c_withdraw_rejects():
        led = fresh(("a", 50))
        raised = False
        try:
            led.withdraw("a", 200)
        except InsufficientFunds:
            raised = True
        return (raised and led.balance("a") == 50), f"raised={raised} balance={led.balance('a')!r}"

    check(
        "withdraw_underfunded_rejected",
        "an underfunded withdraw raises and leaves the balance unchanged",
        c_withdraw_rejects,
    )

    # 4. a VALID transfer moves the funds between accounts.
    def c_transfer_valid_moves():
        led = fresh(("a", 100), ("b", 0))
        led.transfer("a", "b", 40)
        return (led.balance("a") == 60 and led.balance("b") == 40), \
            f"a={led.balance('a')!r} b={led.balance('b')!r}"

    check("transfer_valid_moves", "a fully-funded transfer moves the funds", c_transfer_valid_moves)

    # 5. a VALID transfer conserves the grand total (internal move nets to zero).
    def c_transfer_valid_conserves():
        led = fresh(("a", 100), ("b", 5))
        before = led.total_cents()
        led.transfer("a", "b", 40)
        after = led.total_cents()
        return before == after == 105, f"before={before!r} after={after!r}"

    check(
        "transfer_valid_conserves_total",
        "a fully-funded transfer conserves the grand total",
        c_transfer_valid_conserves,
    )

    # 6. ATOMICITY — an underfunded transfer leaves BOTH balances unchanged.
    #    (This is the core check the planted non-atomic bug fails.)
    def c_transfer_underfunded_atomic():
        led = fresh(("a", 100), ("b", 0))
        try:
            led.transfer("a", "b", 500)  # a only holds 100 -> must be rejected
        except InsufficientFunds:
            pass  # rejection is fine; what matters is NOTHING moved
        a, b = led.balance("a"), led.balance("b")
        return (a == 100 and b == 0), f"a={a!r} b={b!r} (expected a=100 b=0)"

    check(
        "transfer_underfunded_atomic",
        "an underfunded transfer leaves BOTH balances untouched (both legs or neither)",
        c_transfer_underfunded_atomic,
    )

    # 7. CONSERVATION — an underfunded transfer does not create money.
    def c_transfer_underfunded_conserves():
        led = fresh(("a", 100), ("b", 0))
        before = led.total_cents()
        try:
            led.transfer("a", "b", 500)
        except InsufficientFunds:
            pass
        after = led.total_cents()
        return (before == after == 100), f"before={before!r} after={after!r} (no money may be created)"

    check(
        "transfer_underfunded_conserves_total",
        "an underfunded transfer does not change the grand total (no money created)",
        c_transfer_underfunded_conserves,
    )

    # 8. the destination specifically must NOT be credited by a rejected transfer.
    def c_transfer_dest_not_credited():
        led = fresh(("a", 100), ("b", 0))
        try:
            led.transfer("a", "b", 500)
        except InsufficientFunds:
            pass
        return led.balance("b") == 0, f"dest balance={led.balance('b')!r} (must stay 0)"

    check(
        "transfer_underfunded_dest_untouched",
        "a rejected transfer never credits the destination",
        c_transfer_dest_not_credited,
    )

    # 9. an exactly-funded transfer (amount == balance) is allowed and conserves total.
    def c_transfer_exact():
        led = fresh(("a", 100), ("b", 0))
        before = led.total_cents()
        led.transfer("a", "b", 100)
        after = led.total_cents()
        return (led.balance("a") == 0 and led.balance("b") == 100 and before == after), \
            f"a={led.balance('a')!r} b={led.balance('b')!r} before={before!r} after={after!r}"

    check(
        "transfer_exact_funds_ok",
        "a transfer of the full balance succeeds and conserves the total",
        c_transfer_exact,
    )

    # 10. a sequence of mixed ops keeps the books balanced against external flow.
    #     external net = deposits - withdrawals (transfers are internal, net zero).
    def c_books_balanced_sequence():
        led = fresh(("a", 0), ("b", 0), ("c", 0))
        external = 0
        led.deposit("a", 1000); external += 1000
        led.deposit("b", 200); external += 200
        led.transfer("a", "c", 300)           # internal, net 0
        led.withdraw("b", 50); external -= 50
        try:
            led.transfer("c", "a", 999)       # c holds 300 -> rejected, net 0
        except InsufficientFunds:
            pass
        led.transfer("a", "b", 100)           # internal, net 0
        total = led.total_cents()
        return total == external, f"total={total!r} expected_external={external!r}"

    check(
        "books_stay_balanced_over_sequence",
        "after mixed deposits/withdrawals/transfers the total equals net external flow",
        c_books_balanced_sequence,
    )


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 10

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "fix_ledger",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
