# kvit-coder: redesign for driving a strong hosted model

Companion to `review.md`, which describes the program as it stands at commit
`fe77715` and lists its defects. This document describes what to build instead.
It assumes the reader has not read the review, so the parts of the current
design that matter here are restated where they come up.

## 1. What the agent is for now

kvit-coder was written to drive models running on local hardware, where the
engineering problem was compensating for models that mis-format tool calls,
repeat themselves, and miscount line numbers. The target now is Muse Spark 1.3
reached through the opencode.ai endpoint: a capable hosted reasoning model with
a 1M-token context, spoken to over OpenAI's Responses API. That model needs
almost none of the compensation, and it needs several things the current program
has never had.

The design below is shaped by four requirements, in order of how much they
constrain the architecture:

1. **Steering.** You must be able to type a message while a step is executing
   and have the model see it, without killing the turn.
2. **Long-running and observed commands.** Test suites, builds, dev servers and
   log watchers must be startable, watchable, and able to report back. The
   current 3-minute ceiling and the run-to-completion-then-report model do not
   cover this.
3. **A thin interactive shell.** The interactive process must not hold agent
   state, manage turn cancellation, or reimplement the loop. That constraint is
   deliberate and is preserved.
4. **No weak-model tax.** The recovery machinery built for local models should
   not run when it is not needed.

Nothing here requires changing the tool interface, the registry, prompt
generation, the MCP client, the transport layer, or the benchmark harness.
Section 15 lists what stays untouched.

## 2. The organizing idea: one input channel

The current loop has exactly one way for anything to enter the conversation: the
model asks for a tool, the tool returns, the result is appended. The user speaks
only by starting a new process. Nothing else can reach the model.

The redesign gives the agent a single **inbox** that three producers write to:

- lines you type, at any moment
- events from background processes and observers
- system notices the runner already generates (loop detection, plan state)

Every write carries a `wake` flag. A message written with `wake=false` sits in
the queue and is picked up by whatever turn happens next, costing nothing extra.
A message written with `wake=true` also rouses an agent that is sitting idle.
The queue is bounded and drops its oldest entry when full, so no producer ever
blocks on a slow or idle agent.

Every message also carries its **kind** — `user_line`, `process_event`,
`notice` — because one consumer has to tell them apart. A pending question
(section 6) takes its answer from something you typed and must never take it
from a build that happened to finish while you were deciding.

The loop drains the inbox at the top of every iteration and turns whatever it
finds into messages. When the model answers without asking for a tool, the agent
does not exit — it blocks on the inbox and returns when something with `wake`
set arrives.

That flag is the whole cost-control policy. A dev server logging a request
writes `wake=false` and the model learns about it, free, the next time it takes
a turn for some other reason. A test run going red writes `wake=true` and gets
a turn of its own. Deciding which is which happens at the write site, where the
producer knows what the event means, rather than in a rule the loop has to
consult. The design is taken from maki's `SessionMailbox`
(`github.com/tontinton/maki`, `maki-agent/src/mailbox.rs`), which splits the
same queue into `drain()` and `claim_wake()`.

That single change collapses three features into one mechanism. Steering is an
inbox write while the loop is running. Prompting for the next turn is an inbox
read while the loop is idle. Waking the agent because a build finished is an
inbox write from a process watcher. There is no separate REPL, no separate
notification path, and no event loop beyond "read the inbox."

## 3. Process model

Two binaries stay, with their responsibilities sharpened.

**`kvit-coder` — the agent.** Owns everything with a lifetime: the conversation,
the checkpoint repository, MCP server connections, temp files, background
processes, and the terminal for as long as it runs. It reads steering lines from
stdin, writes output to stdout and stderr, and saves the session after every
answer. It runs for a whole working session rather than a single turn.

**`kvit-coder-ui` — the launcher.** Picks the config, resolves or creates the
session name, pins `KVIT_RUN_ID` in the environment so every child shares one
prompt-cache identity, spawns the agent with stdin, stdout and stderr inherited,
and waits. It handles session listing, switching and deletion between agent
runs, and restarts the agent if it exits and you want to continue.

The launcher never sees a turn, never holds message history, and has no
cancellation logic. That is the property worth protecting, and moving from
"spawn per turn" to "spawn per session" preserves it while removing everything
that per-turn spawning throws away: checkpoints that only reach inside one turn,
path approvals re-asked every turn, MCP servers respawned every turn at up to 20
seconds each, temp files deleted while the model still holds their paths, and
background processes that cannot exist at all.

Headless use is the same binary with the interactive behaviour off:
`kvit-coder -p "prompt"` runs one conversation, kills any background processes it
started, and exits. Benchmarks are unaffected.

### Failure and exit

- First Ctrl-C aborts the current step and returns the agent to the inbox. What
  that means precisely is set out in section 5.
- Second Ctrl-C within a few seconds exits the agent.
- If the agent exits unexpectedly, the launcher reports the exit code and offers
  to restart with the same session, which reloads the saved history.

## 4. The turn loop

```
run(session):
    for {
        msgs := inbox.drain()            // steering, process events, notices
        if msgs.empty && idle {
            msgs = inbox.wait()          // block; this is the prompt
        }
        history.append(msgs)

        budget := iteration_budget       // per user message, not per process

        for i := 0; i < budget; i++ {
            history.append(inbox.drain())    // steering lands here mid-turn

            resp := model.call(history)
            if resp.no_tool_calls {
                print(resp.text)
                session.save(history)
                idle = true
                break                    // back to the outer wait
            }

            for tc := range resp.tool_calls {
                history.append(execute(tc))
            }
            history.append(watchers.poll())  // process events as reminders
        }
    }
```

Three points about this shape:

**The drain sits at the top of the iteration**, immediately before the model
call, at the place where `internal/agent/runner.go:196` currently reads messages
for file-first mode. Every tool call from the previous iteration already has its
result appended by then, so the history is well-formed for both wire protocols
and a user message can be appended safely.

**The iteration budget is per user message, not per process.** The current
`max_tool_iterations` counts for the life of the process; once the process is
the whole session, that becomes meaningless. Reset the counter each time a user
message enters the history. A value around 200 is a sane ceiling for one
instruction.

**Reaching the budget is reported, not silent.** The current loop ends without
printing anything when the cap is hit. The agent should say so, save, and go
idle with the history intact so you can steer it onward.

## 5. Steering

Mechanically this is small, because the agent already owns the terminal.

- The launcher passes `os.Stdin` to the child instead of `nil`
  (`internal/tui/ui.go:281` today sets it to nil).
- The agent starts a goroutine running a line scanner over stdin, appending each
  line to the inbox and echoing `→ queued` so you know it was received.
- The reader only starts when stdin is a character device, so piped `-p` runs
  and the benchmark harness are unaffected.
- Path-safety prompts are unaffected: they open `/dev/tty` directly
  (`internal/config/config.go:795`) rather than reading stdin.

### Two channels, not one

Typing a line queues it. Ctrl-C aborts. They are separate mechanisms with
separate semantics, and both are cheap because neither tries to resume anything.

**A queued line** is picked up at the next iteration boundary, which is at most
one model call and one tool batch away. Nothing is cancelled and the step runs
to its natural end. This is the common case: a correction, an extra constraint,
a "also update the tests."

**Ctrl-C aborts the step.** It does not attempt to salvage the half-finished
turn, and that is what makes it simple:

1. Cancel the iteration context, which stops the in-flight model call and any
   running tool.
2. Kill every background process group.
3. Write down what was aborted, as ordinary history the model reads next turn:
   an error result for the tool that was running, including whatever output it
   had produced; `Error: Cancelled by user` for tool calls that were issued but
   never reached; a note naming the background processes that were killed; and,
   when the model call itself was cut off, a marker that the assistant turn was
   abandoned.
4. Save the session and go idle at the inbox.

The next thing you type continues a conversation whose history explains exactly
what stopped and where, so the model does not re-plan from a state it thinks
still holds.

Most of step 3 already exists. `handlePostIteration` appends
`Error: Cancelled by user` for unexecuted tool calls, `handleLLMError` appends
`[Operation cancelled by user]` when a model call is cancelled mid-tool-loop,
and both `executeTools` and `executeToolWithTimeout` already check
`ctx.Done()` at the right points. What is missing is a SIGINT handler that
cancels the iteration context rather than calling `os.Exit(130)`
(`internal/workspace/lock.go:55`), the process kill in step 2, and going idle
instead of exiting.

Two rough edges to accept initially. The terminal echoes what you type
interleaved with agent output, which is untidy until the agent draws its own
input line in raw mode. And a steering message arrives as a plain user message,
so the model may treat it as a new instruction rather than a correction; wrapping
it in a marker such as `<user-steering>` and saying in the system prompt what
that means is worth doing from the start.

## 6. Questions

The model needs to ask sometimes: which of two approaches to take, whether a
schema change is acceptable, which of three plausible files you meant. A
question is a tool call whose result comes from a person rather than from the
machine, so it needs no new input path. It reads from the inbox, like everything
else.

### One reader, three display states

The line reader from section 5 is always running. What changes is what is drawn
above it and how the agent reads the next line:

| Agent state | Prompt shows | The next line is |
|---|---|---|
| idle | bare prompt | a new instruction |
| step running, no question | bare prompt | steering |
| question pending | the question and its numbered options | the answer |

One input surface in three modes, rather than two interfaces contending for
stdin. The hint on the input line (`[1-2, or type]`) says which mode you are in.

### Tool shape

```
Question {questions: [{question, header, options: [{label, description}], multi}]}
    → [{question, answers: [label, ...] | free_text | dismissed}]
```

Batching several questions into one call matters more here than in an agent
paying per token: each separate `Question` call costs a context replay and a
reasoning pass. Say so in the tool description rather than merely permitting it.

Structured options are worth requiring for a reason beyond convenience. Making
the model enumerate the choices forces it to work out what it is actually
asking, and a question with no options is usually one that has not been thought
through. Free text stays available regardless, because the useful answer is
often none of the offered ones.

### Which line counts as the answer

Record the time the question was displayed and accept only a `user_line` that
arrives after it. Lines already queued were typed before the model asked, so
they cannot be answering it: append those to history as ordinary steering
first, then wait. Without this rule a stale steering line is silently consumed
as the answer to a question you never read.

### Running order within a turn

When a turn contains `Question` alongside other tool calls, run `Question`
last. Then `Shell.start` plus `Question` behaves as it reads — the build starts,
and you are asked while it runs. Question-first would block before the process
ever started, which is the only arrangement in which asking would hold up
background work.

### What keeps running while a question waits

Blocking the step costs nothing, because background processes are not in the
loop. They are OS processes with their own process groups: a build started in an
earlier turn keeps building while you decide, its output keeps teeing to your
terminal, and its exit event lands in the inbox on its own schedule. The only
thing that stops is the model's turn, which was stopping regardless — it asked
because it could not proceed.

Process events should be **displayed while the question is on screen**, since
they can change your answer. The message-kind rule from section 2 is what makes
that safe:

```
[bg1 exited 1: 3 tests failed]

Use the existing retry wrapper, or write a new one?
  1) Reuse internal/http.Retry  — same backoff, already tested
  2) New wrapper in this package — no shared state
  [1-2, or type]
```

### Blocking, dismissal, and the headless fallback

Interactive: block indefinitely. A long-lived agent that waits costs nothing,
and a timeout firing while you are away is worse than waiting.

No controlling terminal: return at once with "no one available to answer; use
your judgement and state what you assumed." This is the fallback the MCP confirm
policy already uses, where `ask_*` degrades to `block` rather than hanging. Skip
it and every thinkbench task that asks a question hangs until its budget
expires.

Ctrl-C is dismissal and follows section 5's abort semantics: record "question
dismissed by user" as the tool result, stop the step, go idle. The question and
its dismissal are both in history, which is what stops the model asking again.

Typing free text rather than an option number is the graceful escape. If the
build failed and the choice has gone stale, a sentence returns as the answer and
the model reads it as redirection. Answering and steering collapse into the same
keystrokes, so you never have to decide which you are doing before you type.

### The non-blocking variant, and why not

A question the model posts before carrying on with other work would have to
receive its answer later as a loose user message rather than as the tool's
result, which breaks the call-and-result pairing the Responses API expects, and
the model would have to handle an answer to something it has already moved past.
The blocking version is simpler and gives up nothing identifiable.

### Prompt guidance

The mechanism is small; the failure mode is not mechanical. A question costs
about what any tool call costs, which is cheap set against a wrong assumption
that causes twenty turns of rework — so the tool pays for itself when the
questions are good and turns a five-minute task into twenty minutes of prompting
when they are not. That is prompt work: ask when the answer changes what you
build and the code cannot tell you; do not ask what reading would answer; do not
ask permission to do the thing you were asked to do; put related questions in
one call.

### How much this adds

Once the agent goes idle after an answer instead of exiting (section 3), most of
this is already available: the model can end its turn with a question, you
reply, and the history is all still there. What the tool adds is narrower than
it first appears — the turn keeps its momentum instead of ending in a wrap-up
the model then resumes from, the options come back machine-readable instead of
parsed out of your prose, and the answer is bound to its question in history
rather than floating as a loose user message. That is worth a tool, but it is a
smaller gap than it would be in an agent that exits after every answer, so
shipping the long-lived loop first and seeing whether asking-by-stopping annoys
you is a defensible order.

## 7. Background processes and observers

### Tool surface

```
Shell.start   {command, cwd, name}                → {id}
Shell.output  {id, cursor}                        → {output, cursor, running}
Shell.status  {id}                                → running | exited(code)
Shell.list    {}                                  → [{id, name, running, age}]
Shell.wait    {id, until_regex, timeout}          → blocks; output + why it returned
Shell.kill    {id}
Observe.add   {command, every, report_when}       → {id}   periodic probe
Observe.remove{id}
```

`Shell.start` returns immediately, so the 3-minute ceiling on synchronous
commands stops being the binding constraint. `Shell.wait` opts out of the
hardcoded 15-second per-tool timeout through the existing `SelfTimeoutTool`
interface (`internal/tools/tool.go:46`), which MCP tools already use for the
same reason.

An `Observe` entry is a background process that produces output on a timer — a
health probe, a `git status`, a queue depth. It shares the registry and the
reporting policy with `Shell.start`, so there is one mechanism with two
producers.

### Output handling

Each process gets a bounded ring buffer in memory plus a full log file under the
session's temp directory. Two consumers read from it independently:

- **The terminal**, live, so you can watch a build scroll past. This costs
  nothing and is where most of the value of "observing" actually lands, because
  watching progress is a human need.
- **The model**, only when it asks (`Shell.output`, `Shell.wait`) or when a
  registered condition fires.

Keeping those separate matters because every model wake-up replays the entire
conversation — the Responses requests set `Store: false`
(`internal/llm/responses.go:127`), so nothing is held server-side — and at high
reasoning effort a "still compiling" poll is an expensive way to learn nothing.

### When the model is told

After each tool batch, the runner polls the watcher registry and appends notes
to the last tool result as `<system-reminder>` text, reusing the mechanism loop
detection already uses (`internal/agent/runner_iteration.go:100`). Three
conditions produce a note:

- a process exited, with its code and last output
- a regex the model registered matched new output
- an observer's output changed since the last report

Nothing is reported on a timer, and unwatched output never reaches the model.
Cost stays proportional to events rather than to elapsed time.

**Waking an idle agent** needs no separate mechanism, only the `wake` flag from
section 2. A process exiting non-zero, or a pattern the model asked to be told
about, writes `wake=true` and gets a turn. Everything else writes `wake=false`
and rides along with the next turn for nothing. The registration API decides the
flag: `Shell.wait` and an explicit `until` pattern imply `wake=true`, while
`Observe.add` defaults to `wake=false` unless asked otherwise.

### Lifetime

Processes belong to the agent process. They survive turns, because the agent
survives turns. They die on Ctrl-C and on exit, both through the existing
`killProcessGroup` path (`internal/tools/shell.go:417`), and both are reported
into the history so the model knows the dev server it started is gone rather
than assuming it is still answering on port 3000.

Tying process death to interrupt is deliberate: an interrupt usually means the
approach was wrong, and leaving a test watcher and a stale server running from
an abandoned approach is worse than restarting them. If a long-lived server
turns out to be worth keeping across an abort, the narrow fix is a
`keep_on_interrupt` flag on `Shell.start` rather than a change to the default.

## 8. Live output for foreground commands

Independently of background processes, the synchronous shell tool should tee to
the terminal as output arrives rather than after the command exits. Today
`cmd.Stdout` goes to a buffer that spills to a temp file
(`internal/tools/shell.go:311`) and nothing is displayed until the process ends,
so a two-minute test run shows nothing for two minutes. An `io.MultiWriter` over
the buffer and a terminal writer, gated by the existing verbosity setting, is
the whole change.

## 9. Command permissions

The current shell guard is a set of regexes over the raw command string
(`internal/tools/shell.go:19-70`). It matches binaries in command position,
which avoids the obvious false positives, but it cannot see structure. Three
consequences show up daily: `awk` is refused outright whenever the Edit tool is
enabled, so it is unusable as a pipeline filter; `curl`, `wget` and `nc` are
blocked with no configuration override, so the agent cannot fetch a URL or probe
a server it just started; and a chained command is judged as one string, so
`git diff && rm -rf /` is evaluated as whatever the first pattern happens to
match.

Replace it with permissions derived from the command's syntax tree. The approach
is maki's (`maki-agent/src/permissions.rs`); the Go equivalent is
`mvdan.cc/sh/v3/syntax`, a bash parser with no cgo, which matters for a project
that currently builds without any.

**Deriving scopes.** Parse the command, walk the tree, and split at chains,
pipes, subshells and command substitutions. Each simple command yields a scope:
the program name, generalized to `<program> *`. Refuse to generalize when the
first token is a shell keyword — `if`, `for`, `while`, `case` and the rest —
because `<cmd> *` is only sound when `<cmd>` names one program. The parse also
hands you each command's arguments, so path extraction stops being the regex
heuristic it is today (`extractPaths`, `internal/tools/shell.go:613`) and
becomes a walk over argument nodes.

**Deciding.** Every scope from one command is checked independently and any deny
wins, so a chained command cannot smuggle a second program in behind an approved
first one. Rules live in three tiers, evaluated denies-first:

- **builtin** — writes inside the workspace, reads of workspace files
- **config** — persisted rules, project-local and global
- **session** — grants made during this run, held in memory

**Granting.** Four answers to a prompt: once, for this session, always in this
project, always everywhere. The last two append a rule to the config file, which
is what `UpdateConfigFile` currently claims to do while writing nothing
(`internal/agent/permissions.go:60`). With a long-lived agent a session grant now
means something — it lasts the working session rather than one turn.

**What this buys.** The unconditional bans become default deny rules you can
override. `awk` in a pipeline is a scope you allow once and forget. `curl` is a
permission rather than a wall. `sudo` and `rm -rf /` stay denied at the builtin
tier with no way to grant them by accident. The existing path-safety modes keep
working on the paths the parse extracts.

## 10. Batched tool calls

Tool calls execute one at a time (`internal/agent/runner_tools.go:36`), which is
correct and cheap. The expense is elsewhere: when the model wants four
independent things, it either asks for them in one assistant turn — fine — or,
more often, discovers it needs the next one only after seeing the last, and each
discovery costs a full context replay plus a reasoning pass. At
`reasoning_effort: high` that round trip, not the tokens, is what you wait for.

A `Batch` tool lets the model request several calls in one:

```
Batch {calls: [{tool, args}, ...]}   → [{tool, ok, result}, ...]
```

Rules that keep it simple: no nesting, a cap of around ten calls, every result
labelled with its index and tool name, and one failure does not abort the rest —
the model gets a result array with some entries marked failed.

Concurrency is decided per tool rather than per batch. Read, Search and Index
are safe to run at once. Edit, Write and Shell serialize, because they mutate the
workspace and share the pending-edit state. A batch mixing both runs its
read-only calls concurrently and its mutating calls in order.

This does nothing for genuinely dependent chains, where step two needs step one's
output. That is what appendix A.2 addresses, and it is the reason to build this
one first: `Batch` is an afternoon and covers the common case.

## 11. Model profiles

Add `agent.profile` with values `strong` and `weak`, defaulting to `strong`.

Under `strong` the following do not run:

| Machinery | Where | Why it goes |
|---|---|---|
| Backtracking | `internal/agent/backtrack.go` | Rewinding history hides the error from a model that can read it |
| Duplicate-call kill switch | `runner.go:156`, `runner_tools.go:211` | Three identical calls should not end a run |
| Edit and Write confirm handshakes | `internal/tools/filesystem.go` | Doubles the round trips per edit; the Write one is not even configurable today |
| Text-scraped tool calls | `registry.go:268` | Five syntaxes of guesswork for a model that emits structured calls |
| Fuzzy matching, autoindent correction | `edit_fuzzy.go`, `edit_apply_line.go` | Silent repair of edits the model got right |
| Anomaly interrogation | `internal/agent/interrogate.go` | Benchmark instrumentation; an extra model call per anomaly |
| Empty-response and malformed-call retries | `runner_iteration.go` | Symptoms of models this profile does not target |

One narrow exception survives into `strong`. A model that asks a question, has
it dismissed, and immediately re-asks the identical question is the one loop the
profile would otherwise stop catching, so keep a duplicate check scoped to
`Question` alone rather than reviving the general one.

`weak` keeps all of it, so the local-model work stays usable and stays
benchmarkable. The edit surface under `strong` is one mode: search and replace
on exact text, applied immediately, with a clean no-match error when the search
text is wrong. Run thinkbench before and after the split so the decision rests
on scores rather than on argument.

## 12. Context management

With one process per session and a 1M-token window, the conversation will grow
for hours. Three pieces are needed.

**Accounting.** Track prompt tokens against `llm.context` after every response.
It is currently used only to render a display string. Warn at a threshold, and
expose the number in the status line.

**Compaction.** At a configurable fraction of the window, summarize the older
part of the conversation into a single message, keep the most recent N turns
verbatim, and replace superseded tool outputs — a file read whose file was later
edited, a directory listing from an hour ago — with a one-line placeholder.
Expose it as a command as well, so you can compact deliberately before starting
something long.

One correctness constraint governs the implementation. Compaction must operate
on whole groups of an assistant turn plus all of its tool results. Dropping a
`function_call` while keeping its `function_call_output`, or the reverse,
produces an item list the Responses API will reject. Group first, then decide
what to keep.

**Honest 400 handling.** Today any HTTP 400 is assumed to be context overflow;
the handler rewrites every trailing tool result to a placeholder and retries
(`internal/agent/runner_llm.go:110`). With real token accounting, overflow is
predicted rather than discovered, and a 400 can be surfaced as what it usually
is — a malformed request — instead of corrupting the transcript.

## 13. Session persistence

The session file is the only thing that survives an agent restart, so what it
drops matters.

- **Persist `ReasoningBlocks` and `ToolCallItemIDs`.** Both are tagged
  `json:"-"` today (`internal/llm/types.go:24,27`), so a resumed session replays
  `function_call` items whose matching `reasoning` items are gone. Test whether
  the endpoint tolerates this before deciding whether persisting them is
  required or merely better.
- **Save after every answer**, not only at the end of the process, and save on
  cancellation. Ctrl-C currently discards the whole conversation because the
  only signal handler cleans up the workspace lock and calls `os.Exit(130)`
  (`internal/workspace/lock.go:55`).
- **Record compaction as an event** so a reloaded session does not lose the
  summary or silently re-expand.

## 14. The system prompt

The generated prompt is currently a role line, a capability list, a four-step
workflow, one editing example, three guidelines, and the tool documentation
(`internal/prompt/prompt.go:201`). For a strong model this is the cheapest lever
available and it needs three additions:

- **An environment block**: operating system, shell, working directory, date,
  git branch and short status, and a top-level directory listing. Regenerate it
  per session, not per turn, so the prompt prefix stays stable for caching.
- **Working guidance**: verify changes by running something, prefer the tools
  over shell equivalents where both exist, say what was left undone.
- **The new mechanisms**: what a `<user-steering>` message means, that
  background processes exist and how to watch one, what a process-event reminder
  looks like when it arrives, and when to use `Question` — the guidance in
  section 6 belongs here rather than in the tool description, because it is
  about judgement rather than syntax.

Keep generation driven by the enabled tool set, as it is now. The prompt should
still shrink when tools are disabled.

## 15. What stays as it is

- The `Tool` interface, the registry, and prompt-section generation. Adding a
  tool touches one file and one line of `SetupRegistry`.
- Deterministic sorted tool specs, which exist so the prompt prefix stays
  byte-identical between requests and the server cache keeps hitting.
- The transport: retry with backoff, permanent-500 detection, truncated-body
  repair, and the Responses translation with encrypted-reasoning replay.
- The MCP client, which is the cheapest route to filling tool gaps without
  writing tools.
- The path-safety modes and the `/dev/tty` prompting they use. The command
  blocklist they sit alongside is replaced by section 9.
- All three benchmark harnesses, and thinkbench in particular — it is the only
  way to tell whether a change to the agent helped.

## 16. Migration

Each stage leaves a working program and is independently useful.

**Stage 1 — fixes that stand alone (hours).** Persist the session name from the
launcher so continuity works at all; pin `KVIT_RUN_ID` so the prompt cache
survives; derive the temp and checkpoint directories from the session name; drop
the duplicate `GetGenerationStats` calls and give the HTTP client a timeout;
remove the stray `DEBUG:` line at `internal/tools/tempfile_manager.go:28`; fix
the stale interrogation test. None of these depend on the redesign, and they are
worth doing whether or not it proceeds.

**Stage 2 — the inbox and steering.** Add the inbox with its bounded queue and
`wake` flag, move the drain into the loop, pass `os.Stdin` to the child, add the
reader goroutine and the tty guard. At the end of this stage the agent still
exits after its answer; you can steer a multi-step turn but nothing more. This
is the smallest change that proves the mechanism.

**Stage 3 — the long-lived agent.** Make the agent block on the inbox after an
answer instead of exiting, reset the iteration budget per user message, replace
the lock's `os.Exit(130)` handler with abort-the-step semantics, save after
every answer, and reduce the launcher to spawn-once-per-session. Checkpoints, approvals and MCP connections
start persisting as a side effect.

**Stage 4 — questions (section 6).** Add message kinds to the inbox, the
`Question` tool, the three display states, the after-the-question ordering rule,
and the no-tty fallback. Small, and it depends only on stage 2's inbox — the
part that shows process events beside a pending question can wait for stage 5.

**Stage 5 — background processes.** Registry, the `Shell.*` and `Observe.*`
tools, the terminal tee, and the watcher poll after each tool batch. Idle
wake-up falls out of the `wake` flag rather than needing its own design.

**Stage 6 — permissions (section 9).** Swap the regex blocklist for scopes
derived from the bash syntax tree, add the three rule tiers and the four grant
answers, and persist always-grants to config. Independent of everything above,
so it can move earlier if the current blocklist starts costing you time. The
existing shell tests give you a ready-made corpus: every command they assert on
should reach the same verdict through the new path.

**Stage 7 — `Batch` (section 10).** An afternoon, once the tool registry is
otherwise settled.

**Stage 8 — profiles and context.** Split `strong` and `weak`, measure with
thinkbench, then add token accounting and compaction.

**Stage 9 — the prompt.** Rewrite it once the mechanisms it must describe exist.

Before stage 2, write a table-driven test that drives `Runner.Run` against a
scripted fake model client and asserts on the resulting message history. The
loop is about to grow an inbox, a watcher poll, and an idle state, and there is
currently no test that exercises it at all.

## 17. Open questions

- **Does the endpoint accept replayed `function_call` items without their
  `reasoning` items?** This decides whether session resume works today and
  whether persisting reasoning blocks is required. Test it first; it gates
  stage 3.
- **How long is the wait for a queued line in practice?** At high reasoning
  effort a correction can sit through a long model call. If that becomes
  irritating, the fix is not resuming a half-finished turn but letting a queued
  line optionally abort the in-flight call the way Ctrl-C does, keeping the same
  write-down-what-stopped semantics.
- **Should any process survive an abort?** The default is that none do. A dev
  server you restart on every interrupt may argue for `keep_on_interrupt`.
- **What should `Observe.add` default its `wake` flag to?** Reporting every
  change without waking is the conservative default, but an observer you set up
  precisely because you want to be interrupted argues the other way. Use will
  settle it; the flag makes either answer a one-line change.
- **Does `Question` earn its place next to asking by stopping?** Once the agent
  goes idle after an answer, ending a turn with a question already works. Build
  the loop first, then judge whether the tool is adding anything beyond momentum
  and machine-readable options.
- **How much does `Batch` actually get used?** A model that plans well will
  batch; one that works step by step will not, whatever the prompt says. If it
  goes unused, that is evidence for appendix A.2 rather than against batching.
- **Does compaction hurt more than it helps at 1M tokens?** Possibly the window
  is large enough that dropping stale tool outputs is sufficient and
  summarization is unnecessary. Accounting comes first; compaction may turn out
  to be a smaller job than section 12 implies.
- **Should the launcher survive at all?** Once the agent is long-lived, the
  launcher does three things: pick a session, pin an environment variable, and
  restart on crash. That may be a shell script.

## Appendix A — deferred ideas from maki

Ideas from `github.com/tontinton/maki`, a Rust coding agent built around minimal
token consumption, that are attractive but not part of the plan above. Recorded
here so a later reader knows they were considered and where to look. Quotations
are from maki's tool descriptions and README.

### A.1 An `index` tool

Registered description: *"Return a compact overview of a source file: imports,
type definitions, function signatures, and structure with their line numbers
surrounded by []. ~70-90% more efficient than reading the full file."* The
README claims the tool *"adds 59 tok/turn but saves 224 tok/turn on read calls,
saving 165 tok/turn."* The model indexes a file, sees the structure with line
numbers, and reads only the ranges it needs.

Attractive because the benefit that survives a 1M-token window is not the token
saving but the attention: a skeleton plus two targeted reads leaves less
irrelevant material competing for the model's attention than a whole-file read.
Deferred because it needs per-language parsing to be worth much. If it is
revisited, the cheap first version is `go/ast` for Go files and `ctags -x` for
everything else, before committing to tree-sitter grammars.

### A.2 Code execution as a tool

Registered description: *"Execute Python in a sandbox where every tool is an
async function. Use for chained/dependent tool calls and filtering/processing
results, e.g. filtering web tool output."* Every tool becomes an awaitable
function returning a string, a `gather()` helper runs several at once, imports
are limited to `re`, `asyncio`, `sys`, `os` and `json`, and only what the script
prints enters the conversation.

This is the strongest idea on maki's list for a subscription user, because it
removes model round trips from the middle of a dependent chain — the thing
`Batch` cannot help with. Five dependent calls become one, and the intermediate
results the model needed only in order to pick a file never reach the context at
all. Deferred because it needs a sandboxed interpreter with bindings back into
the tool registry, which is the largest single build on this page.

There is a cheap approximation worth trying first: unblock `python3 -c`, which
`allow_interpreters: false` currently refuses, and say in the system prompt that
filtering output before returning it is preferred. That gets a fraction of the
benefit with no new code, and it measures whether the model would use the full
version.

### A.3 A read-only research subagent

maki's `task` tool launches subagents with a `subagent_type` — *"`research`
(default): Read-only tools. For codebase exploration or gathering context"* and
*"`general`: Full tool access"* — an optional `output_schema` the subagent must
satisfy through a `structured_output` tool, and a concurrency semaphore.
Subagents run in isolated sessions and the parent never sees their transcript.

The appeal is that an exploration subagent is compaction that happens before the
tokens are spent, which is simpler and safer than summarizing afterwards: twenty
reads and eight greps to answer "where is auth handled" cost the main
conversation one paragraph, and nothing has to keep `function_call` and
`function_call_output` items paired while pruning. If compaction (section 12)
turns out to be fiddly, build this instead.

maki's model tiers — weak, medium, strong, chosen per task — do not apply while
kvit-coder drives one model on one subscription.

### A.4 Showing more of what happened

maki's stated position is that agent operations should not be hidden: exact line
counts read, token usage per turn, and subagent activity in its own window. The
kvit-coder default is `verbose: 0`, a one-line summary per tool. Once background
processes exist there is more worth showing — a running-process list, bytes read,
context used against the window — and the writer already has the hooks. Small,
and worth doing opportunistically rather than as a stage.

### A.5 A writable memory

maki has a `memory` tool with `/memory` commands to view, edit and delete what
the agent has stored, described as *"A `memory` tool to keep long term context,
just tell maki to remember something."* kvit-coder reads `CLAUDE.md` but has
nothing writable. A `Memory.write` appending to a project file is perhaps fifty
lines and gives continuity that survives both compaction and a new session.

### A.6 Considered and rejected

- **Tools as Lua plugins.** Every maki tool, including `bash`, `read` and
  `edit`, is a plugin directory with a `plugin.toml` and an `init.lua`. For a
  single-user Go codebase where the `Tool` interface already makes a new tool one
  file plus one line of `SetupRegistry`, this is a large refactor with no clear
  payoff.
- **A high-frame-rate TUI.** maki runs ratatui at 60 FPS with SIMD-accelerated
  animation. That belongs to a program whose interactive process owns the agent;
  the launcher here is deliberately thin.
- **Streaming.** Worth noting only because maki's `streaming.rs` sits beside
  `compaction.rs` in the same module. It is not a requirement here.
