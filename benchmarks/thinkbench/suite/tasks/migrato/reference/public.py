"""Reference migrato.public — a standard-library SQLite migration runner.

Contract (see ../brief.txt "## Contract"):
  * checksum = sha256 hex of raw file bytes
  * migrations applied in ascending NUMERIC order of the leading digits of the filename
  * apply_migrations returns {"applied": [filename...], "error": None|truthy}
  * migration_status returns [{"filename": str, "applied": bool}, ...]
  * a checksum mismatch on an already-applied migration is signalled as an ERROR RESULT
    (apply_migrations returns error=truthy, does not raise, stops before later migrations)
"""
import hashlib
import os
import re
import sqlite3
from datetime import datetime, timezone

_TABLE = "schema_migrations"
_LEADING_DIGITS = re.compile(r"^(\d+)")


# --- helpers -----------------------------------------------------------------
def _checksum_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _checksum_file(path: str) -> str:
    with open(path, "rb") as f:
        return _checksum_bytes(f.read())


def _order_key(filename: str):
    """Numeric sort key from the leading digit run, or None if the file is not a migration."""
    m = _LEADING_DIGITS.match(filename)
    if not m:
        return None
    return int(m.group(1))


def _split_up(text: str) -> str:
    """Return the 'up' SQL: the body after `-- migrate:up` (until `-- migrate:down`/EOF),
    or the whole body when no `-- migrate:up` marker is present."""
    up_idx = None
    down_idx = None
    for m in re.finditer(r"(?im)^\s*--\s*migrate:(up|down)\s*$", text):
        if m.group(1).lower() == "up" and up_idx is None:
            up_idx = m.end()
        elif m.group(1).lower() == "down" and down_idx is None:
            down_idx = m.start()
    if up_idx is None:
        return text
    return text[up_idx:down_idx] if (down_idx is not None and down_idx > up_idx) else text[up_idx:]


def _connect(db_path: str) -> sqlite3.Connection:
    return sqlite3.connect(db_path)


def _ensure_table(conn: sqlite3.Connection) -> None:
    conn.execute(
        f"CREATE TABLE IF NOT EXISTS {_TABLE} ("
        "filename TEXT PRIMARY KEY, "
        "checksum TEXT NOT NULL, "
        "applied_at TEXT NOT NULL)"
    )
    conn.commit()


def _table_exists(conn: sqlite3.Connection) -> bool:
    row = conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table' AND name=?", (_TABLE,)
    ).fetchone()
    return row is not None


def _applied_map(conn: sqlite3.Connection) -> dict:
    """filename -> checksum for every recorded migration (empty if table absent)."""
    if not _table_exists(conn):
        return {}
    return {fn: cs for fn, cs in conn.execute(f"SELECT filename, checksum FROM {_TABLE}")}


# --- public API --------------------------------------------------------------
def init_migration_table(db_path: str) -> None:
    conn = _connect(db_path)
    try:
        _ensure_table(conn)
    finally:
        conn.close()


def discover_migrations(migrations_dir: str) -> list:
    out = []
    try:
        names = os.listdir(migrations_dir)
    except (FileNotFoundError, NotADirectoryError):
        return out
    for name in names:
        full = os.path.join(migrations_dir, name)
        if not os.path.isfile(full):
            continue
        if not name.endswith(".sql"):
            continue
        key = _order_key(name)
        if key is None:
            continue
        out.append(
            {
                "filename": name,
                "order": key,
                "checksum": _checksum_file(full),
                "path": full,
            }
        )
    out.sort(key=lambda d: (d["order"], d["filename"]))
    return out


def apply_migrations(db_path: str, migrations_dir: str) -> dict:
    conn = _connect(db_path)
    applied_now = []
    error = None
    try:
        _ensure_table(conn)
        recorded = _applied_map(conn)
        for mig in discover_migrations(migrations_dir):
            fn, checksum, path = mig["filename"], mig["checksum"], mig["path"]
            if fn in recorded:
                if recorded[fn] != checksum:
                    # already-applied migration changed on disk -> error result, stop here.
                    error = {
                        "type": "checksum_mismatch",
                        "filename": fn,
                        "message": f"checksum mismatch for already-applied migration {fn}",
                    }
                    break
                continue  # already applied, unchanged -> idempotent skip
            with open(path, "r", encoding="utf-8") as f:
                up_sql = _split_up(f.read())
            try:
                conn.executescript(up_sql)
                conn.execute(
                    f"INSERT INTO {_TABLE} (filename, checksum, applied_at) VALUES (?, ?, ?)",
                    (fn, checksum, datetime.now(timezone.utc).isoformat()),
                )
                conn.commit()
            except Exception as e:  # noqa: BLE001 - a failing migration rolls back + stops
                conn.rollback()
                error = {
                    "type": "migration_failed",
                    "filename": fn,
                    "message": f"{type(e).__name__}: {e}",
                }
                break
            applied_now.append(fn)
        return {"applied": applied_now, "error": error}
    finally:
        conn.close()


def migration_status(db_path: str, migrations_dir: str) -> list:
    conn = _connect(db_path)
    try:
        recorded = _applied_map(conn)
    finally:
        conn.close()
    status = []
    for mig in discover_migrations(migrations_dir):
        fn, checksum = mig["filename"], mig["checksum"]
        is_applied = fn in recorded
        status.append(
            {
                "filename": fn,
                "applied": bool(is_applied),
                "checksum": checksum,
                "checksum_mismatch": bool(is_applied and recorded[fn] != checksum),
            }
        )
    return status
