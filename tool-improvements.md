# Tool Improvements — Edit Reliability & Anomaly Diagnostics

Status: **all three improvements implemented** (Jun 2026). Off by default; opt in via config.
Author context: distilled from the qwen3.6-27 vs gpt-oss-20b vs devstral-small-2 benchmark
investigation (Jun 2026).

Implementation summary:

- **Improvement 1 (autoindent)** — config: `tools.edit.smart_first_line_indent` (default
  off, active only with `preview_mode`) + `tools.edit.max_autoindent_fix` (default 1). Pure
  helper `ReconcileFirstLineIndent` in `internal/tools/edit_apply_line.go`, wired into
  `callLineMode` (`edit.go`) via `FinalizeEditWithCorrection` (`edit_common.go`). New
  zero-arg tool `Edit.undo_autoindent` (`filesystem.go`), allowlisted in
  `CheckPendingEditBlockWithState` and registered in `setup.go`. Tests in
  `edit_autoindent_test.go`.
- **Improvement 2 (interrogation)** — config block `diagnostics:`
  (`internal/config/config.go`). `Interrogator` in `internal/agent/interrogate.go`, wired
  into the duplicate-call / FATAL / pending-blocked / alternating-loop / backtrack triggers
  across `runner_tools.go` and `runner_iteration.go`. Observability only: it logs JSONL to
  `interrogate_log_dir`, prints the Q&A on screen, and does not alter recovery. Tests in
  `interrogate_test.go`.
- **Improvement 3 (explicit delete + `""`-blank flip)** — config
  `tools.edit.explicit_delete` (default off). New `DeleteLines` tool
  (`internal/tools/edit_delete.go`), `""`-blank flip in `callLineMode` (`edit.go`), prompt
  + schema variants in `edit_line.go`, registration in `setup.go`. Tests in
  `edit_delete_test.go`.

---

## Background & motivation

`lines` edit mode is the right default for the small local models this project targets.
Evidence:

| Model | Suite result | Notes |
|-------|--------------|-------|
| gpt-oss-20b (`terminal-20b-20260110`) | **100%** (28/28) | E7 emitted `    return 42;` with correct 4-space indent; C3 multi-file edit passed |
| devstral-small-2 (`benchmark-devstral-s2-20260217-235540`) | **99%** (277/280) | E11 indentation 100%; only E7 failures were a Search runaway + one wrong-content edit, not indentation |
| qwen3.6-27 (`benchmark-qwen3.6-27-20260610-232159`) | **90%** (253/280) | E7 **0%**, C2 70%, C3 **0%** |

So `lines` mode is **not** broken — qwen3.6-27 is an outlier, and the goal is to lift
qwen without regressing the models that already pass. Switching to `searchreplace`/`patch`
is rejected: those formats have their own well-documented failure modes (oldString-not-found
loops, patch-syntax failures up to ~50% on untrained models) and would regress 20b/devstral.

Two qwen-specific failure clusters were isolated:

1. **First-line indentation under-count** (E7 0%, C2 70%, E11 occasional). qwen drops
   leading whitespace on the *first* line of `new_text` while continuation lines (after the
   first `\n`) come out correct.
2. **Pending-edit loop → FATAL** (C3 0%). qwen re-emits an identical `Edit` instead of
   `Edit.confirm`, trips the duplicate-call guard, and the run is cancelled. (Tracked
   separately; this doc specs the *diagnostic* for it, see Improvement 2.)

### Key diagnostic finding (drives Improvement 1)

In an E11 re-run, qwen produced:

```
new_text = "      validate_input()\n        transform_data()"
             ^^^^^^ 6 spaces (wrong, should be 8)
                                  ^^^^^^^^ 8 spaces (correct)
```

then **cancelled** the edit, dropped to raw `Shell` (`printf > file`), and wrote the whole
file with *perfect* 8-space indentation — burning 23 turns / 157k tokens / 92s to do a
two-line edit, bypassing the edit+review cycle entirely.

Conclusions:

- qwen's indentation **knowledge is intact** (the `printf` proves it) — so prompt examples
  cannot fix this; it is mechanical, not conceptual.
- qwen **authors** both lines (they are new content, not copied off the gutter), gets the
  continuation line right and the first line wrong → the cause is **positional**: the first
  line, being bound to `start_line`, is under-produced; continuation lines are free content
  and come out correct.
- The pattern is consistent across failures: E7 `   return 42;` (3 vs 4), C2 ` port: 8080`
  (1 vs 2), E11 first line (6 vs 8, continuation 8 ✓).
- That E11 "✓ PASS" is a **false positive**: the pass rate over-counts edit-tool success
  because some greens are ugly shell workarounds that skip diff review and restore points.

---

## Improvement 1 — First-line indentation auto-correction ("autoindent")

### Summary

At the **apply layer** of `lines` mode, when replacing line(s), detect the case where the
first line of `new_text` is under-indented relative to the line it replaces, and re-indent
that first line to match. Continuation lines are never touched. The correction is:

- **opt-in** via a config flag (default off), and active **only in `preview_mode`** so a
  correction always has a recourse (the undo command below),
- **reversible** via a single zero-arg `Edit.undo_autoindent` command,
- **transparent** — when it fires, the diff/result returned to the model carries a note
  explaining what was corrected and how to revert it,
- a **no-op when the model already indents correctly**, so models that pass today
  (20b, devstral) are unaffected and cannot regress.

### Why the apply layer (not presentation, not prompt)

- **Presentation** (e.g. changing the `   N│content` Read gutter at `filesystem.go:840`):
  rejected — it changes what *every* model reads to fix one model, risking regressions in
  models that already count the current format correctly.
- **Prompt example**: rejected — the lines-mode prompt (`edit_line.go:44`) already shows
  `"    return 43\n"` with 4-space indent and qwen still under-indents. Knowledge isn't the gap.
- **Apply layer**: invisible to the model's input, has the original file as ground truth,
  and is a no-op when unneeded. Correct place for a mechanical fix.

### The rule

On a **replace** (`end_line` present, i.e. not insert), with the feature enabled and
`preview_mode` on:

1. `origIndent` = leading whitespace (spaces/tabs) of the original file line at `start_line`.
2. `firstIndent` = leading whitespace of `new_text`'s first line.
3. Apply correction **only if all hold**:
   - it is replace mode (not insert), and `new_text` is non-empty;
   - the first line is **non-blank** after trimming (skip deletions / empty-line edits);
   - `len(firstIndent) < len(origIndent)` (model under-indented);
   - `origIndent` has `firstIndent` as a **proper prefix** (same whitespace style, just
     fewer chars) — prevents mixing tabs/spaces or fabricating indentation;
   - **dedent guard**: the deficit `len(origIndent) - len(firstIndent)` is `≤ MaxAutoindentFix`
     (default **1** — see "Why off-by-one default" below). At the default this fires only on
     an exact off-by-one under-indent.
4. Correction = replace the first line's leading whitespace with `origIndent` (i.e. add the
   missing leading character(s) — a space *or tab*, matching the file's style); leave the
   rest of `new_text` (continuation lines) byte-for-byte unchanged.

### Why off-by-one default

The dropped-indentation deficit observed for qwen3.6-27 is variable, but overwhelmingly **1**:

| Case | Expected | Actual | Dropped |
|------|----------|--------|---------|
| E7 runs 1–6, 8–10 | 4 | 3 | **1** |
| C2 runs 8, 10 | 2 | 1 | **1** |
| E11 benchmark run 2 | 8 | 7 | **1** |
| E11 live re-run | 8 | 6 | 2 |
| E7 run 7 | 4 | 0 | 4 (full strip) |

So `MaxAutoindentFix: 1` is the **minimal-intervention** default: it fixes the dominant
off-by-one case and **no-ops** on everything else (off-by-2, full strip), rather than risk
over-correcting. A partial fix on a deeper drop wouldn't help anyway (7 spaces is still a
Python `IndentationError`). Widening the cap is a knob to revisit only if the benchmark shows
the off-by-one fix alone leaves too much on the table — weighed against the larger over-snap
risk on legitimate multi-level dedents.

Worked cases (all corrected): E7 `   return 42;`→`    return 42;`; C2 ` port`→`  port`;
E11 `      validate_input()\n        transform_data()`→`        validate_input()\n        transform_data()`.

### Transparency & override (model feedback loop)

When the correction fires, the tool result (and the preview diff in `preview_mode`) MUST
include a note so the model can review and override — this turns a silent heuristic into a
reviewable correction that fits the existing edit→review→approve cycle:

- Add a result field, e.g.:
  ```json
  {
    "indent_autocorrected": {
      "line": 4,
      "from_spaces": 6,
      "to_spaces": 8,
      "message": "First line indentation was auto-corrected from 6 to 8 spaces to match the line being replaced. If this dedent was intentional, call Edit.undo_autoindent to revert it and get an updated diff."
    }
  }
  ```
- In `preview_mode`, surface this note alongside the diff (the `next_step` text) so the
  model can review the correction before confirming.
- The note should name the corrected line and the before/after indent width explicitly.

#### Override: `Edit.undo_autoindent` (preview mode)

Primary override path. A zero-arg sibling of `Edit.confirm` / `Edit.cancel`. When a pending
edit had its first line auto-corrected and the model disagrees, it calls
`Edit.undo_autoindent {}` and the tool:

1. recomputes the pending edit from the model's **original** (un-corrected) `new_text`,
2. re-stores it as the pending edit with `autoindent` suppressed (so a later `Edit.confirm`
   applies the model's indentation verbatim, no re-correction),
3. returns a fresh `pending_confirmation` diff.

Why a command and not a re-issue or a parameter: the model doesn't have to reconstruct
`path` / line range / a possibly-multiline `new_text` — re-emitting a complex call correctly
is exactly what small models fumble (and risks re-introducing the original under-indent or
unrelated drift). A zero-arg command is deterministic and far more reliable, and it keeps the
edit schema unchanged (no extra parameter for a model to misuse). It is the **single**
override path — there is no `autoindent` request parameter.

Behavior / edges:
- Allowed only while a pending edit exists (same allowlist as `Edit.confirm`/`cancel`).
- If the pending edit was **not** auto-corrected → friendly no-op message.
- If there is **no** pending edit → friendly error (mirrors confirm/cancel with nothing pending).
- One-way for the pending edit (to re-apply the fix, `Edit.cancel` + re-issue). Could
  generalize later to `Edit.revert_autofix` if other auto-corrections are added.
- Because this is the only recourse, auto-correction is **gated to `preview_mode`** — if
  preview is off, the feature is inactive (no silent in-place correction without an undo path).

### API / schema changes

- **Config** (`EditToolConfig` in `internal/config/config.go`):
  - `smart_first_line_indent bool` (default `false`) — enables the feature (only active when
    `preview_mode` is also on).
  - `max_autoindent_fix int` (default `1`) — the dedent-guard threshold (max deficit, in
    chars, that will be corrected). `1` = fix only exact off-by-one under-indents.
  - No change to the `Edit` request schema — there is no `autoindent` parameter.
- **New tool** `Edit.undo_autoindent` (zero-arg): reverts the auto-correction on the current
  pending edit and returns a fresh diff (see above). Register it like `Edit.confirm`/`cancel`
  with a one-line `Description()`. A spurious call when nothing is corrected → friendly no-op.
- **No standing prompt prose.** Do **not** give it a `PromptSection()` entry. kvit advertises
  every registered tool (`registry.Specs()` → `Tools:` in `runner_llm.go:37`), so it will be
  in the tools array (like confirm/cancel), but its *instructions* are surfaced only in the
  contextual correction note — the model is told about it exactly when it's relevant, not on
  every turn.
- **Optional, stricter:** to keep it out of the advertised list entirely until needed, filter
  it from the per-request specs unless a pending auto-corrected edit exists. Trade-off: breaks
  the "all tools always advertised" convention and invalidates the prompt cache on toggle
  turns (the `Specs()` ordering exists for cache stability). Only worth it if minimizing the
  tool list matters more than the occasional cache miss; corrections are rare so default to
  the always-registered approach above.

### Implementation location (for later)

- Pure helper `ReconcileFirstLineIndent(content, startLine, endLine, newText, maxFix)` →
  adjusted `newText` + a small struct describing whether/what it changed (for the note),
  in `internal/tools/edit_apply_line.go`.
- Call it from `callLineMode` in `internal/tools/edit.go` (after reading `oldContent`,
  before `ApplyLineEdit`), gated on `cfg.Tools.Edit.SmartFirstLineIndent && cfg.Tools.Edit.PreviewMode`.
- Thread the "what changed" struct into `BuildEditPreviewResult` / `BuildEditSuccessResult`
  (`edit_common.go`) so the note reaches the model.
- For `Edit.undo_autoindent`: store the **original (un-corrected) `new_text`** plus the
  correction metadata in the `pendingEdit` struct (`StorePendingEdit` in `edit_common.go`),
  so undo can recompute the pending edit without the fix. Register the new tool and add it to
  the pending-edit allowlist in `CheckPendingEditBlockWithState` (`filesystem.go`, alongside
  `Edit.confirm`/`Edit.cancel`).

### Scope & non-goals

- Applies to **lines mode, replace only**. Not insert (no reference line), not new files,
  not searchreplace/patch.
- Fixes the **first line only**. Assumes continuation-line indentation is reliable
  (validated: E11 continuation was correct). It deliberately does not police continuation
  lines — if a future model also mangles those, that is a separate problem.
- Not a substitute for the anchor/“lite-patch” idea (Improvement 2 of the prior discussion);
  this is the simplest first try. If benchmarks show the dedent guard is too aggressive or
  continuation reliability doesn't hold, revisit the suffix-anchor approach.

### Known limitation — intentional dedents

The one thing this can get wrong: the model *deliberately* wants the line less indented and
the correction pushes it back up. With the off-by-one default this is mostly theoretical, but
not entirely:

- **Space-indented code (the common case): essentially safe.** Real dedents move a whole
  level — ≥2 (or 4) spaces — which exceeds `max_autoindent_fix: 1`, so the correction simply
  **never fires** on them. A *one-space* dedent is not a thing models intend in block-indented
  code (it would itself be a style/syntax error), so there's nothing legitimate for the
  off-by-one rule to clobber.
- **Tab-indented code: the real exposure.** There 1 char = one full level, so a legitimate
  one-level dedent (2 tabs → 1 tab) has deficit 1 and *would* be wrongly corrected.
  → Mitigation: add a **spaces-only guard** — skip correction when `origIndent`/`firstIndent`
  contain tabs. Costs nothing on the benchmark (E7/C2/E11 are all space-indented) and removes
  the tab dedent risk entirely. Recommended.
- **Alignment (non-block) indent**, e.g. a line aligned under an opening paren at an odd
  column: a rare 1-space re-align could be nudged. For *replace*, though, the reference is the
  line's own prior indent, so snapping usually *restores* the original alignment rather than
  breaking it.

Backstops for whatever slips through: the feature is **opt-in** (flag off by default) and
**only active in `preview_mode`**, so a correction is always reviewable — the model sees
"first line auto-corrected 6→7" in the diff *before confirming* and can `Edit.undo_autoindent`
(then confirm the model's original indentation). The deliberate-dedent worry is exactly what
that review step is there to catch.

Net: with `max_autoindent_fix: 1` + spaces-only, the realistic blast radius is ~nil for
space-indented code; validate with a deliberate-dedent test case regardless.

### Validation plan

1. Unit tests in `edit_apply_line_test.go`: E7/C2/E11-shaped corrections; no-op when already
   correct; insert mode untouched; blank-first-line untouched; tab indentation;
   deliberate-4-space-dedent **not** corrected (guard); `Edit.undo_autoindent` reverts a
   pending corrected edit (and a following `Edit.confirm` applies the original indentation).
2. Benchmark with `smart_first_line_indent: true`:
   - qwen3.6-27 — expect E7/C2/E11 to improve.
   - gpt-oss-20b **and** devstral-small-2 — expect **no regression** (correction should be a
     no-op for them; confirm pass rates and that `indent_autocorrected` rarely/never fires).

---

## Improvement 2 — Model interrogation for anomaly logging / debugging

### Summary

When the agent detects "odd" model behavior (loops, repeated identical calls, ignored
confirm/cancel, etc.), optionally **interrogate the model** — ask it to explain why it did
what it did — and **log the Q&A to a side channel** for offline analysis. This is an
**observability** feature first: its value is the captured explanation, used to design the
real fix. It is **not** a standalone recovery mechanism.

### Why

- qwen's C3 failure is a loop the harness *already detects* (it emits "DUPLICATE CALL ERROR",
  "Alternating loop detected", and finally "FATAL: Edit called 3 times with identical
  arguments"). At those exact points we have a model that is misbehaving and no record of
  *why*. Interrogation captures the reasoning.
- A natural experiment (qwen routing around the edit tool via `printf`) already told us more
  than a guess would; structured interrogation generalizes that for cases we can't eyeball.

### Honest limits (must be designed around)

- **Pure rewind reproduces the loop.** Backtracking (`BacktrackConfig`) already drops recent
  messages and retries at the same point; same context → same output. So interrogation's
  value is the *log*, not the recovery. Recovery still needs a deterministic intervention
  (inject a corrective — see `inject_user_message` — auto-resolve the pending edit, or a
  reworded tool result). Do not wire recovery to trust the model's self-explanation.
- **Self-explanations are often confabulated.** Treat logged reasons as an **aggregate clue**
  across many runs that a human reads to find the common misconception and bake a static fix —
  not per-instance ground truth that control flow branches on.
- **Cost.** Each interrogation is an extra round-trip; gate it and cap it.

### Triggers ("odd behavior")

Hook into existing detectors. Trigger interrogation when any of these first cross a threshold
(each configurable; suggested defaults in parens):

| Trigger | Source today | Default |
|---------|--------------|---------|
| N consecutive **identical** tool calls (same name+args) | duplicate-call guard | 2 |
| Repeated **pending-edit BLOCKED** for same path | `CheckPendingEditBlockWithState` (`filesystem.go`) | 2 |
| **Alternating loop** detected (A/B/A/B) | loop detector | on first detect |
| About to hit a **FATAL**/cancel from a loop guard | duplicate-call FATAL | always (last-chance capture) |
| Repeated **backtrack** retries at same point | `BacktrackConfig` | at `max_retries - 1` |

Interrogation fires **once per anomaly episode** (not per repeated call) and is itself
hard-capped per task to avoid interrogation loops.

### Mechanism

1. On trigger, pause normal recovery and send the model a **targeted question** referencing
   the concrete situation (templated per trigger; see below).
2. Capture the model's reply.
3. **Side-channel log** the episode (do **not** leave the Q&A in production history — it
   would pollute context). Then resume the pre-existing recovery path unchanged for this
   first pass (observability-only mode).
4. Optionally (separate, later flag) feed a derived corrective back via `inject_user_message`.
   Out of scope for the first version.

### Question templates (examples)

- Identical-call / pending loop:
  > "You just issued this tool call again with identical arguments:\n`<call>`\nThe previous
  > result was:\n`<result>`\nExplain: (1) why you expected a different outcome, (2) what you
  > believe the current file/tool state is, (3) what you think you must do next to make
  > progress. Answer concisely; this is diagnostic and will not change the task."

- Ignored confirm/cancel (pending edit):
  > "An edit is pending with this diff:\n`<diff>`\nYou are required to call `Edit.confirm`
  > or `Edit.cancel` and nothing else. You instead called `<call>`. Explain why — did you
  > not see the pending state, did you expect the edit was already applied, or something else?"

Templates should always state that the answer is for diagnostics and won't be graded, to
reduce the model performing rather than reporting.

### Logging schema (side channel)

JSON-lines under e.g. `benchmarks/.kvit-coder-benchmark/interrogations/<run-id>.jsonl`, one
record per episode:

```json
{
  "ts": "...", "model": "qwen3.6-27", "benchmark": "C3", "run": 1,
  "trigger": "duplicate_call", "trigger_count": 3,
  "offending_call": {"tool": "Edit", "args": {...}},
  "prior_result": "...",
  "history_excerpt": ["...last K messages..."],
  "question": "...", "answer": "...",
  "context_tokens": 5234, "turn": 6
}
```

Keep records self-contained enough to analyze without the full transcript. Redact paths if
audit redaction is on (`safety.audit.redact_secrets` precedent).

### Config

- `tools.edit` / a new `diagnostics` block:
  - `interrogate_on_anomaly bool` (default `false`).
  - `interrogate_triggers []string` (subset of the table above; default all).
  - `interrogate_identical_threshold int` (default 2).
  - `interrogate_max_per_task int` (default 3) — hard cap.
  - `interrogate_log_dir string`.
  - `interrogate_then_inject bool` (default `false`) — reserved for the later recovery
    variant; no-op in v1.

### Scope & non-goals (v1)

- Observability only: capture and log; do **not** alter recovery based on answers.
- Keep interrogation turns out of production history.
- Pair with the existing duplicate-call FATAL — interrogate as a *last-chance capture* right
  before cancel, so even fatal episodes are explained.

### Validation plan

Run qwen3.6-27 on C2/C3/E7 with `interrogate_on_anomaly: true`, then read the logged answers
to confirm/refute the hypotheses:

- first-line indent: does qwen *intend* the correct indent but mis-emit (mechanical), or
  believe the shorter indent is correct (perceptual)? (Informs whether autoindent is the
  right fix or whether a perceptual hint could help.)
- C3 loop: does qwen not realize `Edit.confirm` is required, think the edit already applied,
  or not see the pending state? (Informs the deterministic C3 fix.)

---

## Improvement 3 — Explicit delete tool + line-addressed-consistent empty semantics

Status: **implemented** (Jun 2026). Off by default; opt in via `tools.edit.explicit_delete`.

Implementation: config `tools.edit.explicit_delete` (default off). New `DeleteLines`
tool in `internal/tools/edit_delete.go` (routes through `FinalizeEdit` /
`ApplyLineEdit(.., "")`, applied by `Edit.confirm` / `Edit.cancel`), registered in
`setup.go`. The `""`-blank flip is in `callLineMode` (`edit.go`), gated on the flag, for
replace mode only. Prompt + schema flag-on variants in `edit_line.go`. Tests in
`edit_delete_test.go`. Note: `DeleteLines` is **not** added to the pending-edit allowlist
in `CheckPendingEditBlockWithState` — like `Edit`, it is blocked while another edit is
pending (forcing confirm/cancel first), which preserves the single-pending-edit invariant
and loop protection; its own pending edit is resolved by the already-allowlisted
`Edit.confirm`/`Edit.cancel`.

### Motivation

"Delete a line = replace it with an empty string" is a *learned convention*, not an
intuition. Multiple models (not just qwen) burn turns rediscovering it, and the line-range
framing actively fights it: when the model addresses `start_line=5, end_line=5`, it expects
line 5 to *persist* as the target, so `new_text=""` reads as **"make line 5 blank"** — but
the tool's byte-splice semantics treat `""` as **"remove line 5."** That collision is the
E9 failure (model wants a blank line, sends `""`, line vanishes, loops → shell) and the
general "how do I delete?" friction.

Diagnostic basis: the tool's encoding is *complete and unambiguous* (`""`=delete,
`"\n"`=blank — see Improvement-1 discussion and `edit_apply_line.go:161-164`), and 20b/devstral
navigate it correctly. The problem is the **byte-level trailing-newline convention deciding
line existence under a line-addressed frame** — it's unintuitive, so weaker models trip and
even strong ones spend tokens on it.

### The change (coherent, not additive)

Two parts, gated together behind one flag:

1. **New tool `DeleteLines {path, start_line, end_line?}`** — removes lines. First-class,
   obviously-named delete affordance. Edit-mode-independent (works the same in lines /
   searchreplace / patch configs, since it's purely line-range based).
2. **Flip lines-mode `""` to mean "blank the addressed line(s)"** (keep them, empty) instead
   of delete. `"\n"` remains blank as a back-compat alias. Now the model's line-addressed
   expectation and the tool agree.

Why coherent over additive (add `DeleteLines` but keep `""`=delete): additive leaves `""`
overloaded and creates *two* delete paths, so the E9/blank confusion is unfixed — you pay the
cost (new surface) without the payoff. The delete tool's whole value is that it lets `""`
become intuitive.

### Semantics (flag off vs on)

| Intent | Today (flag off) | Coherent (flag on) |
|--------|------------------|--------------------|
| Replace line(s) with text | `Edit{new_text:"text"}` | same |
| **Blank** a line (keep it) | `Edit{new_text:"\n"}` | `Edit{new_text:""}` *or* `"\n"` |
| **Delete** line(s) | `Edit{new_text:""}` | `DeleteLines{start,end}` |

Replace stays internally consistent: `Edit` replace `[a,b]` with `new_text` always yields the
new_text content as the new line(s); with `""` that's a single empty line (range → one blank
line), never "vanishes."

### `DeleteLines` design

- **Params:** `path` (req), `start_line` (req, 1-based), `end_line` (optional, default
  `start_line`; inclusive). Validate `start_line ≥ 1`, `end_line ≥ start_line`, within file;
  friendly errors mirroring the other edit tools.
- **Preview/confirm:** routes through the existing pending-edit machinery — stores a pending
  edit (old = the lines, new = removed), returns `pending_confirmation` with a diff showing
  the removed lines + surrounding context; applied by `Edit.confirm`, discarded by
  `Edit.cancel` (already `Write.*` synonyms). Allowlist it in `CheckPendingEditBlockWithState`.
- **Reuse:** `BaseEditTool` (`ValidateAndResolvePath`, `ReadFileForEdit`), `FinalizeEdit` /
  `BuildEditPreviewResult` (`edit_common.go`), and `ApplyLineEdit(content, start, end, "")`
  for the actual removal (its existing `""`→delete behavior is exactly right here). Large
  files: `StreamingLineReplace(..., "")` (note: verify the empty-replacement streaming path
  removes the range cleanly).

### `""`-blank flip implementation

- In `callLineMode` (`edit.go`), when `explicit_delete` is on and it's a **replace**
  (`end_line` set) with `new_text == ""`, treat it as a blank line — i.e. substitute
  `"\n"` before `ApplyLineEdit` (which already blanks on `"\n"`). One-line change, gated.
- Insert mode (`""`, no `end_line`) stays a no-op/insert-nothing as today.
- Interaction with Improvement 1: `ReconcileFirstLineIndent` already early-returns on
  `newText == ""` and blank first lines (`edit_apply_line.go:54,67`), so autoindent is
  unaffected either way.

### API / schema / prompt

- **Config** (`EditToolConfig`): `explicit_delete bool` (default `false`). When on: register
  `DeleteLines`, apply the `""`-blank flip. When off: today's behavior exactly (no new tool,
  `""`=delete).
- **New tool** `DeleteLines` registered in `setup.go`; allowlisted while a pending edit
  exists. (Namespacing: top-level `DeleteLines` reads as a primary op; `Edit.delete_lines` is
  the alternative if you prefer keeping it in the Edit family — pick one.)
- **Prompt** (`LineEditPromptSection`, flag-on variant): "To remove lines, use `DeleteLines`.
  `Edit` with empty `new_text` blanks the line (keeps it)." Drop/replace the `""`=delete
  bullet. This is what nudges 20b/devstral to move their deletes onto `DeleteLines`.

### Scope & non-goals

- The `""`-blank flip is **lines-mode only**. searchreplace's `replace:""` (delete matched
  text) and patch's `-` lines are already content-based and intuitive — out of scope.
- `DeleteLines` itself is mode-independent and useful regardless of edit mode.
- Flag-gated and opt-in; default off ⇒ zero change for anyone not opting in.

### Validation plan

Run qwen + 20b + devstral with `explicit_delete: true`:
- **qwen:** E9 (blank via `""`) should pass; delete cases via `DeleteLines` get cleaner/shorter.
- **20b / devstral:** **E3 is the regression watch** — confirm they pick up `DeleteLines`
  from the prompt and still delete (rather than blanking via the now-flipped `""`). Also
  **compare token/turn counts flag-on vs flag-off** — hypothesis is the explicit tool *lowers*
  them by removing the "rediscover the empty-string convention" step. Decide default from the
  numbers.
- **Unit tests:** `DeleteLines` single / range / out-of-bounds / preview+confirm; `""` blanks
  under flag (replace); `""` still deletes with flag off (back-compat); `"\n"` blanks in both.

---

## Suggested order

1. **Improvement 1 (autoindent)** — smallest, opt-in, no-op for working models. *(implemented)*
2. **Improvement 2 (interrogation, observability mode)** — capture qwen's reasoning on the
   *remaining* failures (esp. C3, and now E9 blank-vs-delete) to confirm root causes. *(implemented)*
3. **Improvement 3 (explicit delete + `""`-blank flip)** — coherent fix for the delete/blank
   confusion; opt-in flag, A/B-benchmarked on 20b/devstral for E3 regression + token deltas.
4. Revisit deferred ideas (suffix-anchor "lite patch", C3 identical-Edit→confirm,
   pending-edit guard reordering / non-backtrackable pending block) using what (1)–(3) reveal.
