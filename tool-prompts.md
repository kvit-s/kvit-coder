# Tool Preference Probe — Prompts

Purpose: find out what file-editing tool **API qwen3.6-27 (and gpt-oss-20b, devstral-small-2)
were trained to use** — so we can decide whether to align kvit-coder's edit tools to their
trained distribution (vs. keep tuning line-mode). Driven by the benchmark finding that qwen's
behaviors (insert-by-default, parallel/batched edits, `""`-to-blank, not calling `confirm`)
look like echoes of some *other* editor's API.

## How to run

- **Fresh chat per prompt.** No prior context, no pasted earlier answers — contamination
  defeats the point. The models have **no tools** here; they answer in plain text (which is
  fine — "write the exact call you'd make" still elicits the trained format).
- **2–3× each, per model.** Stable answers across repeats ≈ real training signal; answers
  that drift ≈ confabulation (discount them).
- **Same prompts on all three models** — qwen3.6-27, gpt-oss-20b, devstral-small-2. The
  decision hinges on whether they **converge** (adopt one API for everyone) or **diverge**
  (per-model tool profiles).
- **Don't lead the witness.** The prompts deliberately avoid kvit-coder's vocabulary
  (`Edit`, `start_line`, `end_line`, `Edit.confirm`, "lines mode"). If you reword, keep them
  neutral — naming our tools makes the model anchor on them instead of its training.
- **Weight behavior over words.** Treat every answer as a hypothesis, then cross-check against
  the behavioral tells already observed (listed at the bottom).

## Standing preamble — prepend to EVERY prompt

Without an explicit frame, the models answer about *general* file editing and drift to shell
(qwen answered P7 entirely in `sed -i`). Anchor every prompt with this line so the answer
reflects its **agentic** tool training, not Unix CLI:

```
You are operating as an autonomous coding agent inside your preferred agentic harness, and you
edit files only through that harness's structured tool/function calls — never a terminal,
shell, or commands like sed/awk/cat. Answer purely in terms of those structured tool calls.
```

(The shell drift is itself a signal — qwen fell back to `sed`/`printf` in the benchmark when
the structured edit tool was unfamiliar. Note it if it happens, then re-ask *with* the
preamble to get the structured-tool answer.)

---

## Section 1 — Open elicitation (what it reaches for unprompted)

**P1 — name the tools**
```
When you work as a coding agent and need to modify files, which file-editing tools/functions
do you use? For each one, give the exact tool name, its parameters (names and types), and a
one-line description of what it does. List them the way you'd see them in a tool/function
schema.
```

**P2 — first reach**
```
You need to change one line inside an existing source file. What is the very first tool call
you reach for? Write it out exactly — tool name and all arguments — as you would actually
emit it.
```

---

## Section 2 — Concrete generation (the strongest signal)

Make it *do the task* with its ideal tools, rather than describe them in the abstract.

**P3 — single file, mixed operations**
```
Assume you have your ideal file-editing tools — the ones you are most fluent with. Task, in
the file src/app.py:
  - replace the body of calculate() (it currently spans 4 lines) with a single line: return 42
  - add a logging line right after the function's opening line
  - remove an unused import at the top of the file
Write the EXACT tool call(s) you would make — exact tool names, all parameter names and
values, in the exact format/syntax you'd naturally use. Do it in one call or several —
whatever you would actually do. Do not explain; just emit the calls.
```

**P4 — multiple files (the multi-edit / batching probe)**
```
Assume your ideal file-editing tools. You need to rename the function oldFunc to newFunc at
its one call site in each of three files: handlers/a.py, handlers/b.py, and util/c.py.
Write the EXACT tool call(s) you would make to do all three. Emit exactly what you'd send —
nothing else.
```

---

## Section 3 — Targeted tells

**P5 — apply vs. confirm (the biggest tell)**
```
When you call a file-editing tool, what do you expect to happen? Specifically: is the change
applied immediately and the result returned to you, or do you expect a separate confirmation
step (some kind of confirm/approve action) before it takes effect? Answer plainly, then say
which behavior you were trained to expect.
```

**P6 — one call or many (multi-edit preference)**
```
When you need to change several places at once — multiple spots in one file, or across
several files — do you prefer a single tool call that carries all the edits, or one call per
edit? Show the exact shape of the call(s) you'd use for two edits in the same file.
```

**P7 — insert vs. replace vs. delete**
```
Using your harness's structured edit tool(s) — not shell/sed — show exactly how you express
each of these three operations, as separate concrete tool calls:
  (a) replace existing line(s) with new content
  (b) insert new line(s) without removing anything
  (c) delete line(s) entirely
If these are the same tool or different tools, do it however you actually would.
```

**P8 — blank a line vs. remove it**
```
With your harness's structured edit tool (not shell/sed), show two tool calls:
  (1) make an existing line empty but keep the line in place
  (2) remove the line entirely so it no longer exists
Emit both calls exactly.
```

**P9 — how do you locate the edit (line numbers vs. content)**
```
Working through your harness's structured edit tool (not shell), when you target where an edit
goes, do you identify the location by line number, by quoting the existing text to be changed,
by a diff/patch with context, or some other way? Which do you find most reliable, and which
were you trained on? Show one tool call in your most-preferred form.
```

---

## Section 4 — Comparative (forced ranking)

**P10 — rank the known formats**
```
Rank these file-editing approaches from most to least fluent for you, and briefly say why:
  (1) search-and-replace: provide the exact old text and the new text
  (2) line-addressed: provide line numbers and the replacement text
  (3) unified-diff / patch: provide a patch with context lines and +/- markers
  (4) whole-file rewrite: provide the entire new file contents
Then: which single approach would make you most reliable, if you could only have one?
```

---

## What to look for (interpretation guide)

Map answers → design decisions, but only when they **agree with the behavioral tells**:

| If the model says… | It points to… |
|--------------------|---------------|
| "applied immediately, no confirm" (P5) | drop the preview/confirm gate for this model (auto-apply mode) |
| "one call with all edits" / batches in P3–P4, P6 | build the multi-file **MultiEdit** (Improvement 4) |
| "search/replace, old text → new text" (P9, P10) | line-number paradigm is off-distribution → searchreplace mode or per-model profile |
| insert / replace / delete shown as **distinct** ops (P7) | explicit Insert / Replace / DeleteLines split (Improvement 4) |
| `""`/empty = blank, distinct delete call (P8) | confirms the Improvement-3 direction |
| line numbers least reliable (P9, P10) | reinforces not relying on `start_line` reproduction |

**Convergence vs. divergence:** if qwen, 20b, and devstral land on the *same* API → adopt it
globally. If they split → per-model tool profiles (the harness already advertises tools via
`registry.Specs()`, so the edit family can be swapped per model).

## Behavioral tells already observed (cross-check against these)

From `benchmark-qwen3.6-27-20260612` + interrogation logs, qwen actually:
- **never calls confirm**, tries to "verify" after editing → expects **auto-apply**.
- emits **parallel/batched edit calls** (one per site) → expects a **multi-edit** affordance.
- **omits the end-of-range** argument → expects replace without explicit line-range end markers.
- reaches for **empty text to blank** a line → search/replace mental model.

If the chat answers match this cluster, confidence is high and it reads as a
**`str_replace` + `MultiEdit`, auto-applied** editor (Claude-Code / opencode family). If the
answers contradict the behavior, trust the behavior.

## Log your results

For each (model, prompt, run) capture: the verbatim answer, and a one-word tag for the tell
(`auto-apply`, `batch`, `str-replace`, `line-num`, `patch`, `split-ops`, …). A quick table:

| Model | Prompt | Run | Key tell | Notes |
|-------|--------|-----|----------|-------|
| qwen3.6-27 | P5 | 1 | | |
| … | | | | |
