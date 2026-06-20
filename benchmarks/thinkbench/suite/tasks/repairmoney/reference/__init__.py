"""repairmoney — tiny money helpers over integer cents (FIXED reference).

Re-exports the public API from :mod:`repairmoney.public` so both ``repairmoney``
and ``repairmoney.public`` import paths work.

This is the intended, contract-conformant solution. It is NOT shown to the
agent — it exists to self-test the held-out grader (``../grade.py``) and to
anchor what "correct" means for this task.
"""

from .public import format_cents, split_evenly

__all__ = ["format_cents", "split_evenly"]
