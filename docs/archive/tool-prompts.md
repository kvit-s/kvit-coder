# Tool preference probe — prompts and model answers

Consolidated June 2026 from eight files: the probe itself (`tool-prompts.md`)
and seven files of raw model answers (`tool-prompts-p1.md` through `p7.md`).

The question behind it was whether kvit-coder's edit tools should be reshaped to
match what qwen3.6-27, gpt-oss-20b and devstral-small-2 appeared to have been
trained on, after benchmark runs showed those models inserting by default,
batching edits, passing `""` to blank a line, and skipping the confirm
handshake. The probe asked each model, with no tools attached, to describe and
write out the file-editing calls it reaches for.

It is kept for the record. The program no longer targets those models, and
`docs/redesign.md` §11 replaces the per-model tool shaping this was gathering
evidence for with two profiles.

---

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


---

# Answers — probe 1

When you work as a coding agent and need to modify files, which file-editing tools/functions
do you use? For each one, give the exact tool name, its parameters (names and types), and a
one-line description of what it does. List them the way you'd see them in a tool/function
schema.

A1:----------------

As an AI, I don't have native filesystem access; I operate through the tool registry provided by the host platform or agent framework. When integrated into a standard coding agent environment (e.g., MCP, OpenAI Function Calling, or similar), these are the exact file-editing tools I use, formatted as a tool/function schema:

```
Tool: read_file
Parameters: { file_path: string, encoding?: string }
Description: Reads and returns the full text contents of a file at the specified path.

Tool: write_file
Parameters: { file_path: string, content: string, encoding?: string }
Description: Creates a new file or completely overwrites an existing file with the provided content.

Tool: apply_patch
Parameters: { file_path: string, patch: string, patch_format?: string }
Description: Applies a unified diff to an existing file, modifying only the targeted lines without rewriting the entire file.

Tool: edit_file
Parameters: { file_path: string, old_string: string, new_string: string, max_lines?: number }
Description: Replaces a specific string block in a file with new content for precise, context-aware surgical edits.
```


A2:----------------

As an AI coding agent, my file-manipulation capabilities are provided by the host environment or framework. In standard coding-agent workflows, I operate against the following tool schema:

```
tool: read_file
parameters:
  - name: file_path
    type: string
description: Reads and returns the complete text content of a specified file.

tool: write_file
parameters:
  - name: file_path
    type: string
  - name: content
    type: string
description: Creates a new file or completely overwrites an existing file with the provided content.

tool: apply_str_replace
parameters:
  - name: file_path
    type: string
  - name: old_string
    type: string
  - name: new_string
    type: string
description: Replaces the first exact match of old_string in a file with new_string.

tool: insert_text_at_line
parameters:
  - name: file_path
    type: string
  - name: line_number
    type: integer
  - name: content
    type: string
description: Inserts text at a specific 1-indexed line, shifting all subsequent lines downward.

tool: delete_file
parameters:
  - name: file_path
    type: string
description: Permanently removes a file from the filesystem.

tool: search_files
parameters:
  - name: pattern
    type: string
  - name: path
    type: string (optional)
description: Returns file paths matching a glob or regex pattern within a directory tree.

tool: list_directory
parameters:
  - name: path
    type: string
description: Returns a flat list of files and subdirectories contained in the specified path.
```

*Note: Actual tool availability, naming conventions, and parameter constraints depend on the specific agent framework or IDE environment (e.g., Aider, Cursor, Continue, GitHub Copilot Workspace, or custom MCP/Function-Calling setups). The schema above reflects the standard set I consistently use for reliable, atomic file editing.*


A3:--------------------
Here is the standard function-calling schema I interface with when deployed in coding agent frameworks. Each entry follows the OpenAI/Anthropic tool schema convention:

```json
[
  {
    "name": "read_file",
    "description": "Reads and returns the complete contents of a specified file.",
    "parameters": {
      "type": "object",
      "properties": {
        "file_path": { "type": "string" }
      },
      "required": ["file_path"]
    }
  },
  {
    "name": "write_file",
    "description": "Creates a new file or overwrites an existing one with the provided content.",
    "parameters": {
      "type": "object",
      "properties": {
        "file_path": { "type": "string" },
        "content": { "type": "string" }
      },
      "required": ["file_path", "content"]
    }
  },
  {
    "name": "apply_patch",
    "description": "Applies a unified diff patch to precisely modify specific lines in an existing file.",
    "parameters": {
      "type": "object",
      "properties": {
        "file_path": { "type": "string" },
        "patch": { "type": "string" },
        "encoding": { "type": "string" }
      },
      "required": ["file_path", "patch"]
    }
  },
  {
    "name": "replace_in_file",
    "description": "Replaces exact string matches in a file with new content, optionally limiting the number of replacements.",
    "parameters": {
      "type": "object",
      "properties": {
        "file_path": { "type": "string" },
        "old_text": { "type": "string" },
        "new_text": { "type": "string" },
        "count": { "type": "integer" }
      },
      "required": ["file_path", "old_text", "new_text"]
    }
  },
  {
    "name": "list_directory",
    "description": "Returns a structured list of files and subdirectories within a specified path.",
    "parameters": {
      "type": "object",
      "properties": {
        "path": { "type": "string" },
        "recursive": { "type": "boolean" }
      },
      "required": ["path"]
    }
  },
  {
    "name": "search_files",
    "description": "Finds files or file contents matching a glob pattern or regular expression.",
    "parameters": {
      "type": "object",
      "properties": {
        "pattern": { "type": "string" },
        "path": { "type": "string" },
        "regex": { "type": "boolean" }
      },
      "required": ["pattern"]
    }
  }
]
```

**Note:** As a language model, I don't execute filesystem operations natively. When integrated into an agentic environment, I generate structured calls to these schemas, and the host executor (e.g., Python runtime, sandbox, or IDE backend) performs the actual I/O. The parameter types, names, and descriptions above reflect the canonical interface used across modern coding agent frameworks.

---

# Answers — probe 2

You need to change one line inside an existing source file. What is the very first tool call
you reach for? Write it out exactly — tool name and all arguments — as you would actually
emit it.


A1:-------------------
edit_file(path="src/main.py", old_string=" original_line", new_string=" updated_line")

A2:-------------------
call_tool(tool_name="str_replace_editor", arguments={"command": "str_replace", "path": "./src/main.py", "old_str": "    return False", "new_str": "    return True"})

A2:-------------------
read_file(path="path/to/file.ext")
(in reasoning I see it did the following:  In typical AI agent/tool-use scenarios, before modifying a file, I usually need to *read* it first to understand the context, find the exact line, and ensure the change is correct.)

---

# Answers — probe 3

Assume you have your ideal file-editing tools — the ones you are most fluent with. Task, in
the file src/app.py:
  - replace the body of calculate() (it currently spans 4 lines) with a single line: return 42
  - add a logging line right after the function's opening line
  - remove an unused import at the top of the file
Write the EXACT tool call(s) you would make — exact tool names, all parameter names and
values, in the exact format/syntax you'd naturally use. Do it in one call or several —
whatever you would actually do. Do not explain; just emit the calls.

A1:------------------
str_replace_editor(command="replace", path="src/app.py", old_str="import unused_module\n\n\ndef calculate():\n existing_line_1\n existing_line_2\n existing_line_3\n existing_line_4", new_str="\n\n\ndef calculate():\n logging.info("calculate called")\n return 42")

A2:------------------
apply_patch(file="src/app.py", diff="--- a/src/app.py\n+++ b/src/app.py\n@@ -1,5 +1,4 @@\n-import unused_module\n import os\n\n@@ -10,7 +9,7 @@\ndef calculate():\n- # original line 1\n- # original line 2\n- # original line 3\n- # original line 4\n+ logging.info("calculation started")\n+ return 42\n")

A3:------------------
{
  "tool": "file_replace",
  "parameters": {
    "path": "src/app.py",
    "old_str": "    # existing line 1\n    # existing line 2\n    # existing line 3\n    # existing line 4",
    "new_str": "    return 42"
  }
}
{
  "tool": "file_replace",
  "parameters": {
    "path": "src/app.py",
    "old_str": "def calculate():\n    ",
    "new_str": "def calculate():\n    logging.info(\"calculate called\")\n    "
  }
}
{
  "tool": "file_replace",
  "parameters": {
    "path": "src/app.py",
    "old_str": "import unused_module\n",
    "new_str": ""
  }
}

---

# Answers — probe 4

Assume your ideal file-editing tools. You need to rename the function oldFunc to newFunc at
its one call site in each of three files: handlers/a.py, handlers/b.py, and util/c.py.
Write the EXACT tool call(s) you would make to do all three. Emit exactly what you'd send —
nothing else.

A1:-------------
replace_in_file("handlers/a.py", "oldFunc", "newFunc")
replace_in_file("handlers/b.py", "oldFunc", "newFunc")
replace_in_file("util/c.py", "oldFunc", "newFunc")

A2:-------------
replace_in_file(file="handlers/a.py", old="oldFunc", new="newFunc")
replace_in_file(file="handlers/b.py", old="oldFunc", new="newFunc")
replace_in_file(file="util/c.py", old="oldFunc", new="newFunc")

A3:------------
search_replace(file="handlers/a.py", old="oldFunc", new="newFunc")
search_replace(file="handlers/b.py", old="oldFunc", new="newFunc")
search_replace(file="util/c.py", old="oldFunc", new="newFunc")

---

# Answers — probe 5

When you call a file-editing tool, what do you expect to happen? Specifically: is the change
applied immediately and the result returned to you, or do you expect a separate confirmation
step (some kind of confirm/approve action) before it takes effect? Answer plainly, then say
which behavior you were trained to expect.

A1:------------------
I expect the change to be applied immediately and for the tool to return a result (such as a success/failure status or the updated content) in the next step. I was trained to expect immediate application without a separate confirmation or approval step. Any confirmation workflow would be a platform-specific design choice, not the default pattern I'm optimized for.

A2:------------------
I expect the change to be applied immediately and the result or status returned to me. I was trained to expect immediate application with no separate confirmation step; any review, diff preview, or approval is handled by the host environment or user interface, not by the tool protocol itself.

A3:------------------
I expect the change to be applied immediately and the result returned to me so I can continue the task or report the outcome. I was trained to expect immediate execution with a returned result, not a separate confirmation step before the change takes effect.


---

# Answers — probe 6

When you need to change several places at once — multiple spots in one file, or across
several files — do you prefer a single tool call that carries all the edits, or one call per
edit? Show the exact shape of the call(s) you'd use for two edits in the same file.


A1:----------------
I prefer a **single tool call that carries all edits**. This keeps changes atomic, avoids intermediate invalid states, reduces round-trip latency, and makes error handling/rollbacks straightforward.

For two edits in the same file, I'd use this exact shape:

```json
{
  "tool": "apply_edits",
  "parameters": {
    "file_path": "src/utils.py",
    "edits": [
      {
        "start_line": 14,
        "end_line": 16,
        "new_content": "def safe_divide(a, b):\n    if b == 0:\n        return None\n    return a / b"
      },
      {
        "start_line": 42,
        "end_line": 42,
        "new_content": "    # Added caching for performance"
      }
    ]
  }
}
```

**Conventions assumed:**
- `start_line` / `end_line` are **1-indexed and inclusive**
- `new_content` replaces the exact span (use `""` for pure deletions)
- Internally, edits are applied **bottom-up** to preserve line offsets
- For cross-file changes, I'd extend the payload to a `files` array with the same structure per file, still in one call

This matches the pattern used by production coding assistants and LLM tool schemas because it's deterministic, minimizes tool overhead, and guarantees all related changes land together.

A2:-------------------
I strongly prefer a **single tool call that carries all edits**. It guarantees atomicity, reduces round-trip latency, prevents partial or conflicting state, and makes validation/rollback straightforward.

Here’s the exact shape I’d use for two edits in the same file (using a widely compatible `edit_file` convention):

```json
{
  "tool": "edit_file",
  "parameters": {
    "file_path": "src/config.ts",
    "edits": [
      {
        "match": "const TIMEOUT = 5000;",
        "replace": "const TIMEOUT = 3000;",
        "occurrence": 1
      },
      {
        "match": "export default { debug: false };",
        "replace": "export default { debug: true, verbose: true };",
        "occurrence": 1
      }
    ]
  }
}
```

**Why this shape:**
- `match` uses exact literal text (or supports regex/line ranges if the tool allows) to avoid ambiguous line-number drift.
- `occurrence` ensures predictable behavior when the same string appears multiple times.
- Grouping in `edits` lets the executor apply them in a single transaction, verify consistency, and fail fast if any replacement is ambiguous or conflicting.

If the underlying tooling only supports line-range or JSON-patch semantics, I adapt to that schema, but I always request batched, atomic edit payloads rather than sequential single-edit calls.


A3:--------------------
I prefer a **single tool call** that carries all the edits when the underlying tool supports it. This minimizes round-trips, ensures atomic application (all-or-nothing), and keeps the edit context consistent.

If the available tool accepts a batch of operations, here’s the exact shape I’d use for two edits in the same file:

```json
{
  "name": "apply_file_edits",
  "arguments": {
    "file_path": "src/utils.py",
    "edits": [
      {
        "start_line": 14,
        "end_line": 14,
        "new_text": "def validate_input(data: dict) -> bool:\n    return isinstance(data, dict) and len(data) > 0"
      },
      {
        "start_line": 32,
        "end_line": 33,
        "new_text": "    result = process(data, timeout=30)\n    return result"
      }
    ]
  }
}
```

**Notes on the shape:**
- `start_line`/`end_line` are typically 1-indexed and inclusive (varies by tool).
- `new_text` replaces the entire span from `start_line` to `end_line`.
- If the tool only supports one edit per call, I’d emit two identical-shaped calls with one item in `edits` each, but batched is strongly preferred for performance and reliability.
- Exact field names (`file_path`, `edits`, `start_line`, etc.) depend on the platform’s schema, but the structural pattern above is standard across modern file-editing tools.



---

# Answers — probe 7

Using your preferred file-editing tool(s), show exactly how you express each of these three
operations, as separate concrete calls:
  (a) replace existing line(s) with new content
  (b) insert new line(s) without removing anything
  (c) delete line(s) entirely
If these are the same tool or different tools, do it however you actually would.

A1:-----------------
I use **`sed`** (Stream Editor) for these operations. It's POSIX-compliant, universally available, and designed specifically for line-addressed text transformations. Below are the exact calls I'd run in a terminal, using GNU `sed` syntax (standard on Linux). I always include a backup suffix in practice (`-i.bak`), but I'll show the clean `-i` for clarity.

### (a) Replace existing line(s) with new content
```bash
sed -i '5,7c\// Updated configuration block\n// New settings go here' config.txt
```
**How it works:** `5,7` addresses lines 5 through 7. `c\` is the *change* command, which discards the matched lines and replaces them with the text that follows. `\n` inserts a literal newline in GNU `sed`.

### (b) Insert new line(s) without removing anything
```bash
sed -i '4i\# Inserted header before line 4' config.txt
```
**How it works:** `4` addresses line 4. `i\` is the *insert* command, which places the specified text **above** the addressed line(s). (Use `a\` to append *after* a line instead.)

### (c) Delete line(s) entirely
```bash
sed -i '10,12d' config.txt
```
**How it works:** `10,12` addresses lines 10 through 12. `d` is the *delete* command, which removes the addressed lines and continues processing the rest of the file.

---

**Practical notes on how I actually run these:**
- **Addressing flexibility:** Line numbers (`5,7`) are just one option. I often use regex (`/pattern/`), line ranges (`/^#/,/^$/`), or variables (`sed -i "${LINE_NUM}c\${NEW_TEXT}" file`).
- **Safety:** In scripts, I always use `sed -i.bak` (or `--in-place=numbered`) to keep backups. On macOS/BSD, the in-place flag requires `sed -i '' '...'` instead of `-i`.
- **Multi-line edits:** For complex replacements spanning dozens of lines, I switch to Python (`fileinput` module) or `ed` for stricter state control, but for atomic line operations, `sed` remains my default.

A2:--------------------
I use `sed` (the POSIX stream editor) for all three. Here are the exact, separate commands I’d run against a file named `config.txt`, targeting line 5 as the example address:

**(a) Replace existing line(s) with new content**
```bash
sed -i '5c\Replacement content here' config.txt
```

**(b) Insert new line(s) without removing anything**
```bash
sed -i '5i\New line inserted above' config.txt
```

**(c) Delete line(s) entirely**
```bash
sed -i '5d' config.txt
```

**Key details on how I actually use these:**
- `-i` edits the file in-place. (Omit it to preview changes to stdout without modifying the file.)
- `5` is the address. It can be a line number, a range (`3,5`), or a regex pattern (`/pattern/`).
- `c\`, `i\`, and `d` are the edit commands: **c**hange/replace, **i**nsert (before the address), and **d**elete.
- For multi-line inserts/replacements, I use literal `\n` sequences:  
  `sed -i '5c\Line 1\nLine 2' config.txt`
- **macOS/BSD note:** The built-in `sed` requires an explicit empty string for in-place editing: `sed -i '' '5d' config.txt`. I alias `sed` to `gsed` on macOS or use `-i ''` when portability is needed.

This is exactly how I script file edits in automation: deterministic, non-interactive, and universally available.

A3:-------------------------
I’ll use **GNU `sed`** (the standard Unix stream editor) for all three. It’s purpose-built for precise, line-oriented, in-place file manipulation and avoids the overhead of launching an interactive editor. Each command below is a complete, executable call targeting a specific line range (ranges can be adjusted to single lines like `3` or patterns like `/^# TODO/` as needed).

**(a) Replace existing line(s) with new content**
```bash
sed -i '2,4c\Replacement line 1\nReplacement line 2' file.txt
```
`2,4` addresses lines 2 through 4. `c\` is the *change* command, which removes the addressed lines and substitutes them with the following text. `\n` inserts a newline for multi-line content.

**(b) Insert new line(s) without removing anything**
```bash
sed -i '3i\Inserted line 1\nInserted line 2' file.txt
```
`3` addresses line 3. `i\` is the *insert* command, which places the specified text immediately before the addressed line. All existing lines remain intact and shift down.

**(c) Delete line(s) entirely**
```bash
sed -i '5,7d' file.txt
```
`5,7` addresses lines 5 through 7. `d` is the *delete* command, which removes the addressed lines completely from the file.

**Practical notes:**
- `-i` enables in-place editing. On macOS/BSD systems, use `-i ''` instead.
- Line ranges like `2,4` can be changed to a single number (e.g., `3`) for one-line operations, or to regex addresses (e.g., `/^# comment$/`) for content-based targeting.
- These are concrete, single-command calls that modify the file directly without launching an interactive editor or spawning subshells.