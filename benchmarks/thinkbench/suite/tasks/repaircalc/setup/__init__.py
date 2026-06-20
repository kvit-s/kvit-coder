"""repaircalc — a tiny arithmetic expression evaluator (no ``eval``).

Public API lives in :mod:`repaircalc.public`. The evaluator is supposed to
support ``+ - * /``, parentheses, and integer/decimal literals, with standard
operator precedence and left-to-right associativity.
"""

from .public import evaluate, CalcError

__all__ = ["evaluate", "CalcError"]
