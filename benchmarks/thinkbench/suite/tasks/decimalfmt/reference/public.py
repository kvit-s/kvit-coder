"""decimalfmt.public — integer-cents money formatting (stdlib only).

Two inverse functions over amounts held as a whole number of *cents* (an int):

* ``format_amount(cents, places=2, sep=",")`` renders the amount as a human
  string: a sign (only when negative) in front, the integer part grouped in
  threes from the RIGHT with ``sep``, then a ``.`` and exactly ``places``
  fractional digits (zero-padded). E.g. ``-1234567 -> "-12,345.67"`` and
  ``5 -> "0.05"``.
* ``parse_amount(s, places=2, sep=",")`` is its inverse: it strips the grouping
  separators and the decimal point, honours a leading ``-``, and returns the
  integer number of cents. ``format_amount`` then ``parse_amount`` round-trips.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).

Standard library only.
"""

from __future__ import annotations


class MoneyError(ValueError):
    """Raised when an amount or format argument is malformed."""


def _check_places(places: int) -> None:
    if not isinstance(places, int) or isinstance(places, bool) or places < 0:
        raise MoneyError(f"places must be a non-negative int, got {places!r}")


def _group(digits: str, sep: str) -> str:
    """Group a run of decimal digits in threes, counting from the RIGHT."""
    parts = []
    while len(digits) > 3:
        parts.append(digits[-3:])
        digits = digits[:-3]
    parts.append(digits)
    return sep.join(reversed(parts))


def format_amount(cents: int, places: int = 2, sep: str = ",") -> str:
    """Render ``cents`` (an integer number of cents) as a grouped decimal string.

    The sign prefixes the whole number; the integer part is grouped in threes
    from the right; the fractional part is exactly ``places`` digits, zero-padded
    on the left (so 5 cents -> ``"0.05"``).
    """
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
        body = grouped + "." + str(frac).rjust(places, "0")
    else:
        body = grouped
    return ("-" + body) if neg else body


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
    text = text.replace(sep, "")  # drop the grouping separators before parsing

    if "." in text:
        whole_str, frac_str = text.split(".", 1)
    else:
        whole_str, frac_str = text, ""
    if "." in frac_str:
        raise MoneyError(f"multiple decimal points in {s!r}")

    # Pad / truncate the fractional run to exactly `places` digits.
    frac_str = (frac_str + "0" * places)[:places]
    whole_str = whole_str or "0"
    if not whole_str.isdigit() or (frac_str and not frac_str.isdigit()):
        raise MoneyError(f"non-numeric amount {s!r}")

    value = int(whole_str) * (10 ** places) + (int(frac_str) if frac_str else 0)
    return -value if neg else value
