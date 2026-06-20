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
  on the right to ``width``.

Every returned line is supposed to be exactly ``width`` characters wide.

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
    space across the gaps as evenly as possible.

    BUG (uneven distribution): when the spaces do not divide evenly across the
    gaps, the leftover ("extra") spaces are pushed onto the RIGHT-most gaps
    instead of the left-most ones, so later gaps come out wider than earlier
    ones -- the reverse of the intended typographic look. Lines whose spaces
    happen to divide evenly (no remainder) are unaffected and still look right.
    """
    word_chars = sum(len(w) for w in line)
    gaps = len(line) - 1
    total_spaces = width - word_chars
    base, extra = divmod(total_spaces, gaps)
    out = []
    for i, w in enumerate(line[:-1]):
        out.append(w)
        # BUG: the last ``extra`` gaps get the surplus, not the first ``extra``.
        out.append(" " * (base + (1 if i >= gaps - extra else 0)))
    out.append(line[-1])
    return "".join(out)


def justify(words: List[str], width: int) -> List[str]:
    """Justify ``words`` into lines of exactly ``width`` characters.

    Returns one string per line. The last line (and any single-word line) is
    supposed to be left-justified and padded on the right; all other lines are
    fully justified.
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
        if len(line) == 1:
            # BUG (single-word line not padded): a lone word is emitted as-is,
            # so the line is SHORTER than ``width`` -- it should be left-padded
            # on the right with spaces out to ``width``.
            out.append(line[0])
        else:
            # BUG (no last-line special case): the final line is fully justified
            # just like the interior lines, stretching its single spaces into
            # wide gaps, instead of being left-justified with single spaces and
            # padded on the right.
            out.append(_distribute(line, width))
    return out
