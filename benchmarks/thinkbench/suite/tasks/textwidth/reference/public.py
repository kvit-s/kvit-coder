"""A greedy word-wrap utility.

:func:`wrap` packs whole words onto lines, greedily, so that each emitted line
is at most ``width`` characters wide. Words are separated in the output by a
single space. Runs of arbitrary whitespace in the input (spaces, tabs,
newlines, multiple spaces) collapse to single-space word boundaries -- the input
is tokenised purely on whitespace, and only the words matter.

A word that is itself longer than ``width`` cannot fit on any line, so it is
hard-broken across as many full-width pieces as needed (the wrapper never emits
a line wider than ``width`` when ``width >= 1``).

Example
-------
    >>> wrap("the quick brown fox", 9)
    ['the quick', 'brown fox']
    >>> wrap("", 5)
    []
    >>> wrap("supercalifragilistic", 7)
    ['superca', 'lifragi', 'listic']
"""

from __future__ import annotations

from typing import List


def wrap(text: str, width: int) -> List[str]:
    """Greedily wrap ``text`` into lines of at most ``width`` characters.

    Parameters
    ----------
    text:
        The text to wrap. Tokenised on arbitrary whitespace; runs of whitespace
        collapse to a single word boundary.
    width:
        The maximum line width (a positive integer). Whole words are packed up
        to this width, single-space-joined. A word longer than ``width`` is
        hard-broken into ``width``-sized pieces.

    Returns
    -------
    list of str
        The wrapped lines. No line exceeds ``width`` characters (for
        ``width >= 1``). Empty / all-whitespace input yields ``[]`` (no trailing
        empty line). ``width <= 0`` also yields ``[]``.
    """
    # Tokenise on ANY run of whitespace; this collapses multiple spaces, tabs
    # and newlines and drops leading/trailing whitespace entirely.
    words = text.split()
    if width <= 0 or not words:
        return []

    lines: List[str] = []
    current = ""  # the line being built (already single-space-joined)

    for word in words:
        # A word wider than the whole line can never fit; flush whatever we have
        # and hard-break the word into full-width pieces.
        if len(word) > width:
            if current:
                lines.append(current)
                current = ""
            while len(word) > width:
                lines.append(word[:width])
                word = word[width:]
            # The remaining tail (< width, possibly empty) seeds the next line.
            current = word
            continue

        if not current:
            # First word on a fresh line: it fits (len(word) <= width).
            current = word
        elif len(current) + 1 + len(word) <= width:
            # Word plus its joining space still fits on the current line.
            current += " " + word
        else:
            # Doesn't fit: flush the current line and start a new one.
            lines.append(current)
            current = word

    if current:
        lines.append(current)
    return lines
