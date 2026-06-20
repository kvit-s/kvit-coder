"""decimalfmt.public — integer-cents money formatting (stdlib only).

Two inverse functions over amounts held as a whole number of *cents* (an int):

* ``format_amount(cents, places=2, sep=",")`` renders the amount as a human
  string: a sign (only when negative) in front, the integer part grouped in
  threes from the right with ``sep``, then a ``.`` and exactly ``places``
  fractional digits (zero-padded). E.g. ``-1234567 -> "-12,345.67"`` and
  ``5 -> "0.05"``.
* ``parse_amount(s, places=2, sep=",")`` is its inverse: it strips the grouping
  separators and the decimal point, honours a leading ``-``, and returns the
  integer number of cents. ``format_amount`` then ``parse_amount`` round-trips.

Standard library only.
"""

from __future__ import annotations


class MoneyError(ValueError):
    """Raised when an amount or format argument is malformed."""


def _check_places(places: int) -> None:
    if not isinstance(places, int) or isinstance(places, bool) or places < 0:
        raise MoneyError(f"places must be a non-negative int, got {places!r}")


def _group(digits: str, sep: str) -> str:
    """Group a run of decimal digits in threes.

    BUG (grouping direction): this chunks the integer string LEFT-to-right
    instead of from the right, so the FIRST group ends up size-3 and the LAST
    group absorbs the remainder. ``"1234567"`` becomes ``"123,456,7"`` rather
    than the correct ``"1,234,567"``. Only bites once the integer part has 4+
    digits (i.e. >= $1000), so small amounts still look fine.
    """
    parts = [digits[i:i + 3] for i in range(0, len(digits), 3)]
    return sep.join(parts)


def format_amount(cents: int, places: int = 2, sep: str = ",") -> str:
    """Render ``cents`` (an integer number of cents) as a grouped decimal string."""
    if not isinstance(cents, int) or isinstance(cents, bool):
        raise MoneyError(f"cents must be an int, got {type(cents).__name__}")
    _check_places(places)

    neg = cents < 0
    n = -cents if neg else cents
    scale = 10 ** places
    whole = n // scale
    frac = n % scale

    grouped = _group(str(whole), sep)
    if places > 0:
        # BUG (fractional zero-pad): plain ``str(frac)`` drops the leading zero,
        # so 5 cents renders as ``.5`` instead of ``.05`` and a whole-dollar
        # amount renders ``.0`` instead of ``.00``.
        frac_str = str(frac)
        # BUG (sign placement): the minus is glued onto the FRACTIONAL part, i.e.
        # placed after the decimal separator instead of in front of the whole
        # number. ``-1234567`` comes out ``"...-..."`` mid-string rather than a
        # leading ``-``.
        body = grouped + "." + ("-" if neg else "") + frac_str
    else:
        body = ("-" if neg else "") + grouped
    return body


def parse_amount(s: str, places: int = 2, sep: str = ",") -> int:
    """Inverse of :func:`format_amount`: parse a grouped decimal string back to
    an integer number of cents."""
    if not isinstance(s, str):
        raise MoneyError(f"s must be a str, got {type(s).__name__}")
    _check_places(places)

    text = s.strip()
    if not text:
        raise MoneyError("empty string")
    neg = text.startswith("-")
    if neg:
        text = text[1:]
    # BUG (round-trip): the grouping separators are NOT stripped here, so a
    # formatted string that contains ``sep`` fails to parse back to its cents.
    # (Pairs with the grouping bug: even a correctly grouped string won't
    # round-trip until both this and the formatter are fixed.)

    if "." in text:
        whole_str, frac_str = text.split(".", 1)
    else:
        whole_str, frac_str = text, ""
    if "." in frac_str:
        raise MoneyError(f"multiple decimal points in {s!r}")

    frac_str = (frac_str + "0" * places)[:places]
    whole_str = whole_str or "0"
    if not whole_str.isdigit() or (frac_str and not frac_str.isdigit()):
        raise MoneyError(f"non-numeric amount {s!r}")

    value = int(whole_str) * (10 ** places) + (int(frac_str) if frac_str else 0)
    return -value if neg else value
