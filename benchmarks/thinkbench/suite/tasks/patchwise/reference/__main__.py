"""Reference patchwise CLI.

    python -m patchwise diff  old.txt new.txt
    python -m patchwise apply old.txt patch.diff --out new.txt
"""
import sys

from .public import PatchError, apply_patch, unified_diff


def _read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def _write(path, text):
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


def main(argv):
    if not argv:
        sys.stderr.write("usage: patchwise <diff|apply> ...\n")
        return 2
    cmd, rest = argv[0], argv[1:]

    positional, out_path = [], None
    i = 0
    while i < len(rest):
        if rest[i] == "--out" and i + 1 < len(rest):
            out_path = rest[i + 1]
            i += 2
        else:
            positional.append(rest[i])
            i += 1

    if cmd == "diff":
        if len(positional) < 2:
            sys.stderr.write("usage: patchwise diff old new\n")
            return 2
        old = _read(positional[0])
        new = _read(positional[1])
        sys.stdout.write(unified_diff(old, new, fromfile=positional[0], tofile=positional[1]))
        return 0
    elif cmd == "apply":
        if len(positional) < 2:
            sys.stderr.write("usage: patchwise apply old patch --out new\n")
            return 2
        old = _read(positional[0])
        patch = _read(positional[1])
        try:
            new = apply_patch(old, patch)
        except PatchError as e:
            sys.stderr.write(f"patch failed: {e}\n")
            return 1
        if out_path:
            _write(out_path, new)
        else:
            sys.stdout.write(new)
        return 0
    else:
        sys.stderr.write(f"unknown command {cmd!r}\n")
        return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
