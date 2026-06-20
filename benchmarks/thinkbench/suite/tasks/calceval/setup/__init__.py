"""calceval — a tiny infix arithmetic evaluator (stdlib only).

Public API lives in :mod:`calceval.public`. ``evaluate(expr)`` parses an infix
arithmetic expression (``+ - * / ^``, parentheses, unary minus) and returns its
value as a ``float`` — WITHOUT using ``eval``.
"""

from .public import evaluate, CalcError

__all__ = ["evaluate", "CalcError"]
