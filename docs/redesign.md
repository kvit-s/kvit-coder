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

The design below is shaped by five requirements, in order of how much they
constrain the architecture:

1. **Steering.** You must be able to type a message while a turn is in
   progress and have the model see it, without killing the turn.
2. **Long-running and observed commands.** Test suites, builds, dev servers and
   log watchers must be startable, watchable, and able to report back. The
   current 3-minute ceiling and the run-to-completion-then-report model do not
   cover this.
3. **One process per turn.** The agent runs a turn and exits; the driver spawns
   it again for the next one. Headless is the common case, the driver holds no
   agent state, and a wedged turn ends when its process does. Everything with a
   longer life belongs in the session directory rather than in memory.
4. **A durable session.** A session is a record you can open months later and
   read, not a runtime artefact.
5. **No weak-model tax.** The recovery machinery built for local models should
   not run when it is not needed.

Nothing here requires changing the tool interface, the registry, prompt
generation, the MCP client, the transport layer, or the benchmark harness.
Section 15 lists what stays untouched.

### Two words, fixed

- A **turn** is one whole request: everything from an instruction of yours to
  the model's final answer. A turn can contain many model calls and dozens of
  tool calls.
- An **iteration** is one model call inside a turn: a request goes out, an
  answer comes back, and whatever tools it asked for are executed. The code
  already uses this name — `runner_iteration.go`, `iterCtx` — so the document
  keeps it.

The inbox is drained once per iteration. A turn is also one process, so
"per turn" and "per process" mean the same thing below.

## 2. The organizing idea: one input channel

The current loop has exactly one way for anything to enter the conversation: the
model asks for a tool, the tool returns, the result is appended. The user speaks
only by starting a new process. Nothing else can reach the model.

The redesign gives the agent a single **inbox** that three producers write to:

- lines you type, at any moment
- events from background processes and observers
- system notices the runner already generates (loop detection, plan state)

Every message carries its **kind** — `user_line`, `process_event`, `notice`.
The queue is bounded and drops its oldest entry when full, so no producer ever
blocks on a busy agent.

Your lines reach it two ways, and nothing downstream can tell them apart.
**Stdin** covers the case where a terminal is attached, which includes a plain
`kvit-coder -p "..."` typed at a prompt. **The session inbox directory**
(section 3) covers a run started from a script, backgrounded, or watched from
another window. Both push `user_line`, so the drain and the question reader
behave identically whichever side the text came from.

**Draining takes everything.** Once per iteration, immediately before the model
call, the loop empties the queue and appends what it found to the history. It
inspects nothing and decides nothing; the model sorts out what the messages
mean. Once per iteration is not a tuning choice: history can only be appended
where it is well-formed, meaning every tool call already has its result, and
that point falls between iterations. Draining less often means draining once per
turn, which is exactly what would leave a steering line queued until a
fifteen-iteration turn finished.

Nothing waits for a scheduled moment. A line arriving during a blocking tool
makes that tool return, and the drain and the next request follow immediately;
the arrival is what produces the iteration. The ceiling on how long "typed" is
from "seen" is whatever single operation is in flight — a model call that has
to come back, or a tool that has to return — and for a tool watching the inbox
that ceiling is zero. That is the argument for the inbox exposing a signal
channel any long-running tool can select on, rather than special-casing the
waiting tools.

The one decision the agent does make is how an item is **represented**. A
`user_line` becomes a user message, because it came from you. A `process_event`
does not: telling the model "the user said: bg1 exited 1" attributes a machine
event to a person and invites a reply rather than an action, so an event is
appended to the last tool result as a `<system-reminder>`, the mechanism loop
detection already uses (`internal/agent/runner_iteration.go:100`).

**Between turns there is no agent.** The process exits when the turn ends
(section 3), so anything arriving in the meantime — a line you type, a detached
process exiting — waits in the session directory and is read when the next turn
starts. There is no idle loop to wake and no notification path to build.

That leaves one mechanism doing two jobs inside a turn. Steering is an inbox
write while the loop runs; a question (section 6) is an inbox read that accepts
only `user_line`. Holding the prompt between turns is the driver's job, not the
agent's.

The queue is modelled on maki's `SessionMailbox` (`github.com/tontinton/maki`,
`maki-agent/src/mailbox.rs`), minus its per-message `wake` flag. maki needs that
flag to tell an idle session which messages deserve a turn. Here there is no
idle session to wake: within a turn every message goes to the model at the next
iteration, and between turns nothing is running to be woken.

## 3. Process model

**`kvit-coder` is one process per turn.** It reads the session, runs the turn to
a final answer, appends what happened, and exits. Everything with a lifetime
longer than a turn lives in the session directory rather than in memory.

**The driver** — `kvit-coder-ui`, a shell script, or anything else — spawns the
agent once per turn with `-s <session>` and waits. It holds no message history,
manages no cancellation, and knows nothing about iterations.

Headless is the common case and the reason this shape is worth keeping: no UI
process, a simpler startup path, a smaller resident footprint, and a wedged turn
that ends when its process does rather than needing a way out.

Per-turn processes work because the state is in the right place, which is the
part the current code gets wrong. The conversation goes to an append-only
`history.jsonl`; the checkpoint shadow repository is keyed on the session
instead of `time.Now().UnixNano()`; session-scoped permission grants and spilled
tool output live in the session directory instead of process memory; and
`KVIT_RUN_ID` is pinned by the driver so every turn shares one prompt-cache
identity. Section 13 gives the layout.

Two things do cost something, and both are bounded:

- **MCP servers reconnect every turn.** Connections are made concurrently, so
  the cost is the slowest server rather than the sum — a second or two for a
  stdio server, and the 20-second figure in the config is a timeout, not a
  typical time. It is a reason to prefer HTTP MCP servers where you have the
  choice.
- **A process only outlives its turn if it is detached.** See section 7: a child
  of the agent dies with the turn, which is right for a test run and wrong for a
  dev server, so the two get separate tools.

### The session directory

Steering has to work for a headless run, and stdin cannot always provide it: a
run started from a script, backgrounded, or watched from another window has no
terminal to type into. The answer is a directory rather than a listener, because
**the drain is already a poll** — the inbox is consumed at iteration boundaries
and nowhere else, so a socket would deliver nothing sooner than a directory scan
does.

A producer writes to a temporary name inside `inbox/` and `rename()`s it into
place, which is atomic on one filesystem, so the agent never reads a
half-written message. Names carry a monotonic prefix so they sort into arrival
order. At each drain the agent reads what is there, appends it as `user_line`,
and unlinks. The file disappearing is the acknowledgement.

```
kvit-coder steer -s my-feature "also update the tests"
kvit-coder steer "also update the tests"     # no -s: the only running turn
echo "also update the tests" > ~/.kvit-coder/sessions/my-feature/inbox/$(date +%s%N)
```

`steer` is a convenience that gets the naming and the atomic rename right, not a
second mechanism — the shell redirect below it does the same job when the binary
is not to hand. That openness is the point: a script, an editor macro, a cron
job, another agent, or a container with the directory bind-mounted can all reach
a running turn without a client or a protocol. A line written while no turn is
running is read at the start of the next one.

The `-s`-less form matters more than it looks. A headless run auto-generates its
session name, so requiring `-s` would mean looking the name up before you could
say anything. Targeting the single running turn, and erroring when there is more
than one, makes steering something you can type without preparation.

**Watching needs no protocol.** With the conversation appended to
`history.jsonl` as it happens (section 13), looking in on a long turn from
another terminal is `tail -f` on that file, and answering is a file dropped in
`inbox/`. There is no subscriber list, no streaming format, and no listener to
maintain.

Anything able to write your home directory can inject instructions into a
running turn, which would be equally true of a socket owned by the same user.
Mode 0700 on the directory, a size cap on what is read, and a log line for
anything oversized.

### Failure and exit

- The turn ends when the model answers without asking for a tool. The agent
  appends the answer, releases what it owns, and exits; the driver returns to
  its prompt.
- Ctrl-C aborts the current iteration and ends the turn early. What "abort"
  does precisely is set out in section 5; the process then exits as it would
  after any other turn.
- A crash leaves `history.jsonl` intact up to the last thing that happened, so
  the next turn resumes from there. Stale pidfiles and a non-empty `inbox/` are
  the only cleanup, and both are handled at the start of the next turn.

## 4. The turn loop

```
turn(session):
    history := session.load()            // history.jsonl, replayed
    history.append(inbox.drain())        // anything left since the last turn
    history.append(user_prompt)          // the -p argument

    for i := 0; i < iteration_budget; i++ {
        history.append(inbox.drain())    // steering lands here

        resp := model.call(history)
        if resp.no_tool_calls {
            print(resp.text)
            session.append(resp)
            return                       // turn over, process exits
        }

        for tc := range resp.tool_calls {
            session.append(execute(tc))
        }
        session.append(watchers.poll())  // detached-process events as reminders
    }
    report("iteration budget reached")
```

One loop, not two. A turn is one process and one pass through this; the driver
supplies the next turn by starting another.

**The drain sits at the top of each iteration**, immediately before the model
call, at the place where `internal/agent/runner.go:196` currently reads messages
for file-first mode. Every tool call from the previous iteration already has its
result appended by then, so the history is well-formed for both wire protocols
and a user message can be appended safely.

**The iteration budget is per turn**, which is now the same as per process. The
current `max_tool_iterations` already reads that way; what changes is that a
value like 200 becomes a ceiling on one instruction rather than on a session.

**Reaching the budget is reported, not silent.** The current loop ends without
printing anything when the cap is hit. Say so, append it to the history, and
exit, so the next turn starts with the fact that the last one ran out rather
than with an unexplained gap.

**Appending happens as it goes**, not at the end. Each message is written to
`history.jsonl` when it occurs, so a crash costs one message rather than a turn,
and `tail -f` has something to follow.

## 5. Steering

Mechanically this is small, because the agent already owns the terminal.

- The driver passes `os.Stdin` to the agent instead of `nil`
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
one model call and one tool batch away. Nothing is cancelled and the turn
continues uninterrupted. This is the common case: a correction, an extra constraint,
a "also update the tests."

**Ctrl-C aborts the iteration and ends the turn.** It does not attempt to
salvage the half-finished turn, and that is what makes it simple:

1. Cancel the iteration context, which stops the in-flight model call and any
   running tool.
2. Kill every background process group.
3. Write down what was aborted, as ordinary history the model reads next turn:
   an error result for the tool that was running, including whatever output it
   had produced; `Error: Cancelled by user` for tool calls that were issued but
   never reached; a note naming the background processes that were killed; and,
   when the model call itself was cut off, a marker that the assistant turn was
   abandoned.
4. Exit. The driver returns to its prompt.

The next turn starts from a history that explains exactly what stopped and
where, so the model does not re-plan from a state it thinks still holds.

Most of item 3 already exists. `handlePostIteration` appends
`Error: Cancelled by user` for unexecuted tool calls, `handleLLMError` appends
`[Operation cancelled by user]` when a model call is cancelled mid-tool-loop,
and both `executeTools` and `executeToolWithTimeout` already check
`ctx.Done()` at the right points. What is missing is a SIGINT handler that
cancels the iteration context rather than calling `os.Exit(130)`
(`internal/workspace/lock.go:55`) — the exit is right, the abrupt exit before
anything is written down is not — plus the process kill in item 2.

Two rough edges to accept initially. The terminal echoes what you type
interleaved with agent output, which is untidy until the agent draws its own
input line in raw mode. And a steering message arrives as a plain user message,
so the model may treat it as a new instruction rather than a correction; wrapping
it in a marker such as `<user-steering>` and saying in the system prompt what
that means is worth doing from the start.

(Note, added later: raw mode was rejected after it caused too many issues, and
the echo problem is handled without it instead — an empty `Enter` pauses the
turn at its next iteration boundary and prompts for steering, so the line is
typed while no output flows. See `steer-pause.md` in the repository root.)

## 6. Questions

The model needs to ask sometimes: which of two approaches to take, whether a
schema change is acceptable, which of three plausible files you meant. A
question is a tool call whose result comes from a person rather than from the
machine, so it needs no new input path. It reads from the inbox, like everything
else.

### One reader, two modes

The line reader from section 5 runs for the length of the turn. What changes is
what is drawn above it and how the agent reads the next line:

| Agent state | Shows | The next line is |
|---|---|---|
| turn running, no question | nothing in particular | steering |
| question pending | the question and its numbered options | the answer |

One input surface, not two interfaces contending for stdin. The hint on the
input line (`[1-2, or type]`) says which mode you are in. Between turns the
agent is not running at all and the prompt belongs to the driver.

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

Blocking the turn costs nothing, because background processes are not in the
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

Interactive: block indefinitely. A turn that waits costs nothing, and a
timeout firing while you are away is worse than waiting.

No terminal: write the question where a watcher can see it, wait up to
`question_timeout` for an answer to appear in the session inbox, then return
"no one answered; use your judgement and state what you assumed." Default the
timeout to zero, so benchmarks and scripted runs fall back at once — the
behaviour the MCP confirm policy already has, where `ask_*` degrades to `block`
rather than hanging. A supervised headless run sets it to a few minutes.

A timeout rather than a presence check is what makes this work without a
listener. Nothing has to know whether a person is watching the session
directory: waiting briefly and proceeding is the same answer either way, and it
needs no heartbeat, no subscriber list, and no client to be running.

Ctrl-C is dismissal and follows section 5's abort semantics: record "question
dismissed by user" as the tool result and end the turn. The question and its
dismissal are both in history, which is what stops the model asking again.

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

A model can already ask by ending its turn: it says "which of these two?", the
driver prompts, and you reply as the next turn. What the tool adds is that the
turn keeps its plan rather than ending in a wrap-up the next turn has to resume
from, the options come back machine-readable instead of parsed out of your
prose, and the answer is bound to its question in the record rather than
floating as a loose user message. The first of those is the real one, and it is
worth more here than it would be in an agent that never exits, because ending a
turn also ends the process.

## 7. Background processes and observers

### Two lifetimes

A process only outlives its turn if it is not a child of the agent, so the two
cases get separate tools rather than a flag:

- **Turn-scoped** (`Shell.run`) — a child of the agent, killed when the turn
  ends. Test suites, builds, anything the model waits on and acts upon inside
  one turn.
- **Session-scoped** (`Shell.start`) — started detached with `setsid`, its
  output to `proc/<id>.log` and its pid to `proc/<id>.pid` in the session
  directory. The OS keeps it alive; the filesystem keeps the metadata. A later
  turn reads the directory and knows exactly what is running, and can tail it,
  poll it, or kill it.

That is what lets background work span turns without a supervisor or a daemon,
and it is why per-turn processes cost nothing here.

### Tool surface

```
Shell.run     {command, cwd, timeout}             → runs to completion, dies with the turn
Shell.start   {command, cwd, name}                → {id}   detached, survives the turn
Shell.output  {id, cursor}                        → {output, cursor, running}
Shell.status  {id}                                → running | exited(code)
Shell.list    {}                                  → [{id, name, running, age}]
Shell.kill    {id}
Observe.wait  {id, poll, report, until, max_wait} → blocks; output + why it returned
Observe.add   {command, every, report}            → {id}   periodic probe
Observe.remove{id}
```

`Shell.start` returns immediately, so the 3-minute ceiling on synchronous
commands stops being the binding constraint. `Observe.wait` opts out of the
hardcoded 15-second per-tool timeout through the existing `SelfTimeoutTool`
interface (`internal/tools/tool.go:46`), which MCP tools already use for the
same reason.

An `Observe.add` entry is a process that produces output on a timer — a health
probe, a `git status`, a queue depth. It shares the registry and the reporting
rules with `Shell.start`, so there is one mechanism with two producers.

### Watching, and who decides what

`Observe.wait` blocks and returns when something is worth a turn. The model's
next call is the decision about what to do next, so the agent needs no state
machine: waiting longer is calling `wait` again, changing the cadence is calling
it with a different `poll`, giving up is calling `Shell.kill`.

```
iteration N:   Observe.wait {id: bg1, poll: 60s, report: changed, max_wait: 30m}
               blocks, waking on its own timer, process exit, a matched
               pattern, or a user_line arriving
               → returns what it has and why it returned
iteration N+1: the model reads the report and decides
```

**How often the tool looks and how often the model is told are separate
settings.** `poll` is the tool's cadence; `report` decides when it returns —
`changed`, `match`, `exit`, or `always`. Default to something other than
`always`, because a model writing "check every minute" usually means "tell me
when something happens" and a tick is the easiest way to say it. `max_wait` is
the heartbeat ceiling, so the model gets a turn eventually even when nothing
happens.

This is guidance rather than a limit — no floor on how often the model may look.
The prompt side of a poll is largely cached, so the cost is not the replayed
conversation; it is the output, and at high reasoning effort every report buys a
fresh reasoning pass measured in thousands of tokens and tens of seconds. On a
subscription that is rate limit and wall clock rather than money. Make it
visible instead of capping it: have the return say how long it waited and how
many ticks produced nothing, so both you and the model can see when polling is
not paying.

Cached prefixes cut the other way and are worth knowing about. Providers
typically hold them for a few minutes of inactivity, so a thirty-minute silent
wait can cost a full cold prefill on the far side while a sixty-second poll
keeps the cache warm throughout. The TTL on the opencode.ai endpoint is unknown
and worth measuring, since it sets the natural upper bound on how long a wait
should go silent.

The model is not obliged to block at all. With other work to do it does that
work and calls `Shell.output` between pieces of it, using the same tools without
waiting.

### Output handling

Each process gets a bounded ring buffer in memory plus a full log file under the
session directory. Two consumers read from it independently:

- **The terminal**, live, so you can watch a build scroll past. This costs
  nothing and is where most of the value of "observing" actually lands, because
  watching progress is a human need.
- **The model**, only when it asks (`Shell.output`, `Observe.wait`) or when a
  condition it registered fires.

### When the model is told

After each tool batch, the runner polls the registry and appends notes to the
last tool result as `<system-reminder>` text, reusing the mechanism loop
detection already uses (`internal/agent/runner_iteration.go:100`). Three
conditions produce a note: a session-scoped process exited, a regex the model
registered matched new output, or an observer's output changed since the last
report. Nothing is reported on a timer, and unwatched output never reaches the
model.

Between turns nobody is listening, so an event that arrives then waits in the
session directory and is read at the start of the next turn (section 2). There
is no idle agent to wake.

### Interrupt and exit

Turn-scoped processes die when the turn ends, whether it ended by answering or
by Ctrl-C. Session-scoped processes survive a turn ending normally and are
killed by Ctrl-C, both through the existing `killProcessGroup` path
(`internal/tools/shell.go:417`), and the kill is written into the history so the
model knows the dev server it started is gone rather than assuming it is still
answering on port 3000.

Tying session-scoped death to interrupt is deliberate: an interrupt usually
means the approach was wrong, and leaving a test watcher and a stale server
running from an abandoned approach is worse than restarting them. If a
long-lived server turns out to be worth keeping across an abort, the narrow fix
is a `keep_on_interrupt` flag on `Shell.start` rather than a change to the
default.

The start of every turn reconciles the registry against reality: a pidfile whose
process is gone becomes an exit event, and its log is available to read.

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
(`internal/agent/permissions.go:60`). Session grants go to the session
directory rather than process memory, so they last the session rather than the
turn that made them.

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

This does nothing for dependent chains, where the second call needs the first
call's output. That is what appendix A.2 addresses, and it is the reason to build this
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

With a 1M-token window this may never become urgent, so the plan is one piece
built now and one deferred until it does.

**Token accounting, now.** Track prompt tokens against `llm.context` after every
response — it is currently used only to render a display string — and show
context used against the window in the status line. That is what turns "will I
ever need compaction" from a guess into an observation after a week of use.

**Compaction, later, as a command.** When it is needed, compacting **ends the
session and starts a new one** rather than editing the current one. The old
session stays a complete, immutable record; the new one opens with a handoff and
a `parent` reference back. Everything else shuts down at the boundary — detached
processes are killed and named in the handoff, so nothing is left running that
no session owns.

That shape avoids the one hazard in-place compaction has. Pruning a live history
must remove whole groups of an assistant turn plus all of its tool results, or
the Responses item list comes back malformed with a `function_call` whose output
is gone. A new session opens with plain text and no tool items, so there is
nothing to pair and nothing to orphan. It also fits per-turn processes: the
agent writes the summary, records the successor and exits, and the next turn
starts from a different file. `kvit-coder compact -s my-feature` can be a
standalone command that makes one model call, uses no tools, and touches nothing
in the loop.

What the new session opens with is deliberately not settled here. A prose
summary is the obvious answer and probably the weakest; what the next session
needs is a handoff — the original goal, what is done, what is in flight, which
files were touched, the current plan, the open questions. That deserves the same
care as the system prompt and can be designed when the tool is.

**What has to be true now so that nothing needs redesigning then**, all of it
worth having anyway:

- history is append-only, so a session is a record rather than a snapshot
- `meta.json` has `parent` and `succeeded_by` fields from the start, unwritten
  until compaction exists — adding fields later is a migration
- session name resolution goes through `meta.json` even when the chain is one
  long, so `-s my-feature` is already an indirection rather than a folder lookup

With those three, compaction is a command that writes a file and sets a field.

**Honest 400 handling** belongs here too. Today any HTTP 400 is assumed to be
context overflow; the handler rewrites every trailing tool result to a
placeholder and retries (`internal/agent/runner_llm.go:110`). With real token
accounting, overflow is predicted rather than discovered, and a 400 can be
surfaced as what it usually is — a malformed request — instead of corrupting the
transcript.

## 13. Session persistence

A session is a durable record, not a runtime artefact. Months later you should
be able to open one and read what happened, which the current format does not
support: one flat file rewritten in full after every answer
(`internal/session/manager.go:87`), holding marshalled `llm.Message` values with
no timestamps.

### Layout

```
~/.kvit-coder/sessions/<name>/
    history.jsonl     the record, append-only, never rewritten
    meta.json         created, updated, workspace, model, first prompt,
                      summary, parent, succeeded_by
    checkpoints/      shadow git repo, keyed on the session
    proc/             pidfiles and logs for session-scoped processes
    inbox/            drop files here
    tmp/              spilled tool output
```

`history.jsonl`, `meta.json` and `checkpoints/` are the record and stay for as
long as the session does. `inbox/`, `proc/` and `tmp/` are runtime and are the
only parts safe to clean. Existing flat `<name>.jsonl` files migrate by becoming
the `history.jsonl` of a folder with the same name.

Keying the checkpoint repository on the session rather than on
`time.Now().UnixNano()`, and leaving it in place rather than deleting it at
process exit, is what makes a months-old session answerable about what actually
changed rather than only about what was said.

### The record

One timestamped event per line, appended when it happens: user messages,
assistant messages, each tool call and result with its duration, notices,
interrupts, and the settings in force. Appending as it goes means a crash costs
one message instead of a turn, and it is what makes `tail -f` a working way to
watch a headless run.

Two things the current format drops, for different reasons:

- **`ReasoningContent`** is already persisted when it survives, and whether it
  survives is the `merge_thinking` experiment: on a turn with tool calls it is
  either folded into `Content` or discarded (`internal/llm/middleware.go:57`).
  Keep that switch — it is the point of the experiment — and record which
  setting was in force for the turn, so a session opened next spring answers
  "why is there no reasoning here" with "because it was configured off" rather
  than leaving you to guess.
- **`ReasoningBlocks` and `ToolCallItemIDs`** are `json:"-"`, and for the record
  that costs nothing: the blocks are encrypted by the provider and unreadable by
  anyone in six months. What it may cost is resume correctness, because a
  reloaded session replays `function_call` items whose `reasoning` items are
  gone. That is independent of the experiment — it bites even when reasoning is
  being replayed — and section 17 keeps it as the question to answer first.

### Reading one back

`meta.json` is what makes a listing useful at a distance: created and updated
times, workspace, model, the first prompt, and a one-line summary. A `session
show` command renders `history.jsonl` as a transcript; the raw file stays
readable on its own, which is the property worth protecting when choosing how
much structure to put in each line.

## 14. The system prompt

### What it costs today

Generated from the current config — Read, Edit, Search, Shell, Write — the
system prompt is 5,593 characters, roughly 1,400 tokens, plus about 500 tokens
of tool schemas and descriptions in the request itself. Turning on Plan and
Checkpoint takes it to 6,661 characters, roughly 1,665 tokens. Against a 1M
window that is under two tenths of one percent, and it is the cached prefix, so
once `KVIT_RUN_ID` is pinned (stage 1) it costs almost nothing per request after
the first.

Trimming it is therefore not a context saving, and should not be sold as one.
The reason to rewrite it is that its content is *instructions*, and a capable
model follows instructions — including the ones written for a model that could
not count lines.

### What is in there for a weaker model

The generated prompt is a role line, a capability list, a four-step workflow, one
editing example, three guidelines, and the tool documentation
(`internal/prompt/prompt.go:201`). Four things in it work against the target
model:

- **A fixed procedure.** `# WORKFLOW` numbers the steps: search, then read, then
  edit, then run something to verify. A model that plans better than the
  procedure is told to follow the procedure.
- **A worked example, tuned to the edit mode.** `# EXAMPLE` spends twenty lines
  on an invented Python file, ending in `VERIFY: check diff and after_edit ...
  (no orphaned braces, missing closures, or structural issues)`. That is
  line-mode anxiety, and it is generated per mode — so when config and prompt
  drift, the example instructs the model to call `Edit.confirm` and
  `Edit.cancel` that may not be registered.
- **Rules restating the schema.** `new_text replaces lines start_line through
  end_line EXACTLY`, `ALWAYS read the file before editing to get correct line
  numbers` — the first is the schema, the second is false in searchreplace mode.
- **Narration and non-rules.** `Briefly explain what you're doing when calling a
  tool` shapes output style by accident; `Only use the tools listed below`
  forbids something that is not possible.

### The rule to write against

**The schema is the API documentation. A prompt section carries only what the
schema cannot express**: failure modes, invariants, and interactions between
tools. Applying that to the current sections leaves a short list per tool —

- `Read`: output stops at 150 lines or 24 KB, so a large file arrives in pieces
  and the model should know it did.
- `Edit`: the search text must match byte for byte; multiple matches return an
  error rather than editing the first; an empty search creates a file.
- `Shell`: the shell is stateless, so `cd` does not persist between calls;
  output above the limit is written to a temp file whose path comes back in the
  result.
- `Search`: results degrade to `file:line:match` above 20 matches and spill to a
  temp file above 100.

Everything else — usage lines, parameter lists, examples, "always read before
editing" — comes out.

### What goes in its place

- **An environment block**: operating system, shell, working directory, date,
  git branch and short status, and a top-level directory listing. Generate it
  per session rather than per turn, so the prefix stays stable for caching.
- **The new mechanisms**: what a `<user-steering>` message means, that
  background processes exist and how to park on one, what a process-event
  reminder looks like when it arrives, and when to use `Question` — the guidance
  in section 6 belongs here rather than in the tool description, because it is
  about judgement rather than syntax.
- **Working guidance, kept short**: verify by running something, prefer a tool
  over its shell equivalent where both exist, say what was left undone.

Net size is roughly a wash: perhaps 600 tokens come out of the tool docs and the
environment block puts a few hundred back. The point is what the model is being
told, not how much.

### How to do it without losing the weak-model version

The template system already exists — `prompts.use_templates`, with templates
embedded under `internal/prompt/prompts/` and overridable from disk. Have the
profile from section 11 select the variant, so `weak` keeps the verbose prompts
and stays benchmarkable while `strong` gets the short ones. Keep generation
driven by the enabled tool set either way, so the prompt still shrinks when
tools are disabled.

Measure the trim with thinkbench rather than by reading it. A prompt that got
shorter and scored worse is a prompt that was doing something.

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
driver so continuity works at all; pin `KVIT_RUN_ID` so the prompt cache
survives across turns; drop the duplicate `GetGenerationStats` calls and give
the HTTP client a timeout; remove the stray `DEBUG:` line at
`internal/tools/tempfile_manager.go:28`; fix the stale interrogation test. None
of these depend on the redesign, and they are worth doing whether or not it
proceeds.

**Stage 2 — the session directory (section 13).** Move the session from a flat
file to a folder: append-only `history.jsonl`, `meta.json` with its `parent` and
`succeeded_by` fields, the checkpoint repo keyed on the session, and `tmp/` and
`proc/` in place of the process-scoped temp dir. Name resolution goes through
`meta.json`. This is what makes per-turn processes lose nothing, and everything
after it assumes the folder exists.

**Stage 3 — the inbox and steering.** Add the inbox with its bounded queue and
message kinds, move the drain into the loop, pass `os.Stdin` to the agent, add
the reader goroutine and the tty guard, and add `inbox/` with the `steer` client
— headless runs need steering as much as interactive ones, and both producers
land in the same queue.

**Stage 4 — abort semantics.** Replace the lock's `os.Exit(130)` with cancelling
the iteration, writing down what stopped, and exiting cleanly. Report the
iteration budget being reached rather than ending silently.

**Stage 5 — questions (section 6).** The `Question` tool, the two display modes,
the after-the-question ordering rule, and the `question_timeout` fallback for a
run with no terminal.

**Stage 6 — background processes (section 7).** Split `Shell.run` from
`Shell.start`, add the detached process registry under `proc/`, the reconcile
pass at turn start, `Observe.wait` with its `poll`/`report` split, the terminal
tee, and the registry poll after each tool batch.

**Stage 7 — permissions (section 9).** Swap the regex blocklist for scopes
derived from the bash syntax tree, add the three rule tiers and the four grant
answers, and persist always-grants to config and session grants to the session
directory. Independent of everything above, so it can move earlier if the
current blocklist starts costing you time. The existing shell tests give you a
ready-made corpus: every command they assert on should reach the same verdict
through the new path.

**Stage 8 — `Batch` (section 10).** An afternoon, once the tool registry is
otherwise settled.

**Stage 9 — profiles and accounting.** Split `strong` and `weak`, measure with
thinkbench, then add token accounting and the context display. Compaction waits
until the accounting says it is needed.

**Stage 10 — the prompt (section 14).** Rewrite it once the mechanisms it must
describe exist.

Before stage 3, write a table-driven test that drives `Runner.Run` against a
scripted fake model client and asserts on the resulting message history. The
loop is about to grow an inbox, a registry poll and new abort semantics, and
there is currently no test that exercises it at all.

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
- **Does killing session-scoped processes on Ctrl-C bite?** It is what you
  asked for, and an interrupt usually means the approach was wrong. If losing a
  dev server on every interrupt turns out to be the common case rather than the
  rare one, `keep_on_interrupt` on `Shell.start` is the narrow fix.
- **Does `Question` earn its place next to asking by ending the turn?** A model
  can already ask by answering. Build the rest first, then judge whether keeping
  the turn's plan alive is worth a tool.
- **How much does `Batch` actually get used?** A model that plans well will
  batch; one that works step by step will not, whatever the prompt says. If it
  goes unused, that is evidence for appendix A.2 rather than against batching.
- **Does compaction hurt more than it helps at 1M tokens?** Possibly the window
  is large enough that dropping stale tool outputs is sufficient and
  summarization is unnecessary. Accounting comes first; compaction may turn out
  to be a smaller job than section 12 implies.
- **Should the driver be a Go binary at all?** It picks a session, pins an
  environment variable, spawns per turn and reads a line. That may be a shell
  script, with `kvit-coder-ui` kept only for the input editing it already does.

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
  the driver here is deliberately thin.
- **Streaming.** Worth noting only because maki's `streaming.rs` sits beside
  `compaction.rs` in the same module. It is not a requirement here.
