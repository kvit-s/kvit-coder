# kvit-coder: architecture review and readiness for daily agent use

Reviewed at commit `fe77715` (`llm: talk to endpoints that only serve the Responses API`), 7 September 2026.

## 1. What this document is

kvit-coder is a coding agent written in Go: it sends a conversation to an
OpenAI-compatible model endpoint, receives tool calls, runs them against a
workspace directory on disk, feeds the results back, and repeats until the model
answers without asking for a tool. It was written to drive models running on
local hardware, where the interesting engineering problem was compensating for
models that mis-format tool calls, repeat themselves, and misjudge line numbers.
Along the way the project grew a benchmark harness — three families of it — and
most of the recent work went there rather than into the agent.

The current goal is different. Muse Spark 1.3, reached through the opencode.ai
subscription endpoint, is a capable hosted model, and the intention is to use
kvit-coder as an ordinary working agent driving it. This review answers three
questions: what the program does today, which parts hold up under that new use,
and what has to change. Section 9 gives a phased plan; section 10 gives a
configuration you can adopt immediately.

The reader is assumed to know the code exists but not to remember its details,
so section 2 describes the whole system before anything is judged.

## 2. The system as it stands

### 2.1 Size and shape

| Package | Non-test lines | What it does |
|---|---:|---|
| `internal/tools` | 11,064 | Tool implementations: read, write, three edit modes, search, shell, plan, checkpoint, tasks |
| `internal/benchmark` | 5,313 | Three benchmark harnesses (tool benchmarks, haystack retrieval, thinkbench) |
| `internal/agent` | 2,410 | The run loop, loop detection, backtracking, diagnostics |
| `internal/mcp` | 1,578 | Model Context Protocol client (stdio and HTTP transports) |
| `internal/safety` | 1,554 | Shell command parsing and rule evaluation |
| `internal/prompt` | 1,144 | System prompt generation, hardcoded and templated |
| `internal/ui` | 1,094 | Terminal output formatting, input model |
| `internal/config` | 934 | YAML config, path permissions |
| `internal/llm` | 855 | HTTP client, chat-completions and Responses protocols, response normalization |
| `internal/context` | 832 | Tasks tools: a git repo of conversation turns, for context collapsing |
| `internal/checkpoint` | 807 | Shadow git repo of the workspace, per-turn snapshots |
| others | ~1,100 | TUI shell, sessions, REPL entry, workspace lock, stats |

Total: 29,272 lines of non-test Go across 102 files, plus 11,764 lines across 39
test files. `go vet ./...` is clean; `go build ./...` succeeds; one test fails
(section 8).

Two binaries are produced. `kvit-coder` is headless: it requires `-p "prompt"`
or a benchmark flag, runs one conversation, prints the final answer to stdout
and everything else to stderr, and exits. `kvit-coder-ui` is an interactive
shell that reads a line of input and then **spawns `kvit-coder -p` as a child
process** for each turn, passing the session name so history passes through
a file in `~/.kvit-coder/sessions/`.

### 2.2 The run loop

`agent.Runner.Run` (`internal/agent/runner.go:181`) is a bounded loop, capped by
`agent.max_tool_iterations` (default 10 when unset, 1000 in the current config).
Each iteration does the following:

1. Calls the model with the full message history and every enabled tool's JSON
   schema, non-streaming, `tool_choice: "auto"`. A goroutine prints a dot per
   second while waiting.
2. On error, classifies it: user cancellation, run-deadline expiry, HTTP 400
   (treated as context overflow, see section 6), or fatal.
3. Normalizes the response — fills in missing `type: "function"` on tool calls,
   and if there are no tool calls at all, tries to *parse tool calls out of the
   text* in five different syntaxes (Anthropic-style XML, `<tool_call>` JSON,
   `<function=Name>` XML, `toolname{...}`, and a bare JSON object keyed by tool
   name).
4. If the message has no tool calls, prints it as the final answer and exits the
   loop. If it has tool calls, executes them one at a time, in order.
5. Around each tool call sit several guards: a pending-edit state machine, a
   duplicate-call detector, argument type coercion, a `Check()` validation pass,
   a 15-second blanket timeout, and loop detection over the recent call history.
6. After the tools, injects `<system-reminder>` blocks into the last tool result
   when a loop was detected, ends the checkpoint turn, and optionally re-injects
   the current plan.

Recovery machinery is layered on top: **backtracking** discards the assistant
turn and its history back to a rollback point when a tool call fails with a
"semantic" error, so the model gets another attempt at the same decision point
without the failure in its context; **loop detection** watches for the same
tool called repeatedly with the same arguments, the same tool failing
repeatedly, and alternating cycles such as edit-then-cancel; **interrogation**
(off by default) asks the model in a side conversation to explain why it
repeated itself, and logs the answer to JSONL for later analysis.

### 2.3 Tools

| Tool | Notes |
|---|---|
| `Read` | Files and directories, line mode or byte mode, negative offsets read from the end. Output capped at 150 lines / 24 KB by default. |
| `Write` | Atomic write via temp file and rename. **Overwriting an existing file always requires a second call** to `Write.confirm`. |
| `Edit` | One of three modes chosen in config: `lines` (start/end line replacement), `searchreplace` (exact text match), `patch` (unified diff). Optional preview mode adds an `Edit.confirm`/`Edit.cancel` handshake. Fuzzy matching, first-line indent auto-correction, and an explicit `DeleteLines` tool are all optional extras. |
| `Search` | ripgrep when available, GNU grep otherwise. Pattern, path, file glob, context lines. Results above 20 matches degrade to `file:line:match`, above 100 spill to a temp file. |
| `Shell` / `Shell.advanced` | `sh -c`, process group kill, output spilled to a temp file when large. Default timeout 30 s, capped at 3 minutes. |
| `Plan.*` | Either one idempotent `Plan.write` taking the whole step list, or a five-tool incremental family. Disabled by default. |
| `Checkpoint.*` | List, diff, restore, undo over per-turn snapshots held in a shadow git repo under the system temp directory. Disabled by default. |
| `Tasks.*` | Wrap exploratory work so intermediate turns can be collapsed out of context; includes accept/decline of the file changes a task made. Mutually exclusive with Plan and Checkpoint. Disabled by default. |
| `mcp.<server>.<tool>` | Tools discovered from external MCP servers, namespaced and surfaced as ordinary tools. Disabled by default. |

Tools are registered by `tools.SetupRegistry` purely from config
(`internal/tools/setup.go`), and each tool carries its own documentation
section, category, and sort order, so the system prompt is generated from
whatever is enabled. Internal names containing dots (`Plan.write`) are aliased
to underscore forms for the wire protocol, and rewritten back in prompt text, so
the model sees names that satisfy OpenAI's `^[A-Za-z0-9_-]+$` rule.

### 2.4 The model transport

`internal/llm` speaks two protocols behind one `Chat` method. The default is
OpenAI chat completions. The alternative, added in the most recent commit, is
OpenAI's Responses API, which is the only way the opencode.ai endpoint serves
Muse Spark. The Responses path flattens the message history into an item list,
converts tool schemas to the flat shape that API expects, and — the part that
matters for a reasoning model — stores the opaque encrypted thinking blocks the
model returns and replays them verbatim on the next request of the same tool
loop, so the model keeps its train of thought across tool calls.

The HTTP layer retries network errors, 429s and 5xx up to ten times with
exponential backoff to a 128-second ceiling, recognizes a class of 500s that are
permanent (chat-template validation failures from llama.cpp and vLLM) and does
not retry those, and repairs a truncated JSON body by appending a closing brace,
which works around a llama.cpp `Content-Length` bug.

### 2.5 Configuration, sessions, safety

Configuration is a single YAML file; `config-<suffix>.yaml` is selected
automatically by the benchmark flags. Everything is opt-in: tools are disabled
unless enabled. The current root `config.yaml` points at Muse Spark with
`api_backend: responses`, `reasoning_effort: high`, a per-process
`x-opencode-session` header, `lines` edit mode, and Read, Edit, Search and Shell
enabled.

Sessions are JSON-lines files of the message history under
`~/.kvit-coder/sessions/`, written once at the end of a run. A `CLAUDE.md` in
the launch directory is loaded and prepended to the user's prompt as user-level
context, then stripped again before the session is saved.

Safety has two independent layers. Path safety decides whether a tool may touch
a path outside the workspace root, with modes `block`, `warn`, `ask_once` and
`ask_always`; prompts read from `/dev/tty` directly, so they work even when
stdin is closed. Command safety is a set of pattern rules over the shell string:
privilege escalation, package managers, system control and network binaries are
blocked when they appear in command position, `rm -rf /` and `rm -rf ~` are
blocked, and — when the Edit tool is enabled — `sed -i` and `awk ` are refused
with a message telling the model to use Edit instead. Interpreter one-liners
(`python -c`, `node -e`) are blocked unless `allow_interpreters` is set.

### 2.6 The benchmark harness

Three families share the binary. The tool benchmarks run scripted tasks in a
scratch workspace and score them with declarative validators. The haystack
benchmarks measure needle retrieval across one to five reasoning hops in a large
context. Thinkbench runs 72 autonomous coding tasks from a third-party suite,
each in a fresh workspace confined by bubblewrap or firejail, graded afterwards
by a held-out `grade.py` that never enters the workspace before the agent stops.
Results are written as Markdown reports and resumable CSVs.

## 3. What works well

**The tool abstraction.** A tool is one interface with `Check` and `Call` plus
its own prompt documentation and sort order. Adding a tool touches one new file
and one line in `SetupRegistry`. Specs are emitted in sorted name order
specifically so the prompt prefix stays byte-identical between requests and the
server's prompt cache keeps hitting. That discipline is worth keeping.

**The Responses-API translation.** This is the newest code and the best-argued.
Replaying encrypted reasoning blocks is what makes a reasoning model usable
inside a tool loop at all, and the comments explain the three protocol
differences clearly enough that the next person can maintain it. It is also the
only reason kvit-coder can talk to Muse Spark.

**The transport's failure handling.** Distinguishing permanent 500s from
transient ones, repairing truncated bodies, and backing off on 429s are the
kinds of things you only write after being burned, and they cost nothing when
the endpoint behaves.

**Shell command matching by position.** The blocklist regexes match a binary
only at the start of a command or after a shell separator, with an optional path
prefix. The comment in `internal/tools/shell.go:19` lists the false positives
that motivated it — `grep -r shutdown .`, `man curl`, `echo "adapt the layer"`.
Most agents get this wrong with substring matching.

**The MCP client.** Concurrent connection at startup, per-server timeouts, a
four-level trust policy that falls back to `block` when there is no terminal,
schema sanitization for grammar-constrained templates, and namespaced tool
names. A server that fails is logged and skipped rather than fatal. This is the
cheapest route to closing the tool gaps in section 6, because it needs no new
code in this repository.

**Thinkbench.** A held-out grader, a real OS sandbox, and a preflight test that
proves a read outside the workspace is denied before any task runs. It is the
only part of the project that can tell you whether a change to the agent made
things better or worse, and it applies just as well to agent changes as to model
comparisons.

**The headless contract.** Final answer on stdout, progress on stderr, optional
`--json` with token and timing statistics. That makes the binary scriptable, and
it is why the interactive shell could be built as a thin wrapper at all.

## 4. What was built for weak models and now gets in the way

Roughly a third of the agent code exists to survive models that could not be
trusted with a file edit. Against Muse Spark most of it is inert overhead, and
several pieces actively cause harm.

**The duplicate-call kill switch.** Three consecutive tool calls with identical
arguments terminate the entire run
(`maxConsecutiveDuplicates = 3`, `internal/agent/runner.go:156`;
`internal/agent/runner_tools.go:211`). Before that, the second identical call
gets an error telling the model it is looping. A strong model repeating
`git status` or re-running the same test command after a change it believes it
made has done nothing wrong, and losing the run for it is a poor trade.

**Backtracking.** When a tool call fails with an error classified as semantic,
the assistant turn is discarded and history is rewound so the model retries the
same decision without seeing its own mistake. That helps a model that cannot
read an error message. It hurts a model that can: the error is the single most
useful piece of context available, and hiding it invites the same mistake again
under a different guise. It is enabled in the current config with
`max_retries: 5`.

**The confirm handshakes.** Preview mode makes every edit a two-call sequence
(`Edit` then `Edit.confirm`), and `Write` demands `Write.confirm` for any
overwrite regardless of configuration (`internal/tools/filesystem.go:1135`).
Preview mode is off in the current config; the Write handshake is not
configurable and cannot be turned off.

**Three edit modes and their support machinery.** `internal/tools/edit*.go` is
7,854 lines including tests: line-range edits, exact search/replace, unified
patches, fuzzy matching with a configurable threshold, first-line indent
auto-correction with an undo tool, an explicit delete tool, a read-before-edit
tracker, and a pending-edit state machine that counts how many times the model
ignored a pending confirmation. One mode — exact search/replace, applied
immediately — is what a capable model needs.

**Text-scraped tool calls.** `Registry.ExtractToolCallsFromText`
(`internal/tools/registry.go:268`) parses five syntaxes out of assistant text
when no structured tool calls are present. It only runs when `tool_calls` is
empty, so it is mostly harmless, but the last fallback scans any JSON object in
the message for a key matching a tool name. An answer that legitimately contains
`{"Read": {...}}` would be converted into a tool call.

**The interrogation diagnostics.** Every anomaly can trigger an extra model call
asking the model to explain itself, logged to JSONL. Useful for benchmark
forensics and pure cost in normal use. Off by default; keep it off.

The right structural move is to make these a *profile* rather than a set of
defaults: keep the code, gate it behind something like `agent.profile: weak`,
and let the strong-model path skip all of it. That preserves the local-model
work — which is the interesting part of the project's history — without paying
for it every turn.

## 5. Defects found

| # | Where | Problem |
|---|---|---|
| 1 | `internal/agent/interrogate_test.go:43` | `TestBuildInterrogationQuestion` fails. The test asserts the generated question contains "make progress" and "diagnostics only"; the prompt was reworded and the test was not. It is the only failing test in the suite. |
| 2 | `internal/agent/runner.go:256`, `internal/agent/runner_llm.go:221` | `GetGenerationStats` is called **twice per iteration** — once inside `processLLMResponse` for token and cost accounting, once again in `Run` for the backtrack cost figure. It hits `<base_url>/generation?id=…`, an OpenRouter-specific endpoint that opencode.ai does not serve, so both calls are wasted round trips. Worse, they pass `context.Background()`, and the shared `http.Client` has no `Timeout` set, so a hanging endpoint hangs the agent with no way out. |
| 3 | `internal/llm/client.go:74` | `DisableKeepAlives: true` was a workaround for llama.cpp EOF behaviour. Against a hosted HTTPS endpoint it forces a fresh TCP and TLS handshake on every request, including all ten retry attempts. |
| 4 | `internal/config/permissions.go:49` (and the denied/allowed loops above it) | Workspace containment is tested with `strings.HasPrefix(absPath, workspaceAbs)` with no separator check, so `/home/sk/kvit-coder-notes` counts as inside `/home/sk/kvit-coder`. The tool-side `NormalizeAndValidatePath` does this correctly with `filepath.Rel`; this copy does not. No symlink resolution happens anywhere, so a symlink inside the workspace pointing out of it is followed. |
| 5 | `internal/agent/runner.go:194` | When the iteration cap is reached the loop simply ends. Nothing is printed, no final answer is produced, and the caller cannot distinguish "finished" from "ran out of turns". |
| 6 | `internal/repl/repl.go:72` and `internal/workspace/lock.go:55` | The exec path runs with `context.Background()`, and the only SIGINT handler is the workspace lock's, which cleans up the lock file and calls `os.Exit(130)`. Ctrl-C therefore discards the entire conversation: the session file is written only after `Run` returns normally. |
| 7 | `internal/agent/permissions.go:23,60` | `PromptForPermission` and `UpdateConfigFile` are dead code — nothing calls them. `UpdateConfigFile` prints `✓ Added <path> to allowed_paths in .kvit-coder.yaml` and then returns without writing anything. |
| 8 | `internal/agent/runner_llm.go:110` | Any HTTP 400 is assumed to be context overflow. The handler rewrites every trailing tool result to a placeholder saying the output could not be processed, retries twice, then injects "try a different approach" messages. Against a hosted API a 400 usually means a malformed request — a bad tool schema, an item the Responses API rejected — and this turns a diagnosable error into a corrupted transcript and a confusing loop. |
| 9 | `internal/llm/types.go:24,27` | `ReasoningBlocks` and `ToolCallItemIDs` are tagged `json:"-"`, so they are not persisted in the session file. Resuming a session with `-s` replays `function_call` items whose matching `reasoning` items are gone. Whether the endpoint accepts that needs testing; if it does not, session resume is broken for the Responses backend. |
| 10 | `internal/agent/runner_tools.go:376` | The blanket 15-second per-tool timeout is a hardcoded constant. A `Search` across a large tree, or a `Read` of a large file on a slow disk, fails as a timeout with no way to raise it. |
| 11 | `internal/tools/shell.go:516` | `awk ` is refused outright whenever the Edit tool is enabled, which also blocks it as a pipeline filter (`ps aux \| awk '{print $2}'`). `curl`, `wget` and `nc` are blocked unconditionally with no configuration override, so the agent cannot fetch a URL or probe a server it just started. `python -c` and friends are blocked unless `allow_interpreters` is set, which is off in the root config. |
| 12 | `internal/llm/middleware.go:105` | `PreventConsecutiveAssistant` silently drops the last message when it is an assistant message. In the current flow it rarely fires, but it deletes conversation content without a trace when it does. |
| 13 | `internal/mcpclient/` | Empty directory left behind. |

## 6. Gaps that block daily use

These are absences rather than bugs — things the program never had, which a
model-agnostic daily agent needs.

**Command execution is too short-lived.** The default shell timeout is 30
seconds and the hard ceiling is 3 minutes
(`internal/tools/shell.go:295`). A Go build of this repository, a full test
suite, an `npm install`, or a Docker build exceeds that. There is no way to
start a process in the background, tail its output, or check on it later, so
anything long-running is simply unavailable to the agent.

**Nothing streams.** Every request is `Stream: false`. With
`reasoning_effort: high` the user watches a row of dots for as long as the model
thinks, sees nothing of the reasoning, and gets the whole turn at once. It also
means an interrupted request yields nothing at all, and there is no way to tell
a stuck request from a slow one.

**No context management.** The iteration cap is set to 1000, the context window
is 1M tokens, and nothing measures how full it is: `llm.context` is used only
for a display string. There is no compaction, no summarization, no dropping of
stale tool output, and no warning as the window fills. A long session grows
until the endpoint rejects it, at which point defect 8 turns the rejection into
a corrupted transcript. The `Tasks.*` tools are the closest thing to an answer,
but they require the model to opt into wrapping its own work, and enabling them
disables Plan and Checkpoint.

**The interactive shell is a wrapper, not an agent.** `kvit-coder-ui` spawns a
fresh `kvit-coder -p` per turn (`internal/tui/ui.go:267`). The consequences
compound: the model process starts cold each turn, the session file is read and
rewritten in full each turn, the checkpoint session ID is regenerated from the
clock each turn so per-turn snapshots and undo never accumulate, path
confirmations granted in one turn are asked again in the next because they live
in process memory, Ctrl-C kills the child and loses the turn, and there is no
live status, no queued input, and no way to interrupt a running tool and keep
the conversation. For daily use this is the single largest structural problem.

**The system prompt says very little.** The generated prompt is a role line, a
capability list, a four-step workflow, one worked editing example, three
guidelines, and the tool documentation
(`internal/prompt/prompt.go:201`). It contains no environment information — no
operating system, no shell, no date, no working directory listing, no git branch
or status, no indication of what kind of project this is. It gives no guidance
on verifying work, on when to stop, on how much to explain, or on matching
existing code conventions. For a strong model the prompt is the cheapest lever
available and it is barely being pulled.

**Missing tools.** There is no glob or file-listing tool (directory listing is a
`Read` on a directory, and anything recursive goes through the shell), no
multi-edit that applies several edits to one file in a call, no web fetch, no
git-aware helper, no subagent or delegation mechanism, and no image input.
Several of these are available immediately through MCP servers.

**The approval model is all-or-nothing.** Inside the workspace, every shell
command runs without asking; outside it, path safety asks once per path. There
is no per-command approval, no allowlist that grants routine commands and
prompts for the rest, and no dry-run. `git push` is permitted unless
`safety.git.block_push` is set, and `rm -rf build` inside the workspace is
permitted outright.

**Cost and token reporting is shaped for the wrong provider.** Everything
interesting — native token counts, cached tokens, cost, cache discount, prompt
and generation latency — comes from the OpenRouter `/generation` endpoint, so
against opencode.ai those fields stay zero and the printed statistics are
`usage` numbers only.

## 7. Repository hygiene

Of 734 tracked files, 556 are under `benchmarks/`, and most of those are dated
Markdown reports and terminal captures from individual runs. Fifteen planning
and design documents sit in the repository root alongside the code
(`eval-plan.md` alone is 42 KB). Compiled binaries are correctly gitignored, and
`.gitignore` covers the benchmark runtime directories.

None of this affects behaviour, but it makes the project hard to re-enter, and
it makes `git log` and file search noisy. A `docs/` directory for the design
notes and an `archive/` or separate branch for historical benchmark reports
would leave a root that reads as an agent project rather than an experiment log.

## 8. Testing

39 test files, 11,764 lines, covering the edit modes heavily (fuzzy matching,
patch application, line application, autoindent, deletion), plus registry, path
utilities, shell validation, config, MCP protocol handling, sessions,
checkpoints and prompt templates. One test fails, and it is a stale assertion
rather than a real regression (defect 1).

What is not covered is the agent loop itself. There is no test that drives
`Runner.Run` against a fake model and asserts on the resulting message history,
which is exactly where the risky logic lives: backtracking, loop interventions,
the 400 handler, the duplicate-call kill switch, iteration-cap behaviour. A
table-driven test with a scripted fake `llm.Client` would make the changes
proposed below far safer to attempt.

## 9. Recommended plan

Ordered by value per unit of work. Phases 0 and 1 are what stand between the
current state and comfortable daily use.

### Phase 0 — configuration and small fixes (a few hours)

1. Fix the stale interrogation test (defect 1).
2. Delete both `GetGenerationStats` calls, or gate them behind a config flag
   that is off unless the endpoint is OpenRouter, and give the `http.Client` an
   explicit timeout regardless (defect 2).
3. Make `DisableKeepAlives` conditional — on for plain-HTTP local endpoints, off
   otherwise (defect 3).
4. Turn off backtracking, raise or remove the duplicate-call kill switch, and
   set the shell defaults higher (section 10).
5. Print a clear message when the iteration cap is reached, and return a flag on
   `RunResult` so callers can tell (defect 5).
6. Add `shell.allow_interpreters: true` and make the `awk`, `curl`, `wget`
   blocks configurable rather than unconditional (defect 11).

### Phase 1 — the things you will notice every day (one to two days)

1. **Interrupt handling.** Install a SIGINT handler in `main` that cancels the
   run context instead of exiting; let `Run` return normally; save the session
   before exiting. A second Ctrl-C can hard-exit. This alone removes the worst
   daily failure mode (defect 6).
2. **Streaming.** Implement SSE parsing for both backends and print assistant
   text and reasoning summaries as they arrive. This is the largest single
   improvement to the felt experience of a slow, high-effort reasoning model.
3. **Shell execution that fits real work.** Raise the default timeout to
   something like 120 s and the ceiling to 10 minutes, make both configurable,
   and add a background mode: start a command, return a handle, let the model
   poll output and kill it. Tests, builds and dev servers all need this.
4. **A real system prompt.** Add an environment block — OS, shell, working
   directory, date, git branch and short status, top-level directory listing —
   and guidance on verification and stopping. Keep it generated from enabled
   tools as it is now.

### Phase 2 — one binary, one loop (two to four days)

Fold the interactive shell into the agent process: keep `kvit-coder -p` as the
headless entry point, and make the interactive mode a loop around the same
in-process `Runner` rather than a subprocess spawner. That gives, at once, a
persistent checkpoint session across turns, remembered path approvals,
cancellation of a running tool without losing the conversation, live status
display, and history kept in memory rather than round-tripped through a file.
The existing `internal/ui` input model and writer can be reused as they are.

### Phase 3 — context and tools (a few days)

1. **Token budget and compaction.** Track prompt tokens against `llm.context`
   after every turn, warn at a threshold, and implement compaction: summarize
   the older half of the conversation, keep the last N turns verbatim, and
   replace stale tool outputs with a one-line note. Expose it as a `:compact`
   command as well as an automatic trigger.
2. **Fix the 400 handler** (defect 8): surface the real error, and only apply
   the tool-output-rewriting path when the response body actually indicates a
   length or context problem.
3. **Missing tools.** A `Glob` tool and a `MultiEdit` tool are small and
   immediately useful. For web fetch and git, prefer MCP servers over new code.
4. **Session resume for the Responses backend** (defect 9): persist
   `ReasoningBlocks` and `ToolCallItemIDs`, or verify that resuming without them
   is accepted and document that.

### Phase 4 — retire the weak-model machinery (a day, mostly deletion)

Introduce `agent.profile` with values `strong` (default) and `weak`. Under
`strong`, skip backtracking, the duplicate-call kill switch, the confirm
handshakes, the text tool-call scraper, autoindent correction and fuzzy
matching. Pick `searchreplace` with `exact_match_only` as the one supported edit
mode for that profile and leave the others reachable only under `weak`. Run
thinkbench before and after so the decision rests on measured scores rather than
on this document's argument.

## 10. Configuration to adopt now

Against Muse Spark, with no code changes, these edits to `config.yaml` remove
the most obstructive weak-model behaviour:

```yaml
agent:
  max_tool_iterations: 200      # 1000 turns of a 1M-token context is not a real budget

backtrack:
  enabled: false                # let the model see its own errors

tools:
  read:
    max_read_size_kb: 128       # was 24 — the model has a 1M context, use it
    max_partial_lines: 800      # was 150
    show_line_numbers: false    # correct for searchreplace mode

  edit:
    mode: "searchreplace"       # was "lines" — content-based, no line arithmetic
    preview_mode: false
    exact_match_only: true
    fuzzy_threshold: 0.0
    read_before_edit_msgs: 0

  search:
    max_snippet_results: 40
    max_compact_results: 300

  shell:
    enabled: true
    allow_interpreters: true    # python3 -c is a legitimate way to check something

diagnostics:
  interrogate_on_anomaly: false
```

The three limits that config cannot reach are the shell timeout ceiling
(3 minutes), the blanket 15-second tool timeout, and the duplicate-call kill
switch at three identical calls. Those need the Phase 0 code changes.

## 11. Summary judgement

The tool layer, the transport, the MCP client and the benchmark harness are
solid work that transfers directly to the new use. The agent loop is where the
project's history shows: it is built around distrust of the model, and against
Muse Spark that distrust costs turns, hides errors, and occasionally kills runs
outright. The interactive experience is the weakest part, because spawning a
process per turn gives up state that a working agent needs.

The gap between where kvit-coder is and where it needs to be for daily use is
smaller than the line count suggests. Interrupt handling, streaming, longer
shell commands, a fuller system prompt, and one binary instead of two would
account for most of it, and none of that requires touching the parts that
already work.
