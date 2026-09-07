# Adding MCP (Model Context Protocol) support to kvit-coder

## Overview

kvit-coder is an LLM coding agent that drives self-hosted, OpenAI-compatible models
through a fixed set of built-in tools: file read/edit, code search, a shell, and a few
planning and checkpoint helpers. Every tool is a Go type that satisfies one interface and
is registered into a single in-process registry; the registry turns those tools into the
`tools` array of the chat-completions request and dispatches the model's tool calls back to
the right Go method.

The Model Context Protocol (MCP) is an open client–server protocol that lets an external
process expose tools, resources, and prompt templates to an agent over JSON-RPC 2.0. An MCP
*server* is either a local subprocess that speaks JSON-RPC over its standard input/output
("stdio" transport) or a remote HTTP endpoint ("streamable HTTP" transport). A large
ecosystem of ready-made servers already exists — filesystem access, git, GitHub, Postgres,
Playwright, web fetch, and many more. Adding an MCP *client* to kvit-coder lets it use any of
those servers without writing a bespoke Go integration for each one: the user lists a server
in config, kvit-coder connects, asks the server what tools it offers, and surfaces each one
to the model as if it were a native tool.

This document describes how to add that client. The goal of the first milestone is narrow and
concrete: **expose the tools advertised by configured MCP servers as ordinary agent tools**,
so they flow through the existing agent loop, loop detection, and prompt generation with no
special-casing in the hot path. MCP *resources* and *prompts* are deferred to a later phase
and sketched at the end.

The design rests on one observation about the current code: a tool is anything that
implements `tools.Tool` and is placed in the registry under the name the model will call. If
each remote MCP tool is wrapped in an adapter that implements that interface, the rest of the
system needs almost no changes. The sections below establish that integration surface, then
specify the adapter, the new package, the config, the wiring, and the risks.

---

## 1. The integration surface as it exists today

This section describes the parts of the current code that MCP must plug into. Everything here
is present today; the design in Section 2 builds on it.

### 1.1 The `Tool` interface

Every tool implements [internal/tools/tool.go](internal/tools/tool.go), a nine-method
interface. The methods divide into three groups:

- **Identity and schema**, read when building the request to the model:
  `Name() string`, `Description() string`, `JSONSchema() map[string]any`.
- **Execution**, called by the agent loop: `Check(ctx, args) error` (validation and user
  confirmation, run before execution) and `Call(ctx, args) (any, error)` (the actual work).
- **Documentation**, read when building the system prompt:
  `PromptCategory() string`, `PromptOrder() int`, `PromptSection() string`,
  `PromptTemplateName() string`.

`JSONSchema()` returns a raw `map[string]any` that is passed straight through to the model as
the function's parameter schema. This matters: an MCP server already hands us its tool's
`inputSchema` as JSON Schema, so the adapter can return it almost verbatim.

### 1.2 The registry and how tools reach the model

`Registry` ([internal/tools/registry.go](internal/tools/registry.go)) is a
`map[string]Tool` keyed by tool name. Two methods are central:

- `Specs()` ([registry.go:65](internal/tools/registry.go#L65)) sorts the tool names and emits
  one `llm.ToolSpec` per tool, copying `Name()`, `Description()`, and `JSONSchema()` into the
  spec. The sorted order exists to keep the request body byte-stable for prompt caching on the
  model server. The resulting `[]llm.ToolSpec` becomes `ChatRequest.Tools`
  ([internal/llm/types.go:66](internal/llm/types.go#L66)).
- `Get(name)` ([registry.go:60](internal/tools/registry.go#L60)) is how the agent loop finds
  the tool to run for a given call.

The registry also owns text-fallback parsing for models that emit tool calls as prose
(`ExtractToolCallsFromText`, `LooksLikeMalformedToolCall`). Both iterate over the registered
tool names, so any registered MCP tool is automatically covered.

### 1.3 How the agent loop dispatches a call

The loop lives in [internal/agent/runner_tools.go](internal/tools). For each tool call in an
assistant message, `executeSingleTool`
([runner_tools.go:74](internal/agent/runner_tools.go#L74)):

1. looks the tool up with `r.registry.Get(tc.Function.Name)`; an unknown name produces a
   backtrackable "Unknown tool" semantic error;
2. checks pending-edit state (irrelevant to MCP tools, which set none);
3. normalizes arguments with `tools.NormalizeToolCallArguments(tool, args)`
   ([internal/tools/argument_normalizer.go:70](internal/tools/argument_normalizer.go#L70)),
   which is schema-driven and is a safe pass-through for schemas it does not recognize;
4. runs `tool.Check(ctx, args)`;
5. detects an immediate duplicate call (same name + same arguments as the previous call) and,
   past a threshold, aborts the run to break infinite loops;
6. runs the tool through `executeToolWithTimeout`
   ([runner_tools.go:338](internal/agent/runner_tools.go#L338)), appends the result as a
   `tool`-role message, and records it for loop detection.

Two details in step 6 directly constrain the MCP design:

- **A hard 15-second timeout is applied to every tool except `Shell` and `Shell.advanced`**
  ([runner_tools.go:364](internal/agent/runner_tools.go#L364)). MCP tool calls (a web fetch, a
  database query, a browser action) routinely exceed this. MCP tools must be exempted from
  this blanket timeout and given their own configurable per-call deadline.
- **The result is serialized with `json.MarshalIndent`**
  ([runner_tools.go:406](internal/agent/runner_tools.go#L406)) and a one-line summary is
  produced by `ui.GetResultSummary`. Whatever the adapter's `Call` returns must serialize to
  something the model can read.

Error handling distinguishes *backtrackable* (recoverable, model-misuse) errors from fatal
ones via `tools.IsBacktrackable` / `tools.SemanticErrorf` / `tools.WrapAsSemantic`
([internal/tools/errors.go](internal/tools/errors.go)). The success/error string convention
the loop recognizes is a leading `Error:` or a `"success": false` field
([runner_tools.go:299](internal/agent/runner_tools.go#L299)).

### 1.4 How tools are enabled and the prompt is built

`SetupRegistry` ([internal/tools/setup.go:30](internal/tools/setup.go#L30)) is a plain
synchronous function: it reads `cfg.Tools.*`, constructs the enabled built-in tools, and
returns a populated registry. It is called once in
[cmd/kvit-coder/main.go:338](cmd/kvit-coder/main.go#L338), alongside the other
session-scoped managers (`tempFileMgr`, `planManager`, `checkpointMgr`), each of which is
created in `main`, passed into `SetupConfig`, and cleaned up with a `defer`.

The system prompt's per-tool documentation is generated by category. `GenerateToolPrompt`
([registry.go:376](internal/tools/registry.go#L376)) and `EnabledCategories`
([registry.go:440](internal/tools/registry.go#L440)) both iterate a **hardcoded** category
list, `{"filesystem", "shell", "plan", "checkpoint"}`, and `CategoryHeaders`
([registry.go:31](internal/tools/registry.go#L31)) maps each category to a markdown header.
A new category for MCP tools requires touching these three places.

### 1.5 The single wiring point

The `kvit-coder-ui` binary is only a terminal front-end; it launches the headless
`kvit-coder` binary as a subprocess (see its `-agent-path` flag) and does not build a
registry itself. The Go wiring that constructs the registry and runner therefore lives in
exactly two places that share the same code path: `cmd/kvit-coder/main.go` for normal runs,
and the benchmark executors ([internal/benchmark/executor.go](internal/benchmark),
`thinkbench_executor.go`), which receive the already-built `runner` and `systemPrompt`. MCP
setup needs to happen once, in `main`, before the registry is handed to the prompt generator
and runner.

---

## 2. Design

### 2.1 The core idea

Introduce a new package, `internal/mcp`, with three pieces:

1. a **client** that connects to one MCP server and exposes `Initialize`, `ListTools`, and
   `CallTool`;
2. a **manager** that owns all configured servers' clients, connects them at startup, and
   produces tool adapters;
3. an **`MCPTool` adapter** that implements `tools.Tool` by delegating to a client.

The manager is created in `main`, like `checkpointMgr`; its adapters are registered into the
existing registry; its connections are closed with a `defer`. Once registered, an MCP tool is
indistinguishable from a built-in tool to `Specs()`, the agent loop, loop detection, and the
text-fallback parser.

```
config.yaml (mcp.servers)
        │
        ▼
  mcp.Manager ──connect──►  one mcp.Client per server  ──stdio/http──►  MCP server process
        │                          │
        │   ListTools()            │ tools/list
        ▼                          ▼
  []*MCPTool  ──register──►  tools.Registry  ──Specs()──►  ChatRequest.Tools  ──►  model
        ▲                          │
        └──────────────────────────┘  registry.Get(name) ──► MCPTool.Call ──► tools/call
```

### 2.2 Dependency decision: official SDK vs. minimal in-house client

There is an official Go SDK for MCP (`github.com/modelcontextprotocol/go-sdk`) that
implements the JSON-RPC framing, the `initialize` handshake, both transports, and the typed
request/response shapes. The trade-off:

- **Official SDK.** Removes the bulk of transport and protocol-version risk and tracks spec
  changes. Cost: a non-trivial new dependency tree in a project that today has a deliberately
  small one (the only heavyweight deps are `bubbletea` and `zap`). The exact module path,
  version, and surface should be verified at integration time and pinned in `go.mod`.
- **Minimal in-house client.** A few hundred lines: spawn the subprocess, a goroutine reading
  newline-delimited JSON-RPC messages off stdout, a request map keyed by JSON-RPC id, and the
  three methods we need (`initialize`, `tools/list`, `tools/call`). Cost: we own protocol
  correctness and any future spec drift.

**Recommendation:** start with the official SDK for the stdio transport to de-risk the
protocol, behind a thin `internal/mcp.Client` interface so the implementation can be swapped.
If the dependency footprint proves unacceptable, the in-house client implements the same
interface. Keeping `internal/mcp.Client` as our own interface (not the SDK's types leaking
into `internal/tools`) is the important architectural boundary regardless of which backs it.

### 2.3 Config

Add an `MCP` section to `Config` ([internal/config/config.go:14](internal/config/config.go#L14))
and a matching struct. It is absent by default, so existing configs are unaffected and the
feature is zero-cost when unused.

```yaml
mcp:
  enabled: true
  startup_timeout: 20        # seconds to connect + list tools per server
  call_timeout: 120          # default per-tool-call deadline (overridable per server)
  confirm: "ask_once"        # "block" | "ask_once" | "ask_always" | "trust"
  servers:
    - name: "filesystem"     # used to namespace tool names; must be unique
      enabled: true
      transport: "stdio"     # "stdio" | "http"
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/sk/kvit-coder"]
      env: ["NODE_ENV=production"]   # appended to inherited env, like Shell InjectEnv
      call_timeout: 60       # optional per-server override
      tools: { allow: [], deny: [] }  # optional name filters (post-namespacing or raw)
    - name: "fetch"
      transport: "http"
      url: "http://127.0.0.1:9000/mcp"
      headers: ["Authorization=Bearer ${FETCH_TOKEN}"]
```

```go
type MCPConfig struct {
    Enabled        bool             `yaml:"enabled"`
    StartupTimeout int              `yaml:"startup_timeout"` // default 20
    CallTimeout    int              `yaml:"call_timeout"`    // default 120
    Confirm        string           `yaml:"confirm"`         // default "ask_once"
    Servers        []MCPServerConfig `yaml:"servers"`
}

type MCPServerConfig struct {
    Name        string         `yaml:"name"`
    Enabled     bool           `yaml:"enabled"`
    Transport   string         `yaml:"transport"` // "stdio" | "http"
    Command     string         `yaml:"command"`   // stdio
    Args        []string       `yaml:"args"`      // stdio
    Env         []string       `yaml:"env"`       // stdio, "KEY=VALUE"
    URL         string         `yaml:"url"`       // http
    Headers     []string       `yaml:"headers"`   // http, "Key=Value"
    CallTimeout int            `yaml:"call_timeout"`
    Tools       MCPToolFilter  `yaml:"tools"`
}
```

Apply defaults in `config.Load` ([config.go:398](internal/config/config.go#L398)) the same
way the read/edit/checkpoint defaults are applied (`StartupTimeout` → 20, `CallTimeout` → 120,
`Confirm` → `"ask_once"`). Note: the `enabled` zero value is `false`, so each server and the
group must be opt-in. This is consistent with the existing "all tools disabled by default"
convention.

### 2.4 Naming and namespacing

Servers choose their own tool names, and two servers (or a server and a built-in) can collide
— `filesystem` exposes a `read` tool, kvit-coder already has `Read`. Register every MCP tool
under a namespaced name:

```
mcp.<server>.<tool>      e.g.  mcp.filesystem.read_file
```

The dotted form matches the convention already used for `Plan.*`, `Tasks.*`, and
`Shell.advanced`, and the generic branch of `displayToolCall`
([runner_tools.go:331](internal/agent/runner_tools.go#L331)) renders it correctly without a
special case. The adapter keeps the *raw* tool name internally for the `tools/call` request
while presenting the namespaced name to the model via `Name()`.

Two cautions:

- Some model tool-calling templates restrict the characters or length of function names.
  Verify against the target model; if dots are a problem, fall back to a double-underscore
  form (`mcp__filesystem__read_file`, the convention Claude Code uses). Make the separator a
  single constant in the `mcp` package so it is trivial to change.
- The namespaced name is what loop detection, the duplicate-call guard, and
  `ExtractToolCallsFromText` all key on. Nothing else needs to know the raw name.

### 2.5 The `MCPTool` adapter

A single type maps the nine interface methods onto one MCP tool description plus a client
handle.

| `tools.Tool` method | MCP behavior |
|---|---|
| `Name()` | namespaced name, `mcp.<server>.<tool>` |
| `Description()` | server's tool `description`, optionally prefixed with the server name for the model's benefit |
| `JSONSchema()` | server's `inputSchema` (JSON Schema), passed through; optionally sanitized (see Section 4) |
| `Check(ctx, args)` | enforce the server's `confirm` policy (Section 2.7); no path-safety check (these are remote) |
| `Call(ctx, args)` | issue `tools/call` with the raw tool name and the args; map the result (below) |
| `PromptCategory()` | `"mcp"` (new category) |
| `PromptOrder()` | stable order, e.g. server index × 100 + tool index, so docs are deterministic |
| `PromptSection()` | a one-line "from server X" doc plus the description; lets the hardcoded prompt path render it |
| `PromptTemplateName()` | `""` (no template; falls back to `PromptSection`) |

`Call` mapping. An MCP `tools/call` response carries a `content` array of typed blocks (text,
image, audio, embedded resource) and an `isError` flag. Map it as:

- Concatenate `text` blocks into the result string. For non-text blocks, emit a compact
  placeholder (`[image 24KB png]`, `[resource uri=...]`) rather than dumping bytes into the
  context; large or binary payloads are summarized, never inlined.
- If `isError` is true, return a Go error built with `tools.SemanticErrorf` so the agent
  loop treats it as backtrackable and lets the model recover, mirroring how built-in tools
  report recoverable misuse. A transport or protocol failure (server crashed, timeout)
  returns a plain error, which the loop records as a non-recoverable tool error.
- On success, return a small struct so `json.MarshalIndent` produces readable output and
  `ui.GetResultSummary` has a field to summarize, e.g. `{"success": true, "content": "..."}`.

The adapter holds a client reference and the per-call timeout; inside `Call` it derives a
context with that deadline from the (now un-capped) tool context.

### 2.6 The manager and lifecycle

`mcp.Manager` owns the set of clients.

- **Construction / connect.** `NewManager(cfg.MCP, logger)` followed by `Connect(ctx)`.
  `Connect` dials each enabled server **concurrently**, each under its own
  `startup_timeout`. For stdio, "dial" means spawn the subprocess and run the
  `initialize` → `notifications/initialized` → `tools/list` handshake. A server that fails to
  connect or times out is **logged and skipped**, never fatal: a missing MCP server must not
  prevent the agent from running with its built-in tools. This mirrors how `main` degrades
  gracefully when checkpoint init fails
  ([main.go:298](cmd/kvit-coder/main.go#L298)).
- **Adapters.** `Tools() []tools.Tool` returns one `MCPTool` per discovered tool, after
  applying each server's allow/deny filter.
- **Shutdown.** `Close()` sends shutdown/closes transports and reaps subprocesses; wired as a
  `defer` in `main`. Subprocess handling must avoid zombies on early exit (close stdin, wait
  with a timeout, kill the process group on overrun).

### 2.7 The `Check` / confirmation policy

Built-in tools enforce workspace path safety in `Check`. MCP tools run in a separate process
and can touch anything that process can reach, so workspace path checks do not apply. They
need their own trust model, configured by `mcp.confirm` (and overridable per server):

- `block` — refuse every MCP call (useful to load a server for inspection without letting the
  model invoke it);
- `ask_once` — prompt the user once per distinct tool name, then remember for the session
  (reuse the `/dev/tty` prompt pattern from
  [config.go:580](internal/config/config.go#L580));
- `ask_always` — prompt on every call;
- `trust` — no prompt.

`Check` returns a non-backtrackable error when the user declines, which surfaces to the model
as a refusal it should not retry. For headless/benchmark runs (no TTY), `ask_*` must fall back
to `block` rather than hang.

### 2.8 Wiring into `main`

In `cmd/kvit-coder/main.go`, after the config load and before `SetupRegistry`:

```go
// Initialize MCP (no-op when cfg.MCP.Enabled is false or no servers configured)
mcpMgr := mcp.NewManager(cfg.MCP, writer)
if err := mcpMgr.Connect(context.Background()); err != nil {
    writer.Warn(fmt.Sprintf("MCP: some servers failed to connect: %v", err))
}
defer mcpMgr.Close()
```

Pass the manager into `SetupConfig` and register its adapters at the end of `SetupRegistry`
([setup.go](internal/tools/setup.go)):

```go
// SetupConfig gains: MCPTools []Tool   // adapters from the manager (nil when disabled)
for _, t := range sc.MCPTools {
    registry.Enable(t)
    debug(fmt.Sprintf("Enabled MCP tool: %s", t.Name()))
}
```

Passing the already-built `[]tools.Tool` (rather than the manager) keeps `internal/tools`
free of any dependency on `internal/mcp`, preventing an import cycle (`mcp` depends on
`tools` for the `Tool` interface, not the reverse). The benchmark CLI paths
([internal/benchmark/cli.go](internal/benchmark), `thinkbench_cli.go`) build their registry
through the same `main` flow, so they inherit MCP tools automatically; confirm they do not
re-build the registry independently.

### 2.9 Prompt documentation

Add `"mcp"` to the category machinery so MCP tools get their own section:

- `CategoryHeaders` ([registry.go:31](internal/tools/registry.go#L31)):
  `"mcp": "## MCP Tools (external servers)"`.
- the category slice in `GenerateToolPrompt`
  ([registry.go:381](internal/tools/registry.go#L381)) and `categoryOrder` in
  `EnabledCategories` ([registry.go:441](internal/tools/registry.go#L441)): append `"mcp"`.

The template-based prompt path
([internal/prompt/prompt.go:132](internal/prompt/prompt.go#L132)) already iterates
`EnabledCategories()` and falls back to `PromptSection()` when a tool has no template, so MCP
tools render there without new template files. The section should make clear to the model
that these tools come from external servers and may have side effects outside the workspace.

---

## 3. Implementation phases

**Phase 0 — skeleton and decision.** Add the `internal/mcp` package with the `Client`
interface, the `MCPConfig`/`MCPServerConfig` structs, and config defaults. Decide SDK vs.
in-house and pin the dependency. No behavior yet.

**Phase 1 — stdio tools, end to end.** Implement the stdio client (handshake, `tools/list`,
`tools/call`), the manager (concurrent connect, graceful skip, `Close`), and the `MCPTool`
adapter. Wire into `main` and `SetupRegistry`. Land the `"mcp"` prompt category. Exempt MCP
tools from the 15s blanket timeout
([runner_tools.go:364](internal/agent/runner_tools.go#L364)) and route them through the
per-call deadline. At the end of this phase, a configured stdio server's tools are callable by
the model.

**Phase 2 — robustness.** Result/error mapping polish (content blocks, `isError` →
backtrackable), the `confirm` trust model with TTY fallback, allow/deny filters, schema
sanitization for restrictive model templates, and structured logging of MCP traffic for
debugging.

**Phase 3 — HTTP transport.** Add the streamable-HTTP client behind the same `Client`
interface; header/token expansion from env.

**Phase 4 — resources and prompts.** Surface MCP *resources* as context the user can attach
(an `@server:uri` mention resolved via `resources/read`) and MCP *prompts* as user-invokable
templates (`prompts/get`). Subscribe to `notifications/tools/list_changed` to refresh the
registry mid-session if a server's tool set changes.

**Phase 5 — surfacing.** A `/mcp` status view (connected servers, tool counts, last error) in
the headless output and the TUI; show MCP tool calls distinctly in the writer.

---

## 4. Risks and edge cases

- **The 15s tool timeout.** The single hardest constraint; addressed by exempting MCP tools
  alongside `Shell` and applying a per-call deadline. Easy to miss, since the timeout is a
  blanket `if name != "Shell" && name != "Shell.advanced"`.
- **Schema features local templates can't constrain.** Grammar-constrained tool calling on
  some llama.cpp/vLLM builds rejects `$ref`, `oneOf`, `allOf`, or deeply nested schemas. Add
  an optional sanitizer that flattens/strips unsupported constructs and, worst case, falls
  back to a permissive `{"type":"object"}`. Keep it opt-in so faithful schemas are the
  default.
- **Name collisions and illegal names.** Solved by namespacing; the separator is a single
  constant to retune if a model rejects dots.
- **Startup must not block or crash the agent.** Connect concurrently, time-box each server,
  skip failures with a warning. A server that hangs on `initialize` cannot stall the whole
  run.
- **Subprocess lifecycle.** Close stdin, `Wait` with a timeout, kill the process group on
  overrun to avoid orphaned `npx`/node processes. Verify behavior on agent `Ctrl-C` and on
  benchmark teardown.
- **Security and trust.** MCP tools bypass workspace path safety entirely and may carry
  secrets in `env`/`headers`. The `confirm` policy is the gate; document clearly that
  enabling a server grants the model whatever that server can do. Never log expanded
  secrets (the safety audit redactor in [internal/safety](internal/safety) is a reference for
  redaction).
- **Concurrency in the client.** The agent loop is single-threaded per session, but a stdio
  client still needs a reader goroutine and an id-keyed pending-request map to correlate
  responses, plus handling of server-initiated notifications.
- **Duplicate-call guard vs. side-effecting tools.** A non-idempotent MCP tool legitimately
  called twice with identical args will trip the duplicate-call abort
  ([runner_tools.go:191](internal/agent/runner_tools.go#L191)). Acceptable initially; note it,
  and consider a per-tool opt-out later.
- **Result size.** Tool results land in the context window verbatim. Cap MCP text results
  (reuse the temp-file spill pattern that `Search`/`Shell` use via `TempFileManager`) and
  summarize binary blocks.
- **Protocol version drift.** Pin the supported MCP protocol version, negotiate it in
  `initialize`, and fail a server cleanly if it requires a newer one.

---

## 5. Testing

- **Adapter unit tests.** Drive `MCPTool` against a fake `Client` (in-memory): schema
  pass-through, namespacing, text-block concatenation, `isError` → backtrackable
  `SemanticError`, transport failure → plain error, timeout behavior.
- **Manager unit tests.** Concurrent connect with one failing server (verify graceful skip),
  allow/deny filtering, `Close` reaping.
- **Stdio integration test.** Stand up a tiny in-repo stdio MCP server (or the official SDK's
  in-memory transport) exposing one echo tool; assert the model-facing flow:
  `SetupRegistry` registers `mcp.echo.echo`, `Specs()` includes it, and a `Call` round-trips.
- **No-regression guard.** With `mcp.enabled: false` (the default), `SetupRegistry` output,
  `Specs()`, and the generated system prompt must be byte-identical to today — important for
  prompt-cache stability and for the benchmark suite. Add a test asserting the empty-MCP path
  is a no-op.
- **Loop/dup detection.** Confirm an MCP tool participates in loop detection exactly like a
  built-in (same-args repeat is recorded and eventually aborted).

---

## 6. Minimal end-to-end config example

```yaml
llm:
  base_url: "http://127.0.0.1:8080/v1"
  model: "my-model"

workspace:
  root: "."

tools:
  read: { enabled: true }
  edit: { enabled: true, mode: "searchreplace" }
  search: { enabled: true }
  shell: { enabled: true }

mcp:
  enabled: true
  confirm: "ask_once"
  servers:
    - name: "git"
      enabled: true
      transport: "stdio"
      command: "uvx"
      args: ["mcp-server-git", "--repository", "."]
```

With this config the agent gains tools such as `mcp.git.git_status` and `mcp.git.git_log`
alongside its built-in `Read`/`Edit`/`Search`/`Shell`, and the user is prompted once before
the first call to each.
