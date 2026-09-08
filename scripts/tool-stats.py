#!/usr/bin/env python3
"""Count how often each tool was actually called, across every saved session.

Reads the session directories under ~/.kvit-coder/sessions (or a directory
given on the command line) and prints four things: how many calls each tool
received, which offered tools received none, what the shell was used for, and
the same counts per session.

What a session offered the model is read from the session itself, from the
"settings" line each turn writes. Sessions recorded before that line carried a
tool list contribute their calls but not their offers, so the "never called"
list is against the tools any recorded session is known to have offered.

The point is the second of those. A tool that is registered costs prompt tokens
and a line in the model's list of options whether or not it is ever used, so
knowing which ones go untouched is what tells you whether the tool set matches
the way the model actually works.

Usage:
    scripts/tool-stats.py                      # ~/.kvit-coder/sessions
    scripts/tool-stats.py /path/to/sessions
    scripts/tool-stats.py --old                # include pre-directory sessions

Pass --old to also count the flat <name>.jsonl files that sessions used before
they became directories. Those ran against earlier configurations and, often, a
different model, so they are left out by default: mixing them into the totals
answers a question about the past rather than about the tool set you have now.
"""

import argparse
import collections
import glob
import json
import os
import sys

# Dots in tool names are rewritten to underscores for endpoints that reject
# them, so the same tool appears under two spellings across sessions. Counting
# them separately would split every namespaced tool in half.
def canonical(name):
    return name.replace("_", ".") if "_" in name and "." not in name else name


def tool_calls_in_message(message):
    """Yield the tool names an assistant message asked for."""
    for call in message.get("tool_calls") or []:
        function = call.get("function") or {}
        name = function.get("name") or call.get("name")
        if name:
            yield canonical(name)


def shell_program(message):
    """Yield the program each Shell call in this message runs."""
    for call in message.get("tool_calls") or []:
        function = call.get("function") or {}
        if not canonical(function.get("name") or "").startswith("Shell"):
            continue
        try:
            args = json.loads(function.get("arguments") or "{}")
        except json.JSONDecodeError:
            continue
        command = (args.get("command") or "").strip()
        if command:
            # The first word, without its directory: /usr/bin/grep is grep.
            yield command.split()[0].split("/")[-1]


def read_session_dir(path):
    """Count one session directory.

    Returns (tool counts, shell programs, prompts, tools offered).
    """
    tools = collections.Counter()
    programs = collections.Counter()
    offered = set()
    prompts = 0
    with open(os.path.join(path, "history.jsonl")) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            # tool_result is the authority on what ran: it is written whether
            # the call succeeded or came back an error, and it is written once
            # per call, whereas an assistant message can be replayed.
            if event.get("kind") == "tool_result" and event.get("tool"):
                tools[canonical(event["tool"])] += 1
            if event.get("kind") == "settings":
                offered.update(canonical(n) for n in event.get("tools") or [])
            message = event.get("message")
            if isinstance(message, dict):
                if message.get("role") == "user":
                    prompts += 1
                programs.update(shell_program(message))
    return tools, programs, prompts, offered


def read_flat_session(path):
    """Count one pre-directory session: one llm.Message per line, no events."""
    tools = collections.Counter()
    programs = collections.Counter()
    prompts = 0
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                message = json.loads(line)
            except json.JSONDecodeError:
                continue
            if not isinstance(message, dict):
                continue
            if message.get("role") == "user":
                prompts += 1
            if message.get("role") == "assistant":
                tools.update(tool_calls_in_message(message))
                programs.update(shell_program(message))
    return tools, programs, prompts


def table(counter, label):
    total = sum(counter.values())
    if not total:
        print(f"  (none)")
        return
    for name, n in counter.most_common():
        print(f"  {n:6d}  {100 * n / total:5.1f}%  {name}")
    print(f"  {total:6d}  100.0%  {label}")


def main():
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("sessions", nargs="?",
                        default=os.path.expanduser("~/.kvit-coder/sessions"),
                        help="session directory (default: ~/.kvit-coder/sessions)")
    parser.add_argument("--old", action="store_true",
                        help="also count the flat .jsonl sessions from before session directories")
    args = parser.parse_args()

    if not os.path.isdir(args.sessions):
        sys.exit(f"no such directory: {args.sessions}")

    tools = collections.Counter()
    programs = collections.Counter()
    offered = set()
    per_session = {}
    sessions_recording_offers = 0
    prompts = 0

    for path in sorted(glob.glob(os.path.join(args.sessions, "*/"))):
        if not os.path.exists(os.path.join(path, "history.jsonl")):
            continue
        t, p, n, o = read_session_dir(path)
        tools.update(t)
        programs.update(p)
        offered |= o
        if o:
            sessions_recording_offers += 1
        prompts += n
        per_session[os.path.basename(path.rstrip("/"))] = t

    flat = collections.Counter()
    flat_sessions = 0
    if args.old:
        for path in sorted(glob.glob(os.path.join(args.sessions, "*.jsonl"))):
            t, p, n = read_flat_session(path)
            flat.update(t)
            flat_sessions += 1

    print(f"sessions: {len(per_session)}   instructions: {prompts}   "
          f"tool calls: {sum(tools.values())}")
    print()
    print("=== calls per tool ===")
    table(tools, "TOTAL")

    if offered:
        unused = sorted(name for name in offered if tools.get(name, 0) == 0)
        print()
        print(f"=== offered but never called ({len(unused)} of {len(offered)}) ===")
        print(f"    from the {sessions_recording_offers} session(s) that record what "
              f"was on offer")
        for name in unused:
            print(f"  {name}")
        unoffered = sorted(set(tools) - offered)
        if unoffered:
            print()
            print("=== called in a session that did not record its tool list ===")
            for name in unoffered:
                print(f"  {name}  ({tools[name]})")
    else:
        print()
        print("=== offered but never called ===")
        print("  No session records what it offered. Sessions written before the")
        print("  settings line carried a tool list cannot say, so run one more turn")
        print("  and the next report will have it.")

    print()
    print("=== what Shell was used for (first word of the command) ===")
    table(programs, "TOTAL")

    if args.old:
        print()
        print(f"=== pre-directory sessions ({flat_sessions} files), for comparison only ===")
        table(flat, "TOTAL")

    print()
    print("=== per session ===")
    for name, counts in per_session.items():
        summary = ", ".join(f"{tool}×{n}" for tool, n in counts.most_common())
        print(f"  {name}: {sum(counts.values())} — {summary or 'no tool calls'}")


if __name__ == "__main__":
    main()
