# Ending a turn with a structured report

## 1. What this document is

A proposal, not yet built.

Today a kvit-coder turn ends when the model replies without asking for a tool.
Whatever prose it writes at that moment is the answer: the loop prints it under a
divider and the process exits (`internal/agent/runner_iteration.go:70`). The
prose is unconstrained, so whether the turn says what it changed, whether it
says what it ran to check the change, and whether it states the decision it is
waiting on are all left to the model's judgement on the day.

This document describes replacing that ending with a **structured report**: the
model finishes the turn by calling one tool whose arguments are JSON in a fixed
shape, the tool refuses JSON that does not fit and says exactly what is wrong,
and the turn cannot end until an acceptable report exists. The interactive front
end then draws that JSON as a card at the prompt, where one keypress answers a
decision the report raises and starts the next turn with the answer.

The design is a port. The same feature exists in another agent in this author's
tree, `~/krok`, where the tool is called `submit_report` and the design is
written up in `~/krok/structured-report.md`. This document restates the parts of
it that matter rather than assuming the reader has it open, and it departs from
the original in several places where kvit-coder's architecture makes something
easier or impossible. Section 6 lists those departures.

## 2. What a report is

### 2.1 The three fields

Every report has exactly three fields set by the model:

```json
{
  "task_status": "completed",
  "headline": "One sentence the user reads instead of the report.",
  "blocks": [ { "type": "verification", "...": "..." } ]
}
```

**`task_status`** is one of five values, and the interface renders each as a
chip at the head of the card.

| Value | Meaning | Chip |
|---|---|---|
| `completed` | The work is done and checked. | `DONE` |
| `completed_with_notes` | Done, with a caveat stated in the report. | `DONE · NOTES` |
| `needs_action` | Stopped where the user must choose or answer. | `NEEDS DECISION` |
| `blocked` | Cannot continue without the user or something outside. | `BLOCKED` |
| `failed` | The work did not succeed, with the reason recorded. | `FAILED` |

**`headline`** is one sentence, 1 to 160 characters, no line break. It is the
part most people read, so the instruction to the model is to put the material
fact there rather than a label for it.

**`blocks`** is a list of typed items, 0 to 20. Each has a `type`, a kebab-case
`id` unique within the report, a `summary` of 1 to 240 characters that states its
fact without depending on the other fields, and further fields chosen by its
type. Fewer blocks is better: several routine steps belong in one block's
`details`, not in one block each.

### 2.2 The block types

| Type | For | Required beyond `id` and `summary` |
|---|---|---|
| `change` | File edits and other work output. | — |
| `finding` | Something learned that matters: a root cause, an answer. | `impact`, `importance`, `blocks_current_task` |
| `verification` | How the work was checked: tests, builds, manual checks. | `status` (`passed`/`failed`/`partial`/`not_run`) |
| `warning` | A risk or caveat. | `severity`, `impact` |
| `decision` | A choice only the user can make. | `options`, `recommendation`, `recommendation_reason`, `response_type` |
| `question` | Information the user has to supply. | `question`, `options`, `recommendation_reason`, `response_type` |
| `blocked` | Work that cannot continue. | `blocker`, `required_action`, `options`, `recommendation`, `recommendation_reason`, `response_type` |
| `next_step` | A proposed continuation the user accepts or redirects. | `options`, `recommendation`, `recommendation_reason`, `response_type` |
| `unclassified` | Last resort when nothing else fits. | `reason_unclassified` |

Optional on most types: `impact` (one line on what it means for the user),
`recommendation`, `details` (Markdown, at most 32 KiB, hidden until opened), and
`related_files`.

The last four interactive types — `decision`, `question`, `blocked` and
`next_step` — are the ones that turn into keys on the card. Each holds 2 to 6
options, and every option declares an **effect** that says what picking it does:

- `dispatch` — the option holds a visible `instruction` of at most 1 KiB, and
  picking it sends that instruction as the next turn's prompt.
- `collect` — picking it puts an answer in the composer for you to edit and
  send, rather than starting a turn immediately.
- `resolve` — picking it records that you are stopping here, and no turn starts.

`decision` and `next_step` differ in where the default lies: use `next_step`
when there is an obvious continuation and the accept option should run it, and
`decision` when there is no default forward motion and the user has to pick.

### 2.3 An example

```json
{
  "task_status": "needs_action",
  "headline": "Drag-and-drop ordering works; one filtered-view behaviour needs a decision.",
  "blocks": [
    {
      "type": "verification",
      "id": "reorder-tests",
      "summary": "14/14 relevant tests pass and ordering survives a restart.",
      "status": "passed",
      "evidence": "go test ./internal/order (14 passed)"
    },
    {
      "type": "decision",
      "id": "filtered-reordering",
      "summary": "Choose how reordering behaves while a filter hides some rows.",
      "options": [
        {"id": "preserve", "label": "Preserve hidden positions",
         "consequence": "Hidden rows keep their slots; visible rows move around them.",
         "effect": "dispatch", "instruction": "Implement reordering while preserving hidden-row positions."},
        {"id": "recompute", "label": "Recompute the whole ordering",
         "consequence": "Hidden rows are pushed to the end.",
         "effect": "dispatch", "instruction": "Implement reordering by recomputing the whole ordering."},
        {"id": "disable", "label": "Disable dragging while filtered",
         "effect": "dispatch", "instruction": "Disable dragging while a filter is active."}
      ],
      "recommendation": "preserve",
      "recommendation_reason": "It disturbs the least unrelated state and keeps hidden rows stable.",
      "response_type": "single"
    }
  ]
}
```

### 2.4 What is checked before acceptance

The tool normalizes first — trims surrounding whitespace, strips a code fence
wrapped around a whole `details` value — and then reports every problem it finds
at once, so the model can fix them in one pass rather than one per round trip.

- `headline` is 1 to 160 characters on one line; `summary` is 1 to 240;
  `details` is at most 32 KiB per block.
- Ids match `^[a-z0-9][a-z0-9-]{0,63}$` and are unique within the report.
- `completed` and `completed_with_notes` need at least one `verification`
  block. A `not_run` verification with a stated limitation satisfies this.
- `needs_action` needs at least one `decision`, `question` or `next_step`.
- `blocked` needs at least one `blocked` block.
- `failed` needs a `finding`, `warning` or `blocked` block explaining it.
- Any report holding an interactive block must have status `needs_action` or
  `blocked`.
- An interactive block needs 2 to 6 options with unique ids, an `effect` on
  each, an `instruction` on every `dispatch` option and on no other, and a
  non-empty `recommendation_reason`. A `next_step` needs at least one
  `dispatch` accept option and at least one `collect` or `resolve` alternative.
- A `verification` with status `partial` or `not_run` must state what was not
  verified and why.

## 3. How a turn produces one

### 3.1 The tool

One new tool, `Report`, registered the way every other tool is: its own file
plus a line in `internal/tools/setup.go`, a config field, and a case in
`Config.IsToolEnabled` (`internal/config/config.go:966`). Its documentation for
the system prompt goes in `internal/prompt/prompts/tools/report.tmpl` and is
named by `PromptTemplateName()`.

The tool spec is large — the nine block types are perhaps two thousand tokens of
schema — and it sits in the cached prompt prefix, so it is paid for once per
conversation rather than once per turn. The prefix must stay byte-identical
between requests for that cache to keep hitting, which registering a tool the
ordinary way already respects.

### 3.2 Rejection and repair

kvit-coder already has the mechanism for a tool that refuses its arguments and
explains why. `tools.SemanticErrorf` returns text to the model as the tool's
result, and the model tries again. A rejected report comes back as a list of
problems, each naming the field it is about; the model repairs and resubmits.
A repair budget of three attempts per turn lives as a counter on `ToolContext`,
after which the model is told to stop repairing and end the turn, and the turn
records that no report was produced.

### 3.3 The gate

Two halves, both small.

**Ending the turn on acceptance.** The loop currently ends only when the model
returns a message with no tool calls (`internal/agent/runner.go:359`). An
accepted report must also end it, which is a field on `toolExecutionResult`
(`runner.go:201`) beside the existing `tasksToolExecuted`, and a break where the
loop reads that struct.

**Requiring one before the turn ends.** `handleFinalAnswer`
(`internal/agent/runner_iteration.go:17`) decides what happens when the model
stops asking for tools, and it already has two branches that inject a user
message and return `shouldContinue = true` — one for a malformed tool call, one
for an empty response. A third branch saying "this turn has not submitted a
report; call Report now" is the same shape in the same function.

When a report is required is a configuration value with four settings, defaulting
to `mutating`:

```yaml
tools:
  report:
    enabled: true
    mode: mutating        # off | mutating | tools | always
    max_repair_attempts: 3
    max_blocks: 20
```

`mutating` means a turn that edited a file, ran a command or started a process
must report; a turn that only read and answered need not.

### 3.4 Where the report is stored

Nowhere new. The model submits the report as the arguments of a `Report` tool
call, so it is already in `history.jsonl` as part of the assistant message that
the loop persists as it goes (`internal/agent/runner.go:344`). Adding a separate
report event would put the same JSON in the file twice, with two copies free to
drift apart.

## 4. How the interface shows it

### 4.1 Why the card lives in the front end

`kvit-coder-ui` does not link the agent. It spawns `kvit-coder` as a child
process per turn with the child's standard output wired straight to the terminal
(`internal/tui/ui.go:721`), so while a turn runs the front end is a spectator
and cannot draw anything.

That constraint ends when the turn does. `cmd.Run()` returns, the front end is
back at the top of its loop (`internal/tui/ui.go:147`), and it owns the terminal
until it starts the next turn. It also holds `sessionMgr` and `currentSession`,
so it can open the session directory and read what the turn left there. The
report is a between-turns object, and between turns is exactly when the front
end is in charge.

No new channel between the two processes is needed. The file is the channel,
which is how this program already moves everything that outlives a turn.

### 4.2 Finding the report

One function in `internal/session` scans history backwards for the most recent
assistant message holding a `Report` tool call, and returns its raw arguments
along with whether that message is the last message in the conversation. Three
callers use it, so the card, the `:report` command and the inbox hold in
section 5 cannot disagree about what is showing — the same principle
`wakePoll` follows for the inbox count (`internal/tui/wake.go:50`).

**The card shows when the report is the last message.** Nothing is tracked
across turns, because every case resolves itself:

- A cancelled turn appends its own messages and a `cancelled by user` notice
  (`internal/repl/repl.go:169`), so an earlier report is no longer last and no
  card shows.
- The iteration budget running out does the same at line 166.
- Answering a report starts a turn that appends the prompt and everything after
  it, so the card goes away with no resolved flag to maintain.
- A new report is the last message, so it replaces the old one by being there.

"Last message" is not "last line". Notices and settings events hold no message,
and `Session.Load` already skips them when it rebuilds the conversation
(`internal/session/session.go:379`); the reader filters the same way. This
matters in one real case: a turn that leaves a background process running
appends a notice about it after the report (`internal/repl/repl.go:214`), and a
reader that stopped at the last line would miss the card there.

### 4.3 The card and its keys

The composer is rebuilt for every prompt — `readInput` constructs a fresh
`ui.NewInputModel` and configures it with `SetWakePoll`, `SetStagedImages` and
`SetCompletionBaseDir` (`internal/tui/ui.go:210-245`) — so `SetReport` is the
same shape as three things already there.

The key handling has a close precedent too. `InputModel.Update` already supports
a transient list that owns up/down and esc while the textarea keeps every other
key (`internal/ui/input.go:723-740`, `completionListActive()`). The card is a
second mode of that kind:

| Key | Effect |
|---|---|
| `1`–`9` | Pick that option of the first unresolved interactive block |
| `↑`/`↓` | Move between blocks |
| `d` | Toggle the highlighted block's details |
| `esc` | Dismiss the card for this composer |
| anything else | Goes to the textarea, as now |

### 4.4 What answering does

Each effect from section 2.2 maps onto something the composer already does.
`dispatch` submits its instruction as the next turn's prompt. `collect` seeds
the textarea with a labelled answer so it can be edited before sending, the way
a staged image seeds the list today. `resolve` closes the card without starting
a turn.

Nothing beyond the prompt text has to reach the model. The report is in the
conversation as the tool call the model made, so the next prompt only has to
name the block it is answering.

### 4.5 Getting the card back

`esc` hides the card, which is needed as an escape hatch from the inbox hold in
section 5. The dismissal is a bool inside the running composer, not state kept
between turns, so it lasts only as long as that composer is open.

`:report` brings it back, under the same condition as the automatic card: the
report has to be the last message. Once the conversation has moved past it —
an answer was sent, or a later turn pushed it out of last place — `:report`
prints `No report to show.` and does nothing else. Every caller of the reader
in section 4.2 therefore asks the same question, and there is no second notion
of which report is current.

Its implementation is almost nothing, because a `:` command already ends the
composer: the loop opens a fresh one, the fresh one has no dismissal flag, and
the card reappears on its own. What `:report` adds is a name for that with a
line in `showHelp` (`internal/tui/ui.go:617`), so someone who has just pressed
esc can find the way back, and a message for the case where there is nothing to
show.

## 5. The inbox wake, and why it needs a hold

Background work reaches a session through its inbox: a process exiting, a file
dropped by `kvit-coder steer`. Between turns the front end polls for those and,
finding any, starts a turn with the inbox as the prompt. It does this in two
places, both of which test "the composer text is empty" as a stand-in for "the
user is idle and nothing is in flight":

- **`internal/tui/ui.go:152`**, at the top of the loop, before the composer
  opens at all. After a turn that produced a report, a non-empty inbox starts an
  inbox-only turn and the card is never drawn.
- **`internal/ui/input.go:699`**, in the composer's tick. A card is on screen, a
  build exits, and the composer submits itself out from under the decision.

An unanswered report means something is in flight, so the test becomes empty
text *and* no report awaiting an answer, in both places, from the one reader.

Holding back costs nothing to build, because the badge path exists already:
`input.go:987` renders `[inbox: N pending — submit to include]` when the wake is
held for a user who is typing. The wording stays true here, since picking an
option starts a turn and the turn drains the inbox at its first iteration. The
background events arrive together with the answer instead of in place of it.

**Only interactive reports hold.** A `completed` report with nothing to answer
should not make a finished build wait behind a card that only needs reading, and
bubbletea leaves the last rendered view in the scrollback, so the card stays
legible after the composer quits.

**The tradeoff to accept deliberately** is that an unanswered decision parks
background events in the inbox indefinitely. They are files, so nothing is lost,
but a session left overnight on a card wakes up with a queue. `esc` lifts the
hold for anyone who wants the queue drained first.

## 6. What is not being ported

krok renders its reports inside a long-running program that owns its own
scrollback, and several features follow from that rather than from the report
itself:

- **The report dock and composer takeover.** Between-turns rendering here is a
  card above the prompt, not a pane with its own layout.
- **Dashboard peek.** There is no dashboard.
- **Hooks.** krok notifies a `report_submitted` hook and puts the accepted
  report in its `Stop` and `SubagentStop` payloads. kvit-coder has no hook
  system.
- **Subagent reports.** There are no subagents.
- **Persisted answer batches.** krok records answers as durable
  `report_actions_resolved` updates so several can be validated and applied
  together, and rebuilds staged answers after a restart. Here the front end is
  the thing that exists between turns and holds staged answers in memory until
  they are sent as one prompt. The cost is that answers staged when the front
  end is killed are lost.
- **Schema version 1.0 compatibility and replay.** There is no stored history
  to stay compatible with.

## 7. Work list

| Piece | Where | Lines |
|---|---|---|
| Types and JSON schema | `internal/report/report.go` | ~350 |
| Validation and the problem list | `internal/report/validate.go` | ~350 |
| Plain-text rendering | `internal/report/render.go` | ~250 |
| The tool: parse, validate, accept | `internal/tools/report.go` | ~200 |
| Reader for the most recent report | `internal/session/report.go` | ~50 |
| End the turn on acceptance; require one before the final answer | `internal/agent/runner.go`, `runner_iteration.go` | ~60 |
| Card mode, keys, the wake hold | `internal/ui/input.go` | ~250 |
| Wiring, `:report`, help, the boundary hold | `internal/tui/ui.go` | ~80 |
| Config, registration, prompt template | | ~120 |
| Tests | | 700–900 |

About 1,700 lines of non-test Go. The same feature in krok is 5,400 lines of
Rust in the tool plus roughly 5,900 more for its session gate and its interface;
Go's struct tags and `encoding/json` absorb most of what its 1,767-line
`types.rs` spends on shapes, and the between-turns card is a smaller thing to
build than a live dock.

**Put the types in their own package.** The front-end binary must not import
`internal/tools`. `internal/report` holds the types, the validator and the
renderer, and is imported by the tool, by the session reader and by the
composer.

**Build order.** First the schema with four block types — `change`,
`verification`, `finding`, `next_step` — the gate, and the plain-text rendering
where `writer.Assistant` prints the final answer today. That is roughly 500
lines, it establishes the vocabulary and the enforcement, and the remaining five
types are additive. The card and its keys come second, and the headless paths
(`-p`, `-pq`, the JSON document from `WriteJSONOutput` at
`internal/ui/writer.go:216`) third.

**Draw the card once.** The agent already knows when it is running under the
front end: `session.FromUIVar` is set so the child skips the banner and prompt
echo the front end has printed (`internal/tui/ui.go:727`). The same flag
suppresses the agent's own rendering of the report, so under the front end the
card is drawn once, by the front end, and a headless run prints it itself.

## 8. Decisions still open

**The verification requirement misfires on turns that only answer a question.**
krok's own write-up records this: `completed` demands a `verification` block, so
a turn that explained something and changed nothing puts the explanation in a
verification block's evidence field, where it does not belong. Make the
requirement follow what the turn did — a turn that changed files needs a
verification block, a turn that only read is satisfied by a `finding` — rather
than porting the rule as written.

**Whether the model emits this schema reliably.** Nine block types with
per-type required fields is a large strict-JSON demand. kvit-coder does have
`argument_normalizer.go` and `toolcall_normalizer.go` for repairing malformed
model JSON, but those run under the `weak` agent profile and the default is
`strong`, so they are off. Send the schema at the configured endpoint for a
handful of representative turns before committing to all nine types.
