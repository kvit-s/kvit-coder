# Which tools the model actually uses

kvit-coder gives the model a set of tools for reading files, editing them,
searching, running shell commands, starting background processes and asking the
person a question. Every one of them costs something whether or not it is ever
used. Each tool puts its schema and a paragraph of documentation into the
request, and each adds a line to the list of options the model reads before
deciding what to do next. A tool nobody calls is paid for on every request and
returns nothing.

This document says which tools are being called and which are not, and how to
regenerate the numbers as more sessions accumulate. It does not recommend
removing anything: the sample so far is too small for that, and the point of
writing the procedure down is to be able to decide later on evidence.

## Where the numbers come from

A session is a directory under `~/.kvit-coder/sessions/<name>/`, and its
`history.jsonl` is an append-only record with one JSON object per line. Three
kinds of line matter here:

- `{"kind": "tool_result", "tool": "Read", "ms": 12, ...}`: one per call that
  ran, written whether the call succeeded or came back an error. This is what
  gets counted, because an assistant message can be replayed across turns while
  a result is written once.
- `{"kind": "settings", "model": "...", "tools": ["Read", "Shell", ...]}`:
  written at the start of each turn, recording what the model was offered.
- `{"kind": "message", "message": {...}}`: the conversation itself. The
  assistant messages hold the arguments each tool was called with, which is
  where the breakdown of shell commands below comes from.

Recording the tool list in the `settings` line is what makes "never called"
answerable. The set of registered tools depends on `config.yaml`, on whether
background processes are enabled, and on which MCP servers connected, and all
three change between sessions. Counting old calls against today's configuration
would answer a question about today rather than about the sessions being read.

Sessions written before this field existed have no `tools` list. They still
contribute their calls; they cannot contribute to the "never called" list.

There is also an older format: flat `<name>.jsonl` files holding one message per
line, from before a session became a directory. Those ran against earlier
configurations and mostly a different model, so they are excluded by default and
reported separately under `--old`.

## Current state, 8 September 2026

Eight sessions, 10 instructions, 195 tool calls, all recorded between 20:58 on 7
September and 02:07 on 8 September. Every one ran under `agent.profile: strong`
against Muse Spark 1.3 through the opencode.ai endpoint. The work in them was
conversation about kvit-coder's own terminal interface and permission rules.

### Calls per tool

| calls | share | tool |
|---:|---:|---|
| 114 | 58.5% | `Read` |
| 53 | 27.2% | `Shell` |
| 17 | 8.7% | `Edit` |
| 6 | 3.1% | `Search` |
| 2 | 1.0% | `Batch` |
| 1 | 0.5% | `Write` |
| 1 | 0.5% | `Write.confirm` |
| 1 | 0.5% | `Write.cancel` |
| **195** | | **total** |

The two `Write.confirm` and `Write.cancel` calls are a defect since fixed. Those
tools were registered whatever the configuration said, and `Write` staged every
overwrite of an existing file for confirmation, so the model met a
`pending_confirmation` it had no way to answer. Both are now gated on
`edit.preview_mode`, which the strong profile turns off.

### Never called

Nine of the fifteen tools that `config.yaml` registers received no calls at all:

| tool | what it is |
|---|---|
| `Shell.start` | run a command detached, so it outlives the turn |
| `Shell.output` | read what a detached command has printed so far |
| `Shell.status` | is it still running, and with what exit code |
| `Shell.list` | what is running now |
| `Shell.kill` | stop one |
| `Observe.wait` | block until a process exits, matches a pattern, or times out |
| `Observe.add` | run a command on a timer and report when its output changes |
| `Shell.advanced` | `Shell` with a working directory and a timeout |
| `Question` | ask the person a question and get the answer as the tool's result |

Their names do appear in the session files, but every occurrence is file content
the model read (`CLAUDE.md`, `config.yaml`, the root `README.md`) or the
startup banner listing what was available. None is a call.

### What `Shell` is used for

The 53 `Shell` calls, by the first word of the command:

| calls | share | program |
|---:|---:|---|
| 15 | 27.8% | `grep` |
| 10 | 18.5% | `sed` |
| 8 | 14.8% | `go` |
| 5 | 9.3% | `ls` |
| 5 | 9.3% | `git` |
| 3 | 5.6% | `gofmt` |
| 3 | 5.6% | `find` |
| 2 | 3.7% | `cat` |
| 2 | 3.7% | `cp` |
| 1 | 1.9% | `mkdir` |

`grep` through the shell (15) is used more than twice as often as the `Search`
tool (6), and `find` plus `ls` (8) covers directory listing that no tool offers
at all. The system prompt asks the model to prefer a tool over its shell
equivalent where both exist, and on this evidence it does the opposite.

**Caveat: `Search` was broken when these sessions ran.** Until commit
`a750a25` (8 September 2026) the `Search` grep fallback ran plain `grep`
(BRE) while the model writes ripgrep-style extended regex, so a pattern with
`|` alternation plus an escaped paren such as `Open\(` failed with a bare
`search failed: exit status 2` instead of matching — and a bare `a|b`
silently matched the literal string instead of either branch. No `rg` was
installed on this machine, so every `Search` here took that fallback. The 6
vs 15 split above therefore understates the model's willingness to use
`Search`: some of the `Shell`+`grep` calls are retries after a failed
`Search`. When comparing the two, count `tool_result` lines containing
`search failed: exit status 2` and treat the `grep` calls immediately after
them as `Search`-intended.

### The older sessions, for comparison

The 64 flat-file sessions hold 793 calls: `Read` 364, `Shell` 276, `Edit` 44,
`Search` 42, `Write` 39, `Edit.confirm` 15, `Write.confirm` 7,
`mcp.cag.cag_query` 5, `Edit.cancel` 1. The shape is similar, in that reading and
shelling out account for four calls in five, but these ran against a different
model with the edit confirm handshake enabled and a tool set that had no
`Batch`, no `Question` and no background processes, so they say more about the
old edit machinery than about the current tool set.

## What these numbers support, and what they do not

The sample is eight sessions from a single day, on one kind of task. That is
enough to describe what happened and not enough to conclude that a tool is dead.

**Nothing in these sessions called for a background process.** No dev server, no
long test run, no log to watch. The background-process tools going unused is
what you would expect from this workload rather than evidence against them, and
the case for or against them needs sessions that run a build or a server.

**`Question` is a different case.** Several of those turns contained decisions
where the model guessed rather than asked, and a question would have been
reasonable. That bears on the open question in
[`redesign.md`](redesign.md) §17 about whether `Question` is worth a tool given
that a model can already ask by ending its turn. It is still eight sessions.

**`Batch` at 2 calls in 195 is the one figure that already answers something.**
§17 asks how much `Batch` gets used; at 1% the answer so far is close to not at
all. The redesign says that outcome is evidence for the code-execution tool in
appendix A.2 rather than against batching, because a model that discovers each
next call only after seeing the last cannot batch, whatever the prompt says.

**Reading dominates everything.** `Read` is 58% of calls on its own, and
`Read` plus `Shell` is 86%. If that holds, the tools worth attention are the
ones that make reading cheaper: the `index` tool sketched in appendix A.1, and
whatever would make `Search` preferable to `grep`.

## How to update this

```bash
scripts/tool-stats.py            # ~/.kvit-coder/sessions
scripts/tool-stats.py --old      # also count the pre-directory sessions
scripts/tool-stats.py /path/to/other/sessions
```

The script needs nothing but Python 3 and reads the session files directly. Its
output has the same four sections as this document: calls per tool, offered but
never called, what `Shell` was used for, and the same counts per session.

The "offered but never called" section reads the tool list out of each session's
`settings` line. No session recorded before 8 September 2026 has one, so until
the sessions counted here age out, that section will report fewer offered tools
than are actually registered. It says how many sessions it had a list from.

Three things worth doing when you next regenerate:

1. **Say what the sessions were.** The counts mean something different for a
   week of feature work than for a week of conversation about this program's
   own interface, and nothing in the numbers records which it was. One line of
   description per batch is enough.
2. **Keep the old figures.** Add a dated section rather than overwriting this
   one, so a tool that goes from unused to used is visible as a change rather
   than only as a different number.
3. **Split pre- and post-fix sessions for `Search` vs `grep`.** Everything in
   the 8 September snapshot above ran against the broken `Search` fallback
   (fixed in `a750a25`). Sessions from before the fix carry failed `Search`
   calls (`search failed: exit status 2`) and the `Shell`+`grep` retries that
   followed them; sessions after it do not. Count them separately or the
   `Search`-vs-`grep` comparison mixes two different tools under one name.

## What this feeds into

Four decisions are waiting on more of this data, all of them open questions in
[`redesign.md`](redesign.md) §17:

- whether `Question` is worth a tool, or whether ending the turn to ask is
  enough
- whether `Batch` is used enough to keep, and whether its disuse argues for a
  code-execution tool instead
- whether the background-process tools are used once there is work that needs
  them
- whether `Search` should be changed, or the prompt should stop asking the model
  to prefer it, given how much more often `grep` gets reached for (but see the
  caveat above: the 8 September snapshot's gap is confounded by the bug, so
  this wants post-fix sessions before it answers anything)
