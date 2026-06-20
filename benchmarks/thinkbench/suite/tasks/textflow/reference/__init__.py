"""textflow — full text justification (stdlib only).

Public API lives in :mod:`textflow.public`. ``justify(words, width)`` packs a
list of words greedily and lays them out into lines of exactly ``width``
characters, fully justifying every line but the last (which is left-justified).

This is the reference (fixed) solution. It is NOT shown to the model -- it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).
"""

from .public import justify, JustifyError

__all__ = ["justify", "JustifyError"]
