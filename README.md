# kvit-coder

[![Test](https://github.com/kvit-s/kvit-coder/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/kvit-s/kvit-coder/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/kvit-s/kvit-coder?sort=semver)](https://github.com/kvit-s/kvit-coder/releases/latest)

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

Three benchmark families ship with the program and run through `kvit-coder`,
and each measures something small local models get wrong: tool benchmarks for
whether the model calls tools with the right arguments on small checkable
tasks, haystack for whether it still retrieves a fact once the context is long,
and thinkbench for whether it can finish a coding task scored by a held-out
grader, a test script the agent never sees, over 72 vendored tasks. The task
inputs are checked into git and the run outputs are not. See
[`benchmarks/README.md`](benchmarks/README.md).

## Documents

[`docs/`](docs/) holds the reference documentation: the
[tools the model can call](docs/tools.md), every
[command-line flag](docs/cli.md), every [configuration key](docs/configuration.md),
[sessions and steering](docs/sessions.md), [external tools](docs/mcp.md), the
[memory measurements](docs/agents-ram.md) quoted above, and
[how the program is built](docs/architecture.md): one process per turn, sessions
as directories, and the other design decisions behind it.

## License

MIT
