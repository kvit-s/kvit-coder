# Steer via Enter-to-pause

Status: implemented. Raw mode was explicitly rejected — it caused too many
issues in this app, and the turn stays in cooked mode. This doc specifies the
alternative that shipped: `Enter` pauses at the next steerable moment, you
type, `Enter` resumes.

## 1. Problem

While a turn runs, anything typed on the same terminal reaches the model at its
next iteration as `<user-steering>`. Today that input path is
`startStdinReader` in `cmd/kvit-coder/main.go:714`: a `bufio.Scanner(os.Stdin)`
loop pushing each non-empty line into `internal/inbox.Inbox`:

```go
for scanner.Scan() {
    line := strings.TrimSpace(scanner.Text())
    if line == "" {
        continue
    }
    answering := steering.Awaiting()
    steering.Push(inbox.Message{Kind: inbox.KindUserLine, Text: line})
    if !answering {
        writer.Info("→ queued")
    }
}
```

The terminal stays in cooked/canonical mode, so the kernel echoes keystrokes
and the process sees nothing until `Enter`. Agent output
(`internal/ui/writer.go`: `Thinking`, `ToolCall`, `Info`, `Warn`, `Error`, plus
`\r`-redrawn `ToolProgress` dots) writes to the same terminal concurrently.
Kernel echo and agent `stderr` interleave, so the half-typed line is garbled —
e.g. typed text swallowed between tool-call lines, only `→ queued` /
`steering: ...` afterwards proving what was sent.

True output buffering ("pause output while I type") is impossible in cooked
mode: "while I type" is unobservable, no event arrives between first keystroke
and `Enter`. And a single `Escape` keypress can't be the trigger either — in
cooked mode the `0x1b` byte sits in the kernel buffer until newline, so the
process never sees it.

What *is* observable in cooked mode is a complete line — including an empty
one. Today empty lines are an explicit no-op (`if line == "" { continue }`).
This proposal repurposes them: empty `Enter` means "I want to steer".

## 2. Decision

- No raw mode, no new dep, no TTY takeover, no restore-on-panic audit. The
  turn stays cooked; when output is stopped, cooked echo is perfectly clean.
- Trigger is `Enter` on an empty line. It wakes `Scanner.Scan()` immediately,
  works through `internal/tui/ui.go:runAgent` stdio passthrough untouched, and
  exists on every keyboard including Windows. Rejected alternative `Ctrl-\` /
  `SIGQUIT` would need parent-forwarding (the parent currently only
  catch-and-holds `SIGINT`, `ui.go:333-340`), carries the core-dump default on
  any missed path, and is undiscoverable ("hit `Enter` to pause and steer" vs
  "send SIGQUIT").
- Pause is modal: pause first at a clean boundary, then type. Type-ahead while
  output flows and second-terminal `kvit-coder steer` (`cmd/kvit-coder/steer.go`)
  stay as-is for those who want them.

## 3. UX

```
● (10.8k 1%) Read[limit=100, path="cmd/kvit-coder/steer.go"]
  Search[pattern="steer"]
                                        <- you hit Enter here
  [info] pause requested — will pause at next steerable point
  ...current tool/LLM call finishes, no new output after this line...
  paused — type steering, Enter to resume (empty resumes):
```

You type `actually check the tests first`, hit `Enter`, and the turn continues
immediately — no extra keystroke:

```
  steering: actually check the tests first
● (11.2k 1%) ...
```

- Double `Enter` while the request is pending is a no-op (no second notice).
- `Enter` on the empty pause prompt resumes with no steering.
- `Ctrl-C` during the pause prompt cancels the turn exactly as during a
  `Question` (dismissal semantics, turn saved).
- Pause requested while a `Question`/permission prompt is pending never
  registers — your line just answers the question (see §5 table).

## 4. Detailed design

### 4.1 `internal/inbox.Inbox`: own the pause state

Next to `asking` (`inbox.go:63-66`, with `Awaiting()`), add under the same
`mu`:

- `pauseRequested bool` — an empty `Enter` set it, the loop consumes it.
- `pauseMode bool` — true only while the pause prompt's `Ask` is blocked.

```go
func (i *Inbox) RequestPause() bool  // set flag, report whether newly set
func (i *Inbox) TakePause() bool     // clear flag, report whether it was set
func (i *Inbox) SetPauseMode(on bool)
func (i *Inbox) PauseMode() bool
```

One mutex for reader goroutine and `Runner.Run` — no new atomic in `main`.
`RequestPause`/`TakePause`/`PauseMode` follow the `Awaiting()` locking pattern.

### 4.2 Reader: `startStdinReader` (`cmd/kvit-coder/main.go:714`)

Replace the `if line == "" { continue }` with the empty-line routing:

```go
line := strings.TrimSpace(scanner.Text())
if line == "" {
    switch {
    case steering.PauseMode():
        // The pause prompt is up: empty is a valid answer (resume).
        // Ask returns "" / AskAnswered for it (it only checks Kind
        // and timestamp, not text — inbox.go:310,316).
        steering.Push(inbox.Message{Kind: inbox.KindUserLine, Text: ""})
    case steering.Awaiting():
        // A Question/permission prompt is up: keep today's behavior —
        // empty is ignored, otherwise "" would become a FreeText:""
        // answer via question.go:fill (currently impossible).
        continue
    default:
        if steering.RequestPause() {
            writer.Info("pause requested — will pause at next steerable point")
        }
    }
    continue
}
// non-empty path unchanged: Push + "→ queued" iff !Awaiting().
// While PauseMode is up, Awaiting() is true (Ask holds `asking`), so a
// steering answer is consumed by Ask and never prints "→ queued" — correct.
```

Notes:

- Pushing `Text: ""` for resume is safe downstream: `Runner.drainInbox`
  (`internal/agent/steering.go:39`) skips whitespace-only texts, and `parse`
  rejects them for inbox files — empty never reaches the model.
- The reader keeps exactly one `Scanner`; no second reader of stdin appears,
  preserving the one-reader invariant (`inbox.go:265-268`).

### 4.3 Loop: check at the iteration boundary (`internal/agent/runner.go:268`)

At the top of the iteration, before `pollProcesses()` / `drainInbox(state)`
(~line 284) — the honest "next steerable moment": the current LLM/tool call
finishes first; we never interrupt one to pause sooner:

```go
for i := 0; i < maxIters; i++ {
    ...
    if r.checkPause(ctx, state) { // returns true if the turn was cancelled
        result.Cancelled = true
        ...
        break
    }
    r.pollProcesses()
    r.drainInbox(state)
    ...
}
```

`checkPause` (new, in `internal/agent/steering.go` or a small new file):

```go
func (r *Runner) checkPause(ctx context.Context, state *runState) bool {
    if r.inbox == nil || !r.inbox.TakePause() {
        return false
    }
    r.inbox.SetPauseMode(true)
    defer r.inbox.SetPauseMode(false)
    r.writer.Info("paused — type steering, Enter to resume (empty resumes):")
    text, outcome := r.inbox.Ask(ctx, nil, "", 0) // prompt already printed
    switch outcome {
    case inbox.AskCancelled:
        return true // runCtx done (Ctrl-C): caller ends the turn as cancelled
    case inbox.AskAnswered:
        if strings.TrimSpace(text) != "" {
            // Re-push: Ask consumed the line; drainInbox below must see it
            // through the existing <user-steering> path (steering.go:46-50).
            r.inbox.Push(inbox.Message{Kind: inbox.KindUserLine, Text: text})
        }
        return false
    default: // AskTimedOut cannot happen with timeout 0; treat as resume
        return false
    }
}
```

Why `Ask` with `nil` out: the prompt is printed via `writer` so it follows the
headless→stderr / quiet-suppressed routing; `Ask`'s `out` param is only the
raw `fmt.Fprint`. Why timeout 0: block indefinitely, like `Question` with a
person present (`question.go:232-235`) — a pause that times out while you are
thinking is worse than waiting.

Prompt-wait accounting: wrap the `Ask` with `started := time.Now()` +
`toolCtx.AddPromptWait(...)`, mirroring `main.go`'s `ask` closure, so pause
time is not charged against `blanketToolTimeout` (same rationale as
`runner.go:209-211`).

Gating: only reachable when the stdin reader runs (same `stdinIsATerminal`
gate that sets `toolCtx.SetInteractive(true)` in `main.go`). Piped/headless/
benchmark/quiet/`--json` runs never set `pauseRequested` (empty lines never
arrive; `steer` files aren't empty), so `TakePause()` is always false there —
no Runner flag needed. `r.toolCtx` interactive state can be consulted as a
belt-and-braces skip.

`Ctrl-C` path: `Ask` returns `AskCancelled` when `runCtx` is done; the caller
sets `result.Cancelled`, `defer r.flush(state)` persists, and the existing
interrupt messaging applies. No new exit code.

### 4.4 Why this composes with `Question`

- Pause prompt and `Question` both go through `inbox.Ask`: `Awaiting()` is
  true in both, so `ToolProgress` dots already hold off via the existing
  `awaitingAnswer` suppression (`writer.go: ToolProgress` +
  `SetPromptWatcher`) — no Writer change.
- Stale-line rule unchanged (`inbox.go:270-271,287,310-316`): lines typed
  before the pause prompt was drawn are drained into `notForUs` and pushed
  back, surfacing as ordinary steering at the same-iteration `drainInbox`.
  Now visible rather than surprising.
- No nesting: while a `Question` is pending, empty lines never set the flag
  (§5 row 3); while the pause prompt is up, no tool runs, so no `Question`
  can start. At most one prompt is ever blocked in `Ask`.

## 5. Empty-line truth table (reader)

| State when empty `Enter` arrives | Action |
|---|---|
| Pause prompt up (`PauseMode()==true`) | Push empty `KindUserLine` → `Ask` returns `""`/answered → resume |
| `Question`/permission prompt up (`Awaiting()==true`, not pause) | Ignore (today's behavior; `""` must never become a `Question` answer) |
| Normal turn, no flag yet | `RequestPause()` + `pause requested — will pause at next steerable point` |
| Normal turn, flag already set | No-op (no repeat notice) |
| No terminal / piped / benchmark / `--json` | Unreachable (no reader); flag never set |

Non-empty lines are unchanged in every state (`Push` + `→ queued` iff
`!Awaiting()`).

## 6. What stays as-is

- Cooked mode, single `Scanner` reader, one-reader invariant.
- Inbox file protocol (`Deliver`, `parse`, 64 KiB cap, `maxQueued = 100`),
  `kvit-coder steer` CLI, session `inbox/` layout, `drainInbox` tagging
  (`<user-steering>`, `steering.go:46-50`) and the system-prompt text about it
  (`internal/prompt/prompt.go:610`).
- `Question` protocol, stale-line rule, `Awaiting()` semantics, `→ queued`
  echo, `Ctrl-C` cancel semantics (`installInterruptHandler`), `KVIT_RUN_ID`
  pinning, deterministic tool-spec ordering, path containment via
  `tools.NormalizeAndValidatePath` (see `CLAUDE.md`).
- `internal/tui/ui.go:runAgent`: stdio passthrough unchanged; no new signal
  to forward (the `SIGINT` catch-and-hold block needs no companion).
- `Writer`: no funnel, no mutex, no new method required (prompt via existing
  `Info`; dots already suppress under `Awaiting()`).

## 7. Fallback / gating matrix

| Condition | Behavior |
|---|---|
| stdin not a char device, or `/dev/null`, or stderr piped | No reader (today's `stdinIsATerminal` gate); pause unreachable |
| `--json`, quiet (`-pq`), benchmark/haystack/thinkbench | Same — no reader, no prompt; `TakePause()` always false |
| `TERM=dumb` | Reader still runs (as today); pause works, prompt is plain text — acceptable, no special case |
| `kvit-coder steer` from a second terminal | Works in both modes (inbox-dir delivery); unaffected by pause flag |
| Pause requested, then `Ctrl-C` before boundary | Turn cancels as today; flag dies with the process — nothing to clear |
| Pause prompt up, second terminal delivers `steer` file | `Ask`'s poll drain (`inbox.go:301-304`) can return it as the answer — acceptable: it *is* steering typed during the pause |

## 8. Testing

- `go test ./...`, `go vet ./...` green (per `CLAUDE.md`).
- `internal/inbox`: `RequestPause`/`TakePause` set-then-take, take-clears,
  double-request returns false the second time, `PauseMode` toggle —
  table-driven, no pty.
- `internal/agent` (scripted `LLMClient`, existing `runner_test.go` pattern):
  pre-set pause flag → next `Run` iteration consumes it and `Ask` answer
  appears as `<user-steering>` in `FinalMessages`; empty answer → resume with
  no message; `AskCancelled` → `result.Cancelled`; flag set mid-run via
  `Push`+`RequestPause` from the test goroutine.
- Reader-level test for the §5 table: fake `Inbox` + piped stdin driving the
  `startStdinReader` body (extract the line-handling closure so the test does
  not need a terminal): empty in each state → the mapped action; non-empty
  while `PauseMode` → pushed, no `→ queued`.
- Manual matrix (real terminal, both `./kvit-coder -p` and via
  `./kvit-coder-ui`): `Enter` during streaming output, during `ToolProgress`
  dots, during a `Question` (answers it, no pause notice), double-`Enter`
  (one notice), pause then type steering (appears as `steering: ...` next
  iteration), pause then empty (resumes silently), `Ctrl-C` at pause prompt
  (turn saved, terminal sane), paste a paragraph at pause prompt (single
  steering message per line — document), `Enter` with piped stdin (no-op).

## 9. Risks

- Discoverability: an empty `Enter` doing something is new. Mitigate with the
  notice text on first use per turn and a `README.md` steering-section line
  ("hit `Enter` to pause and steer"). Accidental cost is one extra `Enter`.
- Pause latency: the current LLM/tool call always finishes first; a long
  `Shell` run or model call delays the prompt with only the notice to show.
  Honest limitation — state it in the notice ("at next steerable point"), do
  not add iteration-interrupt machinery in v1.
- Prompt-wait clock: forgetting `AddPromptWait` around the pause `Ask`
  misattributes thinking time to the next tool's `blanketToolTimeout`. The
  spec puts it in `checkPause`; the test should assert it.
- `Ask` with `nil` out: verify the existing `if out != nil && prompt != ""`
  guard (`inbox.go:290`) — yes, nil-safe today.
- Scope creep back to raw mode (per-key pause key, live buffer): explicitly
  out. Any "pause key" other than `Enter` reintroduces cooked-mode
  invisibility or raw-mode ownership — point at §1.

## 10. Build slices

1. `inbox`: `pauseRequested`/`pauseMode` + methods + unit tests. No behavior
   change (nothing calls them yet).
2. `main`: reader empty-line routing (§4.2) + extraction for testability.
   Reader-only; loop ignores the flag until slice 3, so the notice prints but
   no pause happens — flag behind a temporary no-op or land 2+3 together.
   Prefer landing 2+3 together.
3. `agent`: boundary check + `checkPause` + prompt-wait accounting + runner
   tests. End-to-end pause works headless.
4. Docs: `README.md` steering section (`Enter` to pause), `docs/redesign.md`
   rough-edge note (§5 "untidy until raw mode") marked superseded-by-pause.
   Commit on `main`, lowercase area-prefixed (`inbox: ...`, `agent: ...`).

Slices 1–3 keep `scripts/build.sh`, `go test ./...`, `go vet ./...` green.

## 11. Open questions

- Notice wording: `pause requested — will pause at next steerable point` vs
  shorter `pausing at next step…`? Pick one; the test asserts it.
- Should the pause prompt echo the pending queue depth (e.g. "2 line(s)
  already queued, type more or Enter to resume")? Cheap (`len` under `mu` —
  needs an accessor) and removes the "did my earlier type-ahead survive?"
  doubt. Recommend yes if it stays a read-only accessor.
- Paste-at-pause yields one steering message per line (existing `Scanner`
  behavior). Acceptable v1, or join burst lines? Document the choice.

## References

- `cmd/kvit-coder/main.go: startStdinReader`, `stdinIsATerminal`,
  `installInterruptHandler`, `ask` closure with `AddPromptWait`;
  `cmd/kvit-coder/steer.go`
- `internal/inbox/inbox.go` (`Push`, `Drain`, `Ask`, `Awaiting`, `asking`);
  `internal/agent/runner.go:268` loop with `pollProcesses`/`drainInbox`;
  `internal/agent/steering.go:28-62` (`drainInbox`, `<user-steering>`,
  whitespace skip)
- `internal/tools/question.go:ask`/`fill` (indefinite wait when interactive,
  empty-never-an-answer today); `internal/tools/context.go:47-51,98-`
  (`interactive` gate, `AddPromptWait`)
- `internal/ui/writer.go` (`ToolProgress` + `SetPromptWatcher` dot hold);
  `internal/tui/ui.go:runAgent` (passthrough, `SIGINT` hold — no change needed)
- `internal/prompt/prompt.go:610` (steering text); `docs/redesign.md:303-308`
  (raw-mode rough edge, superseded by this)
