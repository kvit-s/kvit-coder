"""datespan — tiny business-day date math (stdlib only).

Public API lives in :mod:`datespan.public`:

* ``add_business_days(start, n)`` advances ``start`` by ``n`` business days
  (Mondays through Fridays; no holiday calendar).
* ``business_days_between(a, b)`` counts business days from ``a`` to ``b``.
"""

from .public import add_business_days, business_days_between

__all__ = ["add_business_days", "business_days_between"]
