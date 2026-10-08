# kvit-coder

[![Test](https://github.com/kvit-s/kvit-coder/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/kvit-s/kvit-coder/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/kvit-s/kvit-coder?sort=semver)](https://github.com/kvit-s/kvit-coder/releases/latest)

A coding agent in Go. Two binaries: `kvit-coder` runs one turn (user question -> multiple steps calling tools -> model's answer) and exits. And `kvit-coder-ui` - the interactive terminal front end.

The agent started as experiment to see how much coding can be done with locally ran weak models
and was extended with features more suitable to strong models. Weak/strong modes change how tools behave
and how much handholding is done. Each model added to the agent flagged as weak or strong.

It uses small amound of RAM (<30 Mb) compared to hundreds Mbs other agents use. See [`docs/agents-ram.md`](docs/agents-ram.md).
This is important to me as I often have dosens of agents opened/running/waiting at any moment.

![kvit-coder-ui running a session](docs/screen.png)

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/kvit-s/kvit-coder/main/install.sh | sh
```

To build from source, see [`CONTRIBUTING.md`](CONTRIBUTING.md).

**Windows (native, 10/11).** Download the installer for your machine
(`kvit-coder_<version>_windows_amd64_setup.exe`, or `arm64`) from the
[latest release](https://github.com/kvit-s/kvit-coder/releases/latest) and run
it. It installs for your user only, without an administrator prompt, into
`%LOCALAPPDATA%\Programs\kvit-coder`, adds that folder to your user PATH, puts
`kc.exe` and `kcu.exe` beside `kvit-coder.exe` and `kvit-coder-ui.exe` as the
short names, and seeds `%USERPROFILE%\.kvit-coder\config.yaml` when there is
none. Open a new terminal afterwards so it sees the new PATH. Uninstall it from
Settings > Apps; that removes the PATH entry and leaves
`%USERPROFILE%\.kvit-coder` alone.

Without the installer, download the portable zip
(`kvit-coder_<version>_windows_amd64_portable.zip`, or `arm64`), extract it
where you want to keep it (e.g. `C:\apps\kvit-coder`), then run the
`install.ps1` inside that folder. It does the same for the folder it runs from:

```powershell
powershell -ExecutionPolicy Bypass -File install.ps1
```

Or install a published release straight into `%USERPROFILE%\bin`, which the
script also puts on the PATH:

```powershell
irm https://raw.githubusercontent.com/kvit-s/kvit-coder/main/install.ps1 | iex
```

**Platforms.** Linux is where this is developed and used daily, including under
Windows Subsystem for Linux (WSL). The macOS builds compile and are published,
but never tested. Native Windows (10/11, amd64 and arm64) builds, runs, and passes tests;
it needs Git for Windows (for `sh.exe`, `grep`, and `git` itself) and installs
via the installer, the portable zip or `install.ps1` (see above). WSL stays supported regardless; benchmarks stay
WSL/unix-only (see `spec/windows.md`).

## Configure

Run `kcu`. With no model configured it opens `:setup`: choose OpenCode Zen,
OpenCode Go, a local llama.cpp, vLLM, Ollama or LM Studio server, or any other
endpoint that speaks the same protocols; paste its key; and tick the models
you want. It fills in each model's protocol, context size and effort levels
from the endpoint and the [models.dev](https://models.dev) catalog, saves them
to `~/.kvit-coder/models.yaml` and the key to `~/.kvit-coder/credentials.json`,
and leaves `config.yaml` alone. `:setup` adds more later, `:models` switches
between them, and `:keys` replaces a key; see
[`docs/cli.md`](docs/cli.md#setting-up-models).

The installers put a copy of [`config.example.yaml`](config.example.yaml) at
`~/.kvit-coder/config.yaml` (on Windows: `%USERPROFILE%\.kvit-coder\config.yaml`),
and kcu offers to write it when there is none. It holds everything except the
model; a model can also be written into it by hand
([`docs/models.md`](docs/models.md)).

Without `-config`, both binaries take the first of `$KVIT_CODER_CONFIG`,
`./config.yaml`, `~/.kvit-coder/config.yaml` (`%USERPROFILE%\.kvit-coder\config.yaml`
on Windows), and `config.yaml` beside the binary with symlinks resolved.

Every key is documented in
[`docs/configuration.md`](docs/configuration.md), including the parts
`config.example.yaml` leaves out: defining more than one model and switching
between them, Model Context Protocol (MCP) servers that provide external tools,
tool groups that hide tools until the model asks for them,
and the path and shell safety rules.

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

The harness includes three benchmarks. These are designed to check basic agentic
skills of weak models. See [`benchmarks/README.md`](benchmarks/README.md).

## Documents

[`docs/`](docs/) holds the reference documentation: the
[tools the model can call](docs/tools.md), every
[command-line flag](docs/cli.md), every [configuration key](docs/configuration.md),
[how to add an OpenCode model](docs/models.md),
[sessions and steering](docs/sessions.md), [external tools](docs/mcp.md), the
[memory measurements](docs/agents-ram.md) quoted above, and
[how the program is built](docs/architecture.md): one process per turn, sessions
as directories, and the other design decisions behind it.

## License

MIT
