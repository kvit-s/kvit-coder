#!/usr/bin/env python3
import argparse
import json
import os
import time
import urllib.error
import urllib.request
from pathlib import Path


def project_dir_name(project_id: str, project_root: str) -> str:
    short_hash = project_id[:8] if len(project_id) > 8 else project_id
    base = Path(project_root).name
    if base:
        return f"{base}_{short_hash}"
    return project_id[:12] if len(project_id) > 12 else project_id


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Time a direct CAG /v1/register call for one fixed corpus chunk."
    )
    parser.add_argument("--server", default=os.environ.get("CAG_SERVER", "http://192.168.8.20:8400"))
    parser.add_argument("--project-id", default="timing-eventbus")
    parser.add_argument("--project-root", default="/tmp/cag-smoke-msn1-eventbus")
    parser.add_argument("--chunk-id", default="chunk_0")
    parser.add_argument("--description", default="EventBus smoke subset")
    parser.add_argument("--corpus", required=True, help="Path to fixed corpus text")
    parser.add_argument("--file-count", type=int, default=28)
    parser.add_argument("--tokens", type=int, default=57464)
    parser.add_argument(
        "--cache-dir",
        default="/mnt/d/projects/cag-mcp/kv-cache",
        help="Local view of server cache dir, used only to report resulting .bin size.",
    )
    args = parser.parse_args()

    corpus = Path(args.corpus).read_text(encoding="utf-8")
    payload = {
        "project_id": args.project_id,
        "project_root": args.project_root,
        "chunks": [
            {
                "chunk_id": args.chunk_id,
                "description": args.description,
                "corpus": corpus,
                "file_count": args.file_count,
                "tokens": args.tokens,
            }
        ],
    }

    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        args.server.rstrip("/") + "/v1/register",
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    t0 = time.monotonic()
    try:
        with urllib.request.urlopen(req, timeout=900) as resp:
            raw = resp.read()
            status = resp.status
    except urllib.error.HTTPError as e:
        print(e.read().decode("utf-8", errors="replace"))
        raise
    wall_ms = int((time.monotonic() - t0) * 1000)

    result = json.loads(raw)
    print(json.dumps({"http_status": status, "wall_ms": wall_ms, "response": result}, indent=2))

    cache_path = (
        Path(args.cache_dir)
        / "projects"
        / project_dir_name(args.project_id, args.project_root)
        / f"{args.chunk_id}.bin"
    )
    if cache_path.exists():
        print(json.dumps({"cache_bin": str(cache_path), "cache_bin_bytes": cache_path.stat().st_size}, indent=2))
    else:
        print(json.dumps({"cache_bin": str(cache_path), "cache_bin_exists": False}, indent=2))

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
