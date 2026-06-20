"""textflow.public — full text justification (stdlib only).

``justify(words, width)`` lays a list of words out into lines of EXACTLY
``width`` characters, the way a typesetter sets a justified paragraph:

* Words are packed greedily: a word joins the current line while it still fits
  (counting one space between adjacent words); otherwise it starts a new line.
* Every line EXCEPT the last is FULLY justified -- the leftover space (after the
  words) is spread across the gaps between words as evenly as possible. When the
  gaps cannot take the spaces evenly, the EXTRA spaces go to the LEFT-most gaps,
  so earlier gaps are (at most one space) wider than later ones.
* A line that holds a SINGLE word has no gap to stretch, so it is left-justified:
  the word followed by enough trailing spaces to reach ``width``.
* The LAST line is left-justified too: words joined by single spaces, then padded
  on the right to ``width`` (this keeps the paragraph's final line ragged-right,
  the conventional look).

Every returned line is exactly ``width`` characters wide.

This is the reference (fixed) solution. It is NOT shown to the model; it exists
only to anchor "correct" and to self-test the held-out grader (``../grade.py``).

Standard library only.
"""

from __future__ import annotations

from typing import List


class JustifyError(ValueError):
    """Raised when justification inputs are malformed or impossible."""


def _pack(words: List[str], width: int) -> List[List[str]]:
    """Greedily group ``words`` into lines that fit within ``width`` (one space
    assumed between adjacent words)."""
    lines: List[List[str]] = []
    cur: List[str] = []
    cur_len = 0  # sum of word lengths on the current line (no spaces yet)
    for w in words:
        # adding ``w`` needs len(cur) spaces (one before each existing word) +
        # the word itself; i.e. cur_len + len(cur) gaps + len(w) <= width.
        if cur and cur_len + len(cur) + len(w) > width:
            lines.append(cur)
            cur = [w]
            cur_len = len(w)
        else:
            cur.append(w)
            cur_len += len(w)
    if cur:
        lines.append(cur)
    return lines


def _distribute(line: List[str], width: int) -> str:
    """Fully justify a multi-word ``line`` to ``width``: spread the leftover
    space across the gaps as evenly as possible, EXTRA spaces to the LEFT gaps."""
    word_chars = sum(len(w) for w in line)
    gaps = len(line) - 1
    total_spaces = width - word_chars
    base, extra = divmod(total_spaces, gaps)
    out = []
    for i, w in enumerate(line[:-1]):
        out.append(w)
        # the first ``extra`` gaps get one more space than the rest
        out.append(" " * (base + (1 if i < extra else 0)))
    out.append(line[-1])
    return "".join(out)


def justify(words: List[str], width: int) -> List[str]:
    """Justify ``words`` into lines of exactly ``width`` characters.

    Returns one string per line. The last line (and any single-word line) is
    left-justified and padded on the right; all other lines are fully justified.
    """
    if not isinstance(words, list):
        raise JustifyError(f"words must be a list, got {type(words).__name__}")
    if not isinstance(width, int) or isinstance(width, bool):
        raise JustifyError("width must be an int")
    if width <= 0:
        raise JustifyError(f"width must be positive, got {width!r}")
    for w in words:
        if not isinstance(w, str):
            raise JustifyError(f"each word must be a str, got {type(w).__name__}")
        if w == "":
            raise JustifyError("words must be non-empty")
        if len(w) > width:
            raise JustifyError(f"word {w!r} is longer than width {width}")

    if not words:
        return []

    lines = _pack(words, width)
    out: List[str] = []
    for idx, line in enumerate(lines):
        is_last = idx == len(lines) - 1
        if is_last or len(line) == 1:
            # left-justify: single spaces between words, pad the right to width
            s = " ".join(line)
            out.append(s + " " * (width - len(s)))
        else:
            out.append(_distribute(line, width))
    return out
