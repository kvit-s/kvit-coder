"""datespan — tiny business-day date math (stdlib only).

Public API lives in :mod:`datespan.public`:

* ``add_business_days(start, n)`` advances ``start`` by ``n`` business days
  (Mondays through Fridays; no holiday calendar), normalizing a weekend
  ``start`` onto the next business day first.
* ``business_days_between(a, b)`` counts the business days strictly after ``a``
  up to and including ``b`` (negative when ``a`` is after ``b``).

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import add_business_days, business_days_between

__all__ = ["add_business_days", "business_days_between"]
