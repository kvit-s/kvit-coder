"""semvercmp -- a Semantic Versioning 2.0 precedence comparator.

The single public entry point is :func:`compare`, which orders two version
strings by SemVer 2.0 *precedence* and returns ``-1``, ``0`` or ``1``.

Precedence is computed field by field:

* ``major``, ``minor`` and ``patch`` are compared numerically, most
  significant first.
* A version WITH a pre-release has LOWER precedence than the otherwise-equal
  version WITHOUT one: ``1.0.0-alpha`` < ``1.0.0``.
* Two pre-releases are compared identifier by identifier, left to right:
    - identifiers made of only digits compare numerically;
    - numeric identifiers always have LOWER precedence than alphanumeric ones;
    - alphanumeric identifiers compare by ASCII (lexical) order;
    - if every shared identifier is equal, the version with MORE identifiers
      has the HIGHER precedence (``1.0.0-alpha`` < ``1.0.0-alpha.1``).
* Build metadata (anything after ``+``) is IGNORED for precedence:
  ``1.0.0+build.1`` and ``1.0.0+build.2`` have equal precedence.

Example
-------
    >>> compare("1.0.0", "2.0.0")
    -1
    >>> compare("1.0.0-alpha", "1.0.0")
    -1
    >>> compare("1.0.0+build.99", "1.0.0")
    0
"""

from __future__ import annotations


def _split(version: str):
    """Break a version string into (major, minor, patch, prerelease-ids).

    Build metadata (the ``+...`` suffix) is stripped and discarded before
    anything else, so it can never influence precedence. The pre-release is
    returned as a list of identifier strings (empty when there is none).
    """
    # Strip build metadata FIRST: it plays no part in precedence.
    plus = version.find("+")
    if plus != -1:
        version = version[:plus]

    dash = version.find("-")
    if dash != -1:
        core = version[:dash]
        pre = version[dash + 1:]
        pre_ids = pre.split(".") if pre else []
    else:
        core = version
        pre_ids = []

    major, minor, patch = (int(p) for p in core.split("."))
    return major, minor, patch, pre_ids


def _cmp_int(a: int, b: int) -> int:
    return (a > b) - (a < b)


def _cmp_ident(a: str, b: str) -> int:
    """Compare two pre-release identifiers per SemVer 2.0 rules."""
    a_num = a.isdigit()
    b_num = b.isdigit()
    if a_num and b_num:
        # Both numeric: compare as numbers (no leading-zero surprises).
        return _cmp_int(int(a), int(b))
    if a_num and not b_num:
        # Numeric identifiers have LOWER precedence than alphanumeric ones.
        return -1
    if b_num and not a_num:
        return 1
    # Both alphanumeric: ASCII / lexical order.
    return (a > b) - (a < b)


def _cmp_prerelease(a_ids, b_ids) -> int:
    """Compare two pre-release identifier lists.

    A non-empty pre-release has lower precedence than an absent one, and when
    every shared identifier is equal the longer list wins.
    """
    # Absence of a pre-release outranks its presence.
    if not a_ids and not b_ids:
        return 0
    if not a_ids:
        return 1   # a is the release, b has a pre-release -> a is higher
    if not b_ids:
        return -1  # a has a pre-release, b is the release -> a is lower

    for x, y in zip(a_ids, b_ids):
        c = _cmp_ident(x, y)
        if c != 0:
            return c
    # All shared identifiers equal: more identifiers -> higher precedence.
    return _cmp_int(len(a_ids), len(b_ids))


def compare(a: str, b: str) -> int:
    """Return -1, 0, or 1 ordering versions ``a`` and ``b`` by SemVer precedence."""
    a_major, a_minor, a_patch, a_pre = _split(a)
    b_major, b_minor, b_patch, b_pre = _split(b)

    for x, y in ((a_major, b_major), (a_minor, b_minor), (a_patch, b_patch)):
        c = _cmp_int(x, y)
        if c != 0:
            return c

    return _cmp_prerelease(a_pre, b_pre)
