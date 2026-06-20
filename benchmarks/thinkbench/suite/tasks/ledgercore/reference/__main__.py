"""Reference ledgercore CLI — all output is JSON."""
import argparse
import json
import sys

from .public import (
    LedgerError,
    append_event,
    export_trial_balance,
    get_account_balance,
    get_account_statement,
    init_db,
)


def main(argv):
    parser = argparse.ArgumentParser(prog="ledgercore")
    sub = parser.add_subparsers(dest="cmd")

    p_init = sub.add_parser("init-db")
    p_init.add_argument("--db", required=True)

    p_append = sub.add_parser("append")
    p_append.add_argument("--db", required=True)
    p_append.add_argument("--event", required=True)

    p_bal = sub.add_parser("balance")
    p_bal.add_argument("--db", required=True)
    p_bal.add_argument("--account", required=True)

    p_stmt = sub.add_parser("statement")
    p_stmt.add_argument("--db", required=True)
    p_stmt.add_argument("--account", required=True)

    p_tb = sub.add_parser("trial-balance")
    p_tb.add_argument("--db", required=True)

    args = parser.parse_args(argv)

    try:
        if args.cmd == "init-db":
            init_db(args.db)
            print(json.dumps({"ok": True, "db": args.db}))
        elif args.cmd == "append":
            with open(args.event) as f:
                event = json.load(f)
            result = append_event(args.db, event)
            print(json.dumps(result))
        elif args.cmd == "balance":
            print(json.dumps(get_account_balance(args.db, args.account)))
        elif args.cmd == "statement":
            print(json.dumps(get_account_statement(args.db, args.account)))
        elif args.cmd == "trial-balance":
            print(json.dumps(export_trial_balance(args.db)))
        else:
            print(json.dumps({"error": "usage: ledgercore <init-db|append|balance|statement|trial-balance>"}))
            return 2
    except LedgerError as e:
        print(json.dumps({"error": type(e).__name__, "message": str(e)}))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
