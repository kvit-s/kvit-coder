"""A greedy word-wrap utility.

:func:`wrap` packs whole words onto lines, greedily, so that each emitted line
is at most ``width`` characters wide. Words are separated in the output by a
single space.

Example
-------
    >>> wrap("the quick brown fox", 9)
    ['the quick', 'brown fox']
"""

from __future__ import annotations

from typing import List


def wrap(text: str, width: int) -> List[str]:
    """Greedily wrap ``text`` into lines of at most ``width`` characters.

    Whole words are packed up to ``width``, single-space-joined.
    """
    # Split the text into words on spaces.
    words = text.split(" ")

    lines: List[str] = []
    current = ""  # the line being built

    for word in words:
        if not current:
            # First word on a fresh line.
            current = word
        elif len(current) + 1 + len(word) < width:
            # Word plus a joining space still fits on the current line.
            current += " " + word
        else:
            # Doesn't fit: flush the current line and start a new one with it.
            lines.append(current)
            current = word

    # Flush the final line.
    lines.append(current)
    return lines
