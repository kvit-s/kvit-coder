# Structured reports for Claude Code

## 1. What this document is

How to get what `docs/structured-reports.md` describes — a turn that ends in
a validated, machine-shaped report with answerable next steps — when the
agent is Claude Code instead of kvit-coder, and whether wrapping Claude Code
in a custom UI over ACP is the way to display it.

Short answer: do not wrap yet, and do not invent a JSON fence. Your current
Stop-hook style gate is already the enforcement point — keep it, but change
what it enforces. Claude Code already ships the report card: it is the
`AskUserQuestion` tool. Teach it as the report format (output style), validate
its use (Stop hook), and you get native buttons in terminal, IDE, desktop and
cloud with no custom UI and no second format to keep in sync.

ACP is the **Agent Client Protocol** (agentclientprotocol.com), not "agent
common protocol". Think LSP but for coding agents: the editor (client)
renders chat, diffs and permission prompts; the agent runs the inference
loop and calls tools; the two talk JSON-RPC over stdio (local) or HTTP/WS
(remote, still maturing). If you wrap Claude Code behind ACP, *you* become
the client and therefore own rendering — that is the appeal and the cost.

## 2. Why a structured report beats a style check

Your current loop — Stop hook reads the final answer, checks writing style,
blocks with "rewrite in the right voice" — has three weaknesses a report
fixes:

1. **Style is unverifiable.** "Wrong voice" is a judgement call the model
   re-litigates every iteration. "Every `needs_action` turn ends with an
   `AskUserQuestion` carrying 2–4 labelled options" is checkable by a
   50-line script. Deterministic checks converge; taste checks loop.
2. **Prose buries the decision.** The thing you act on (do X / answer Y /
   stopped because Z) sits inside paragraphs. A report puts it in a tool
   call with buttons, each labelled with what picking it does.
3. **Prose does not compose.** A transcript holding a well-formed
   `AskUserQuestion` (questions + answers) can feed a changelog, a task
   board, a `/report` log, or a second agent. A well-written paragraph
   cannot, without parsing.

Keep the enforcement point (the Stop hook). Change the contract it enforces.

## 3. What Claude Code gives you, and what it does not

| kvit-coder piece | Claude Code equivalent | Notes |
|---|---|---|
| `Report` tool + validator (`internal/report/`, `internal/tools/report.go`) | **`AskUserQuestion` + Stop-hook validator** (your script) + **output style** (usage rules) | No custom tools in Claude Code. The tool already exists and already renders — the hook validates its use, the style file teaches when to reach for it. No invented JSON. |
| Gate: refuse to end the turn without a report (`runner_iteration.go`) | **Stop hook `decision: "block"` + `reason`** | Same shape: hook returns JSON, model continues. Hard cap is 8 consecutive blocks, then Claude Code overrides you — budget accordingly (section 4.3). |
| Card with digit keys (`internal/ui/report_card.go`) | **`AskUserQuestion` option buttons** | Native card, no code. Renders everywhere; answers resume the same turn instead of starting the next one (section 4.4). |
| Report stored once in `history.jsonl` | Transcript JSONL at `transcript_path` (tool call + `answers`, not `last_assistant_message`) | `last_assistant_message` is text-only — it never contains the `AskUserQuestion` call. The validator must read `transcript_path`. Do not write a second copy. |
| `:report` to bring the card back | `/report` custom slash command (skill) that reads the transcript | Cheap to build, worth building. |
| Inbox hold (section 5 of the main doc) | Nothing to do — no separate front-end process | Claude Code owns its loop; background-task info already arrives in Stop input as `background_tasks` / `session_crons`. |

Two fields in Stop input exist for exactly your use case: `stop_hook_active`
(true when you are already continuing because of a Stop hook — check it or
read the transcript, or you loop on a condition that never resolves) and
`last_assistant_message` (the final *text* — useful for the `completed`
prose check, useless for the `AskUserQuestion` check). The `/goal` command
is a built-in session-scoped prompt-based Stop hook — useful precedent for
"keep working toward a condition".

## 4. How a turn produces one (recommended setup)

Two files, no new processes. This is an evolution of your current hook,
not a rewrite. The core rule, stated once:

> **The `AskUserQuestion` call IS the report. There is no JSON fence to
> emit first and convert afterwards.**

The old draft of this document taught a ` ```json` fence
(`task_status`/`headline`/`blocks`) and then told the model to restate the
options as `AskUserQuestion`. That is two formats for one fact: twice the
tokens, twice the failure modes (fence valid but tool missing, tool present
but fence stale), and a validator that checks the copy instead of the thing
the user clicks. Delete the fence. The preceding assistant text (headline +
what changed + how it was verified) plus one `AskUserQuestion` call carries
everything kvit's schema carries, in the widget the user already answers.

### 4.1 The usage rules live in an output style

Create `~/.claude/output-styles/report.md` (user level) or
`.claude/output-styles/report.md` (project level). It teaches *when* to ask
and *how the text before the ask reads* — not a schema:

```markdown
---
name: Structured report
description: End mutating turns with headline + verification, and ask via AskUserQuestion
keep-coding-instructions: true
---

## How a turn ends

A turn that edited files, ran commands, or started processes ends with
prose plus an AskUserQuestion call, by default. A turn that only read
and answered ends in prose as usual.

**Default: propose what's next.** Write short prose first: one headline
sentence with the material fact, bullets for what changed, one
`Tests:` / `Checks:` line saying how it was verified. Then call
AskUserQuestion with concrete continuations. Never list options in
prose without calling the tool — the call is what renders the buttons.

Assume there IS a next step. After a working turn the user almost
always says "what's next" or "continue", so a flat "done" just costs a
round trip. Always offer 2-3 real continuations (commit, add tests,
changelog, next subtask, review diff...) plus an explicit last option
such as "Stop here". You can ask up to 4 questions per call; put each
follow-up topic in its own question object. Each question: 2-4 options,
a Header of max 12 chars, multiSelect only for genuinely multi-pick
questions. Mark your recommendation by suffixing its label with
" (Recommended)" and giving the why in its description. Put the
consequence of each option in its description. Free text ("Other")
always stays available, so you never need a dedicated "custom" option.

**Rare exception: truly done, nothing to propose.** Only when the user
explicitly said "just do X and stop", or no plausible follow-up exists.
Then the last message is short prose with no question call. Treat this
as under 10% of mutating turns. When in doubt, ask.
```

Why an output style and not CLAUDE.md: styles are switchable per session
(`/output-style report`), apply to the main conversation every request, and
signal "this changes the response format", which is exactly their documented
purpose. Project conventions stay in CLAUDE.md.

### 4.2 Examples: the ask call for each report kind

`AskUserQuestion` input shape (the only shape the model must learn):

```json
{
  "questions": [
    {
      "question": "Full question text shown to the user.",
      "header": "Short",
      "multiSelect": false,
      "options": [
        {"label": "Pick me", "description": "What this does, trade-offs."}
      ]
    }
  ]
}
```

Constraints to teach (validator enforces them): `header` ≤ 12 chars,
2–4 options per question, max ~4 questions per call. Answers come back as
`{ "<question text>": "<label>" }` (array or `", "`-joined for multiSelect),
plus free text when the user types "Other" instead of picking.

**Decision (kvit `decision` block).** Preceding text gives headline +
verification; the call carries the choice. Recommendation is encoded, not
a field:

```json
{
  "questions": [
    {
      "question": "Reordering works; one filtered-view behaviour needs a decision. How should reordering behave while a filter hides some rows?",
      "header": "Filter",
      "multiSelect": false,
      "options": [
        {"label": "Preserve positions (Recommended)", "description": "Hidden rows keep their slots; visible rows move around them. Recommended: disturbs the least unrelated state."},
        {"label": "Recompute ordering", "description": "Hidden rows are pushed to the end. Simpler, but moves rows the user cannot see."},
        {"label": "Disable while filtered", "description": "No dragging until the filter clears. Safest, least capable."}
      ]
    }
  ]
}
```

**Missing info (kvit `question` block).** Same pattern; the free-text path
is the point, so keep labels short and let "Other" do the work:

```json
{
  "questions": [
    {
      "question": "Blocked on credentials: which registry should I push the image to? Pick one or type the URL as Other.",
      "header": "Registry",
      "multiSelect": false,
      "options": [
        {"label": "GHCR (Recommended)", "description": "ghcr.io, works with the existing workflow token."},
        {"label": "Docker Hub", "description": "Needs DOCKER_TOKEN in the environment."}
      ]
    }
  ]
}
```

**Proposed next step (kvit `next_step` block) — the DEFAULT ending.**
Most mutating turns end here, not with a flat "done". The accept option
dispatches the continuation; an explicit stop option emulates `resolve`;
typing instead of picking emulates `collect`:

```json
{
  "questions": [
    {
      "question": "Drag-and-drop ordering works, 14/14 tests pass. How to proceed?",
      "header": "Next",
      "multiSelect": false,
      "options": [
        {"label": "Commit (Recommended)", "description": "Commit the reorder implementation now."},
        {"label": "Changelog too", "description": "Add a changelog entry, then commit."},
        {"label": "Stop here", "description": "Leave the change uncommitted; nothing more runs."}
      ]
    }
  ]
}
```

Multiple pending blocks become multiple question objects in one call (up to
4) or sequential calls — one block per question, never one mega-question
with six options (the tool caps at four; kvit's 2–6 range must be trimmed
to fit).

**Done without a question / failed (kvit `completed`, `failed`, `change` +
`verification` + `warning` blocks) — RARE EXCEPTION, not the norm.** No
tool call at all. Allowed only when the user said "just do X and stop" or
no plausible follow-up exists. Example final text:

> Done: drag-and-drop ordering works and survives restart.
> - Moved reorder logic into `internal/order`, kept hidden-row slots stable.
> - Tests: `go test ./internal/order` — 14/14 pass.

The `Tests:`/`Checks:` line is the `verification` block; the caveat sentence
("with one filtered-view behaviour open" / "leaves X unverified because Y")
is `completed_with_notes` / `warning`. The Stop hook checks this prose
lightly (section 4.3) — strictly enough to keep the habit, loosely enough
to never litigate taste. But the hook's first question is always "why no
AskUserQuestion?": a mutating turn with no ask must justify itself, because
in practice the user follows up with "what's next" anyway.

### 4.3 The Stop hook validates the ask, not a fence (your hook, retargeted)

Same event, different check. Pseudocode for
`.claude/hooks/report-gate.py` (command hook on `Stop`, stdin JSON →
stdout JSON):

1. Read `stop_hook_active`, `transcript_path`. Parse the transcript JSONL
   (not `last_assistant_message` — text-only, never holds the tool call)
   back to the last user message.
2. If the turn changed nothing (no Edit/Write/Bash/etc. tool use since the
   last user message), allow stop — mirrors kvit's `mode: mutating`.
3. If the turn's text lists options/asks ("choose", "how should I proceed",
   "A/B/C") but there is no `AskUserQuestion` tool use after that text →
   `{"decision": "block", "reason": "You offered options in prose without calling AskUserQuestion. Restate them as one AskUserQuestion call (2-4 options, header ≤ 12 chars, recommendation marked ' (Recommended)') and end there."}`.
4. If there is an `AskUserQuestion` call, validate it the way
   `internal/report/validate.go` validates a report — every problem at
   once, not one per round trip: 2–4 options per question, headers ≤ 12
   chars, exactly one `(Recommended)` label per single-pick question,
   descriptions present (they carry consequence + reason), ≤ 4 questions.
   Invalid → `block` with the full list.
5. If the mutating turn ends with no `AskUserQuestion`: treat as suspect, not
   as the norm. The user almost always follows a working turn with "what's
   next" / "continue", so a flat "done" costs a round trip. `block` once
   with "End mutating turns with AskUserQuestion proposing 2-3 continuations
   + 'Stop here' (section 4.2). Only skip the ask when the user said 'just do
   X and stop' or no plausible follow-up exists — then resubmit the same prose
   with a trailing 'No follow-up: <why>.' line." If that justification is
   present, fall through to step 6; otherwise the model should ask.
6. Prose check (no-ask path, and lightly always): headline-length-ish first
   line, presence of a `Tests:`/`Checks:` line or an explicit "left unverified
   because …" note. Missing → `block` once with "End with the headline +
   what-changed + Tests: line per the 'Structured report' output style." Keep
   this check forgiving; the strict half is step 4.
7. Valid → exit 0, no output (allow stop).

Loop protection, learned from kvit's repair budget (3) and reminder budget
(2): count consecutive blocks per session (state file in `/tmp` keyed by
`session_id`, or scan the transcript) and after ~3 failed repairs switch
from `decision: "block"` to `hookSpecificOutput.additionalContext`
("proceed; report unverified") or allow the stop. Claude Code's own 8-block
cap is the backstop, not the plan. Always respect `stop_hook_active` plus a
transcript scan so a never-resolving condition degrades to prose instead of
spinning.

Prefer `additionalContext` over `block` for guidance ("run the test suite
before finishing") and reserve `block` for the checks above: `block`
renders as a hook error, `additionalContext` as feedback. Both continue
under the same cap.

### 4.4 Why the ask call is a better card than a kvit-style overlay

- **Zero rendering code.** Buttons, keyboard selection, free-text "Other",
  and IDE/desktop/cloud parity come with the tool.
- **The answer resumes the same turn.** kvit's card is a between-turns
  object: pick → next turn's prompt. `AskUserQuestion` pauses the turn and
  the user's pick (or typed text) arrives as the tool result inside the
  same turn — one fewer round trip for every decision.
- **Stored once.** Questions + `answers` are already in the transcript
  JSONL. A `/report` skill reads the last assistant text plus the last
  `AskUserQuestion`/`answers` pair — your `:report` equivalent — with no
  sidecar file. (An optional `.claude/reports/*.jsonl` index is fine later
  for changelog automation; treat the transcript as canonical.)
- **Display polish is optional.** A `MessageDisplay` hook (screen-only
  `displayContent`, transcript keeps the original) can compact the
  *completed* prose into a card look. It is garnish — the interactive half
  needs nothing, because the tool already draws it.

## 5. ACP: what it is, and the subscription question

**The protocol in one paragraph.** ACP standardizes editor↔agent
communication the way LSP standardized editor↔language-server: the client
(editor/IDE/your wrapper UI) owns rendering — chat, tool-call cards,
diffs, permission prompts — and drives the session (`prompt`, `cancel`,
`set_mode`); the agent owns the loop — inference, tool calls, file edits —
and reports back over newline-delimited JSON-RPC on stdio. Markdown is the
default text format; diffs and permission flows have first-class types.
Ready-made clients exist (Zed's `agent.profiles.<name>.provider = "acp"`
pointing at a binary path; JetBrains via ACP plugins; standalone `acp-ui`,
`Panda`, `acp-components` for building your own). Capabilities are
negotiated at `initialize`, so clients degrade gracefully.

**Does Claude Code with a subscription support it?** Not natively — there
is no `claude --acp` that spends your Pro/Max seat. Two bridge families
exist, and they differ exactly on this point:

- **Official adapter** (`agentclientprotocol/claude-agent-acp`): speaks ACP
  by driving the Claude *Agent SDK*. Needs an API key / Console billing.
  Does **not** use your Pro/Max subscription. Best protocol fidelity,
  wrong wallet.
- **CLI-wrapping bridges** (e.g. `harukitosa/claude-code-acp`): spawn the
  `claude` CLI as a subprocess and translate ACP↔CLI. Run logged-in, so
  they **do** spend the Pro/Max subscription (and warn: do not set
  `ANTHROPIC_API_KEY`, or billing flips to API). Protocol fidelity is
  whatever the CLI surface exposes — prompt streaming, basic tools,
  session modes — and it breaks when the CLI changes.

So "wrap Claude Code in a UI on my subscription" is possible today via a
CLI bridge, with three caveats: **(a) Terms-of-service grey area** —
Anthropic has treated subscription-through-third-party-clients as abuse in
at least some cases (the opencode/ACP threads report bans; assume the
policy, not the enforcement gap); **(b) fragility** — a subprocess
scraper, not a contract; **(c) feature loss** — your hooks, output styles,
background tasks, crons, and permission modes live in the CLI/TUI and may
not survive translation, and `AskUserQuestion`-style elicitation depends on
what the bridge forwards (in the Agent SDK it surfaces as a `canUseTool`
callback with `toolName == "AskUserQuestion"` — verify your bridge does
this before relying on reports through it).

**What a wrapper would look like if you build one anyway.** Fork or extend
a CLI bridge: spawn `claude-code-acp` from your client over stdio; detect
the `AskUserQuestion` tool call in the streamed session (not a fence —
there is none anymore); render your card from its questions (headline from
the preceding text, option buttons from labels + descriptions); wire pick →
`answers` mapping, typed reply → per-question free text or `response`,
dismiss → stop. Persist nothing the transcript already holds. Budget a week
for the spike, not an afternoon — permission flows, cancellation,
multi-question mapping, and reconnect/prompt-cache behaviour are where the
time goes.

**Verdict.** If the goal is *readable, answerable endings inside Claude
Code*, ACP buys rendering power you do not need — `AskUserQuestion`
already renders — at the price of the two things currently working for you
(subscription billing, hooks). If the goal later becomes *one card UI
across several agents* (Claude + Codex + local models), ACP is the right
abstraction — that multi-agent payoff is what it is designed for — and the
ask-call convention in section 4 ports unchanged, because it is just a tool
call in the transcript.

## 6. Suggested build order

| Step | Work | Pays off |
|---|---|---|
| 1 | Write `.claude/output-styles/report.md` per 4.1 with the three examples from 4.2 | Model knows the shape; `/output-style report` toggles it |
| 2 | Retarget the existing Stop hook per 4.3: prose-without-ask → block; malformed ask → block with all problems; repair cap (~3) + `stop_hook_active` guard; read the transcript, not `last_assistant_message`, for tool calls | Deterministic gate; kills taste loops |
| 3 | Use it for a week; trim option counts/headers that the model keeps getting wrong | Converges the convention before automating on it |
| 4 | `/report` skill: reprint last text + last `AskUserQuestion`/`answers` from transcript | Your `:report` |
| 5 | Optional `MessageDisplay` polish for completed prose; optional `.claude/reports/*.jsonl` index | Readability; feeds changelog/automation later |
| 6 | Only then: ACP client spike on a CLI bridge, card component first, subscription + ToS check before investing | Real custom UI, multi-agent future |

## 7. What not to port

- **A second report format.** The fence-then-convert design is gone on
  purpose. One fact, one encoding: text + `AskUserQuestion`.
- **kvit's wider ranges.** Cap at the tool's limits: 2–4 options per
  question (not 2–6), ≤ ~4 questions per call (not 20 blocks), `header` ≤
  12 chars (no kvit equivalent), recommendation as `" (Recommended)"` +
  reason-in-description (no `recommendation` field).
- **Digit-keypress answers.** No hook or style can claim bare `1`–`9` in
  Claude Code's composer. Option buttons plus "Other" free text are the
  native answer mechanism — accept its rhythm.
- **Prompt-prefix cache discipline.** kvit keeps the `Report` tool schema
  byte-identical so the server cache hits. Your style text is re-sent per
  request under weaker caching — a second reason to keep section 4.1 short
  and put the bulk in examples the model sees once.
- **Inbox hold.** Single-process loop; nothing to hold back. `background_tasks`
  in Stop input already distinguishes "done" from "paused on background work".
- **Repair-via-tool-error.** kvit's `SemanticErrorf` rejects inside the tool
  call. Your rejection arrives as a new user-visible hook message one
  iteration later — noisier, so the "return every problem at once" rule
  matters more, not less.
