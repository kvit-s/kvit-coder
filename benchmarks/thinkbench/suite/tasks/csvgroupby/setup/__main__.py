"""csvgroupby CLI — `python -m csvgroupby "<SQL>" < rows.json`.

Reads a JSON array of row dicts from stdin, runs the query, prints the result
rows as a JSON array to stdout.
"""
import json
import sys

from .public import query


def main(argv):
    if len(argv) < 1:
        print(json.dumps({"error": 'usage: python -m csvgroupby "<SQL>" < rows.json'}))
        return 2
    sql = argv[0]
    try:
        rows = json.load(sys.stdin)
        result = query(rows, sql)
    except Exception as e:  # noqa: BLE001 - surface as a structured error on stderr
        print(json.dumps({"error": f"{type(e).__name__}: {e}"}), file=sys.stderr)
        return 1
    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
