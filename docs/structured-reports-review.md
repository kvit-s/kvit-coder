# Structured reports: why the card rarely proposes anything, and what to change

## 1. The case that prompted this

Session `2026-09-13-zlh457`. User prompt:

> when I run terminal on this ubuntu it always starts with:
> `ls: cannot access '/home/sk/.antigravity-server/bin/': No such file or directory`
> can you find out why and tell me how to fix this

The turn did `Read ~/.bashrc` + `Read ~/.profile` + `Shell` (`ls`, `grep`),
found line 144 (`export PATH="$HOME/.antigravity-server/bin/$(ls ...)/..."`),
and ended with:

- `task_status: completed`, headline about the stale PATH line,
- `finding[cause]` + `verification[repro]`, no interactive block,
- card renders as `DONE` with two lines and `[0 details · esc dismiss · :report reopen]` —
  no digit-key action.

But the fix proposal **was** in the turn: buried in `finding.details`
(hidden, ~1 KiB, collapsed by default):

> How to fix (pick one): 1. delete line 144 … 2. guard the line …
> I did not edit `~/.bashrc` — say the word and I will apply option 1 or 2.

The next user message was the predictable `fix it`, which started a second
turn to do what one keypress could have done.

So the direct answers to the three questions:

1. **Why no `next_step`?** Nothing in what the model actually read told it
   to add one, and everything in the validation economics told it not to
   (section 2). `completed` + prose-in-`details` is the cheapest report
   that validates; `needs_action` + `next_step` is the most expensive.
2. **What is the prompt about the Report tool?** Two different prompts
   exist, and the live one is the shorter, weaker of the two (section 2).
3. **Should a rejection remind the model what a good report looks like?**
   Yes — but the bigger gap is *before* the first submission, not after a
   rejection. The gate nudge and the rejection text both describe the
   schema; neither mentions actionability (sections 3–4).

## 2. What the model actually sees

There are three descriptions of the report, in decreasing order of influence:

### 2.1 The strong-profile section — the live path

Default config is `agent.profile: strong` (`config.yaml:143`,
`internal/config/config_test.go:369`). Under it, prompt generation skips
templates entirely (`internal/prompt/prompt.go:75-76, 546-557`) and uses
`ReportTool.ShortPromptSection()` (`internal/tools/report.go:122-145`):

> End every turn that [mode sentence] with one Report call …
> A report is a status, a one-sentence headline, and typed blocks. Two rules
> the schema cannot enforce for you:
> - completed/completed_with_notes need a verification block …
> - Any block that asks the user something — decision, question, blocked,
>   next_step — makes the report "needs_action" or "blocked", offers 2 to 6
>   options, and says why you recommend the one you recommend …

What is **missing** compared to the long form: the entire
"which block type to use" paragraph. The long template
(`internal/prompt/prompts/tools/report.tmpl:49-55`) has:

> `change` for work output. `finding` for something learned …
> `decision` when only the user can choose. `question` when only the user
> has the information. `blocked` when something outside stops you.
> **`next_step` when there is an obvious continuation to accept or redirect.**

That last sentence — the only place that says *when* to propose a
continuation — never reaches the strong-profile model. What it gets instead
is the JSON Schema enum description (`internal/report/report.go:461-464`):

> 'next_step' is a proposed continuation

Four words, no trigger condition, no example, no statement that an
actionable report is preferred over a `completed` one with instructions in
`details`.

The worked example makes it worse: both the template (`report.tmpl:19-47`)
and the weak-profile `reportExample` (`internal/tools/report.go:170-193`)
demonstrate a `decision`, never a `next_step`. A model imitating the example
learns `decision`; nothing demonstrates the commonest case —
"here is the fix, press 1 to apply it".

### 2.2 The JSON Schema — the fallback teacher

`report.JSONSchema()` (`internal/report/report.go:426-509`) documents each
field's *requirements* (which types need `options`, `recommendation`,
`response_type: single`, dispatch-needs-`instruction`, next_step needs one
`dispatch` + one `collect`/`resolve`), but never the *policy*: when to
choose `needs_action` + `next_step` over `completed`. The `task_status`
description says "A report that asks the user anything is 'needs_action' or
'blocked'" — a constraint, not an encouragement.

### 2.3 The validator — the real teacher

`report.Validate()` (`internal/report/validate.go`) enforces ~15 rules.
The ones that shape behaviour:

- `finding` needs `impact` + `importance` + `blocks_current_task`.
- `completed` on a mutated turn needs a `verification` block.
- interactive blocks need 2–6 options, `recommendation` (+ `reason`,
  + `response_type`), dispatch options need `instruction` ≤ 1 KiB,
  `next_step` needs ≥1 `dispatch` **and** ≥1 `collect`/`resolve`.
- any interactive block forces status `needs_action`/`blocked`.

None of these is wrong individually. Jointly they mean: the minimal
acceptable `completed` report has 2 blocks and ~7 fields; the minimal
acceptable `needs_action` + `next_step` report has 2–3 blocks and ~15 fields
with cross-field consistency (recommendation must name a real option id,
`response_type` must be exactly `single`, instruction only on `dispatch`).
Rational model behaviour under rejection pressure is to submit the former.

## 3. How the Antigravity turn actually flowed

Reconstructed from `~/.kvit-coder/sessions/2026-09-13-zlh457/history.jsonl`:

1. Three tool calls (`Read` ×2 + `Read` miss, then `Read` + `Shell`).
   `Shell` is never `ParallelSafe`, so `Mutates(Shell) == true`
   (`internal/tools/batch.go:287-292`) and `runner_tools.go:78-80` marks the
   turn mutated — even though the shell ran read-only `ls`/`grep`.
2. First `Report` call: `completed` + one `finding`, no `impact`/`importance`/
   `blocks_current_task`, no `verification`. Rejected with exactly the four
   problems the validator is supposed to catch:
   `…/impact: impact is required …`, `…/importance …`,
   `…/blocks_current_task …`,
   `/blocks: This turn changed something, so the report says how that was checked …`
   (attempt 1 of 3).
3. Second `Report` call: same `finding` + the three fields, plus a
   `verification[repro]` with `status: passed` and
   `evidence: ls -la … and grep -rn antigravity …`. Accepted.
   Note what that "verification" is: the investigation itself, relabelled.
   A diagnostic turn that ran `ls` to *find* the cause has nothing to
   *verify*, but the mutated-turn rule demands a `verification` block, so the
   model invents one. This is the "explanation ends up inside a block that
   verified nothing" failure mode `validate.go:88-93` warns about — produced
   here by the rule itself, because read-only `Shell` counts as mutation.
4. The gate (`reportGateMessage`, `runner_iteration.go:377-400`) never fired:
   the model called `Report` unprompted. Its text, for reference:
   "This turn has not submitted a report. Call Report now with what you just
   said, in the schema: a task_status, a one-sentence headline, and the typed
   blocks behind it." — schema-shaped, no mention of `next_step`.
5. The accepted report's `finding.details` contains both fix options *and*
   "say the word and I will apply option 1 or 2" — i.e. the model knew the
   continuation was obvious and expressed it as prose because prose is free
   and `next_step` costs ~8 extra constrained fields plus a status change.

## 4. How flows usually go (session data)

Across `~/.kvit-coder/sessions/*/history.jsonl` (243 sessions on this
machine, September 2026), counting only real `Report` tool results
(`Error: REPORT_REJECTED…` / `Report accepted…`), not file contents that
mention the marker:

- **14 `Report` calls, 7 rejections, 13 acceptances** (one call rejected
  twice before acceptance). Roughly every second first submission fails —
  the "almost always fails" impression is directionally right for first
  attempts, even if retries usually recover within the 3-attempt budget.
  Gate nudges (`has not submitted a report`): 5.
- **Submitted statuses:** `completed` 10, `completed_with_notes` 2,
  `needs_action` 2. **Reports containing any interactive block
  (`decision`/`question`/`blocked`/`next_step`): 2 of 14.** Both were
  hand-built during feature development (sessions `jzvitd`, `v18q3l`), not
  spontaneous model proposals.
- **Rejection causes seen:** missing `finding.impact`/`importance`/
  `blocks_current_task` (the commonest, 5×); mutated-turn-missing-
  verification (5×, always after a `Shell` turn including read-only ones);
  `headline_too_long` (163 chars); empty `summary`; `partial` verification
  without `limitation`; `blocks` sent as a JSON string instead of an array.
  The `d6gehd` turn shows the classic three-round repair: too-long headline
  → missing summary → missing `blocks_current_task`, one layer per trip.

Two patterns stand out:

- **Repairs teach minimal compliance.** Every rejection message
  (`FormatProblems`, `validate.go:444-457`) lists only defects. The model
  learns "add the missing required field and resubmit", never "consider
  whether the user needs a button". After one rejection the model is *less*
  likely to attempt a `next_step`, not more — it just paid a round trip for
  schema friction and wants the cheapest acceptance.
- **The mutated rule misfires on diagnostic turns.** `Shell` always mutates
  (no `ParallelSafe`), so any turn that runs `ls`/`grep`/`git status` owes a
  `verification` block. For a question-answering turn ("why does my terminal
  print this?") the honest report is `completed` + `finding`; the rule forces
  `finding` + pseudo-`verification`. The model complies by double-counting
  its investigation as verification, which trains exactly the cynicism the
  rule was meant to prevent.

## 5. What to change

### 5.0 How the user reads the card (the design target)

Stated reading order, most to least attention:

1. top line — the status chip + `headline`. Decides whether to keep reading.
2. what's next — the interactive blocks (`next_step` / `decision` / `question`):
   the digit-key actions and their labels.
3. maybe the bullet summaries — one-line `change` / `finding` / `verification`
   summaries.
4. almost never `details` — collapsed by default, opened only on suspicion.

Implications for the prompt: the headline must carry the material fact alone;
every obvious continuation must be a keypressable option, never prose in
`details`; summaries must stand on their own because `details` won't be
opened. The Antigravity report inverted this stack: the action lived in layer
4 (hidden `finding.details`: "say the word and I will apply option 1 or 2")
while layers 2–3 carried nothing actionable — hence a whole second turn for
`fix it`.

Goal restated: reduce attention per turn by making layer 2 carry the action —
fix, commit, debug, clean up — as keypresses. That means more `needs_action`
reports with **one or more** `next_step` blocks, not just fewer rejections.
Proposals in priority order:

### 5.1 Tell the strong profile when to use `next_step` (cheap, do first) — including the plural case

Add 3–4 lines to `ShortPromptSection()` (`internal/tools/report.go:122`),
i.e. to the prompt the default model actually reads:

> If there is an obvious continuation the user will likely ask for next
> (apply the fix, commit the change, debug the failed test, run the
> follow-up), add a `next_step` block for EACH proposal — there are often
> several (e.g. `[apply fix]`, `[commit]`, `[debug failed test]`), so emit
> several `next_step` blocks, one per proposal — each with one `dispatch`
> accept option and a `collect`/`resolve` alternative, and set status to
> `needs_action`. One block = one proposal; the options inside a block are
> ways to answer that proposal, not different proposals. Prefer keypresses
> over instructions in `details`: do not bury "say the word and I will …" in
> prose — make it each accept option's `instruction`. The user reads, in
> order: headline, what's-next actions, bullet summaries, almost never
> details — so nothing the user must act on lives only in `details`.

Mirror one sentence of it into the JSON Schema `next_step` mention and into
`reportGateMessage` ("…and if there is an obvious next move, propose it as a
`next_step` block rather than describing it in prose"). Gate + rejection are
the two messages the model reads mid-turn; both currently describe shape
only.

### 5.2 Show `next_step`s, not (only) a `decision`, in the example

Change `reportExample` (`internal/tools/report.go:170-193`) and the template
example (`report.tmpl:19-47`) so the demonstrated interactive shape is one or
preferably two `next_step` blocks — e.g. `[apply the fix]` (dispatch) and
`[commit it]` (dispatch) — each with accept (`dispatch` + full `instruction`)
+ `collect`/`resolve` alternative, `recommendation` + `recommendation_reason`
+ `response_type`. Models imitate examples more than they obey enumerations;
today the only imitable interactive shape is a single `decision`, which hides
both the preferred block type and the normal plural pattern (N proposals = N
blocks; cf. 5.1).

### 5.3 Stop demanding verification for read-only shell use

Options, cheapest first:

- (a) Document that a diagnostic `Shell` turn satisfies the mutated rule
  with `status: not_run` + `limitation`, and say so in the rejection text
  for `status_needs_verification` (it already hints at it — make it the
  primary advice for turns whose only "mutation" was `Shell`).
- (b) Narrow `Mutates` for `Shell`/`Shell.advanced`: treat a command the
  permission system already classifies read-only as non-mutating. Harder,
  touches `internal/permissions` + `runner_tools.go:78`, but removes the
  incentive to write pseudo-verifications.

Without one of these, encouraging `next_step` will collide with the
verification demand: the model will keep spending its repair budget on the
wrong block.

### 5.4 Add a non-blocking actionability reminder to the rejection path (the actual ask)

The request asks: "as an error to llm we should probably add reminder what
is expected out of the report". Careful: appending "also consider next_step"
to every `REPORT_REJECTED` would punish already-failing turns with more
text. Better:

- On **acceptance**, nothing (the turn ends; extra text is wasted).
- On **rejection**, keep listing defects only — but when the submitted
  `details`/`summary` contains an obvious buried proposal (imperatives like
  "say the word", "pick one", "how to fix" — the Antigravity details match
  all three), append one line: "The details propose a follow-up in prose;
  expose it as a `next_step` block with status `needs_action` instead."
  Keyword heuristic, deliberately narrow, so it fires only when the model
  already half-proposed the action.
- Count `Problem.Code`s (they are stable by design, `validate.go:10-13`)
  alongside the existing `tools-stats.md` pipeline, so the next review can
  say which rule actually burns tokens rather than relying on a 14-call
  sample.

## 6. Files touched by this review

- `internal/tools/report.go:122-193` — short prompt + example (edit site for 5.1–5.2).
- `internal/prompt/prompts/tools/report.tmpl:49-78` — long prompt (weak profile only).
- `internal/report/report.go:426-509` — schema descriptions.
- `internal/report/validate.go:96-125, 250-401, 444-457` — rules + rejection text.
- `internal/agent/runner_iteration.go:372-400` — gate nudge text.
- `internal/agent/runner_tools.go:75-80`, `internal/tools/batch.go:280-292` —
  why read-only `Shell` counts as mutation.
- Evidence: `~/.kvit-coder/sessions/2026-09-13-zlh457/history.jsonl`
  (first `Report` args, `REPORT_REJECTED` 4-problem text, accepted args with
  fix-in-`details`); cross-session counts from §4.

## 7. Implementation (2026-09-13)

Implemented 5.1, 5.2 and 5.4; 5.3(b) deliberately deferred:

- **5.1** — `ShortPromptSection()` now states the reading order
  (headline → what's-next actions → bullet summaries → almost never details),
  requires one `next_step` block per proposal with `dispatch` accept +
  `collect`/`resolve` alternative, and forbids burying "say the word …" in
  `details`. Mirrored in one sentence each in the gate nudge
  (`reportGateMessage`), the JSON Schema (`type`, `task_status`, `summary`,
  `details`, `blocks` descriptions) and `docs/structured-reports.md` §2.2.
- **5.2** — both imitable examples (`reportExample` and `report.tmpl`)
  now demonstrate **two** `next_step` blocks (apply the fix, commit it) instead
  of one `decision`. `TestTheDocumentedExamplesAreValid` covers both.
- **5.4** — new `report.BuriedProposalHint()`: when a rejected report proposes
  a follow-up in prose (narrow phrase list) while offering no interactive
  block, the rejection appends one hint line pointing at `next_step` +
  `needs_action`. Covered by `TestBuriedProposalHint` and
  `TestRejectionHintsAtBuriedProposal` (including the negative cases).
- **5.3** — not changed. Narrowing `Mutates` for read-only `Shell` touches
  permissions + the run loop and needs its own review; the `not_run` +
  `limitation` path the rejection text already suggests remains the documented
  answer for diagnostic turns.
- Pre-existing failure noted: `TestHeadlessRunPrintsTheReport` fails on clean
  main too (expects a `DONE ·` chip for a `completed` report); unrelated to
  this change.
