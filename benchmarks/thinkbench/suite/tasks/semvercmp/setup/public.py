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
    """Break a version string into (major, minor, patch, prerelease-ids)."""
    # Find where the core version (major.minor.patch) ends -- the first '-' or
    # '+' marks the start of the trailing identifiers.
    end = len(version)
    for i, ch in enumerate(version):
        if ch in "-+":
            end = i
            break

    core = version[:end]
    suffix = version[end + 1:] if end < len(version) else ""
    # Everything after the core is treated as dotted identifiers (the '+' that
    # introduces build metadata is normalised to a '.' separator).
    pre_ids = suffix.replace("+", ".").split(".") if suffix else []

    major, minor, patch = (int(p) for p in core.split("."))
    return major, minor, patch, pre_ids


def _cmp_int(a: int, b: int) -> int:
    return (a > b) - (a < b)


def _cmp_ident(a: str, b: str) -> int:
    """Compare two pre-release identifiers."""
    # Order identifiers lexically.
    return (a > b) - (a < b)


def _cmp_prerelease(a_ids, b_ids) -> int:
    """Compare two pre-release identifier lists, left to right."""
    for x, y in zip(a_ids, b_ids):
        c = _cmp_ident(x, y)
        if c != 0:
            return c
    return 0


def compare(a: str, b: str) -> int:
    """Return -1, 0, or 1 ordering versions ``a`` and ``b`` by SemVer precedence."""
    a_major, a_minor, a_patch, a_pre = _split(a)
    b_major, b_minor, b_patch, b_pre = _split(b)

    for x, y in ((a_major, b_major), (a_minor, b_minor), (a_patch, b_patch)):
        c = _cmp_int(x, y)
        if c != 0:
            return c

    return _cmp_prerelease(a_pre, b_pre)
