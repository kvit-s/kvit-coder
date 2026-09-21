# Configuration reference

Every key in `config.yaml`, grouped by the section it sits in. The file is found
at the first of `$KVIT_CODER_CONFIG`, `./config.yaml`,
`~/.kvit-coder/config.yaml`, and `config.yaml` beside the binary with symlinks
resolved; `config.example.yaml` in the repository root is a short working
starting point.


## `llm`

| Key | Description |
|-----|-------------|
| `base_url` | OpenAI-compatible API endpoint |
| `api_key` / `api_key_env` | API key or env var name |
| `model` | Model name |
| `api_backend` | Wire protocol: `chat_completions` (default) or `responses` |
| `headers` | Extra `Key=Value` request headers (`${VAR}` expanded; `${KVIT_RUN_ID}` = per-conversation ID) |
| `reasoning_effort` | Thinking budget for a reasoning model (`minimal`, `low`, `medium`, `high`, `xhigh`) |
| `effort_field` | Which field of a `chat_completions` request the effort travels in: `chat_template_kwargs` (default) or `reasoning_effort`. See below |
| `temperature` | Sampling temperature |
| `max_output_tokens` | Max output tokens |
| `context` | Max context size for display (0 = hide) |
| `merge_thinking` | Merge the model's thinking into `content` (endpoints spell it `reasoning_content` or `reasoning`; both are read) |
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

**Where the effort goes on `/chat/completions`.** The `responses` backend has
one place for the reasoning effort, `reasoning.effort`, and always uses it.
Chat completions has two, and they are not interchangeable:

- `chat_template_kwargs` — the effort is passed to the model's chat template as
  `chat_template_kwargs.reasoning_effort`. This is how a model served by a local
  llama.cpp or vLLM reads it; Qwen's template does. It is the default.
- `reasoning_effort` — the top-level field of the same name, which is OpenAI's
  own spelling and what hosted gateways read. opencode.ai's `/chat/completions`
  wants this one.

An endpoint sent the spelling it does not read answers normally and thinks for
as long as it likes, so the wrong value here costs nothing visible: the effort
picked with `:eN` simply never arrives, and nothing says so. Set `effort_field`
per model in the `models:` catalog, next to `api_backend`. The same model can be
served either way by different providers, so it is a property of the endpoint
rather than of the model.

An endpoint that routes by a session header sends every request carrying the
same header value to one backend, so two agents sharing a value compete for the
same prompt cache. `${KVIT_RUN_ID}` in a header expands to an ID derived from
the session name, which keeps concurrent conversations apart while sending
every turn of one conversation to the backend that already holds its prompt
cache. Setting `KVIT_RUN_ID` in the environment pins a value instead.

## `workspace`

| Key | Description |
|-----|-------------|
| `root` | Workspace root directory |
| `lock` | Refuse to start when another agent is working in this directory (default: `false`) |
| `path_safety_mode` | `allow` (what `--yolo` sets), `block`, `warn`, `ask_once` (default), `ask_always` |
| `allowed_paths` / `allowed_read_paths` | Paths allowed outside workspace |
| `denied_paths` | Explicitly denied paths |

## `agent`

| Key | Description |
|-----|-------------|
| `profile` | `strong` (default) or `weak` — see below |
| `max_tool_iterations` | Max tool calls per run |
| `agent_file` | Path to agent instructions file |
| `project_instructions.enabled` | Load project instructions before `-p` prompts (default true) |
| `project_instructions.path` | Project instructions file path; relative paths resolve from the launch directory (default `CLAUDE.md`) |

**Profiles.** The profile also chooses the system prompt. Under `strong` it is
the environment this session is running in, how the session works (steering,
background processes, questions, interruption), and each tool's failure modes —
about 4KB. Under `weak` it keeps the numbered workflow, the worked editing
example and the long tool documentation, about 7KB. Whether the shorter prompt
scores better is a measurement: run thinkbench with each.

A lot of the loop exists to catch a model getting confused:
backtracking away from a bad tool call, ending a turn after three identical
calls, a confirm handshake before an edit is applied, scraping tool calls out of
prose, fuzzy matching and indentation repair, asking the model to explain an
anomaly, retrying an empty answer, and keeping `sed -i`, `eval` and interpreter
one-liners away from a model that would use them instead of the Edit tool. On a model that does not make those mistakes each
one is a tax — a retry that discards good work, a handshake that costs two round
trips per edit, a fuzzy match that silently edits the wrong lines, a refusal to
run a command the model had good reason to run. `profile: strong`, the default,
skips all of it; `profile: weak` reproduces the earlier behaviour exactly.

`agent.profile` is the default for every model. A `models:` entry may set its
own `profile:` (`strong` or `weak`) to override it for that model — a strong
hosted model and a weak local one can share one config that way. Switching
models with `:mN` (or `--model` headless) takes the new entry's profile from
the next turn on; `:h` names each entry's effective profile once any entry
sets one.

## `tools`

Each tool group has `enabled: true/false` plus tool-specific options:

- **`edit.mode`** — `"lines"`, `"searchreplace"`, or `"patch"`
- **`edit.preview_mode`** — Stage edits and overwrites for confirmation, which is
  what registers `Edit.confirm`/`Edit.cancel` and `Write.confirm`/`Write.cancel`.
  Off under `agent.profile: strong`, where Write overwrites directly and the
  previous contents are in the turn's checkpoint.
- **`edit.fuzzy_threshold`** — Fuzzy matching for searchreplace mode (0 = exact only)
- **`edit.read_before_edit_msgs`** — Require a read within N messages before editing
- **`shell.allowed_commands`** / **`shell.disallowed_commands`** — Command allow/blocklists
- **`shell.default_timeout`** / **`shell.max_timeout`** — Seconds a command gets, and the ceiling a call may ask for (default 120 and 600)
- **`batch.enabled`** — Run several independent tool calls in one request
- **`procs.enabled`** — Background processes that outlive a turn (`Shell.start`, `Observe.wait`, …)
- **`procs.kill_on_exit`** — Stop everything the session started when a turn is interrupted (default true)
- **`question.enabled`** — Let the model ask you a question and wait for the answer
- **`question.timeout`** — Seconds a run with no terminal waits for an answer (default 0: fall back at once)
- **`web.api_key_env`** — Name of the environment variable holding the search API key (default `BRAVE_API_KEY`). The key is named rather than written into the file because it is usually shared with other programs
- **`web.search.max_attempts`** — Tries before giving up on a rate-limit refusal (default 4). The provider allows one request per second shared across everything using the key, and refuses the excess rather than queueing it; a refusal costs no quota, so retrying is cheap next to handing the model a failure
- **`web.usage_log`** — JSONL record of what was searched and when. Not a counter: the quota is account-wide and only the provider can see what other machines spent, which is why the response's `x-ratelimit-*` headers are read instead
- **`web.fetch.max_bytes`** / **`web.fetch.timeout`** — Ceiling on HTML read, and seconds for one page (defaults 5 MB and 30)
- **`checkpoint.max_turns`** — Max checkpoints before rotating (default: 100)
- **`tasks.collapse`** — Enable context collapsing (stage 2)
- **`tasks.plan`** — Enable plan-based task tools (stage 3)

## `backtrack`

Automatic retry on failed tool calls (disabled by default):

```yaml
backtrack:
  enabled: true
  max_retries: 5
  inject_user_message: false
```

## `safety`

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

## `prompts`

Optional template-based prompt system:

```yaml
prompts:
  use_templates: false
  templates_dir: ""        # override embedded templates
  hot_reload: false        # reload on each request (dev mode)
```

## `ui`

Terminal output styling, and the window title the interactive front end keeps
up to date (see [Window title](#window-title)):

```yaml
ui:
  markdown: auto           # auto | always | never; auto styles only on a terminal
  terminal_title:
    enabled: true          # false leaves the terminal's title alone
    running: "⏳"           # icon while a turn is running ("" for none)
    asking: "💬"            # icon while a prompt waits for your answer
    waiting: "💤"           # icon while nothing is running and the composer is open
```
