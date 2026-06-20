"""Reference orderplane CLI — all output is JSON."""
import argparse
import json
import sys

from .public import (
    admin_order_view,
    cancel_order,
    customer_order_view,
    export_revenue,
    fulfill_order,
    init_db,
    load_catalog,
    place_order,
    warehouse_picklist,
)


def main(argv):
    parser = argparse.ArgumentParser(prog="orderplane")
    sub = parser.add_subparsers(dest="cmd")

    def add_db(p):
        p.add_argument("--db", required=True)

    p = sub.add_parser("init-db"); add_db(p)
    p = sub.add_parser("load-catalog"); add_db(p); p.add_argument("--catalog", required=True)
    p = sub.add_parser("place-order"); add_db(p); p.add_argument("--order", required=True)
    p = sub.add_parser("cancel"); add_db(p); p.add_argument("--order", required=True); p.add_argument("--reason", required=True)
    p = sub.add_parser("fulfill"); add_db(p); p.add_argument("--order", required=True); p.add_argument("--at", required=True)
    p = sub.add_parser("customer-view"); add_db(p); p.add_argument("--order", required=True)
    p = sub.add_parser("admin-view"); add_db(p); p.add_argument("--order", required=True)
    p = sub.add_parser("picklist"); add_db(p); p.add_argument("--date", required=True)
    p = sub.add_parser("export-revenue"); add_db(p); p.add_argument("--month", required=True)

    args = parser.parse_args(argv)
    if not args.cmd:
        print(json.dumps({"error": "no command"}))
        return 2

    if args.cmd == "init-db":
        init_db(args.db)
        result = {"ok": True}
    elif args.cmd == "load-catalog":
        load_catalog(args.db, args.catalog)
        result = {"ok": True}
    elif args.cmd == "place-order":
        with open(args.order) as f:
            order = json.load(f)
        result = place_order(args.db, order)
    elif args.cmd == "cancel":
        result = cancel_order(args.db, args.order, args.reason)
    elif args.cmd == "fulfill":
        result = fulfill_order(args.db, args.order, args.at)
    elif args.cmd == "customer-view":
        result = customer_order_view(args.db, args.order)
    elif args.cmd == "admin-view":
        result = admin_order_view(args.db, args.order)
    elif args.cmd == "picklist":
        result = warehouse_picklist(args.db, args.date)
    elif args.cmd == "export-revenue":
        result = export_revenue(args.db, args.month)
    else:
        print(json.dumps({"error": f"unknown command {args.cmd!r}"}))
        return 2

    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
