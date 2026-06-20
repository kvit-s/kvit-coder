"""Reference cronsim CLI.

    python -m cronsim next "<expr>" --start <iso> --count <n> [--timezone <tz>]
    python -m cronsim check "<expr>" --at <iso> [--timezone <tz>]

`next` prints one UTC ISO timestamp per line (deterministic order). `check` prints
"true"/"false". Errors print a message to stderr and exit nonzero.
"""
import sys

from .public import next_runs, should_run


def _get_opt(args, name, default=None):
    if name in args:
        i = args.index(name)
        if i + 1 < len(args):
            return args[i + 1]
    return default


def main(argv):
    if not argv:
        print("usage: cronsim <next|check> <expr> [options]", file=sys.stderr)
        return 2
    cmd, rest = argv[0], argv[1:]
    if not rest:
        print("missing cron expression", file=sys.stderr)
        return 2
    expr = rest[0]
    opts = rest[1:]
    tz = _get_opt(opts, "--timezone", "UTC")

    try:
        if cmd == "next":
            start = _get_opt(opts, "--start")
            count = int(_get_opt(opts, "--count", "1"))
            if start is None:
                print("next requires --start", file=sys.stderr)
                return 2
            for ts in next_runs(expr, start, count, tz):
                print(ts)
            return 0
        elif cmd == "check":
            at = _get_opt(opts, "--at")
            if at is None:
                print("check requires --at", file=sys.stderr)
                return 2
            print("true" if should_run(expr, at, tz) else "false")
            return 0
        else:
            print(f"unknown command {cmd!r}", file=sys.stderr)
            return 2
    except Exception as e:  # noqa: BLE001 - surface parse/eval errors as CLI failures
        print(f"error: {type(e).__name__}: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
