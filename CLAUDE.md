# kvit-coder — notes for an agent working in this repository

## What this program is

A coding agent written in Go. It sends a conversation to an OpenAI-compatible
model endpoint, receives tool calls, runs them against a workspace directory,
feeds the results back, and repeats until the model answers without asking for a
tool.

The target is a capable hosted reasoning model — the default config points at
Muse Spark 1.3 through opencode.ai, over OpenAI's Responses API with a 1M-token
context. It was originally written for models running on local hardware, where
the engineering problem was compensating for models that mis-format tool calls,
repeat themselves, and miscount line numbers. That machinery still exists but is
off by default. **Do not treat the compensation code as the normal path.**

## Build, test, run

```bash
scripts/build.sh         # both binaries into the repository root
go test ./...            # all green as of September 2026; keep it that way
go vet ./...
```

`scripts/build.sh` takes `--release` (stripped, `-trimpath`), `--race`, and a
target name (`coder` or `ui`) to build only one. It is the same two
`go build -o <name> ./cmd/<name>` commands with the git version stamped in.
`scripts/release.sh` is the tagged-release build and refuses a dirty tree.

Go 1.25. Direct dependencies are bubbletea/bubbles/lipgloss for the terminal UI,
zap for logging, yaml.v3, go-difflib, fatih/color, and `mvdan.cc/sh/v3` for
parsing shell commands into a syntax tree.

`./kvit-coder` is headless and needs `-p "<prompt>"` (or `-pq` for just the final
answer). `./kvit-coder-ui` is the interactive front end; it does not link the
agent, it spawns `kvit-coder` as a child process per turn.

`scripts/install.sh` symlinks both binaries into `~/.local/bin`, plus `kc` for
the front end, so a session can be started in any directory. With no `-config`,
both binaries take the first of `$KVIT_CODER_CONFIG`, `./config.yaml`,
`~/.kvit-coder/config.yaml`, and `config.yaml` beside the binary with symlinks
resolved (`internal/config/discover.go`); a `config.yaml` with no `llm:` or
`models:` section is skipped as another program's file. The directory the command
runs in is the workspace, because `workspace.root` is `"."`.

## The two facts that shape everything

**One operating-system process per turn.** The agent starts, reads the session
from disk, runs one instruction, appends what happened, and exits. The driver
starts it again for the next one. Nothing that must outlive a turn may live in
memory — it goes in the session directory. A wedged turn ends when its process
does.

**A session is a directory, not a file:**

```
~/.kvit-coder/sessions/<name>/
    history.jsonl   append-only, one timestamped event per line
    meta.json       created/touched times, workspace, model, first prompt
    checkpoints/    shadow git repo the checkpoint tools commit into
    proc/           pidfiles and logs for background processes
    inbox/          files dropped here reach the model next iteration
    tmp/            tool output too large to put in a message
```

See `internal/session/session.go` for the authoritative version of this.

## Package map

| Package | Lines | What it does |
|---|---:|---|
| `internal/tools` | 12,700 | Tool implementations: read, three edit modes, write, search, shell, plan, checkpoint, tasks, batch, question, background processes |
| `internal/benchmark` | 5,300 | Three benchmark harnesses. Reached only through `cmd/kvit-coder` flags; see `docs/bench-refactor.md` for the plan to split it out |
| `internal/agent` | 2,600 | The run loop, plus the weak-model machinery: loop detection, backtracking, anomaly interrogation |
| `internal/mcp` | 1,600 | Model Context Protocol client, stdio and HTTP transports |
| `internal/safety` | 1,550 | Path-safety modes and their `/dev/tty` prompting |
| `internal/prompt` | 1,400 | System prompt generation from templates in `prompts/` |
| `internal/ui` | 1,100 | Terminal output formatting |
| `internal/config` | 1,050 | YAML config and path permissions |
| `internal/llm` | 890 | HTTP client, chat-completions and Responses protocols, response normalization |
| `internal/checkpoint` | 840 | Turn-based file history in a shadow git repo |
| `internal/context` | 830 | Context window accounting |
| `internal/permissions` | 760 | Shell command permission from the parsed syntax tree |
| `internal/session` | 740 | The session directory above |
| `internal/procs` | 510 | Background process registry |
| `internal/repl`, `internal/tui`, `internal/inbox`, `internal/stats`, `internal/workspace` | <350 each | |

Tools are registered in `internal/tools/setup.go`. Adding one touches that file
and the tool's own file. Registered names use dotted namespaces: `Read`, `Edit`,
`Edit.confirm`, `Search`, `Shell`, `Shell.start`, `Shell.output`, `Observe.wait`,
`Batch`, `Question`, `Web.search`, `Web.fetch`, `Plan.*`, `Checkpoint.*`,
`Tasks.*`, and `mcp.<server>.<tool>`.

## Things that will mislead you

**`agent.profile` decides whether half of `internal/agent` runs.** `strong` is
the default and skips backtracking, the duplicate-call kill switch, the confirm
handshake before an edit applies, scraping tool calls out of prose, fuzzy
matching, indentation repair, anomaly interrogation, and retrying an empty
answer. `weak` turns all of it on. Code in `backtrack.go`, `loopdetector.go` and
`interrogate.go` is live but not on the default path — check the profile gate
before concluding a change there affects anything.

**Tool specs are sorted deterministically on purpose.** The prompt prefix must
stay byte-identical between requests so the server-side cache keeps hitting. Do
not introduce map-iteration order into prompt generation.

**`KVIT_RUN_ID` pins the backend.** The opencode.ai endpoint routes by the
`x-opencode-session` header, so two agents running at once must not share a
value, and reopening a session must reuse its old one or the prompt cache is
cold. `session.RunIDFor` derives it from the session name.

**Path containment is not string prefixing.** Use
`tools.NormalizeAndValidatePath`, not `strings.HasPrefix`, or
`/home/x/project-notes` counts as inside `/home/x/project`.

**Benchmark outputs are untracked and must stay that way.** A run writes
timestamped reports and full transcripts into `benchmarks/`. They were 49 MB of
search noise before September 2026. `.gitignore` covers the patterns; do not
add them back.

**`benchmarks/thinkbench/suite/` is vendored third-party data** (Thinkwright,
Apache-2.0). Its graders and reference solutions must stay physically separate
from any workspace the model can read, or thinkbench scores nothing.

## Documents

`docs/README.md` indexes them. The four that describe the program as it is:
`docs/review.md` (what the code was and what was wrong with it),
`docs/redesign.md` (what was built instead), `docs/redesign-plan.md` (stage by
stage, all done, with what is still open), and the root `README.md` (usage).

`docs/redesign-mcp.md` and `docs/bench-refactor.md` are proposals that nobody has
implemented yet.

`docs/archive/` is superseded material. `docs/archive/tool-improvements.md` in
particular describes weak-model edit machinery as though it were the main path;
it is not, and reading it as current guidance will send you the wrong way.

## Conventions

Commit messages are lowercase and say what changed and why, usually with an area
prefix: `llm: talk to endpoints that only serve the Responses API`,
`benchmarks: keep the inputs, stop tracking the outputs`. Body paragraphs say
why the change was made; the diff already shows what changed.

Commit on `main`. This repository keeps a linear history and does not use topic
branches.
