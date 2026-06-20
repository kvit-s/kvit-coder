"""repaircalc — a tiny arithmetic expression evaluator (no ``eval``).

Public API lives in :mod:`repaircalc.public`. The evaluator supports ``+ - * /``,
parentheses, and integer/decimal literals, with standard operator precedence and
left-to-right associativity.

This is the reference (fixed) solution. It is NOT shown to the model — it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import evaluate, CalcError

__all__ = ["evaluate", "CalcError"]
