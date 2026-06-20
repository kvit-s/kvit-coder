"""Reference cellsim CLI — all output is JSON."""
import json
import sys

from .public import evaluate_sheet, explain_cell, get_cell_value, load_sheet


def main(argv):
    if not argv:
        print(json.dumps({"error": "usage: cellsim <eval|cell|explain> sheet.json [CELL]"}))
        return 2
    cmd, rest = argv[0], argv[1:]
    if not rest:
        print(json.dumps({"error": "missing sheet path"}))
        return 2
    sheet = load_sheet(rest[0])

    if cmd == "eval":
        print(json.dumps(evaluate_sheet(sheet)))
        return 0
    if cmd == "cell":
        if len(rest) < 2:
            print(json.dumps({"error": "missing cell name"}))
            return 2
        cell = rest[1]
        try:
            value = get_cell_value(sheet, cell)
            print(json.dumps({"cell": cell, "value": value}))
        except KeyError:
            print(json.dumps({"cell": cell, "value": None, "missing": True}))
        return 0
    if cmd == "explain":
        if len(rest) < 2:
            print(json.dumps({"error": "missing cell name"}))
            return 2
        print(json.dumps(explain_cell(sheet, rest[1])))
        return 0

    print(json.dumps({"error": f"unknown command {cmd!r}"}))
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
