# kvit-coder

A coding agent in Go. Two binaries: `kvit-coder` runs one turn and exits, which is what you
script against, and `kvit-coder-ui` is the interactive terminal front end.
A turn is one run of that loop against a single instruction.

The agent started as experiment to see how much coding can be done with locally ran weak models
and was extended with features more suitable to strong models. Modes adjust how tool behave
how much handholding is done. Each model added to the agent has flag weak or strong for agent
to adapt accordingly.

It uses small amound of RAM (<30 Mb) compared to hundreds Mbs other agents use. See [`docs/agents-ram.md`](docs/agents-ram.md).
This part is important to me as I often have dosens of agents opened/running/waiting at any moment.

![kvit-coder-ui running a session](docs/screen.png)

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

To build from source, see [`CONTRIBUTING.md`](CONTRIBUTING.md).

**Platforms.** Linux is where this is developed and used daily, including under
Windows Subsystem for Linux (WSL). The macOS builds compile and are published,
but nothing has been run on them, so treat them as untested. Windows is not
supported: the program uses Unix process groups, signals and file locking, and
does not compile for it. Under Windows, run it inside WSL, which is a tested
configuration.

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
[`docs/configuration.md`](docs/configuration.md), including the parts
`config.example.yaml` leaves out: defining more than one model and switching
between them, Model Context Protocol (MCP) servers that provide external tools,
tool groups that hide tools until the model asks for them, checkpoints that
record file versions per turn, and the path and shell safety rules.

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

### One process per turn

`kvit-coder` starts, reads the session from disk, runs one instruction, appends
what happened, and exits. If a turn gets stuck, ending its process ends the
turn. The front end keeps no conversation in memory, and memory a turn used
goes back to the operating system when the process exits.

### A session is a directory

Sessions live under `~/.kvit-coder/sessions/<name>/`:

| Entry | Holds |
|---|---|
| `history.jsonl` | Append-only, one timestamped event per line |
| `meta.json` | Created and last-touched times, workspace directory, model name, first prompt |
| `checkpoints/` | A separate git repository that records file versions per turn |
| `proc/` | Pidfiles and logs for background processes |
| `inbox/` | A directory for incoming messages; files dropped here reach the model on the next iteration |
| `tmp/` | Tool output too large to put in a message |

It is plain files, so a session is also a record you can open months later and
read.

### Sending input to a running turn

A line typed at the terminal, a file dropped by `kvit-coder steer` from another
shell, or an event from a background process all go to the same inbox
directory, which the loop checks once per iteration. Pressing Enter on an empty
line asks the turn to pause at the next point where it can stop safely, which
pauses it without ending it.

### Shell permissions from the parsed command

Each shell line is parsed into separate commands and each one is checked on its
own against the built-in rules and the allow and deny lists in `tools.shell`.
`git diff && rm -rf /` counts as two commands and is checked as two, the name
of a program being run is treated differently from the same word appearing as a
`grep` pattern, and a blocked command can be allowed for one call, one session,
or permanently.

### External tools stay hidden until used

A tool group is one visible tool that stands in for a set the model cannot see
until it opens the group. Playwright's 24 browser tools add about 7,400 tokens
of schema and 43 tools to every request when advertised directly, while the
same configuration behind a group adds about 3,400 tokens and 20 tools. Nothing
behind the group starts until a call actually needs it.

### PDF reading without external programs

A PDF is read inside the agent process through PDFium compiled to WebAssembly
and run by wazero, with page ranges and a fallback that renders a page as an
image when the page holds only scanned images with no extractable text.

### Support for small local models

Three benchmark families ship with the program, and each measures something
small local models get wrong: whether the model calls tools with the right
arguments, whether it still retrieves a fact once the context is long, and
whether it can finish a coding task scored by tests it never sees.

## Tools

| Tool | Description |
|------|-------------|
| **Read** | File contents or a directory listing, with partial reads for large files. A PDF is detected from its bytes and its text extracted a page range at a time. Other non-text files — images, archives, databases, compiled output — are named, with the tool or command that opens them, rather than read as bytes. |
| **ReadImage** | Read an image (PNG, JPEG, GIF) so the model can see it. Large images are downscaled. |
| **Edit** | Three modes: **lines** (line ranges), **searchreplace** (find and replace, tolerant of whitespace differences), **patch** (unified diffs). Optional preview mode with confirm or cancel. |
| **Write** | Create a file or overwrite one. |
| **Search** | Regular-expression search with glob filters, through ripgrep. |
| **Shell** | Run a command, with timeouts, a working directory, and permission decided from the parsed command line. |
| **Shell.start / Shell.output / Observe.wait** | Background processes that keep running while the turn continues: a dev server, a long test run, a build. |
| **Batch** | Several independent tool calls in one request. |
| **Question** | Ask the person a question rather than guessing, when there is somebody at the terminal. |
| **Web.search / Web.fetch** | Search through the Brave API, and fetch one page as markdown. Both off by default; the search needs a key. |
| **Web.browsing** | A real browser through a Playwright server, behind a tool group that hides its tools until opened. |
| **Plan.\* / Checkpoint.\* / Tasks.\*** | Plan management, file versions per turn kept in a separate git repository, and shortening of old history to fit the context window. Plan and Checkpoint are one tool group and Tasks is the other; enable one or the other. |
| **mcp.\*** | Tools from external [Model Context Protocol](https://modelcontextprotocol.io) servers, over stdio or HTTP. See [`docs/mcp.md`](docs/mcp.md). |

Sending input mid-turn, grouping independent calls, shell command permissions,
background processes and the question tool are covered in
[`docs/sessions.md`](docs/sessions.md).

## Running commands on your machine

This program executes shell commands and edits files. `workspace.path_safety_mode`
decides what happens when a path outside the workspace directory is read or
written: `allow`, `block`, `warn`, `ask_once` or `ask_always`, where the asking
modes prompt on `/dev/tty` and remember the answer for the run. Shell commands
are parsed into a syntax tree and each simple command in the line is checked on
its own against the built-in rules and the allow and deny lists in
`tools.shell`. A refused command comes back to the model as a tool result saying
what it would have done, so the turn can continue with something else. `--yolo`
disables these prompts for a run.

## Benchmarks

The three families run through `kvit-coder`: tool benchmarks for whether the
model uses tools correctly on small checkable tasks, haystack for retrieval from
a long context, and thinkbench for autonomous coding tasks scored by a held-out
grader, a test script the agent never sees, over 72 vendored tasks. The task
inputs are checked into git and the run outputs are not. See
[`benchmarks/README.md`](benchmarks/README.md).

## Documents

[`docs/`](docs/) holds the reference documentation: every
[command-line flag](docs/cli.md), every [configuration key](docs/configuration.md),
[sessions and steering](docs/sessions.md), [external tools](docs/mcp.md), and the
[memory measurements](docs/agents-ram.md) quoted above.

## License

MIT
