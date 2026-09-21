# kvit-coder redesign: implementation plan

Companion to `redesign.md`, which says what to build and why. This says how, in
what order, and touching which files. Stage numbers match that document from
stage 1 onward; stage 0 is scaffolding the plan adds.

Every stage leaves a program that builds, passes its tests, and can be used.
Nothing here requires a long-lived branch — each stage is a commit or a handful
of them on `main`.

Line references are to the tree at commit `fe77715` and will drift; treat them
as "look here", not as coordinates.

## Status: all eleven stages are done

Every stage below was done, in order, on `main` in September 2026:

| Stage | Commit | Subject |
|---|---|---|
| 0 | `4e54253` | drive the loop from a scripted client |
| 1 | `9558f01` | fixes that stand alone: cost, timeouts, path containment |
| 2 | `ceaff81` | sessions are directories, and hold everything that outlives a turn |
| 3 | `c59a0dd` | one inbox, drained once per iteration |
| 4 | `8a90b05` | ctrl-c ends the turn instead of ending the process |
| 5 | `7d94d74` | the model can ask, and the answer is the tool's result |
| 6 | `59cb9df` | work that outlives the turn that started it |
| 7 | `3ded4e8` | permission from the command's syntax tree, not from its text |
| 8 | `fcca35e` | several independent calls in one request |
| 9 | `90b3b72` | stop paying the weak-model tax by default |
| 10 | `7211dc5` | say what the schema cannot, and stop there |

The stage descriptions below are kept as the record of what was intended and
why. Read them as history; where they disagree with the code, the code is right.

### What is still open

Three things this plan named and did not do.

**Compaction.** `redesign.md` §12 describes it; the "Deferred" section at the
end of this document says why it was left out and what stage 2 did to make it
addable without redesign. It is a standalone `kvit-coder compact -s <name>` that
makes one model call and writes a new session, and nothing in the loop changes.
Stage 9's accounting is what will tell you whether it is needed.

**Two questions about the opencode.ai endpoint,** both a few minutes of
experiment, both gating assumptions the plan was built on. Does the endpoint
accept replayed `function_call` items whose `reasoning` items are absent, which
decides whether resuming a session works at all? And does it cache by the
`x-opencode-session` header, which is the whole premise of pinning
`KVIT_RUN_ID`?

**Appendix A of `redesign.md`,** which holds the ideas from maki that were not
taken: an `index` tool, code execution as a tool, a read-only research subagent,
a writable memory, and richer display.

Two proposals written after this plan are in [`redesign-mcp.md`](redesign-mcp.md)
and [`bench-refactor.md`](archive/bench-refactor.md); neither is built.

---

## Stage 0 — make the loop testable

**Goal.** Be able to drive `Runner.Run` against a scripted model and assert on
the resulting message history, before changing what the loop does.

**Changes.**

- `internal/agent/runner.go:22` — `llmClient` is a concrete `*llm.Client`.
  Extract the two methods the runner uses into an interface declared in
  `internal/agent`:

  ```go
  type LLMClient interface {
      Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)
      GetGenerationStats(context.Context, string) (*llm.GenerationStats, error)
  }
  ```

  Change the field and `RunnerOptions.LLMClient` to that type. `*llm.Client`
  satisfies it as-is, so no caller changes.
- New `internal/agent/fake_client_test.go` — a client that returns a scripted
  sequence of `*llm.ChatResponse` and records the requests it was given.
- New `internal/agent/runner_test.go` — table-driven cases over
  `(script, config) → expected messages`.

**Tests to write, in this order.** They are the regression net for every later
stage:

1. One tool call, then a final answer: history is user, assistant+tool_calls,
   tool result, assistant.
2. Iteration cap reached: loop stops, and (after stage 1) the result says so.
3. Duplicate call detection at the current threshold.
4. Backtrack: on a semantic tool error, history is truncated to the rollback
   point.
5. Loop-detection reminder is appended to the last tool result, not added as a
   new message.
6. Provider error retry path (`Choices[0].Error != nil`).

`ui.Writer` already has `SetStdout`/`SetStderr` (`internal/ui/writer.go:119`),
so tests capture output without touching the process streams.

**Done when.** `go test ./internal/agent/...` covers the six cases and fails if
you change the loop's message-building behaviour.

**Watch for.** `NewRunner` builds an `Interrogator` that makes its own model
calls. Keep `diagnostics.interrogate_on_anomaly` off in test configs, or the
fake client's script drifts out of step.

---

## Stage 1 — fixes that stand alone

**Goal.** Remove the defects that cost you something today. None of this depends
on the redesign; all of it is worth doing regardless.

**Changes.**

| File | Change |
|---|---|
| `internal/agent/interrogate_test.go:43` | Assertions expect "make progress" and "diagnostics only"; the prompt was reworded. Update the test to the current wording. |
| `cmd/kvit-coder-ui/main.go:121` | When `-s` is absent, generate a session name before `tui.New` so `runAgent` always passes `-s`. Without this the default UI has no continuity at all. |
| `cmd/kvit-coder-ui/main.go` | `os.Setenv("KVIT_RUN_ID", <derived from session name>)` before the first spawn. Derive it from the name rather than randomly, so reopening the same session lands on the same backend and its warm cache. |
| `internal/agent/runner.go:256` | Delete the second `GetGenerationStats` call; take the cost figure from the one in `processLLMResponse`. |
| `internal/agent/runner_llm.go:221` | Gate the remaining call behind a new `llm.generation_stats` config flag, default false. It is an OpenRouter endpoint and returns 404 elsewhere. |
| `internal/llm/client.go:70` | Give the `http.Client` an explicit `Timeout`. Today a hung endpoint hangs the agent with no way out. |
| `internal/llm/client.go:74` | Make `DisableKeepAlives` conditional — on for `http://` base URLs (the llama.cpp workaround it was written for), off otherwise, so a hosted endpoint stops paying a TLS handshake per request. |
| `internal/tools/tempfile_manager.go:28` | Delete the unconditional `DEBUG:` line printed to stderr on every startup. |
| `internal/config/permissions.go:49` and the denied/allowed loops above it | Containment is `strings.HasPrefix` with no separator, so `/home/sk/kvit-coder-notes` counts as inside `/home/sk/kvit-coder`. Call `tools.NormalizeAndValidatePath`, or copy its `filepath.Rel` approach. |
| `internal/agent/runner.go` | Add `BudgetExhausted bool` to `RunResult`, set it when the loop ends on the iteration cap, and report it. Today it ends silently with no answer. |
| `internal/tools/setup.go:133`, `internal/tools/shell.go:295` | Raise the default shell timeout from 30s and the ceiling from 3 minutes, and make both config (`tools.shell.default_timeout`, `tools.shell.max_timeout`). Suggested 120s and 10m. |
| `internal/agent/permissions.go` | Delete. `PromptForPermission` and `UpdateConfigFile` are unreferenced, and the latter prints "✓ Added … to allowed_paths" while writing nothing. |
| `internal/mcpclient/` | Empty directory. Remove. |

**Done when.** `go test ./...` passes with no failures, a run against
opencode.ai makes exactly one HTTP request per iteration, and hitting the
iteration cap prints something.

**Watch for.** The `KVIT_RUN_ID` change only helps if the endpoint actually
caches by that header. Measure a second turn's prompt-token cost before and
after to confirm.

---

## Stage 2 — the session directory

**Goal.** Move every piece of state with a life longer than a turn out of
process memory and into the session folder. This is what makes per-turn
processes lose nothing, and every later stage assumes it.

**Target layout** (`redesign.md` §13):

```
~/.kvit-coder/sessions/<name>/
    history.jsonl     append-only, one timestamped event per line
    meta.json         created, updated, workspace, model, first prompt,
                      summary, parent, succeeded_by
    checkpoints/      shadow git repo
    proc/             pidfiles and logs (stage 6)
    inbox/            steering drops (stage 3)
    tmp/              spilled tool output
```

**Changes.**

- `internal/session/manager.go` — rewrite around a folder. Keep
  `ListSessions`, `SessionExists`, `DeleteSession`, `AcquireLock`,
  `GenerateSessionName`. Replace `LoadSession`/`SaveSession`
  (`internal/session/manager.go:53,89`) with:

  ```go
  func Open(name string) (*Session, error)   // creates or opens, resolves chain
  func (s *Session) Load() ([]llm.Message, error)
  func (s *Session) Append(ev Event) error   // one line, fsync-free, append mode
  func (s *Session) Dir() string
  func (s *Session) Meta() *Meta
  ```

- Event shape. One JSON object per line so the file stays greppable:

  ```json
  {"ts":"2026-09-07T14:03:11Z","kind":"message","message":{...}}
  {"ts":"...","kind":"tool_result","tool":"Edit","ms":42,"message":{...}}
  {"ts":"...","kind":"notice","text":"iteration budget reached"}
  {"ts":"...","kind":"settings","merge_thinking":false,"model":"..."}
  ```

  `Load()` filters to `kind` values that carry a `message` and returns them in
  order, so the API history is derived rather than stored.

- **Migration.** On `Open`, if `<name>.jsonl` exists as a file, create the
  folder, wrap each line as `{"kind":"message"}` with the file's mtime, write it
  as `history.jsonl`, and rename the old file to `<name>.jsonl.migrated`.

- `internal/checkpoint/manager.go:79` — `checkpointDir` is
  `os.TempDir()/go-coder-checkpoints-<UnixNano>`. Change `NewManager` to take a
  directory and pass `<session>/checkpoints`. Drop the `Cleanup()` call from the
  defer chain in `cmd/kvit-coder/main.go`; the repo now outlives the process.

- `internal/tools/tempfile_manager.go:27` — take a directory instead of deriving
  one from the workspace root; pass `<session>/tmp`. Stop calling `CleanupAll`
  on exit, so a temp path the model was told about
  (`internal/tools/output_buffer.go:134`) still exists next turn. Clean on
  session delete instead.

- `cmd/kvit-coder/main.go` — reorder startup: resolve the session first, then
  build the checkpoint manager, temp manager and everything else from its
  directory. Benchmark and thinkbench modes have no session; give them an
  ephemeral one under their run directory so the same code path works.

- `internal/repl/repl.go` — append events as they happen instead of one
  `SaveSession` at the end.

**Tests.** Round-trip a session through `Append`/`Load`; migrate a flat file and
check message order and count; confirm a checkpoint made in one process is
visible to the next.

**Done when.** Two consecutive `kvit-coder -p … -s foo` runs share checkpoints,
temp files and permission grants, and `~/.kvit-coder/sessions/foo/history.jsonl`
reads as a transcript with timestamps.

**Watch for.** `AcquireLock` currently locks a session by name; keep it, and put
the lock file inside the folder. The benchmark harness constructs sessions
implicitly — check `internal/benchmark/executor.go` before changing signatures.

---

## Stage 3 — the inbox and steering

**Goal.** A line you type, or a file dropped in `inbox/`, reaches the model at
the next iteration.

**Changes.**

- New package `internal/inbox`:

  ```go
  type Kind string   // "user_line" | "process_event" | "notice"
  type Message struct { Kind Kind; Text string; At time.Time }

  func New(dir string) *Inbox        // dir = <session>/inbox
  func (i *Inbox) Push(Message)      // bounded 100, drops oldest
  func (i *Inbox) Drain() []Message  // memory queue + directory scan
  func (i *Inbox) Signal() <-chan struct{}   // for blocking tools (stage 6)
  ```

  `Drain` reads the directory as part of the same call — the drain is the poll,
  so no fsnotify. Read, then unlink. Ignore files above a size cap and log them.

- Producers write with temp-name-then-`rename()` inside `inbox/`, so a partial
  write is never read. Names carry a monotonic prefix (`<unixnano>-<rand>`).

- `cmd/kvit-coder/main.go` — start a stdin reader goroutine when
  `os.Stdin.Stat()` reports `ModeCharDevice`, pushing each line as `user_line`
  and echoing `→ queued`. Piped runs and the benchmark harness start no reader.

- `internal/agent/runner.go:196` — drain at the top of each iteration, where
  file-first mode currently reads messages. Represent a `user_line` as a user
  message wrapped in `<user-steering>`; represent a `process_event` by appending
  a `<system-reminder>` to the last tool result, the mechanism
  `checkAndHandleLoops` already uses (`internal/agent/runner_iteration.go:100`).

- `steer` subcommand. `cmd/kvit-coder/main.go` uses the `flag` package with no
  subcommands, so intercept before `flag.Parse`:

  ```go
  if len(os.Args) > 1 && os.Args[1] == "steer" { runSteer(os.Args[2:]); return }
  ```

  With no `-s`, find the single session whose `proc/turn.pid` names a live
  process; error when there are none or several. Write the file, report
  delivered.

- Turn presence: write `<session>/proc/turn.pid` at turn start, remove it at
  exit. Stage 6 reuses the same directory.

- `internal/tui/ui.go:281` — set `cmd.Stdin = os.Stdin` instead of `nil`.

**Tests.** Push into the inbox between scripted model responses and assert the
message lands in the next request; drop a file into a temp `inbox/` and assert
`Drain` returns it and unlinks it; assert an oversized file is skipped.

**Done when.** Typing during a multi-iteration turn changes what the model does
next, and `kvit-coder steer "…"` does the same from another terminal.

**Watch for.** Terminal echo interleaves with agent output; accept it for now.
Do not route stdin lines through the filesystem — they go straight into the
memory queue.

---

## Stage 4 — abort semantics

**Goal.** Ctrl-C ends the turn cleanly, writes down what stopped, and exits.

**Changes.**

- `internal/workspace/lock.go:55` — the signal handler calls `os.Exit(130)`,
  which is why an interrupt currently discards the conversation. Remove the
  handler; keep lock cleanup on the normal exit path.
- `cmd/kvit-coder/main.go` — install `signal.Notify`, cancel the run context on
  the first signal, and hard-exit on a second within two seconds.
- `internal/agent/runner.go` — on cancellation, make sure the abort record is
  complete before returning. Most of it exists: `handlePostIteration` appends
  `Error: Cancelled by user` for unexecuted calls, `handleLLMError` appends
  `[Operation cancelled by user]`, and both `executeTools` and
  `executeToolWithTimeout` check `ctx.Done()`. What is missing is the partial
  output of the tool that was running.
- Kill turn-scoped children before exit (the registry arrives in stage 6; until
  then, the shell tool's own process group).

**Tests.** Cancel the context mid-script and assert the history ends with the
abort records and the session file has them.

**Done when.** Ctrl-C during a long turn leaves a session you can continue, with
the interruption visible in `history.jsonl`.

---

## Stage 5 — questions

**Goal.** The model can ask, and the answer arrives as the tool's result.

**Changes.**

- New `internal/tools/question.go`. Schema per `redesign.md` §6: an array of
  `{question, header, options:[{label,description}], multi}`.
- `ToolContext` (`internal/tools/context.go`) gains an inbox reference so the
  tool can read `user_line` messages. It implements `SelfTimeoutTool`
  (`internal/tools/tool.go:46`) so the blanket 15-second timeout does not apply.
- Answer matching: record the display time and accept only a `user_line` that
  arrived after it. Earlier queued lines are appended as steering first.
- Rendering: question and numbered options to stderr, input hint
  `[1-2, or type]`. A bare number selects, `1,3` multi-selects, anything else is
  free text.
- Ordering: `Question` runs last within a tool batch. In
  `internal/agent/runner_tools.go:36`, partition the calls so any `Question`
  sorts to the end.
- No terminal: wait up to `tools.question.timeout` (default 0) for an answer in
  the inbox, then return "no one answered; use your judgement and state what you
  assumed."
- Narrow duplicate check: a `Question` repeated verbatim after a dismissal is
  refused, even under the `strong` profile (stage 9).

**Tests.** Answer arrives before the question was asked → treated as steering,
question still waits. Timeout zero with no tty → immediate fallback. Dismissal
records and does not re-ask.

**Done when.** A question in an interactive run blocks and accepts a numbered
answer; the same run headless with `timeout: 0` proceeds without hanging.

---

## Stage 6 — background processes

**Goal.** Long-running work spans turns, and the model can watch it without a
model call per tick.

**Changes.**

- New package `internal/procs`:

  ```go
  func (r *Registry) Start(cmd, cwd, name string) (id string, err error)
  func (r *Registry) Output(id string, cursor int64) (string, int64, bool)
  func (r *Registry) Status(id string) Status
  func (r *Registry) List() []Info
  func (r *Registry) Kill(id string) error
  func (r *Registry) Reconcile()   // at turn start: dead pid → exit event
  ```

  Detach with `SysProcAttr{Setsid: true}` — `internal/tools/shell.go:412`
  already sets `Setpgid` for the turn-scoped case. State in `<session>/proc/`:
  `<id>.pid`, `<id>.log`, `<id>.json` (command, cwd, started).

- Tool split. `Shell` keeps today's run-to-completion behaviour and dies with
  the turn; new `Shell.start`, `Shell.output`, `Shell.status`, `Shell.list`,
  `Shell.kill` cover the detached case.

- `Observe.wait {id, poll, report, until, max_wait}` — blocks, selecting on its
  own timer, process exit, a compiled `until` pattern, and `inbox.Signal()`.
  Returns what it has plus why it returned, including how long it waited and how
  many ticks produced nothing. `report` is `changed|match|exit|always`,
  defaulting to something other than `always`.

- `Observe.add {command, every, report}` — a periodic probe recorded in the same
  registry.

- Terminal tee: in `internal/tools/shell.go:311`, `cmd.Stdout` currently goes
  only to the buffer. Wrap it in an `io.MultiWriter` with a terminal writer,
  gated by the verbosity setting, so a long command is visible while it runs.

- Registry poll in `handlePostIteration`, appending exit and match events as
  `<system-reminder>` on the last tool result.

- Kill session-scoped processes on Ctrl-C (stage 4's handler) and record it.

**Tests.** Start a detached `sleep`, exit the process, start a new registry over
the same directory, and assert `Reconcile` finds it running; kill it and assert
the exit event. `Observe.wait` returns early when the inbox signals.

**Done when.** A dev server started in one turn is still serving in the next,
`Observe.wait` reports a failing test run without polling the model every
minute, and Ctrl-C leaves nothing running.

**Watch for.** Orphans. Every path that creates a pidfile must have a matching
removal, and `Reconcile` is the backstop.

---

## Stage 7 — permissions

**Goal.** Replace the regex blocklist with scopes derived from the command's
syntax tree, so `awk` in a pipeline and `curl` to a local port stop being walls.

**Changes.**

- Add `mvdan.cc/sh/v3` — a bash parser in pure Go, so the build stays cgo-free.
- New package `internal/permissions`:
  - parse, then split at chains, pipes, subshells and command substitutions
  - one scope per simple command, generalized to `<program> *`, except when the
    first token is a shell keyword (`if`, `for`, `while`, `case`, …), where
    `<cmd> *` is unsound and the segment stays literal
  - extract path arguments from argument nodes, replacing the heuristic in
    `internal/tools/shell.go:613`
  - three rule tiers evaluated denies-first: builtin, config, session
  - four grant answers: once, session, always-local, always-global
- `internal/tools/shell.go:433` — `validateCommand` calls the new package
  instead of walking `cmdBlockedAlways`, `rmRootRe` and the `sed -i` / `awk `
  substring checks.
- Session grants persist to `<session>/permissions.json`; always-grants append
  to the config file, which is what `UpdateConfigFile` claimed to do.

**Tests.** `internal/tools/shell_test.go` is the corpus — every command it
asserts on should reach the same verdict through the new path. Add the cases the
old one could not express: `git diff && rm -rf /` denies on the second segment,
`ps aux | awk '{print $2}'` is allowed, `for f in *; do rm $f; done` is not
wildcarded.

**Done when.** The old blocklist is deleted, the shell tests pass unchanged in
intent, and a `curl` to localhost can be granted for the session.

---

## Stage 8 — `Batch`

**Goal.** Several independent tool calls in one request.

**Changes.**

- New `internal/tools/batch.go`: `Batch {calls: [{tool, args}, …]}` returning a
  labelled result array. No nesting, cap around ten, one failure does not abort
  the rest.
- New optional interface in `internal/tools/tool.go`:

  ```go
  type ParallelSafeTool interface{ ParallelSafe() bool }
  ```

  Read, Search and the index tool return true; Edit, Write and Shell do not,
  because they mutate the workspace and share pending-edit state. A mixed batch
  runs the safe calls concurrently and the rest in order.

**Done when.** A batch of four reads takes about as long as one.

---

## Stage 9 — profiles and accounting

**Goal.** Stop paying the weak-model tax, and find out whether compaction will
ever matter.

**Changes.**

- `agent.profile: strong | weak`, default `strong`. Under `strong`, skip
  backtracking (`internal/agent/backtrack.go`), the duplicate-call kill switch
  (`internal/agent/runner.go:156`), the Edit and Write confirm handshakes, the
  text tool-call scraper (`internal/tools/registry.go:268`), fuzzy matching and
  autoindent correction, anomaly interrogation, and the empty-response retries.
- Token accounting: after each response, compare prompt tokens against
  `llm.context` and show it in the status line. `llm.context` is currently used
  only for a display string.
- Run thinkbench before and after the profile split. A profile that scores worse
  was doing something.

**Done when.** `profile: weak` reproduces today's behaviour, `strong` skips it,
and the status line shows context used against the window.

---

## Stage 10 — the prompt

**Goal.** Say what the schema cannot, and stop instructing a capable model to
behave like a weak one.

**Changes.**

- Cut from `internal/prompt/prompt.go:201`: the numbered `# WORKFLOW`, the
  generated `# EXAMPLE` and its `**Key Rules:**`, and the two non-rules in
  `# GUIDELINES`.
- Trim each `PromptSection()` to failure modes and invariants only — Read's
  truncation limits, Edit's byte-exact match and multiple-match error, the
  shell's statelessness and temp-file spill, Search's result degradation.
- Add an environment block generated per session: OS, shell, working directory,
  date, git branch and short status, top-level listing.
- Document the new mechanisms: `<user-steering>`, background processes and
  `Observe.wait`, process-event reminders, and when to use `Question`.
- Use the existing template system (`prompts.use_templates`, embedded under
  `internal/prompt/prompts/`) with the profile selecting the variant, so `weak`
  keeps the verbose prompts and stays benchmarkable.

**Done when.** Thinkbench scores are unchanged or better with the short prompts,
measured rather than assumed.

---

## Deferred

Compaction (`redesign.md` §12) is not in this plan. Stage 2 makes it addable
without redesign — append-only history, unwritten `parent` and `succeeded_by`
fields, and name resolution through `meta.json` — and stage 9's accounting is
what will tell you whether it is needed. When it is, it is a standalone
`kvit-coder compact -s <name>` that makes one model call and writes a new
session; nothing in the loop changes.

Appendix A of `redesign.md` holds the ideas from maki that are not in this plan:
the `index` tool, code execution as a tool, the read-only research subagent, a
writable memory, and richer display.

---

## Cross-cutting notes

**New packages.** `internal/inbox`, `internal/procs`, `internal/permissions`.

**New dependency.** `mvdan.cc/sh/v3` at stage 7, and nothing else.

**Config keys added**, in the order the stages introduce them:

```yaml
llm:
  generation_stats: false        # stage 1
tools:
  shell:
    default_timeout: 120         # stage 1
    max_timeout: 600             # stage 1
  question:
    timeout: 0                   # stage 5
agent:
  profile: strong                # stage 9
```

**Order dependencies.** Stage 2 before 3 (the inbox lives in the session
directory) and before 6 (`proc/` does too). Stage 3 before 5 and 6 (the question
tool and `Observe.wait` both read the inbox). Stage 4 before 6 (the interrupt
handler kills the registry). Stages 7 and 8 are independent and can move
earlier — 7 in particular, if the current blocklist starts costing you time.

**Two questions to answer with code before relying on them.** Does the
opencode.ai endpoint accept replayed `function_call` items whose `reasoning`
items are absent — which decides whether session resume works at all today? And
does it cache by the `x-opencode-session` header, which is the whole premise of
pinning `KVIT_RUN_ID`? Both are a few minutes of experiment and both gate
assumptions the plan is built on.
