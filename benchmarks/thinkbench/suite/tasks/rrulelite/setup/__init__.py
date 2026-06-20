"""rrulelite — a tiny recurrence-rule expander (stdlib only).

Public API lives in :mod:`rrulelite.public`. ``expand(rule, start, limit)``
walks a recurrence rule forward from ``start`` and returns the list of dates it
generates, capped by ``limit`` and (optionally) by an ``until`` bound.
"""

from .public import expand, RRuleError

__all__ = ["expand", "RRuleError"]
