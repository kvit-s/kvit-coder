"""Reference patchwise.public — line-based unified diff generation and patching.

Standard library only. Generates standard unified diffs (with context lines) and
applies them back, raising a structured ``PatchError`` on context mismatch.

The trailing-newline behavior is preserved through the round trip via the
standard "\\ No newline at end of file" marker.
"""
import difflib

_NO_NEWLINE = "\\ No newline at end of file"


class PatchError(Exception):
    """Raised when a patch cannot be applied (context/removed lines mismatch)."""


def _splitkeep(text):
    """Split into lines, keeping line endings (so trailing-newline info survives)."""
    return text.splitlines(keepends=True)


def unified_diff(old, new, fromfile="old", tofile="new"):
    a = _splitkeep(old)
    b = _splitkeep(new)
    # difflib.unified_diff works on lists of lines; it preserves the missing final
    # newline naturally, but we annotate it with the standard marker so apply_patch
    # can reconstruct trailing-newline behavior unambiguously.
    out = []
    diff = list(difflib.unified_diff(a, b, fromfile=fromfile, tofile=tofile, lineterm="\n"))
    for line in diff:
        out.append(line if line.endswith("\n") else line + "\n")
        if not line.endswith("\n"):
            # a hunk body line without a newline = last line lacks a final newline
            out.append(_NO_NEWLINE + "\n")
    return "".join(out)


def _parse_hunks(patch):
    """Parse a unified diff into a list of hunks.

    Each hunk: {"old_start": int, "lines": [(tag, text_without_newline, has_nl)]}
    tag is ' ', '-', or '+'. Returns [] if there are no hunks.
    """
    lines = patch.splitlines()
    hunks = []
    i = 0
    n = len(lines)
    while i < n:
        line = lines[i]
        if line.startswith("@@"):
            # @@ -l,s +l,s @@
            try:
                old_part = line.split(" ")[1]  # -l,s
                old_start = int(old_part[1:].split(",")[0])
            except (IndexError, ValueError) as e:
                raise PatchError(f"malformed hunk header: {line!r}") from e
            hunk = {"old_start": old_start, "lines": []}
            i += 1
            while i < n:
                hl = lines[i]
                if hl.startswith("@@"):
                    break
                if hl.startswith("--- ") or hl.startswith("+++ "):
                    break
                if hl == _NO_NEWLINE:
                    # mark the previously appended line as lacking a newline
                    if hunk["lines"]:
                        tag, text, _ = hunk["lines"][-1]
                        hunk["lines"][-1] = (tag, text, False)
                    i += 1
                    continue
                if hl and hl[0] in " -+":
                    hunk["lines"].append((hl[0], hl[1:], True))
                    i += 1
                elif hl == "":
                    # a bare empty line inside a hunk = a context blank line
                    hunk["lines"].append((" ", "", True))
                    i += 1
                else:
                    # not part of this hunk
                    break
            hunks.append(hunk)
        else:
            i += 1
    return hunks


def apply_patch(old, patch):
    src = _splitkeep(old)
    # Work with (text_without_newline, had_newline) pairs for precise control.
    src_pairs = []
    for ln in src:
        if ln.endswith("\n"):
            src_pairs.append((ln[:-1], True))
        else:
            src_pairs.append((ln, False))

    hunks = _parse_hunks(patch)
    if not hunks:
        # No hunks at all: an empty/identity patch — return old unchanged.
        return old

    result = []
    src_idx = 0  # index into src_pairs already copied to result
    for hunk in hunks:
        target = hunk["old_start"] - 1  # 1-based -> 0-based; 0 if start was 0
        if target < 0:
            target = 0
        # copy untouched lines before the hunk
        if target > len(src_pairs):
            raise PatchError(
                f"hunk starts at line {hunk['old_start']} but source has "
                f"{len(src_pairs)} lines"
            )
        while src_idx < target:
            result.append(src_pairs[src_idx])
            src_idx += 1

        for tag, text, has_nl in hunk["lines"]:
            if tag == " " or tag == "-":
                if src_idx >= len(src_pairs):
                    raise PatchError(
                        f"context past end of file at hunk line {text!r}"
                    )
                cur_text, cur_nl = src_pairs[src_idx]
                if cur_text != text:
                    raise PatchError(
                        f"context mismatch: expected {text!r}, found {cur_text!r}"
                    )
                if tag == " ":
                    result.append((text, has_nl))
                # consume the source line for both ' ' and '-'
                src_idx += 1
            elif tag == "+":
                result.append((text, has_nl))
    # copy any trailing untouched lines
    while src_idx < len(src_pairs):
        result.append(src_pairs[src_idx])
        src_idx += 1

    parts = []
    for idx, (text, has_nl) in enumerate(result):
        if has_nl:
            parts.append(text + "\n")
        else:
            parts.append(text)
    return "".join(parts)
