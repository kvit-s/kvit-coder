# kvit-coder

An LLM coding agent with tool-use capabilities, designed for local/self-hosted models via OpenAI-compatible APIs. Includes a built-in benchmark suite for evaluating model performance.

## Quick Start

### Build

```bash
go build -o kvit-coder ./cmd/kvit-coder
go build -o kvit-coder-ui ./cmd/kvit-coder-ui
```

### Configure

Create `config.yaml`:

```yaml
llm:
  base_url: "http://127.0.0.1:8080/v1"
  api_key_env: "OPENAI_API_KEY"
  model: "my-model"
  temperature: 0.2
  max_output_tokens: 2048

workspace:
  root: "."

agent:
  max_tool_iterations: 25

tools:
  read:
    enabled: true
  edit:
    enabled: true
    mode: "lines"          # "lines", "searchreplace", or "patch"
  search:
    enabled: true
  shell:
    enabled: true
  checkpoint:
    enabled: true
```

### Run

```bash
# Headless mode (single prompt, exits after completion)
./kvit-coder -p "Find all TODO comments in the codebase"

# Quiet mode (only print final answer)
./kvit-coder -pq "What does main.go do?"

# Interactive TUI
./kvit-coder-ui
```

## Architecture

Two binaries:

- **`kvit-coder`** — Headless agent for automation, scripting, and benchmarking. Requires `-p` or `--benchmark`.
- **`kvit-coder-ui`** — Interactive terminal UI (BubbleTea) with multi-line input, command history, and syntax highlighting.

## Tools

| Tool | Description |
|------|-------------|
| **read** | Read file contents or list directories. Supports partial reads and character mode for large files. |
| **edit** | Modify files. Three modes: **lines** (line ranges), **searchreplace** (find/replace with optional fuzzy matching), **patch** (unified diffs). Optional preview mode with confirm/cancel. |
| **restore_file** | Restore a file to its state at session start. |
| **search** (grep) | Search file contents with regex patterns and glob filters. Uses ripgrep. |
| **shell** | Execute shell commands with configurable timeouts, working directory, and command allow/blocklists. |
| **plan.\*** | Multi-step plan management: create, add/remove/reorder steps, mark complete. |
| **checkpoint.\*** | Turn-based file history: list, restore, diff, undo. Auto-checkpoints after each turn. |
| **Tasks.\*** | Context compression: wrap exploratory work in tasks so intermediate steps can be collapsed. Includes diff review (accept/decline) and rollback. |
| **mcp.\*** | Tools from external [Model Context Protocol](https://modelcontextprotocol.io) servers (stdio or HTTP), surfaced to the model as `mcp.<server>.<tool>`. Disabled by default; see [MCP Servers](#mcp-servers). |

Plan/Checkpoint tools and Tasks tools are mutually exclusive — enable one group or the other.

## CLI Flags

### kvit-coder

| Flag | Description | Default |
|------|-------------|---------|
| `-p <prompt>` | Run with prompt and exit | - |
| `-pq <prompt>` | Quiet mode: only print final response | - |
| `-config <path>` | Config file path | `config.yaml` |
| `-model <name>` | Override model | from config |
| `-base-url <url>` | Override LLM endpoint | from config |
| `-agent-file <path>` | Append file content to system prompt | - |
| `-log <path>` | Log file (empty to disable) | `kvit-coder.log` |
| `--json` | Structured JSON output to stderr | false |
| `-s <name>` | Continue or create named session | - |
| `--sessions` | List sessions | - |
| `--session-show <name>` | Show session history | - |
| `--session-delete <name>` | Delete a session | - |
| `--version` | Show version info | - |

### kvit-coder-ui

| Flag | Description | Default |
|------|-------------|---------|
| `-config <path>` | Config file path | `config.yaml` |
| `-agent-path <path>` | Path to kvit-coder binary | auto-detected |
| `-s <name>` | Continue or create named session | - |

## Sessions

Conversation history persists across runs via named sessions:

```bash
# Start or continue a session
./kvit-coder -p "set up the project" -s my-feature
./kvit-coder -p "now add tests" -s my-feature

# Manage sessions
./kvit-coder --sessions              # list all
./kvit-coder --session-show my-feature   # view history
./kvit-coder --session-delete my-feature # delete
```

Each session is a directory under `~/.kvit-coder/sessions/`:

```
~/.kvit-coder/sessions/my-feature/
    history.jsonl   append-only transcript, one timestamped event per line
    meta.json       when it was created and last used, workspace, model, first prompt
    checkpoints/    the shadow git repository the checkpoint tools commit into
    proc/           which process is running a turn
    inbox/          messages waiting for the running turn
    tmp/            tool output too large to fit in a message
```

Everything that outlives a turn lives there, so a checkpoint made in one run
can be restored in the next and a temp file the model was given the path to is
still readable later. A session left over from when a session was a single
`<name>.jsonl` file is converted the first time it is opened; the old file is
kept as `<name>.jsonl.migrated`.

### Steering a running turn

A message can reach the model between iterations of a turn that is already
running. Type a line at the terminal while the agent works, or send one from
another terminal:

```bash
./kvit-coder steer "actually, check the tests first"
./kvit-coder steer -s my-feature "use the other library"
```

Without `-s` it delivers to the one session that has a turn running, and says
so if there is none or more than one. The message arrives as a user message
tagged `<user-steering>` at the model's next iteration.

### Batching calls

With `tools.batch.enabled`, several independent calls go in one request:

```
Batch({"calls": [
  {"tool": "Read", "args": {"path": "main.go"}},
  {"tool": "Read", "args": {"path": "config.go"}},
  {"tool": "Search", "args": {"pattern": "func main"}}
]})
```

One request and one round of thinking instead of one each. Calls that only read
run at the same time, so four reads take about as long as one; calls that
change the workspace run afterwards, in order. Each call comes back with its own
result or its own error, so one failure does not lose the others. At most ten
calls, and no `Batch` inside a `Batch`.

### Command permissions

Whether a shell command may run is decided from its syntax tree, not from
matching its text. Every simple command in the line is found and judged on its
own, so `git diff && rm -rf /` is refused for its second command, `ps aux | awk
'{print $2}'` runs, and a word like `shutdown` in a grep pattern is an argument
rather than a program.

Refused outright: `sudo`, `su`, `chroot`, package managers, `shutdown`/`reboot`,
`mkfs`, deleting `/` or your home directory, and `sed -i` when the Edit tool is
available. Deleting anything *under* those directories is ordinary cleanup and
is allowed.

Needing permission — dangerous in general, ordinary in context — are `curl`,
`wget`, `nc`, `dd`, `eval` and interpreter one-liners such as `python -c`. At a
terminal you are asked, with four answers: just this once, for the rest of this
session, always for this project, or always everywhere. The last three are
written to permission files under `~/.kvit-coder/permissions/` and the session
directory — never inside the workspace, where the agent could edit them. With
no terminal the command is refused and the model is told which pattern to add
to `tools.shell.allowed_commands`.

`allowed_commands` and `disallowed_commands` still work, now as patterns
matched against each command in the line rather than as a prefix of the whole
string. Setting `allowed_commands` still makes everything else a denial, and a
grant can never open something that is refused outright.

### Background processes

With `tools.procs.enabled`, a command can outlive the turn that started it:

```
Shell.start({"command": "npm run dev", "name": "dev server"})   → {"id": "bg1"}
Observe.wait({"id": "bg1", "until": "Listening on", "report": "match"})
Shell.output({"id": "bg1", "cursor": 4096})
Shell.list({}) / Shell.status({"id": "bg1"}) / Shell.kill({"id": "bg1"})
Observe.add({"command": "git status --short", "every": 60})
```

`Shell` is unchanged: it runs a command to completion and dies with the turn.
`Shell.start` detaches into its own session, so a dev server started in one
turn is still serving in the next, and its output and exit status are recorded
in `<session>/proc/` where the next turn can read them.

`Observe.wait` blocks until the process ends, prints something matching a
pattern, or `max_wait` passes — one tool call however long it takes, where
checking with `Shell.output` in a loop costs a full round of thinking each
time. It also returns early if you type something, so you are not left waiting
for a process whose result no longer matters.

Between iterations the agent asks the registry what has happened and tells the
model about anything that ended, so watching costs nothing until there is
something to say. Ctrl-C stops everything the session started, unless
`tools.procs.kill_on_exit` is false.

### Questions

With `tools.question.enabled`, the model can ask you something and wait for the
answer, which comes back as the tool's result:

```
── retry strategy ──
Reuse the existing retry wrapper, or write a new one?
  1) Reuse internal/http.Retry  — same backoff, already tested
  2) New wrapper in this package  — no shared state
  [1-2, or type]
```

A bare number chooses, `1,3` chooses several, and anything else comes back as
free text — so answering and steering are the same keystrokes and you never
have to decide which you are doing before you type. A line typed before the
question appeared is treated as steering rather than silently taken as the
answer. Ctrl-C dismisses the question and ends the turn; the model is refused
if it asks the same question again.

With a terminal the call waits as long as it takes. With no terminal it waits
`tools.question.timeout` seconds for an answer dropped in the session inbox
(`kvit-coder steer` writes there), then tells the model to proceed on its own
judgement and say what it assumed. The default of zero falls back at once, so a
benchmark or scripted run never hangs on a question.

## Agent File

Append custom instructions to the system prompt without modifying config:

```bash
./kvit-coder -p "refactor auth module" -agent-file AGENT.md
```

Or in config:

```yaml
agent:
  agent_file: AGENT.md
```

## Project Instructions

Headless runs automatically load `CLAUDE.md` from the directory where `kvit-coder` was launched, then apply the `-p` prompt as the user request. The instructions are sent as user-level context, not appended to the system prompt, matching Claude Code's behavior more closely than `agent_file`.

Customize or disable this in config:

```yaml
agent:
  project_instructions:
    enabled: true
    path: CLAUDE.md   # relative to the launch directory, or absolute
```

## MCP Servers

kvit-coder can act as a [Model Context Protocol](https://modelcontextprotocol.io) (MCP) client: it connects to external tool servers, asks each one what tools it offers, and surfaces every tool to the model as if it were built in. This gives the agent access to the existing MCP ecosystem (filesystem, git, GitHub, Postgres, web fetch, Playwright, and more) without a bespoke integration for each.

Discovered tools are namespaced `mcp.<server>.<tool>` (e.g. `mcp.git.git_status`) and flow through the normal agent loop, loop detection, and prompt generation. The feature is **disabled by default** and zero-cost when unused.

```yaml
mcp:
  enabled: true
  startup_timeout: 20        # seconds to connect + list tools, per server
  call_timeout: 120          # default per-tool-call deadline (overridable per server)
  confirm: "ask_once"        # block | ask_once | ask_always | trust
  sanitize_schemas: false    # flatten $ref/oneOf/allOf for strict tool-calling templates
  servers:
    - name: "git"            # namespaces tools as mcp.git.<tool>; must be unique
      enabled: true
      transport: "stdio"     # stdio (subprocess) | http (streamable HTTP)
      command: "uvx"
      args: ["mcp-server-git", "--repository", "."]
      env: ["KEY=VALUE"]     # appended to inherited env (supports ${VAR})
    - name: "fetch"
      enabled: true
      transport: "http"
      url: "http://127.0.0.1:9000/mcp"
      headers: ["Authorization=Bearer ${FETCH_TOKEN}"]   # supports ${VAR}
      call_timeout: 60       # per-server override
      confirm: "trust"       # per-server override
      tools:
        allow: []            # raw tool-name allowlist (empty = all)
        deny: ["dangerous_tool"]
```

| Group key | Description |
|-----------|-------------|
| `enabled` | Master toggle for the MCP client |
| `startup_timeout` | Per-server connect + `tools/list` deadline, seconds (default 20) |
| `call_timeout` | Default per-call deadline, seconds (default 120) |
| `confirm` | Default trust policy (see below); default `ask_once` |
| `sanitize_schemas` | Strip JSON Schema constructs (`$ref`, `oneOf`, `allOf`, …) some grammar-constrained templates reject (default false) |

| Server key | Description |
|------------|-------------|
| `name` | Unique server name; namespaces its tools |
| `enabled` | Opt this server in (default false) |
| `transport` | `stdio` (default) or `http` |
| `command` / `args` / `env` | stdio: executable, arguments, extra `KEY=VALUE` env (appended to inherited; `${VAR}` expanded) |
| `cwd` | stdio: subprocess working directory; empty defaults to the workspace root, a relative path resolves against it. Set this for servers that index their own working directory. |
| `url` / `headers` | http: endpoint and extra `Key=Value` request headers (`${VAR}` expanded) |
| `call_timeout` / `confirm` | Optional per-server overrides of the group defaults |
| `tools.allow` / `tools.deny` | Optional raw tool-name filters (allow first, then deny) |

**Trust model.** MCP tools run in a separate process and bypass workspace path safety entirely — enabling a server grants the model whatever that server can do. The `confirm` policy is the gate:

- `block` — refuse every call (load a server for inspection only)
- `ask_once` — prompt once per distinct tool name, then remember for the session
- `ask_always` — prompt before every call
- `trust` — never prompt

In a headless/benchmark run with no controlling terminal, `ask_*` falls back to `block` rather than hanging — use `confirm: trust` for trusted servers there.

**Behavior notes.** Connections are made concurrently at startup; a server that fails or times out is logged and skipped, never fatal. MCP calls are exempt from the 15s blanket tool timeout and use `call_timeout` instead. Large text results are spilled to a temp file with a truncated preview; binary/image blocks are summarized rather than inlined. A server tool reporting an error is recoverable (the model can retry differently); a transport failure is not.

## Configuration Reference

### `llm`

| Key | Description |
|-----|-------------|
| `base_url` | OpenAI-compatible API endpoint |
| `api_key` / `api_key_env` | API key or env var name |
| `model` | Model name |
| `api_backend` | Wire protocol: `chat_completions` (default) or `responses` |
| `headers` | Extra `Key=Value` request headers (`${VAR}` expanded; `${KVIT_RUN_ID}` = per-conversation ID) |
| `reasoning_effort` | Thinking budget for a reasoning model (`responses` backend only) |
| `temperature` | Sampling temperature |
| `max_output_tokens` | Max output tokens |
| `context` | Max context size for display (0 = hide) |
| `merge_thinking` | Merge `reasoning_content` into `content` |
| `verbose` | Tool output verbosity (0 = off, N = show up to N lines) |
| `generation_stats` | Ask the endpoint for per-request cost and native token counts (OpenRouter only; default false) |
| `benchmark_cmd` | External command for benchmarks (`{prompt}` placeholder) |

**Endpoints that only serve `/responses`.** Some hosted models are offered only
through OpenAI's Responses API and answer `/chat/completions` with an error.
Setting `api_backend: "responses"` makes kvit-coder speak that protocol
instead: tool calls and results are translated on the way out and back, and the
opaque thinking blocks a reasoning model returns are replayed to it on the
following request so a tool loop keeps its train of thought. `headers` covers
endpoints that also demand a routing or session header of their own.
`benchmarks/config-muse-spark.yaml` is a working example.

An endpoint that routes by a session header sends every request carrying the
same header value to one backend, so two agents sharing a value compete for the
same prompt cache. `${KVIT_RUN_ID}` in a header expands to an ID derived from
the session name, which keeps concurrent conversations apart while sending
every turn of one conversation to the backend that already holds its prompt
cache. Setting `KVIT_RUN_ID` in the environment pins a value instead.

### `workspace`

| Key | Description |
|-----|-------------|
| `root` | Workspace root directory |
| `path_safety_mode` | `block`, `warn`, `ask_once` (default), `ask_always` |
| `allowed_paths` / `allowed_read_paths` | Paths allowed outside workspace |
| `denied_paths` | Explicitly denied paths |

### `agent`

| Key | Description |
|-----|-------------|
| `max_tool_iterations` | Max tool calls per run |
| `agent_file` | Path to agent instructions file |
| `project_instructions.enabled` | Load project instructions before `-p` prompts (default true) |
| `project_instructions.path` | Project instructions file path; relative paths resolve from the launch directory (default `CLAUDE.md`) |

### `tools`

Each tool group has `enabled: true/false` plus tool-specific options:

- **`edit.mode`** — `"lines"`, `"searchreplace"`, or `"patch"`
- **`edit.preview_mode`** — Enable confirm/cancel workflow for edits
- **`edit.fuzzy_threshold`** — Fuzzy matching for searchreplace mode (0 = exact only)
- **`edit.read_before_edit_msgs`** — Require a read within N messages before editing
- **`shell.allowed_commands`** / **`shell.disallowed_commands`** — Command allow/blocklists
- **`shell.default_timeout`** / **`shell.max_timeout`** — Seconds a command gets, and the ceiling a call may ask for (default 120 and 600)
- **`batch.enabled`** — Run several independent tool calls in one request
- **`procs.enabled`** — Background processes that outlive a turn (`Shell.start`, `Observe.wait`, …)
- **`procs.kill_on_exit`** — Stop everything the session started when a turn is interrupted (default true)
- **`question.enabled`** — Let the model ask you a question and wait for the answer
- **`question.timeout`** — Seconds a run with no terminal waits for an answer (default 0: fall back at once)
- **`checkpoint.max_turns`** — Max checkpoints before rotating (default: 100)
- **`tasks.collapse`** — Enable context collapsing (stage 2)
- **`tasks.plan`** — Enable plan-based task tools (stage 3)

### `backtrack`

Automatic retry on failed tool calls (disabled by default):

```yaml
backtrack:
  enabled: true
  max_retries: 5
  inject_user_message: false
```

### `safety`

Enterprise safety controls (all disabled by default):

```yaml
safety:
  strict_mode: false       # fail-closed on parse errors
  paranoid_mode: false      # aggressive restrictions
  audit:
    enabled: false
    log_dir: "~/.kvit-coder/safety-logs"
  git:
    block_push: false
    block_hard_reset: false
  rm:
    block_workspace_root: false
  interpreters:
    block_one_liners: false
```

### `prompts`

Optional template-based prompt system:

```yaml
prompts:
  use_templates: false
  templates_dir: ""        # override embedded templates
  hot_reload: false        # reload on each request (dev mode)
```

## Benchmarking

### Run Benchmarks

```bash
# Run all benchmarks (uses config-mymodel.yaml)
./kvit-coder --benchmark mymodel

# Custom runs, category filter, specific IDs
./kvit-coder --benchmark mymodel -n 5
./kvit-coder --benchmark mymodel --benchmark-category search,edit
./kvit-coder --benchmark mymodel --benchmark-id S1,S2,R1

# List available benchmarks
./kvit-coder --benchmark-list
```

### Benchmark Categories

| Category | Description |
|----------|-------------|
| **search** | Code pattern search |
| **read** | File reading and directory listing |
| **edit** | File creation and modification |
| **shell** | Shell command execution |
| **compound** | Multi-step tasks combining multiple tools |

### External LLM Support

Benchmark an external tool (e.g., Claude Code):

```yaml
llm:
  benchmark_cmd: "claude -p {prompt} --allowedTools Edit Bash Read"
```

### Haystack Benchmarks

Needle retrieval in large context windows (1-5 hop reasoning, no tools required):

```bash
./kvit-coder --bench-haystack mymodel
./kvit-coder --bench-haystack mymodel --bench-haystack-id 1H1,2H1
```

### Thinkbench Benchmarks

[Thinkbench](benchmarks/thinkbench/suite/README.md) (by Thinkwright, Apache-2.0) is a suite of 72 autonomous coding-agent tasks across five types — `implement`, `bug-fix`, `feature-add`, `repair-to-green` (60 graded), and `ambiguous-spec` (12 observed). Each graded task runs the agent in a fresh, hard-sandboxed workspace from a single `brief.txt`; after the agent stops a held-out `grade.py` is dropped in and produces a continuous `passed/total` score.

```bash
# Run all graded thinkbench tasks (uses config-mymodel.yaml), 3 trials each
./kvit-coder --bench-thinkbench mymodel

# Specific tasks / types / trial count
./kvit-coder --bench-thinkbench mymodel --bench-thinkbench-id base62,backoff
./kvit-coder --bench-thinkbench mymodel --bench-thinkbench-types bug-fix,feature-add
./kvit-coder --bench-thinkbench mymodel -n 1
```

Outputs `results.json`, `RESULTS.md`, and a resumable CSV under `benchmarks/thinkbench/results/` (`overall_graded` / `by_type` / `by_task` / `observed` rollups, with both **mean score** and **full-pass rate**).

**Requirements & sandboxing:**

- **`uv`** must be on PATH. A shared environment is auto-provisioned at `benchmarks/thinkbench/.uv` (pinned via `thinkbench.uv_python`) and used as the default `python3`/`pip` for both the agent's `run_command` and the grader.
- Each run is confined to a per-run workspace for **reads as well as writes**: `path_safety_mode=block` covers the structured file tools, and an OS sandbox (auto-detected **bubblewrap** → **firejail**) confines arbitrary shell. A preflight smoke test proves a read outside the workspace is denied before any task runs. Held-out graders and reference solutions live under `suite/` and are never copied into a workspace pre-grade.

Config block (in `config-<suffix>.yaml`):

```yaml
thinkbench:
  suite_dir: "benchmarks/thinkbench/suite"   # optional; auto-discovered
  timeout_per_run: 600                        # seconds; implement tasks are slow
  uv_python: "3.11"
  sandbox: "auto"   # auto | bwrap | firejail | none | require
  types: ["implement", "bug-fix", "feature-add", "repair-to-green"]
  trials: 3          # default trials per task; overridden by -n
  include_observed: false   # also run the 12 ungraded ambiguous-spec tasks (persist-only)
```

### Adding Custom Benchmarks

Define in `benchmarks/benchmarks.yaml`:

```yaml
benchmarks:
  - id: MY1
    name: "My Custom Benchmark"
    category: custom
    goal: "Test something specific"
    setup:
      - file: "test.go"
        content: |
          package main
          func myFunc() {}
    task: "Find the function named myFunc"
    validation:
      - type: output_contains
        expected: "test.go"
```

Validation types: `file_contains`, `file_equals`, `file_exists`, `file_not_exists`, `file_line_count`, `tool_called`, `tool_called_with`, `output_contains`, `output_not_contains`, `output_matches`, `multi_tool_calls`

## License

MIT
