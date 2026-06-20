"""Reference confstack CLI — `python -m confstack show ...` prints merged JSON."""
import json
import os
import sys

from .public import load_config


def _load_json_object(path):
    if not path or not os.path.exists(path):
        return {}
    with open(path) as f:
        data = json.load(f)
    return data if isinstance(data, dict) else {}


def main(argv):
    if not argv or argv[0] != "show":
        print(json.dumps({"error": "usage: confstack show [--defaults f] [--config f] [-- <flags>]"}))
        return 2

    rest = argv[1:]
    defaults_path = None
    config_path = None
    cli_args = []
    i = 0
    while i < len(rest):
        tok = rest[i]
        if tok == "--":
            cli_args = rest[i + 1:]
            break
        if tok == "--defaults" and i + 1 < len(rest):
            defaults_path = rest[i + 1]
            i += 2
        elif tok == "--config" and i + 1 < len(rest):
            config_path = rest[i + 1]
            i += 2
        else:
            i += 1

    defaults = _load_json_object(defaults_path)
    merged = load_config(defaults, config_path, dict(os.environ), cli_args)
    print(json.dumps(merged, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
