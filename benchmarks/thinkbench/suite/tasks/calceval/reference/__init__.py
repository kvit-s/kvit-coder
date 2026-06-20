"""calceval — a tiny infix arithmetic evaluator (stdlib only).

Public API lives in :mod:`calceval.public`. ``evaluate(expr)`` parses an infix
arithmetic expression (``+ - * / ^``, parentheses, unary minus) and returns its
value as a ``float`` — WITHOUT using ``eval``.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import evaluate, CalcError

__all__ = ["evaluate", "CalcError"]
