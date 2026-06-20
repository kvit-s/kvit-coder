"""cronmatch — a tiny 5-field cron matcher (stdlib only).

Public API lives in :mod:`cronmatch.public`. ``matches(cron_expr, dt)`` parses a
5-field cron expression (minute, hour, day-of-month, month, day-of-week) and
returns whether ``dt`` is due under it, supporting ``*``, single values, ranges
``a-b``, steps ``*/n`` and ``a-b/n``, and comma lists.
"""

from .public import matches, CronError

__all__ = ["matches", "CronError"]
