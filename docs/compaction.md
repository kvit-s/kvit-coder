# Compaction

> Status: implemented in October 2026; "Where it is implemented" lists the
> code. The appendix records what five open-source agents do, which the design
> draws on. The keys are also listed in
> [configuration.md](configuration.md#compaction).

## What compaction is for

kvit-coder sends the whole conversation to the model on every request. The
default model has a 1M-token context window, so most sessions never come close
to it, but nothing bounds a session. Each tool result is capped (`Read` returns
at most 150 lines or 24 KB, and larger shell output goes to the session's
`tmp/` with a pointer), which limits one result but not how many accumulate:
300 tool calls at about 6,000 tokens each come to 1.8M tokens, within a single
turn. Compaction keeps such a session going by replacing older parts of what is
sent to the model with shorter forms, while `history.jsonl` keeps everything.

Because the agent runs as one process per turn (`docs/architecture.md`),
compaction has to be computable from the session directory when a turn starts,
has to survive the process being killed, and has to be recorded by appending to
`history.jsonl` like everything else.

## Design

### Constraints

1. **One process per turn.** Whatever compaction decides must be on disk before
   the process can exit, and the next process must arrive at the same messages
   from the file alone.
2. **`history.jsonl` is only appended to.** `Append` opens the file with
   `O_APPEND` for each line, a rollback is a line saying how many messages to
   drop, and `tail -f` follows a running turn. Compaction is therefore a new
   kind of line; rewriting or truncating the file, as maki and codex do, is
   ruled out.
3. **The prompt cache.** The opencode.ai endpoint caches the start of each
   request, routed by the `x-opencode-session` header (`KVIT_RUN_ID`), and a
   request whose beginning matches an earlier one pays a fraction of the price
   for that part. So anything compaction inserts goes after the system prompt
   and tool specs, which stay byte-identical, and the point where word-for-word
   messages begin stays put between compactions, because moving it changes the
   request from that point on.
4. **`Load()` has other readers.** `Session.Load` and `readHistory` feed
   `--session-show`, the message counts in the session list
   (`internal/session/manager.go:159`), the session replay sent to an editor
   over the Agent Client Protocol (`cmd/kvit-coder/acp.go:426`), `LastReport`
   (which decides whether the front end draws the report card), and
   `migrateFlatFile`, which writes a new `history.jsonl` from what
   `readHistory` returns. All of them want the full conversation, so the
   compacted context must come from a separate function.
5. **Few, long turns.** Sessions here tend to have a handful of turns, each
   with heavy tool traces, and a turn's final report is a small, faithful
   account of what the turn did. The design relies on both: older turns shrink
   to their prompt and report, and the only part the model has to compact is
   the turn in progress.

### What sessions look like

The defaults below were checked against the 67 sessions in one user's
`~/.kvit-coder/sessions` in October 2026, 107 turns in all. Sizes are
characters ÷ 4 and leave out encrypted thinking, the system prompt and the tool
specs, so the server's counts would be higher.

| | median | 90th percentile | largest |
|---|---:|---:|---:|
| turns per session | 1 | 3 | 9 |
| tokens per turn | 31k | 128k | 590k |
| tokens per final answer | 550 | 950 | 5.3k |
| tokens per step (a tool call and its results) | 540 | 2.2k | 13k |

Half the sessions are a single turn, and the largest session is one turn of
about 590k tokens, so compacting the turn in progress is the path that matters
most. A turn is about 82 times the size of its final answer at the median,
which is what makes turn summaries worth having. Of the 107 turns, 21 ended
without any answer (cancelled, out of iterations, or failed), 85 with prose and
1 with a `Report` call.

### Overview

Two things happen as the context fills, at two points:

- **Tier 0**: the agent stops merging the model's readable thinking into its
  answers, even with `merge_thinking: true`, and stops asking the endpoint for
  that thinking. This only slows growth.
- **Tier 1**: the context is rewritten. The **turn in progress**, called turn N
  below, is the current turn together with any turns just before it that ended
  without an answer: their work is in no final answer, and they are the turns a
  user follows with "continue". The rewritten context is:

```
system prompt + tools
turns 1 … N-2:  one turn summary each, or one summary of them all   by code, or by the model when too large
turn N-1:       turn summary                                       by code
turn N:         its user input, word for word,
                plus its work before the tail, compacted            the model writes the compacted part
last ~20k tokens of turn N, word for word, starting at a step boundary
```

A **turn summary** is a past turn reduced to the user's input and the final
report the agent ended it with. It is built by code from `history.jsonl` and
never stored. The model is asked to compact only the part of turn N that comes
before the word-for-word tail, and its output has to fit in the space tier 1
keeps free; the rewritten context as a whole does not have to, because the
turn summaries replace far larger tool traces. When tier 1 fires at the start
of a turn that follows an answered turn, turn N has nothing in it yet, so the
rewrite is done entirely by code with no model call.

After the rewrite, if the context is still above 20% of the window, the model
is also asked to replace the turn summaries of turns 1 … N-2 with one summary of
them. The summary of turn N-1 stays as it is.

### When each tier fires

Both points are measured on the request about to be sent, checked before every
request:

```
W = context window (llm.context of the model the turn runs with)
P = size of the request about to be sent
  = the server's prompt_tokens for the previous request
    + (characters added since then ÷ 3)
T = how much the turn in progress has added since it began, summed step by
    step from the server's counts, so a rewrite inside the turn does not reset it
S = room for the model's compacted version of turn N    = W/32, kept between 4k and 32k
G = room for one more step: the reply (including re-sent encrypted thinking)
    plus the tool results
  = the largest growth from one request to the next seen in this turn, at least 16k
α = turn_headroom_ratio, default 0.5
F = the size of the context right after the last rewrite in this turn,
    0 before the first

tier 1 fires when the free space W − P is less than
        max(S + G, min(α × T, (W − F) / 2))
tier 0 point = min(0.8 × W, the tier 1 point)
```

The free space has to cover two things. α × T is room for the turn to keep
going: a turn that has already added T tokens is likely to need a good part of
that again, so a long turn is compacted while it still has room to work, and a
short turn runs almost to the end of the window. S + G is what the compaction
request itself needs, explained below. The cap keeps α × T from asking for more
room than a rewrite can give. A rewrite cannot make the context smaller than F
— the system prompt, the tools, the compacted text and the tail — so the cap is
half the space above F, and the next rewrite waits until the context has grown
by that much. Without it, a long turn on a small window compacts again a step or
two after every rewrite: in a trial with a 40k window, the context after a
rewrite was 17k, and with a cap of half the window the rewrites came at 26k,
22k, 21k and 21k. Before the first rewrite F is 0 and the cap is W/2, which is
what the examples below use.

With a 1M window and about 20k of system prompt and tools, S is 31k and G at
least 16k:

- A turn that has added 30k keeps a margin of 47k, so tier 1 fires at about
  953k tokens (95%), which only happens when earlier turns filled the window.
- A turn of 500k with 480k left needs a margin of 250k, so nothing happens.
- A session that is one long turn is compacted once that turn passes about
  650k, when the context is about 67% full.
- With 600k of earlier turns and 100k in the current turn, the margin is 50k,
  so tier 1 fires at 950k; code then reduces the earlier turns to summaries
  and the model compacts the first 80k of the current one.

Tier 0 fires at 800k unless tier 1 comes first, as it does for the single long
turn, where both fire together. With a 200k window and a short turn, S is 6k
and G's minimum 12.5k (see below), so tier 1 fires at about 181k (91%).

For turns that the turn in progress spans from earlier processes, the starting
value of T is estimated from their messages.

The margin S + G makes the compaction request fit. If the check passes, the
request goes out, and its reply and tool results add at most G. At the next
check, the compaction request — at most that size plus a short instruction —
still has S tokens left for the model's answer. A step that grows by more than
G, for example a `Batch` of ten large reads, is handled under "Failure".

The characters ÷ 3 estimate covers only what was added since the last request,
because counting the whole conversation at a fixed number of characters per
token is unreliable over hundreds of thousands of tokens; the bulk of P is the
server's own count. At the start of a turn there is no previous request in this
process, so each assistant message's line in `history.jsonl` records the
`prompt_tokens` of the request that produced it, in a new `prompt_tokens` field
on the event. The estimate at turn start is the last recorded count plus
characters ÷ 3 for what came after it. When that count was measured by a
different model (the user switched models) or before a compaction record, the
estimate is characters ÷ 3 over the whole context, plus the tool specs every
request carries, which are not messages. The size recorded after a rewrite
counts the tool specs the same way: without them the first trial's estimate
after a rewrite was 10.5k against a server count of 15.4k, and with them it was
17.2k against 15.8k.

Small windows need the fixed numbers scaled down: on a 32k model the 20k tail
plus 16k is already more than the window. The tail is therefore
min(20k, W/8), and G's minimum is min(16k, W/16).

### Tier 0: stop merging thinking

With `merge_thinking: true`, `NormalizeResponse` (`internal/llm/middleware.go`)
puts the model's readable thinking in front of its answer and stores the result
as the message content. Thinking already merged cannot be separated again, so
tier 0 stops further growth: past the tier 0 point the turn continues as if
`merge_thinking` were false, and new thinking is dropped when it arrives.
Earlier answers keep their merged thinking; a session that merges thinking
simply reaches the tier points sooner. Tier 0 is judged on the size of the
request before any rewrite, so reaching the tier 1 point stops the merging too,
and it stays stopped for the rest of the turn; the next turn starts merging
again if its context is below the point.

- `initRunState` (`internal/agent/runner.go`) sets the normalizer's flag from
  the estimate at turn start, so a turn that starts past the point never merges.
- After each response, when P passes the point, the turn sets the normalizer's
  flag to false (a new `SetMergeThinking(bool)` on `ResponseNormalizer`, whose
  flag is private today), warns once, and appends a notice to the history.
  `checkThinkingArrives` reads this flag rather than `r.cfg.LLM.MergeThinking`,
  or it would warn about thinking the turn chose to drop.
- The turn also stops asking for readable thinking, so no output tokens are
  paid for text that is thrown away. `cmd/kvit-coder/turn.go` sets
  `WithReasoningSummary(cfg.ReasoningSummaryOrDefault())` when it builds the
  client; a setter is added behind an optional interface, so the stand-in
  client in the tests needs no change. The `reasoning` parameters are not part
  of the input, so changing them leaves the cached prefix intact.

### Tier 1: turn summaries

Turn boundaries are the `settings` lines that `repl.RunTurn` appends at the
start of each turn (`Session.Settings`). User-role messages cannot mark turns,
because the loop inserts user messages of its own. A failed settings write is
logged only at debug level today (`internal/repl/repl.go:144`); it is reported
like a failed prompt write instead, since a missing line merges two turns into
one summary.

Each turn summary is one user message and one assistant message:

- **User input.** The turn's prompt (the first user message after its settings
  line), followed by any `<user-steering>` messages, which the person typed
  while the turn ran (`internal/agent/steering.go`). Every other user-role
  message was inserted by the agent — the report reminder, "continue",
  server-error notes — and is left out. Attached pictures become an
  `[image: <path>]` line. A turn started by the inbox without a prompt (a wake
  turn) uses the inbox message that started it, or `[turn started by the
  inbox]` when there is none.
- **Final report.** Under `kcu`, which passes `--structured` on every turn, a
  turn that changed files or ran commands ends with a call to the `Report`
  tool rather than a prose answer (`report.mode: mutating`;
  `reportGateMessage` in `internal/agent/runner_iteration.go`). The summary
  uses the accepted report rendered as text — headline and blocks, laid out as
  in `internal/report/render.go` — and otherwise the last assistant message
  without tool calls. A turn that ended without an answer belongs to the turn
  in progress until a later turn ends with one; after that, its summary gives
  the reason from the notice recorded for it, for example
  `[the turn ended without an answer: cancelled by user]`.
- **Activity line.** Appended to the report: the files changed (`Edit`,
  `Write`), the files read (`Read`) and the shell commands run, taken from the
  turn's tool-call arguments, each list deduplicated and capped. It records
  what the turn touched at the cost of one line, without a model call.

### Tier 1: the word-for-word tail and the cut

The **cut** is the position where the word-for-word tail begins. To place it,
walk back from the end of the conversation adding up message sizes until 20k
tokens are reached, then move back to the start of the step that contains that
point. A step starts at an assistant message; its tool calls and their results
follow it, so the tail never separates a tool call from its result and is at
least 20k tokens long. The cut never goes before the start of turn N: if turn N
so far is shorter than 20k tokens, the cut is the start of turn N, nothing is
left to compact, and no model call is made.

The cut is recorded as a message position: the number of conversation
messages, counted the way `Load()` counts them (after rollbacks are applied),
that come before the first message kept word for word. Positions stay valid
because the file is only appended to. The one thing that could break them is a
rollback reaching back past a compaction record, so a rollback after a
compaction record is clamped to the messages after its cut. Rollbacks only
happen with `agent.profile: weak`, and the backtracking state is reset when a
compaction runs.

The cut moves only when tier 1 fires again. A cut recomputed on every request
would change where the word-for-word messages begin each time and miss the
prompt cache from there on.

### Tier 1: compacting turn N

The messages before the cut end with the tool results of a finished step, which
is exactly where an earlier request of this turn ended. The compaction request
is therefore that earlier request — the same system prompt, tool specs,
`tool_choice` and messages, with the encrypted thinking the runner holds in
memory for them — followed by one user message with the instruction. The
server has already processed all of it but the instruction, so nearly the whole
request is billed as cached input. Sending no tools or a different system
prompt, as opencode does, would change the start of the request and lose that.
When turn N spans turns that ran in earlier processes, their messages are
rebuilt from the file, which has no encrypted thinking, so the cache covers the
request only up to the first assistant message of the earliest of them.

The request goes through the session's own client and model, unless
`compaction.model` names another entry, and its answer is capped at S. The
`summarizer: true` entry that writes session titles is not the default: its
client is set up for a three-word answer (`summarizerClientFor` in
`internal/repl/title.go`: 8-second timeout, no retries, the lowest reasoning
effort), and a different model cannot use the session's cache.

The instruction asks the model to compact the work of the current turn so far
into terse Markdown bullets, for itself to continue from, keeping exact file
paths, symbols, commands and error messages, under five headings. "Learned" is
what the work found out that the rest of the request needs — what the files and
outputs it read actually say, how the code works — and the instruction says to
record the content rather than the fact of reading it; with a heading that only
asked what was established, a trial's compacted text listed which line ranges
of each file had been read and nothing of what they said. The other headings are
what was changed, what was tried and failed and why, decisions and their
reasons, and what remains to do for the user's request. It says that the
messages after this point will be kept word for word, so the compacted text
should end where they begin; that anything from earlier in the conversation
which the rest of the turn depends on — including text compacted by an earlier
rewrite in this turn — must be kept; that the user's request itself is kept
separately and need not be restated; and that the answer must not call tools
or talk about itself. A focus given with `:compact <focus>` and the configured
`compaction.instructions` are appended as `Additional focus: …`. Temperature
and reasoning effort stay at the session's settings, since the request has to
match the session's earlier request.

In the rewritten context, turn N appears as one user message holding its user
input word for word (the prompt of each turn it spans, in order, and any
steering typed before the cut), followed by the compacted text under a fixed label, `Work done so far in this
turn, compacted:`. The tail that follows starts with an assistant message, so
user and assistant messages keep alternating; some backends reject or merge two
messages of the same role in a row.

When tier 1 fires a second time in the same turn, the request is again the
messages before the new cut. They include the user message with the earlier
compacted text, so the model's new compacted text replaces it and covers both.

### Tier 1: summarizing older turns

After the rewrite, if the context is above `older_turns_fraction` of the window
(default 0.2, so 200k at 1M), the turn summaries of turns 1 … N-2 are replaced
by one summary. The request is the system prompt, the tools, those turn
summaries rebuilt from `history.jsonl`, and an instruction asking for terse
Markdown under: the user's goals, constraints and preferences the user stated,
what has been done, decisions and their reasons, and relevant files. Its answer
is capped at S, and it goes into the context as a user message with a fixed
question ("What happened earlier in this session?") and an assistant message
with the summary, which is how opencode and maki place
theirs.

Because turn summaries can always be rebuilt from the file, each such summary
is written from the turn summaries themselves rather than from the previous
summary, and so never becomes a summary of a summary. Only when the turn
summaries of turns 1 … N-2 would take more than half the window is the request
instead the previous summary plus the turn summaries that came after it.

### When compaction runs

- **Before each request inside a turn.** If P has reached the tier 1 point, the
  runner writes everything pending to the session (`flush`), compacts, appends
  the compaction record, and replaces its messages with the rewritten context:
  the inserted part built from the file, then its own in-memory messages from
  the cut onward, which keep their encrypted thinking. It sets `persistedUpTo`
  to the length of the new list, since every message in it is already on disk,
  and remembers the conversation position of its first word-for-word message so
  that the next cut is recorded in the same terms.
- **At turn start**, in `repl.RunTurn` before the turn's settings line and
  prompt are written: if the estimate has reached the tier 1 point, all earlier
  turns become turn summaries, with the summary of older turns when the result
  is above 20%. If the previous turn ended without an answer, it is part of the
  turn in progress and is compacted by the model as described above; otherwise
  no model call is made.
- **When a request is rejected for length.** Only an error saying the context
  is too long counts: an OpenAI-style `context_length_exceeded` code, the usual
  "maximum context length" and "prompt is too long" wordings, or HTTP 413.
  Today any `API error 400` is treated as overflow
  (`internal/agent/runner_llm.go:147`), which would also compact on a malformed
  schema or a bad image. Tier 1 runs at once; the compaction request is shorter
  than the rejected one by at least the tail.
- **On request.** `:compact [focus]` in `kcu` runs
  `kvit-coder --compact [--focus <text>] -s <name>` as its own process, which
  takes the session lock, rewrites the context as at turn start, summarizes
  turns 1 … N-2 regardless of the 20% threshold, and exits without running a
  turn. The focus is its own flag because the command-line parser stops at the
  first argument that is not a flag. It works with `compaction.auto: false`.

The existing 80% warning stays for sessions with `auto: false`; with auto on,
the tier 0 and tier 1 notices replace it. Each rewrite appears as one line in
the turn's output and in `--session-show`, for example
`context compacted: 953k → 61k tokens`.

A subagent runs inside the parent's turn and has no session to record a
compaction in, so it keeps today's behaviour.

### The compaction record

```go
KindCompaction Kind = "compaction"

// Fields on a compaction line:
From         int    `json:"from"`                    // position where the turn in progress began
Cut          int    `json:"cut"`                     // position of the first message kept word for word
Compacted    string `json:"compacted,omitempty"`     // the model's compacted text for the turn the cut is in
OlderSummary string `json:"older_summary,omitempty"` // one summary of the turns before the last two
OlderUpTo    int    `json:"older_up_to,omitempty"`   // position of the first message it does not cover
TokensBefore int    `json:"tokens_before,omitempty"`
TokensAfter  int    `json:"tokens_after,omitempty"`
Model        string `json:"model,omitempty"`         // the model that wrote the text
```

The compacted context uses the latest record for the cut and the compacted
text, and the latest record that has an older-turns summary for that summary;
a rewrite that did not need one leaves the previous one in force. Turn
summaries fill the gap between the older-turns summary and the turn the cut is
in.

`Load()` and `readHistory` skip the new kind, as they already skip notices and
settings, so every current reader still sees the full conversation. The
compacted context comes from a new `Session.LoadContext()`, which reads the
same file and applies the records. One function builds it from the
conversation, the turn boundaries, the notices and the records; the runner uses
the same function when it compacts inside a turn, so the context a turn
continues with and the context the next process loads are the same.

### Encrypted thinking

With the Responses API, each assistant message of the turn in progress is sent
back with its encrypted thinking blocks (`ReasoningBlocks`, replayed by
`messagesToResponsesInput` in `internal/llm/responses.go`). Neither the blocks
nor the tool-call item ids (`ToolCallItemIDs`) are written to disk, so nothing
of them is kept from one turn to the next. Within a turn they accumulate, and
`prompt_tokens` counts them, so their growth moves P like any other growth, and
tier 1 removes the blocks of every message before the cut with those messages.

Any code that removes thinking blocks from a message must also clear that
message's tool-call item ids, as `stripReplayState` in `internal/repl/repl.go`
does: OpenAI's Responses API rejects a tool-call item whose id was issued
together with a thinking item that is not sent back. This has not been checked
against the opencode endpoint, because nothing in compaction removes blocks
from a message it keeps: the messages before the cut are dropped whole, and the
tail keeps its blocks and ids together.

The agent log records the size of each response's thinking blocks (an
`encrypted thinking` line with `encrypted_bytes`). Measured with Muse Spark 1.3
on the opencode Go endpoint in October 2026, a step that calls a tool brings
back one block of about 1.4 KB, at `low` effort and at `xhigh` alike, and the
step that answers brings back more: about 11 KB at `xhigh`. A `Read` of 150 lines
adds about 2,300 to 3,000 tokens to the same requests, so the re-sent thinking
is a small part of a turn's growth, and nothing beyond compaction is done about
it.

### Failure

The history file is never changed, so no failure loses history; a failure only
leaves the compacted context with less information in it.

- **The compaction request fails** (an error, a refusal, an answer cut off at S,
  or a tool call): it is retried once. If it fails again, the record is written
  without compacted text, and turn N's messages before the cut stay in the
  context with every tool output replaced by
  `[output removed to fit the context window]`. The model's own messages and
  its tool calls are small, so this still frees most of the space.
- **The compaction request is itself too long**, because one step grew by more
  than G: it is sent with the oldest tool outputs of turn N before the cut
  replaced by the same note until it fits, oldest first.
- **The summary of older turns fails**: the turn summaries stay, and a warning
  names the cause.
- **The context is still nearly full after a rewrite**, which needs many turns
  without a successful older-turns summary or a tail larger than the window can
  hold: a warning says to start a new session, and the turn goes on until the
  endpoint refuses a request, when the existing blank-and-retry path and its
  error take over.

### Configuration

```yaml
compaction:
  auto: true                     # rewrite at the tier 1 point; :compact works either way
  thinking_cutoff_fraction: 0.8  # tier 0: stop merging thinking at this share of the window
  keep_recent_tokens: 20000      # word-for-word tail of the turn in progress (at most W/8)
  compacted_turn_tokens: 0       # S, the compacted text's budget; 0 means W/32 within 4k–32k
  step_tokens: 16000             # G's minimum (at most W/16)
  turn_headroom_ratio: 0.5       # α: keep this share of the turn's own length free
  older_turns_fraction: 0.2      # summarize turns 1 … N-2 when the rewrite is above this share
  instructions: ""               # appended to every compaction and summary request
  model: ""                      # empty: the session's own model and client
```

### Session chain fields

`Meta.Parent` and `Meta.SucceededBy` in `internal/session/session.go`, and the
loop in `Manager.Open` that follows `succeeded_by`, stay for a future feature
that forks a session into a new one. Compaction does not use them: a new
session directory per compaction would separate `proc/`, `inbox/` and `tmp/`
from the conversation, break `tail -f`, and need old links cleaned up. The
comments on them say so.

## Where it is implemented

| File | What it holds |
|---|---|
| `internal/compaction/settings.go` | The `compaction:` section resolved against a window, and the tier formula: `Margin`, `Tier1Due`, `Tier0Due` |
| `internal/compaction/compactor.go` | `Compactor.Compact`: the cut, the request that compacts the turn in progress, the summary of older turns, the fallback, the record |
| `internal/compaction/estimate.go` | Size estimates, and `StartEstimate` for the first request of a turn |
| `internal/compaction/prompts.go` | The two instructions |
| `internal/session/timeline.go` | `Timeline`: the history with turns, notices and compaction records placed by position; the rollback floor |
| `internal/session/context.go` | `BuildContext` and `LoadContext`, turn summaries, the turn in progress, `InProgressFrom` |
| `internal/session/activity.go` | The activity line |
| `internal/agent/compaction.go` | The checks before each request, compaction inside a turn and after a refusal for length, tier 0, the thinking-size log line |
| `internal/repl/compaction.go` | Compaction at turn start, and `CompactSession` for `--compact` |
| `internal/llm/overflow.go` | `IsContextOverflow` |

`cmd/kvit-coder/turn.go` builds the compactor for each turn with the turn's
client and tool specs (or the client of `compaction.model`),
`cmd/kvit-coder/main.go` has `--compact` and `--focus`, and `:compact` is in
`internal/tui/ui.go`.

### Tests

The `compaction_test.go` file in each of `internal/session`,
`internal/compaction`, `internal/agent` and `internal/repl` covers:

- `Load()` returning the same messages before and after a compaction record,
  and `--session-show` marking each rewrite between the right messages.
- The tier points of the worked examples above, the `(W − F)/2` cap, small
  windows, and no window at all.
- The estimate at turn start from a recorded count, and the fallback after a
  model switch.
- Turn summaries for a turn ending in an accepted report, a prose answer, a
  turn out of iterations and a wake turn; steering kept, agent-inserted
  messages dropped, pictures replaced, a rejected report not counted as an
  answer, the activity line.
- The turn in progress reaching back over turns without an answer, with each
  spanned prompt kept.
- The cut starting a step at least `keep_recent_tokens` from the end, and a
  rollback after a record stopping at its cut.
- The compaction request being the context up to the cut plus the
  instruction, with the turn's tools; the record rebuilding the same context the
  compactor returned; a second rewrite replacing the first compacted text;
  project instructions kept in front of a folded prompt.
- The fallback after two failed requests, unusable answers (a tool call, an
  answer cut off, an empty one) retried, and tool outputs removed from the
  request after a refusal for length without touching the caller's messages.
- Compaction at turn start by code alone after an answered turn, and by the
  model after a cancelled one.
- The summary of older turns: above the threshold, not below it, on request
  with a focus, and from the previous summary when the turn summaries are too
  large.
- In the loop: compaction before the request that would not fit, with every
  message written to the session once and the server's counts recorded; none
  without a `CompactionStart` or with `auto` off; compaction and a retry after
  a refusal for length, and not after another 400; tier 0 alone and together
  with tier 1; no second rewrite straight after the first; and a whole turn
  against a real session whose history rebuilds exactly the context the loop
  ended with.

### Checked against the live model

Three trials in October 2026 ran `kvit-coder -p` with Muse Spark 1.3 on the
opencode Go endpoint, `low` effort, and a copy of the config with the window set
to 40k, so that a turn reading six Go files 150 lines at a time would have to
compact. The first two found the problems described above (the compacted text
recording what was read rather than what it said, the estimate after a rewrite
leaving out the tool specs, and repeated rewrites from the W/2 cap). In the
third, tier 0 and tier 1 fired together at 31.1k, the context went to 17.2k
(15.8k by the server's next count), the turn finished without another rewrite,
and the compacted text listed each file's types and functions. A second turn on
the same session loaded the compacted context from the history and answered a
question about the files without reading them again; `--compact --focus` then
rewrote 18.2k to 10.3k and summarized the first turn; and `--session-show`
showed both rewrites where they happened.

## Not planned

- **A new session per compaction**, for the reasons under "Session chain
  fields".
- **Compacting in the background before it is needed**, as codex does from 65%:
  it needs a process that outlives the turn.
- **Compaction as a tool the model can call**: all five agents keep it in the
  program, and a model that can trigger it invites loops.
- **Summarizing every turn as it ends**: it pays for a model call in sessions
  that never come near the window, and the final report already serves.
- **Removing older encrypted thinking before every request**: it would change
  the previous message before each request and lose the cache for it every
  time, and tier 1 already removes the blocks before the cut.

## Appendix: what other agents do

All five converge on one shape:

> summarize head → keep tail verbatim → store a pointer overlay in the same
> session → rebuild the system prompt fresh → check before every turn,
> recover on overflow, offer manual `/compact`.

Differences are constants, storage encoding, and plumbing. Unanimous details:
structured Markdown with exact paths/commands/errors preserved, terse bullets,
"don't mention compaction"; tool-call/result pairs never split; the system
prompt is excluded from the summary; failure never loses history.

### opencode — `anomalyco/opencode` (was `sst/opencode`), tag `v1.19.4`

The most complete, two generations in parallel.

- v1: `packages/opencode/src/session/compaction.ts`
  (`select` / `turns` / `splitTurn` / `preserveRecentBudget` / `serialize` /
  `completedCompactions` / `summaryText`; `PRUNE_MINIMUM=20_000`,
  `PRUNE_PROTECT=40_000`, `TOOL_OUTPUT_MAX_CHARS=2_000`, keep
  `clamp(25% usable, 2k–15k)`).
- v2: `packages/core/src/session/compaction.ts` (`buildPrompt` / `select` /
  `serialize` / `settings` / `make->{compactIfNeeded, compactAfterOverflow}`;
  `DEFAULT_BUFFER=20_000`, `DEFAULT_KEEP_TOKENS=8_000`,
  `SUMMARY_OUTPUT_TOKENS=4_096`).
- Threshold (`session/overflow.ts`): `usable = model.limit.input - reserved`,
  `reserved = cfg.compaction.reserved ?? min(20_000, maxOutput)`; overflow iff
  `total >= usable`.
- Trigger: after every finished assistant message (`prompt.ts` runLoop);
  mid-stream `processor.ts needsCompaction` on overflow or
  `ContextOverflowError` → returns `"compact"`; manual `/compact` runs the
  same pipeline with `auto: false`.
- Separate prune pass after every loop: tool outputs older than the 2 newest
  user turns are stamped compacted past a 20k/40k shield (`skill` exempt) and
  render as `[Old tool result content cleared]`.
- Prompt: `Objective / Important Details / Work State{Completed,Active,Blocked}
  / Next Move / Relevant Files`; single user message, `tools:{}, system:[]`,
  compaction-agent model else current model.
- Storage: same session, append-only. `create()` appends user +
  `CompactionPart{auto, overflow, tail_start_id}`; `process()` appends the
  summary assistant message; `message-v2.ts filterCompacted()` reorders to
  `[compaction-user, summary, ...tail, ...new]` at read time.
- Config: `compaction:{auto, prune, reserved, preserve_recent_tokens,
  tail_turns, buffer, keep.tokens}` plus two experimental hooks.

### pi agent — `earendil-works/pi` (was `badlogic/pi-mono`), `packages/coding-agent/`

The best-documented (24KB `docs/compaction.md` plus per-regression tests); a
cleaner opencode.

- `core/compaction/compaction.ts` (`shouldCompact` / `calculateContextTokens`
  / `estimateTokens=chars/4` / `findCutPoint` / `prepareCompaction` /
  `compact` / `generateSummary`), `utils.ts` (`serializeConversation`,
  2000-char truncation), `branch-summarization.ts`, `session-manager.ts`
  (`CompactionEntry` / `BranchSummaryEntry` / `ContextEditEntry`,
  `buildSessionProjection`), `agent-session.ts` orchestration.
- Trigger: `tokens > window - reserveTokens` (default reserve 16384).
  Checked in `prepareNextTurn`, before a new prompt, and as overflow/length
  recovery (persist `context_edit` omissions first, then compact, then retry).
  Manual `/compact [instructions]` appends `Additional focus: …`;
  `enabled: false` disables auto, manual still works.
- Cut: walk back accumulating `keepRecentTokens` (default 20k) to the nearest
  valid boundary — never on a tool result. Split-turn (one user span over
  budget) merges two summaries (history + turn-prefix).
- Serialize: `[User] / [Assistant] / [Assistant tool calls] / [Tool result]`;
  thinking dropped, system rebuilt, prior summary passed iteratively via
  `<previous-summary>` (PRESERVE+ADD+UPDATE).
- Storage: JSONL with tree `id/parentId`; append
  `CompactionEntry{summary, firstKeptEntryId, tokensBefore, usage,
  details:{readFiles, modifiedFiles}}`. Raw rows stay for audit/billing, leave
  the context. Same model, `cacheRetention: none`; `length`/error/tool-call
  during summarization fails with no write.
- Config: `{compaction:{enabled, reserveTokens, keepRecentTokens,
  modelOverrides}}` plus `session_before_compact` (cancel or custom summary),
  `session_compact_failed`, `session_before_tree`.

### maki — `github.com/tontinton/maki` (Rust)

The smallest code, the only rewrite-based storage.

- `maki-agent/src/agent/compaction.rs` (`compact_history` /
  `finish_compact` / `compact(manual, carry=0)` / `steer_compaction`
  before-hook / `reserved` / `usable` / `is_overflow` /
  `prepare_collapse` / `collapse_tool_results` / `truncate_oldest_round`);
  prompts `prompts/compaction.md + compaction_user.md` via `include_str!`;
  `history.rs History::restart()`, `frame.rs fit_frame()`, `run.rs` loop,
  `maki-config AgentConfig`, `maki-storage/sessions.rs` + archive.
- Trigger: `usable = window - reserved`,
  `reserved = min(max(buffer default 20%, min_output), 50% window)`. Auto
  before a turn (Lua `agent.compact.before` may skip once); provider-too-long
  is forced; the summary itself retries 3× on overflow (collapse results,
  then drop oldest rounds). `MAKI_DISABLE_AUTOCOMPACT=1` kills auto only.
- Keep: manual carries 0; auto carries unanswered messages + newest tool
  results in a 64KiB byte budget; orphans/thinking/images stripped, old
  results → `"[tool result]"`, context-updates never survive.
- Prompt (short): system "detailed but concise"; user template
  `Goal / Instructions / Discoveries / Accomplished / Relevant
  files-directories / Todo`; no tools, 16384 out; post-message restores
  `todo_write` and nudges memory save.
- Storage: in-memory `History` rewritten to `[What did we do so far?,
  summary] + tail`; the pre-rewrite log is parked in
  `sessions/archive/<id>/<n>.jsonl` (keep 3, ≤32MB, restorable by swap-back).

### codex — `github.com/openai/codex` (Rust, Responses API)

The most machinery: three implementations plus background speculation.

- Local: `codex-rs/core/src/compact.rs` (`run_inline_auto_compact_task`,
  `run_compact_task(Manual/UserRequested/StandaloneTurn)`,
  `build_compacted_history(COMPACT_USER_MESSAGE_MAX_TOKENS=20_000)`).
- Remote V2: `compact_remote_v2.rs` (+ `_attempt.rs` background);
  token-budget no-LLM `start_new_context_window`:
  `compact_token_budget.rs`.
- Wiring `session/turn.rs`: pre-turn, mid/post-turn
  (`context_window_token_status` → `async_compaction::maybe_start` →
  `run_auto_compact(ContextLimit, …)`), plus model/comp-hash downshift path.
- Budget `session/context_window.rs`:
  `full = resolved_window * effective_percent / 100`, buffered variant adds
  `fallback_buffer_tokens`; `scope = Total | BodyAfterPrefix`.
- Background `session/async_compaction.rs`: starts at
  `async_compaction_threshold_percent = 65`, adopted only if
  `history_version` is unchanged.
- Summarized: whole history `for_prompt()`; summary = last assistant message,
  stored `SUMMARY_PREFIX + "\n" + summary`. Preserved: rebuilt initial context
  (prefix + world-state), newest user messages to 20k, named retained outputs,
  prior summaries excluded, `guardian_history / retained_context` carried.
- Prompts `codex-rs/prompts/templates/compact/`: "CONTEXT CHECKPOINT
  COMPACTION… progress/decisions/constraints/next/critical data".
- Storage: `RolloutItem::Compacted(CompactedItem{…})`, JSONL rollout + sqlite
  `state_db`, checkpoint reuse gated by `compaction_model_hash`; telemetry
  `Trigger / Reason / Implementation / Phase / Strategy / Status` + pre/post
  hooks.

### grok — two candidates

- `superagent-ai/grok-cli` (TypeScript, MIT): the small portable one and the
  best template for kvit-coder. `agent/compaction.ts`
  (`shouldCompactContext` / `estimateMessageTokens=chars/4` / `findCutPoint`
  / `prepareCompaction` / `serializeConversation` /
  `generateCompactionSummary`; `DEFAULT_RESERVE=16_384`,
  `DEFAULT_KEEP_RECENT=20_000`, `HEADER=[Context checkpoint summary]`);
  SQLite `~/.grok/grok.db` (`messages(session_id, seq, …)`,
  `compactions(first_kept_seq, summary, tokens_before)`). Pre-flight every
  turn: `tokens > window - max(maxTokens, 16384)`; overflow recovery with
  `relaxCompactionSettings(keep → max(4k, keep/2), force)`; cut never on
  `tool`; split-turn summarized in parallel and joined with `---`; 2000-char
  truncation, encrypted reasoning excluded, previous summary passed as
  `Existing summary:`; same model, `max(512, reserve*0.8)` out. Raw rows
  never deleted; `buildEffectiveTranscript: seq >= firstKeptSeq` prepends the
  summary. No user config, no `/compact` (only unrelated `/recaps`).
- `xai-org/grok-build` (Rust, official harness): heavyweight multi-crate
  (`code_compaction` full-replace, `intra_compaction`
  `StepsOnly|HistoryOnly|HistoryThenSteps|FullReplace`,
  `inter_compaction`, tool-pair-safe tail select). `IntraCompactionConfig`
  `{enabled: false, threshold: 85%, min_steps: 3, min_compactable: 5000,
  max_reduction: 0.8, model: grok-4.20}`; `/compact [text]` spliced into the
  prompt; 9-section `full_replace_summary_prompt.txt`.

### Comparison

| | opencode | pi | maki | codex | grok-cli / grok-build |
|---|---|---|---|---|---|
| auto threshold | `total >= input - reserved`, `reserved=min(20k,maxOut)`; mid-stream cut | `tokens > window - 16384`; pre-turn + pre-prompt + overflow recovery | `size >= window - reserved`, `reserved=min(max(20%,min_out),50%)` | scope/full limits; post-turn %; background 65% | `tokens > window - max(maxTokens,16384)` / 85% |
| manual | `/compact`, works with `auto:false` | `/compact [focus]` | `/compact [guidance]`, carry 0 | `/compact` + confirm | none / `/compact [text]` |
| keep tail | `clamp(25% usable, 2k–15k)` / 8k, turn-granular | 20k, never on tool-result, split-turn double summary | unanswered + 64KiB newest results | newest users to 20k + retained outputs | 20k, snap to non-`tool`, split-turn parallel |
| tool outputs in summary | truncate 2000; prune shield 20k/40k | truncate 2000 | collapse → `[tool result]` | full `for_prompt()` + truncation notice | truncate 2000 |
| system prompt | rebuilt, never summarized | rebuilt | fresh frame | rebuilt + world-state | rebuilt |
| thinking | summarized | dropped | stripped | — | excluded if encrypted |
| prior summary | fold last only | `<previous-summary>` update | — (rewrite) | excluded | excluded, passed as `Existing` |
| storage | same-session append + read-time filter | same-session append `firstKeptEntryId` + projection | rewrite + archive (3, 32MB) | replace live + rollout + checkpoint | same-session overlay / multi-crate |
| failure | — | fail with no write | retry 3× collapsing | `remove_first_item` + recompute | relax keep + force |
