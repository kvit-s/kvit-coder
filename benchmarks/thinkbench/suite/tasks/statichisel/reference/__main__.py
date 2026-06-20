"""Reference statichisel CLI: `python -m statichisel build <source_dir> <output_dir>`.

Prints the build manifest as JSON on success. Exit code 0 on success, 2 on usage error.
"""
import json
import sys

from .public import build_site


def main(argv):
    if len(argv) < 1:
        print(json.dumps({"error": "usage: statichisel build <source_dir> <output_dir>"}))
        return 2
    cmd, rest = argv[0], argv[1:]
    if cmd != "build":
        print(json.dumps({"error": f"unknown command {cmd!r}"}))
        return 2
    if len(rest) < 2:
        print(json.dumps({"error": "build requires <source_dir> <output_dir>"}))
        return 2
    manifest = build_site(rest[0], rest[1])
    print(json.dumps(manifest))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
