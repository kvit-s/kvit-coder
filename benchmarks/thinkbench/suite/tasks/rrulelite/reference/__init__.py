"""rrulelite — a tiny recurrence-rule expander (stdlib only).

Public API lives in :mod:`rrulelite.public`. ``expand(rule, start, limit)``
walks a recurrence rule forward from ``start`` and returns the list of dates it
generates, capped by ``limit`` and (optionally) by an inclusive ``until`` bound.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import expand, RRuleError

__all__ = ["expand", "RRuleError"]
