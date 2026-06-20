#!/usr/bin/env python3
"""Held-out behavior-level oracle for the repair-to-green task `unitconv`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never runs the agent's own ``test_*.py``. It grades the produced
``unitconv`` package against the BRIEF'S CONTRACT (the ``unitconv.public``
``convert`` API), NOT against any particular internal file layout.

This is a SUPERSET of the visible test suite: it re-checks the same behaviors on
MORE length / time / compound-speed / incompatible-dimension cases, all with
expected values computed HERE (never read from the agent's tests). The FIXED
reference passes every check; the planted-bug starter fails a discriminating
subset, so a partial fix lands strictly between 0 and 1.

The three planted (and INTERACTING) bugs in the starter ``unitconv.public``:
  1. compound-unit parsing drops the numerator length scale — a compound
     ``"<length>/<time>"`` is built as if the numerator were always plain metres
     (``km/s`` behaves like ``m/s``), so the ``km`` -> 1000 scale is lost;
  2. factor composition is inverted on the denominator side — the time
     denominator is MULTIPLIED into the factor instead of dividing it (metre*sec
     rather than metre/sec), invisible for ``/s`` (factor 1) but wrong for
     ``/min`` and ``/h``;
  3. incompatible-dimension conversions are not rejected — converting a length
     to a time (or to a speed, etc.) silently runs the arithmetic and returns a
     number instead of raising ``UnitError``.

These interact: a ``km/h -> m/s`` conversion needs BOTH the numerator-scale fix
and the denominator-operator fix, and a cross-dimension call needs the guard.

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


# --- import the produced package ---------------------------------------------
# Contract path is ``unitconv.public``; fall back to the package root
# ``unitconv`` so a submission that re-exports ``convert`` from ``__init__`` (but
# moved it off ``public``) is still graded on behavior, not mis-scored.
import_ok = True
import_detail = ""
convert = None
UnitError = None
try:
    try:
        mod = importlib.import_module("unitconv.public")
    except Exception:  # noqa: BLE001 - try the package root as a fallback
        mod = importlib.import_module("unitconv")
    convert = getattr(mod, "convert")
    UnitError = getattr(mod, "UnitError", None)
    if not (isinstance(UnitError, type) and issubclass(UnitError, BaseException)):
        UnitError = Exception
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


# --- a clean, independent reference oracle computed HERE ----------------------
_LENGTH = {"mm": 0.001, "m": 1.0, "km": 1000.0}
_TIME = {"s": 1.0, "min": 60.0, "h": 3600.0}


def _factor(unit):
    """Return ``(dimension, factor_to_base)`` for ``unit`` — the grader's own,
    independent resolver. Numerator multiplies, denominator divides."""
    unit = unit.strip()
    if "/" in unit:
        num, den = unit.split("/")
        num, den = num.strip(), den.strip()
        return "speed", _LENGTH[num] / _TIME[den]
    if unit in _LENGTH:
        return "length", _LENGTH[unit]
    if unit in _TIME:
        return "time", _TIME[unit]
    raise KeyError(unit)


def oracle(value, from_unit, to_unit):
    """Reference conversion, independent of the submission under test."""
    fd, ff = _factor(from_unit)
    td, tf = _factor(to_unit)
    if fd != td:
        raise ValueError(f"incompatible: {from_unit} ({fd}) vs {to_unit} ({td})")
    return float(value * ff / tf)


_TOL = 1e-9


def value_check(value, from_unit, to_unit):
    """Check ``convert`` matches the oracle's value within float tolerance."""

    def _fn():
        want = oracle(value, from_unit, to_unit)
        got = convert(value, from_unit, to_unit)
        if not isinstance(got, float):
            return False, f"convert({value}, {from_unit!r}, {to_unit!r}) returned {type(got).__name__}, expected float"
        ok = abs(got - want) <= _TOL + _TOL * abs(want)
        return ok, f"convert({value}, {from_unit!r}, {to_unit!r}) = {got!r}, expected {want!r}"

    return _fn


def raises_check(from_unit, to_unit):
    """Check ``convert`` raises ``UnitError`` (not returns, not another error)."""

    def _fn():
        try:
            got = convert(1, from_unit, to_unit)
        except UnitError:
            return True, "raised UnitError"
        except Exception as e:  # noqa: BLE001
            return False, f"raised {type(e).__name__}, expected UnitError"
        return False, f"did not raise: returned {got!r}"

    return _fn


if import_ok:
    # --- baseline: simple same-dimension conversions (pass even buggy) --------
    check("len_mm_to_m", "1000 mm -> 1.0 m", value_check(1000, "mm", "m"))
    check("len_km_to_m", "2 km -> 2000 m", value_check(2, "km", "m"))
    check("len_m_to_mm", "1 m -> 1000 mm", value_check(1, "m", "mm"))
    check("len_km_to_mm", "3 km -> 3_000_000 mm", value_check(3, "km", "mm"))
    check("time_min_to_h", "90 min -> 1.5 h", value_check(90, "min", "h"))
    check("time_h_to_s", "1 h -> 3600 s", value_check(1, "h", "s"))
    check("time_s_to_min", "120 s -> 2 min", value_check(120, "s", "min"))
    check("time_h_to_min", "2 h -> 120 min", value_check(2, "h", "min"))

    # --- compound parsing: numerator length scale (BUG 1) --------------------
    check("compound_km_per_s_parse", "2 km/s -> 2000 m/s (km numerator scale kept)",
          value_check(2, "km/s", "m/s"))
    check("compound_m_per_s_to_km_per_s", "3000 m/s -> 3 km/s",
          value_check(3000, "m/s", "km/s"))

    # --- compound composition: denominator divides (BUG 2) -------------------
    check("compound_m_per_h_compose", "3600 m/h -> 1 m/s (per-hour divides by 3600)",
          value_check(3600, "m/h", "m/s"))
    check("compound_m_per_s_to_m_per_h", "1 m/s -> 3600 m/h",
          value_check(1, "m/s", "m/h"))
    check("compound_m_per_min_compose", "120 m/min -> 2 m/s",
          value_check(120, "m/min", "m/s"))

    # --- compound: both bugs together (real speed conversions) ---------------
    check("speed_kmh_to_ms", "36 km/h -> 10 m/s", value_check(36, "km/h", "m/s"))
    check("speed_kmh_to_ms_2", "72 km/h -> 20 m/s", value_check(72, "km/h", "m/s"))
    check("speed_ms_to_kmh", "10 m/s -> 36 km/h", value_check(10, "m/s", "km/h"))
    check("speed_ms_identity", "5 m/s -> 5 m/s", value_check(5, "m/s", "m/s"))
    check("speed_kmh_identity", "1 km/h -> 1 km/h", value_check(1, "km/h", "km/h"))

    # --- incompatible dimensions must raise (BUG 3) --------------------------
    check("incompat_len_to_time", "length -> time raises (m -> s)",
          raises_check("m", "s"))
    check("incompat_time_to_len", "time -> length raises (min -> km)",
          raises_check("min", "km"))
    check("incompat_speed_to_len", "speed -> length raises (m/s -> km)",
          raises_check("m/s", "km"))
    check("incompat_len_to_speed", "length -> speed raises (m -> km/h)",
          raises_check("m", "km/h"))
    check("incompat_time_to_speed", "time -> speed raises (h -> m/s)",
          raises_check("h", "m/s"))

    # --- unknown units raise -------------------------------------------------
    check("unknown_from", "unknown from-unit raises (ly -> m)",
          raises_check("ly", "m"))
    check("unknown_to", "unknown to-unit raises (m -> ly)",
          raises_check("m", "ly"))
    check("unknown_compound_den", "unknown compound denominator raises (m/foo -> m/s)",
          raises_check("m/foo", "m/s"))


# FIXED DENOMINATOR: the suite always has this many checks, even if import failed
# (so a non-importable submission scores 0.0 over the full denominator, never a
# vacuous 0/0). Keep in lock-step with the checks registered above.
TOTAL_CHECKS = 26

passed = sum(1 for c in checks if c["passed"])
total = TOTAL_CHECKS
card = {
    "task": "unitconv",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)
