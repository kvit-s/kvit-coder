# kvit-coder

A coding agent in Go. It sends a conversation to an OpenAI-compatible model
endpoint, receives tool calls, runs them against a workspace directory on disk,
feeds the results back, and repeats until the model answers without asking for a
tool. Two binaries: `kvit-coder` runs one turn and exits, which is what you
script against, and `kvit-coder-ui` is the interactive terminal front end.

It is unusual in two ways. The agent process lives for exactly one turn, so
everything that has to survive — the conversation, checkpoints, background
processes, files dropped in mid-turn — is on disk in a session directory rather
than in memory. And it is built to be driven by a small model on your own
hardware as well as by a hosted one: `agent.profile: weak` turns on the
machinery for models that mis-format tool calls and miscount line numbers.

It used 27 MB of resident memory when it was measured against the other
agents on the same machine at the same moment: Claude Code averaged 341 MB
across eight sessions, Codex 261 MB, grok 221 MB. The method and the
per-process tables are in [`docs/agents-ram.md`](docs/agents-ram.md).

<!-- A terminal recording belongs here, generated from a committed vhs tape. -->

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/kvit-s/kvit-coder/main/install.sh | sh
```

That works out which build fits the machine, downloads it from the latest
release, checks it against the release's checksums, and puts both binaries in
`~/.local/bin` under four names: `kc` and `kvit-coder` for the headless agent,
`kcu` and `kvit-coder-ui` for the front end. If there is no configuration yet it
leaves one at `~/.kvit-coder/config.yaml` to edit. `KVIT_BIN_DIR` puts the
binaries somewhere else and `KVIT_VERSION` installs a particular release.

With a Go toolchain, and nothing else:

```bash
go install github.com/kvit-s/kvit-coder/cmd/kvit-coder@latest
go install github.com/kvit-s/kvit-coder/cmd/kvit-coder-ui@latest
```

That gives the two long names and no configuration; copy
[`config.example.yaml`](config.example.yaml) to `~/.kvit-coder/config.yaml`
yourself.

From a clone, which is what to do when changing the program:

```bash
scripts/build.sh          # both binaries into the repository root
scripts/install.sh        # and link them from ~/.local/bin
```

Those links point back into the checkout, so a later `scripts/build.sh` updates
them and `scripts/install.sh --uninstall` removes them.

**Platforms.** Linux is where this is developed and used daily, including under
WSL. The macOS builds compile and are published, but nothing has been run on
them, so treat them as untested. Windows is not supported: the program uses Unix
process groups, signals and file locking, and does not compile for it. Under
Windows, run it inside WSL, which is a tested configuration.

## Configure

Copy [`config.example.yaml`](config.example.yaml) to
`~/.kvit-coder/config.yaml` and point the `llm:` block at an endpoint you have —
a local llama.cpp, vLLM or Ollama server, or any hosted API that speaks the same
protocol:

```yaml
llm:
  base_url: "http://localhost:8080/v1"
  model: "your-model"
  api_backend: "chat_completions"   # or "responses"
  api_key_env: "OPENAI_API_KEY"     # a local server usually needs no key
  context: 131072
```

Without `-config`, both binaries take the first of `$KVIT_CODER_CONFIG`,
`./config.yaml`, `~/.kvit-coder/config.yaml`, and `config.yaml` beside the
binary with symlinks resolved. A file belonging to some other program is passed
over rather than loaded: it has to have an `llm:` or `models:` section to count
as one of ours. Every key is documented in
[`docs/configuration.md`](docs/configuration.md), and this repository's own
`config.yaml` is a larger example with several models, an MCP server and a tool
group.

## Run

```bash
# Interactive. The directory it starts in is the workspace.
kcu

# One turn, headless. This is what you script against.
kvit-coder -p "Find all TODO comments in the codebase"

# Only the final answer on stdout.
kvit-coder -pq "What does main.go do?"
```

A turn that fails exits non-zero and says why. On a terminal the final answer is
rendered as styled markdown; piped output, `--json`, `NO_COLOR` and `TERM=dumb`
stay raw. Every flag is in [`docs/cli.md`](docs/cli.md).

## Why it is built this way

**One process per turn.** `kvit-coder` starts, reads the session from disk, runs
one instruction, appends what happened, and exits. A wedged turn ends when its
process does, the front end holds no agent state, and whatever the turn
allocated goes back to the operating system rather than accumulating in a
long-lived heap.

**A session is a directory**, under `~/.kvit-coder/sessions/<name>/`:

| Entry | Holds |
|---|---|
| `history.jsonl` | Append-only, one timestamped event per line |
| `meta.json` | Created and last-touched times, workspace, model, first prompt |
| `checkpoints/` | The shadow git repository the checkpoint tools commit into |
| `proc/` | Pidfiles and logs for background processes |
| `inbox/` | Files dropped here reach the model on the next iteration |
| `tmp/` | Tool output too large to put in a message |

It is plain files, so a session is also a record you can open months later and
read.

**A running turn can be steered.** A line typed at the terminal, a file dropped
by `kvit-coder steer` from another shell, or an event from a background process
all land in the same inbox, which the loop drains once per iteration. Pressing
Enter on an empty line asks the turn to pause at the next safe point, which is
not the same as cancelling it.

**Shell permissions are decided from the parsed command**, not from patterns
matched against its text. `git diff && rm -rf /` is two commands and is judged as
two, a program being invoked is distinguishable from the same word appearing as
a `grep` pattern, and a blocked command can be granted for one call, one session
or for good.

**External tools cost nothing until the model asks for them.** A tool group is
one registered tool standing in for a set the model cannot see until it opens
the group. Playwright's 24 browser tools cost about 7,400 tokens of schema and
43 tools in every request when advertised; behind a group the same configuration
costs about 3,400 tokens and 20 tools, and nothing behind the group is started
until a call actually needs it.

**A PDF is read in this process.** No subprocess, no Python, no service call:
PDFium compiled to WebAssembly, executed by wazero, with page ranges and a
fallback to rendering a page as an image when the page turns out to be a scan.

**Small models are a supported case, not an afterthought.** Three benchmark
harnesses ship with the program, and each measures something a weak model gets
wrong: whether it calls tools with the right arguments, whether it still
retrieves a fact once the context is long, and whether it can finish an
autonomous coding task scored by a held-out grader.

## Tools

| Tool | Description |
|------|-------------|
| **Read** | File contents or a directory listing, with partial reads for large files. A PDF is detected from its bytes and its text extracted a page range at a time. Other non-text files — images, archives, databases, compiled output — are named, with the tool or command that opens them, rather than read as bytes. |
| **ReadImage** | Read an image (PNG, JPEG, GIF) so the model can see it. Large images are downscaled. |
| **Edit** | Three modes: **lines** (line ranges), **searchreplace** (find and replace, optionally fuzzy), **patch** (unified diffs). Optional preview mode with confirm or cancel. |
| **Write** | Create a file or overwrite one. |
| **Search** | Regular-expression search with glob filters, through ripgrep. |
| **Shell** | Run a command, with timeouts, a working directory, and permission decided from the parsed command line. |
| **Shell.start / Shell.output / Observe.wait** | Processes that outlive the turn's iterations: a dev server, a long test run, a build. |
| **Batch** | Several independent tool calls in one request. |
| **Question** | Ask the person a question rather than guessing, when there is somebody at the terminal. |
| **Web.search / Web.fetch** | Search through the Brave API, and fetch one page as markdown. Both off by default; the search needs a key. |
| **Web.browsing** | A real browser through a Playwright MCP server, behind a tool group. |
| **Plan.\* / Checkpoint.\* / Tasks.\*** | Plan management, turn-based file history in a shadow git repository, and context compression. Plan and Checkpoint are one group and Tasks the other; enable one or the other. |
| **mcp.\*** | Tools from external [Model Context Protocol](https://modelcontextprotocol.io) servers, over stdio or HTTP. See [`docs/mcp.md`](docs/mcp.md). |

Sessions, steering, batching, command permissions, background processes and the
question tool are covered in [`docs/sessions.md`](docs/sessions.md).

## Running commands on your machine

This program executes shell commands and edits files, so it is worth knowing
what constrains it. `workspace.path_safety_mode` decides what happens when a
path outside the workspace is touched: `allow`, `block`, `warn`, `ask_once` or
`ask_always`, where the asking modes prompt on `/dev/tty` and remember the
answer for the run. Shell commands are parsed into a syntax tree and each simple
command in the line is judged on its own against the built-in rules and the
allow and deny lists in `tools.shell`. A refused command comes back to the model
as a tool result saying what it would have done, so the turn can continue with
something else. `--yolo` turns the asking off for a run.

## Benchmarks

Three families, all driven by the same binary: tool benchmarks for whether the
model uses tools correctly on small checkable tasks, haystack for retrieval from
a long context, and thinkbench for autonomous coding tasks scored by a held-out
grader over 72 vendored tasks. Inputs are tracked and outputs are not. See
[`benchmarks/README.md`](benchmarks/README.md).

## Documents

[`docs/`](docs/) holds the design notes.
[`docs/redesign.md`](docs/redesign.md) describes what was built and is the place
to start; [`docs/redesign-plan.md`](docs/redesign-plan.md) records how, stage by
stage, and what remains open. [`docs/archive/`](docs/archive/) is superseded
material, kept for the record.

## License

MIT
