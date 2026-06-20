"""Reference ticketflow CLI — output is a single JSON object."""
import argparse
import json
import sys

from .public import assign_tickets


def _load(path):
    with open(path) as f:
        return json.load(f)


def main(argv):
    parser = argparse.ArgumentParser(prog="ticketflow", add_help=True)
    sub = parser.add_subparsers(dest="command")

    assign_p = sub.add_parser("assign", help="assign tickets to agents")
    assign_p.add_argument("--config", required=True)
    assign_p.add_argument("--tickets", required=True)
    assign_p.add_argument("--agents", required=True)

    args = parser.parse_args(argv)

    if args.command == "assign":
        config = _load(args.config)
        tickets = _load(args.tickets)
        agents = _load(args.agents)
        result = assign_tickets(config, tickets, agents)
        print(json.dumps(result))
        return 0

    parser.print_help(sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
