"""ledgerfix.public — an in-memory money ledger over integer cents.

The ledger holds a set of named accounts, each with an integer-cent balance.
Money only enters via :meth:`Ledger.deposit` and only leaves via
:meth:`Ledger.withdraw`; :meth:`Ledger.transfer` moves money *between* two
existing accounts and must never change the grand total held by the ledger.

All amounts are non-negative integer cents. Withdrawals and the outbound leg of
a transfer are rejected (with :class:`InsufficientFunds`) when the source
account does not hold enough to cover them — accounts may never go negative.

Standard library only.
"""

from __future__ import annotations

from typing import Dict


class LedgerError(Exception):
    """Base class for all rejected ledger operations."""


class UnknownAccount(LedgerError):
    """An operation referenced an account that was never opened."""


class InsufficientFunds(LedgerError):
    """A debit would overdraw an account (accounts may not go negative)."""


class InvalidAmount(LedgerError):
    """An amount was negative or not an integer number of cents."""


class Ledger:
    """A tiny in-memory double-entry ledger keyed by account name.

    Balances are integer cents. The invariant the ledger upholds is that the
    sum of all account balances changes only by external deposits (which add)
    and withdrawals (which subtract) — internal transfers conserve the total.
    """

    def __init__(self) -> None:
        self._balances: Dict[str, int] = {}

    # -- account lifecycle -------------------------------------------------

    def open_account(self, account: str, opening_cents: int = 0) -> None:
        """Open ``account`` with an optional non-negative opening balance."""
        _check_amount(opening_cents)
        if account in self._balances:
            raise LedgerError(f"account {account!r} already exists")
        self._balances[account] = int(opening_cents)

    def _require(self, account: str) -> None:
        if account not in self._balances:
            raise UnknownAccount(f"account {account!r} was never opened")

    # -- single-account operations ----------------------------------------

    def deposit(self, account: str, amount_cents: int) -> int:
        """Credit ``account`` by ``amount_cents``; return the new balance."""
        _check_amount(amount_cents)
        self._require(account)
        self._balances[account] += int(amount_cents)
        return self._balances[account]

    def withdraw(self, account: str, amount_cents: int) -> int:
        """Debit ``account`` by ``amount_cents``; return the new balance.

        Rejects the debit with :class:`InsufficientFunds` if the account does
        not hold at least ``amount_cents`` — balances may never go negative.
        """
        _check_amount(amount_cents)
        self._require(account)
        if self._balances[account] < amount_cents:
            raise InsufficientFunds(
                f"account {account!r} balance {self._balances[account]} "
                f"cannot cover debit of {amount_cents}"
            )
        self._balances[account] -= int(amount_cents)
        return self._balances[account]

    # -- transfer ----------------------------------------------------------

    def transfer(self, src: str, dst: str, amount_cents: int) -> None:
        """Move ``amount_cents`` from ``src`` to ``dst``.

        Both accounts must already exist. The destination is credited and the
        source is debited; an underfunded source must leave both balances
        untouched.
        """
        _check_amount(amount_cents)
        self._require(src)
        self._require(dst)

        # Credit the destination, then debit the source.
        self.deposit(dst, amount_cents)
        try:
            self.withdraw(src, amount_cents)
        except InsufficientFunds:
            pass

    # -- reporting ---------------------------------------------------------

    def balance(self, account: str) -> int:
        """Return the integer-cent balance of ``account``."""
        self._require(account)
        return self._balances[account]

    def total_cents(self) -> int:
        """Return the conserved grand total of all account balances."""
        return sum(self._balances.values())

    def accounts(self) -> Dict[str, int]:
        """Return a copy of the ``account -> balance_cents`` mapping."""
        return dict(self._balances)


def _check_amount(amount_cents: int) -> None:
    if isinstance(amount_cents, bool) or not isinstance(amount_cents, int):
        raise InvalidAmount(f"amount must be an integer number of cents, got {amount_cents!r}")
    if amount_cents < 0:
        raise InvalidAmount(f"amount must be non-negative, got {amount_cents}")
