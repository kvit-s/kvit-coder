"""repairmoney.public — money helpers over integer cents (FIXED).

Two helpers operate on amounts expressed as a whole number of integer cents
(so $12.34 is the int ``1234``; negative amounts are debts/refunds):

* :func:`format_cents` renders an integer-cent amount as a currency string.
* :func:`split_evenly` splits a total into ``n`` parts that sum to the total.

Standard library only.
"""

from __future__ import annotations

from typing import List


def format_cents(cents: int) -> str:
    """Render an integer-cent amount as a ``$D.DD`` currency string.

    Examples::

        format_cents(1234)   -> "$12.34"
        format_cents(-1234)  -> "-$12.34"
        format_cents(5)      -> "$0.05"
        format_cents(0)      -> "$0.00"

    The sign goes in FRONT of the currency symbol and the cents are always
    zero-padded to two digits.
    """
    if not isinstance(cents, int) or isinstance(cents, bool):
        raise TypeError(f"cents must be an int, got {cents!r}")

    sign = "-" if cents < 0 else ""
    dollars = abs(cents) // 100
    rem = abs(cents) % 100
    return f"{sign}${dollars}.{rem:02d}"


def split_evenly(cents: int, n: int) -> List[int]:
    """Split ``cents`` into ``n`` integer-cent parts that sum to ``cents``.

    The base share is ``cents // n``; any leftover cents are handed out one each
    to the first parts, so the returned parts always sum back to ``cents`` and
    differ from one another by at most one cent.

    Examples::

        split_evenly(1000, 3) -> [334, 333, 333]   # sums to 1000
        split_evenly(1000, 4) -> [250, 250, 250, 250]
        split_evenly(100, 1)  -> [100]
    """
    if not isinstance(n, int) or isinstance(n, bool):
        raise TypeError(f"n must be an int, got {n!r}")
    if n <= 0:
        raise ValueError(f"n must be a positive integer, got {n}")

    base = cents // n
    parts = [base] * n
    remainder = cents - base * n
    for i in range(remainder):
        parts[i] += 1
    return parts
