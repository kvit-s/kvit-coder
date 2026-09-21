# External tools, project instructions and the agent file

Three ways to change what the model is given without changing the program: tools
from Model Context Protocol servers, per-project instructions read from a file
in the workspace, and the agent file that carries a session's task description.


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

## Project instructions

Headless runs automatically load `CLAUDE.md` from the directory where `kvit-coder` was launched, then apply the `-p` prompt as the user request. The instructions are sent as user-level context, not appended to the system prompt, matching Claude Code's behavior more closely than `agent_file`.

Customize or disable this in config:

```yaml
agent:
  project_instructions:
    enabled: true
    path: CLAUDE.md   # relative to the launch directory, or absolute
```

## The agent file

Append custom instructions to the system prompt without modifying config:

```bash
./kvit-coder -p "refactor auth module" -agent-file AGENT.md
```

Or in config:

```yaml
agent:
  agent_file: AGENT.md
```
