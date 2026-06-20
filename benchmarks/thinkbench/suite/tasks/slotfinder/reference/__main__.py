"""Reference slotfinder CLI — output is a JSON array of slots.

Usage:
    python -m slotfinder find <request.json>
"""
import json
import sys

from .public import find_slots


def main(argv):
    if len(argv) < 2 or argv[0] != "find":
        print(json.dumps({"error": "usage: slotfinder find <request.json>"}))
        return 2
    path = argv[1]
    with open(path) as f:
        request = json.load(f)
    slots = find_slots(request)
    print(json.dumps(slots))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
