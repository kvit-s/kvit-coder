"""Reference flagwise CLI — all output is JSON on stdout.

    python -m flagwise eval --config flags.json --flag new_checkout --context user.json
    python -m flagwise eval-all --config flags.json --context user.json
"""
import json
import sys

from .public import evaluate_all, evaluate_flag, load_config


def _parse(rest):
    opts = {}
    i = 0
    while i < len(rest):
        tok = rest[i]
        if tok.startswith("--") and i + 1 < len(rest):
            opts[tok[2:]] = rest[i + 1]
            i += 2
        else:
            i += 1
    return opts


def main(argv):
    if not argv:
        print(json.dumps({"error": "usage: flagwise <eval|eval-all> --config f [--flag k] --context f"}))
        return 2
    cmd, opts = argv[0], _parse(argv[1:])

    if "config" not in opts:
        print(json.dumps({"error": "missing --config"}))
        return 2
    config = load_config(opts["config"])
    context = load_config(opts["context"]) if "context" in opts else {}

    if cmd == "eval":
        if "flag" not in opts:
            print(json.dumps({"error": "missing --flag"}))
            return 2
        print(json.dumps(evaluate_flag(config, opts["flag"], context)))
    elif cmd == "eval-all":
        print(json.dumps(evaluate_all(config, context)))
    else:
        print(json.dumps({"error": f"unknown command {cmd!r}"}))
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
