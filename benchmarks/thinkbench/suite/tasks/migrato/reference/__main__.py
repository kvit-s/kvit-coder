"""Reference migrato CLI: `python -m migrato <status|up> --db <path> --migrations <dir>`."""
import json
import sys

from .public import apply_migrations, migration_status


def _parse(rest):
    db = None
    migrations = None
    i = 0
    while i < len(rest):
        if rest[i] == "--db" and i + 1 < len(rest):
            db = rest[i + 1]
            i += 2
        elif rest[i] == "--migrations" and i + 1 < len(rest):
            migrations = rest[i + 1]
            i += 2
        else:
            i += 1
    return db, migrations


def main(argv):
    if not argv:
        print(json.dumps({"error": "usage: migrato <status|up> --db <path> --migrations <dir>"}))
        return 2
    cmd, rest = argv[0], argv[1:]
    db, migrations = _parse(rest)
    if db is None or migrations is None:
        print(json.dumps({"error": "both --db and --migrations are required"}))
        return 2

    if cmd == "status":
        print(json.dumps({"status": migration_status(db, migrations)}))
        return 0
    if cmd == "up":
        result = apply_migrations(db, migrations)
        print(json.dumps(result))
        return 0
    print(json.dumps({"error": f"unknown command {cmd!r}"}))
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
