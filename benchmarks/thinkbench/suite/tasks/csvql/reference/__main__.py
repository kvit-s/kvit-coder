"""Reference csvql CLI — `python -m csvql query <path.csv> "<SQL>"`.

Prints the result rows as a JSON array to stdout.
"""
import json
import sys

from .public import query_csv


def main(argv):
    if len(argv) < 3 or argv[0] != "query":
        print(json.dumps({"error": 'usage: csvql query <path.csv> "<SQL query>"'}))
        return 2
    path = argv[1]
    query = argv[2]
    try:
        rows = query_csv(path, query)
    except Exception as e:  # noqa: BLE001 - surface as a structured error on stderr
        print(json.dumps({"error": f"{type(e).__name__}: {e}"}), file=sys.stderr)
        return 1
    print(json.dumps(rows))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
