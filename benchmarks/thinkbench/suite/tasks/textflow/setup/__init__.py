"""textflow — full text justification (stdlib only).

Public API lives in :mod:`textflow.public`. ``justify(words, width)`` packs a
list of words greedily and lays them out into lines of exactly ``width``
characters, fully justifying every line but the last (which is left-justified).
"""

from .public import justify, JustifyError

__all__ = ["justify", "JustifyError"]
