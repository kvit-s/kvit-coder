"""unitconv.public — a small unit converter (stdlib only).

``convert(value, from_unit, to_unit)`` converts a numeric ``value`` from one
unit to another, returning a ``float``. Three dimensions are supported:

* LENGTH:  ``"mm"``, ``"m"``, ``"km"``     (canonical base: metre)
* TIME:    ``"s"``, ``"min"``, ``"h"``     (canonical base: second)
* SPEED:   ``"m/s"``, ``"km/h"``           (compound length/time)

A SIMPLE unit converts to its canonical base by a single factor (``km`` -> 1000
metres, ``min`` -> 60 seconds). A COMPOUND unit is written ``"<length>/<time>"``
and its factor-to-base (metres per second) is the LENGTH factor divided by the
TIME factor: ``km/h`` = (1000 m) / (3600 s) = 1000/3600 m/s.

Conversions are only meaningful WITHIN a dimension. Converting across
incompatible dimensions (e.g. a length to a time, or a length to a speed) is an
error and raises :class:`UnitError`.

Standard library only.
"""

from __future__ import annotations


class UnitError(ValueError):
    """Raised when a unit is unknown or the two units are incompatible."""


# Each simple unit maps to (dimension, factor-to-canonical-base).
#   length base = metre, time base = second.
_LENGTH = {"mm": 0.001, "m": 1.0, "km": 1000.0}
_TIME = {"s": 1.0, "min": 60.0, "h": 3600.0}

_SIMPLE = {}
for _u, _f in _LENGTH.items():
    _SIMPLE[_u] = ("length", _f)
for _u, _f in _TIME.items():
    _SIMPLE[_u] = ("time", _f)


def _resolve(unit):
    """Return ``(dimension, factor_to_base)`` for ``unit``.

    A simple unit looks itself up directly. A compound ``"<length>/<time>"``
    unit has dimension ``"speed"`` and a factor of (length factor) / (time
    factor): the numerator multiplies, the denominator divides.
    """
    if not isinstance(unit, str):
        raise UnitError(f"unit must be a str, got {type(unit).__name__}")
    unit = unit.strip()

    if "/" in unit:
        parts = unit.split("/")
        if len(parts) != 2:
            raise UnitError(f"malformed compound unit {unit!r}")
        num, den = parts[0].strip(), parts[1].strip()
        if num not in _LENGTH:
            raise UnitError(f"compound numerator must be a length, got {num!r}")
        if den not in _TIME:
            raise UnitError(f"compound denominator must be a time, got {den!r}")
        # numerator multiplies, denominator divides
        return "speed", _LENGTH[num] / _TIME[den]

    if unit in _SIMPLE:
        return _SIMPLE[unit]

    raise UnitError(f"unknown unit {unit!r}")


def convert(value, from_unit, to_unit):
    """Convert ``value`` from ``from_unit`` to ``to_unit`` and return a float.

    Raises :class:`UnitError` if a unit is unknown or if the two units belong to
    different dimensions (e.g. converting a length to a time).
    """
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise UnitError(f"value must be a number, got {type(value).__name__}")

    from_dim, from_factor = _resolve(from_unit)
    to_dim, to_factor = _resolve(to_unit)

    if from_dim != to_dim:
        raise UnitError(
            f"incompatible units: {from_unit!r} is {from_dim}, "
            f"{to_unit!r} is {to_dim}"
        )

    # value -> canonical base -> target unit
    base = value * from_factor
    return float(base / to_factor)
