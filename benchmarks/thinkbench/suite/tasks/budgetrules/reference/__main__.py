"""Reference budgetrules CLI — all output is JSON on stdout."""
import json
import sys

from .public import categorize, summarize


def _load(path):
    with open(path) as f:
        return json.load(f)


def main(argv):
    if not argv:
        print(json.dumps({"error": "usage: budgetrules <categorize|summarize> ..."}))
        return 2
    cmd, rest = argv[0], argv[1:]

    if cmd == "categorize":
        txns_path = rules_path = None
        i = 0
        while i < len(rest):
            if rest[i] == "--transactions" and i + 1 < len(rest):
                txns_path, i = rest[i + 1], i + 2
            elif rest[i] == "--rules" and i + 1 < len(rest):
                rules_path, i = rest[i + 1], i + 2
            else:
                i += 1
        if txns_path is None or rules_path is None:
            print(json.dumps({"error": "categorize needs --transactions and --rules"}))
            return 2
        result = categorize(_load(txns_path), _load(rules_path))
        print(json.dumps(result))
        return 0

    if cmd == "summarize":
        positional = [a for a in rest if not a.startswith("--")]
        if not positional:
            print(json.dumps({"error": "summarize needs a categorized.json path"}))
            return 2
        print(json.dumps(summarize(_load(positional[0]))))
        return 0

    print(json.dumps({"error": f"unknown command {cmd!r}"}))
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
