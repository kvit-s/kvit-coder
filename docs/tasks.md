# Tasks

The `Tasks.*` tools let the model mark a stretch of work whose intermediate
steps do not need to stay in the conversation. Exploration, trial tests, log
analysis, debugging: many tool calls in, one summary out. File changes made
inside the stretch are reviewed as a diff at the end and kept or discarded
as a unit.

This page covers what the tools are, how a task is supposed to run, and how
the machinery underneath works. The one-line version is in
[`tools.md`](tools.md).

## The tools

Five tools, registered together when `tools.tasks.enabled` is true
(`internal/tools/setup.go`). Internal names use dots; the model sees
underscores (see naming below), so `Tasks.Start` below is `Tasks_Start` in
a transcript.

| Tool | Arguments | What it does |
|------|-----------|--------------|
| `Tasks.Start` | `task` (string, required): what the stretch of work is for | Opens a task: records a checkpoint of the workspace, notes the Start on the parent branch, and moves to a new branch for the work |
| `Tasks.Finish` | `summary` (string, required): what came out of it; `success` (boolean, default true): false when the goal was not reached | Closes the task with a summary and the diff since the Start checkpoint. A large diff arrives truncated to a head+tail preview with the full text in `diff_file` for review. With changes, the next call must be Accept or Decline; without changes, work continues normally |
| `Tasks.AcceptDiff` | none | Keeps the file changes. The workspace already holds them; the call only records the decision |
| `Tasks.DeclineDiff` | none | Discards the file changes by rolling the workspace back to the pre-task checkpoint, then records the decision |
| `Tasks.RevertFile` | `path` (string, required) | Puts one file back to its state at the start of the conversation, via the checkpoint manager |

Older sessions may show a sixth name, `Tasks.RevertToTaskStart`. It no
longer exists in this tree.

## Expected use

The lifecycle is Start, work, Finish, then Accept or Decline:

1. **Start before the disposable work, not after it.** The call belongs at
   the point where the model plans several steps whose outputs matter only
   as a summary: reading around a codebase to answer a question, filtering
   logs down to findings, trying things to locate a bug. The `task`
   argument names the goal in one line.
2. **Do the work inside the task.** Reads, shells, edits, test runs —
   everything between Start and Finish lands on the task branch.
3. **Finish with a summary, even on failure.** `Tasks.Finish` takes the
   condensed result (`summary`, required even when `success` is false) and
   returns the diff of workspace changes plus what to do next. That summary
   plus diff is what survives; the individual steps do not need to. A diff
   past 150 lines / 24KB arrives truncated (preview plus `diff_file`,
   `diff_truncated`, totals and a `hint` naming the file) — Read the file
   before Accept/Decline when the preview is not enough.
4. **Resolve the diff before anything else.** When Finish reports changes,
   every tool except `Tasks.AcceptDiff` and `Tasks.DeclineDiff` is refused
   until one of them is called. `Tasks.Start` of a next task is refused
   too. When Finish reports no changes, there is nothing to resolve.

Reach for Tasks when the middle of the work is expendable and the end is
what the conversation needs: searching logs or data, exploring code to
answer a question, debugging through many checks to one root cause. Do not
reach for it for a single read or a single edit, or to track a to-do list —
it is context compression, not plan management, and it holds at most the
state the predicates below derive (open task, pending diff, depth), never
a list of steps.

A worked example of the lifecycle going wrong — exploration outside the
task, one late Start, implementation inside, no Finish — is kept next to
its transcript in `spec/notes/task-use-example.md`.

## Architecture

### Where the state lives

One operating-system process runs per turn, so everything a task needs in
its next turn is on disk. Enabling Tasks adds one subdirectory to the
session layout in [`sessions.md`](sessions.md):

```
~/.kvit-coder/sessions/<name>/
    history.jsonl   the session transcript (unchanged)
    tasks/          a small git repository holding turns.jsonl on branches "0", "1", ...
```

`internal/context.Manager` owns that repository (`cmd/kvit-coder/turn.go`
builds it at `<session-dir>/tasks`). Branch `0` is the starting point.
`Tasks.Start` appends the Start assistant/tool pair to `turns.jsonl` on the
parent branch, commits, and checks out a new numbered branch for the work.
`Tasks.Finish` appends the Finish pair on the task branch, commits,
switches back to the parent (`ParentBranch` is branch number minus one),
and *replaces* the Start tool result there with the summary plus diff, so
the parent keeps one condensed entry where the whole stretch was. Accept
and Decline append their own pair on the parent and commit. The task
branch with its detailed steps remains in the git history but is no longer
current — that is the compression: the live conversation holds the
summary, the steps stay reachable in the repo.

### Checkpoints do the file side

`Manager` holds no file contents itself; it delegates to the checkpoint
manager that owns the shadow git repository in `checkpoints/`. Start
creates a checkpoint named `checkpoint_<turn>` *before* any work, and its
id travels inside the Start tool result (`_internal.checkpoint_id`).
Finish diffs the workspace against it (`GetDiffSinceCheckpoint` →
`checkpoint.Diff`), which decides `has_changes` and the pending-diff gate.
Accept changes nothing on disk — the files are already as the task left
them. Decline rolls back to the checkpoint (`RollbackToCheckpoint` →
`checkpoint.Restore`). `Tasks.RevertFile` restores one file to the start
of the conversation (`RestoreFileToStart` → `checkpoint.RestoreFile`).

### Predicates and the one gate

All decisions are derived from the current branch's `turns.jsonl` on each
call (`internal/context/manager.go`):

- `HasUnfinishedTaskInHistory` — the newest `Tasks.Start` tool result
  still says `task started` (a finished one was replaced with the summary).
- `GetTaskCheckpoint` — the checkpoint id of the open task, if any.
- `GetPendingDiff` — set only after a Finish whose replaced Start entry
  carries `has_changes`, until Accept or Decline lands.
- `GetTaskDepth`, `CountTurnsSinceTaskStart` — nesting depth and assistant
  turns since the open Start, for warnings.
- `CanExecuteTool` — the single enforcement point: while a pending diff
  exists, only Accept and Decline may run. There is deliberately no wider
  lock: during a long open task every other tool stays available, and
  `Report` has no unfinished-task check, so a turn can currently be
  accepted with a task still open.

Each tool's `Check` applies the predicate that fits it: Start refuses on a
pending diff, Finish refuses outside a task or on an empty summary,
Accept/Decline refuse without a pending diff, RevertFile refuses on a
pending diff or an empty path.

### The `[turn_N]` prefix and runtime notices

Every non-empty assistant message is stamped before storage
(`Runner.processLLMResponse` → `Middleware.ProcessAssistantMessage`,
`internal/context/middleware.go`). The prefix `[turn_N]` is
`Manager.GetTurnCount()+1`, and `GetTurnCount` counts assistant-role
entries in `tasks/turns.jsonl` — not lines in `history.jsonl`, not LLM
iterations. Normal turns never append there; only Start, Finish, Accept
and Decline do. So a session with one Start shows `[turn_1]` on everything
before it and a frozen `[turn_2]` on everything after, however many tool
calls follow. Dozens of identical `[turn_2]` lines plus a repeated
`<runtime-notice>` are one open task stamping each message, not repeated
task calls.

The notice appended after the prefix fires only inside an open task.
Turn counts are real responses produced inside the task in this turn, not
`Tasks.*` pairs in the repo (which stay near zero while the model works, so
repo-derived counts never fire — the 7xmo86 failure):

- past `task_warn_turns` responses (default 5), a nudge to summarize;
  past `task_critical_turns` (default 10), a stronger one;
- near the loop's hard turn limit (`agent.max_tool_iterations`): once the
  task is long or 10 or fewer iterations remain, the notice names the
  position (`Iteration X of Y (hard turn limit in Z)`) and tells the model
  to finish with `Tasks.Finish` before the budget runs out;
- nesting depth past `max_nested_depth` (default 2);
- when `notify_file_changes` is on (default true),
  `<n> files modified in this task` from the checkpoint manager's
  `GetModifiedFiles`.

Counters are per turn process (one OS process per turn), so a task carried
into a new turn restarts its count there.

### Transcript lines

The call line (`Tasks.Start[task="..."]`) prints like any other tool, but
tool results stay silent on success by design — so each Tasks call also
leaves a one-line `[tasks]` outcome right after it
(`tasksTranscriptSummary` in `internal/agent/runner_tools.go`): the Start
names branch, parent and checkpoint (`[tasks] started branch 1 (parent 0,
checkpoint_197): ...`), the Finish names the summary, `has_changes` and
the diff size, Accept/Decline record the decision, and RevertFile names
the path. Failures already print as errors, so the lifecycle reads
call-by-call in the terminal transcript without verbose mode.

### Naming across the provider boundary

Provider function names must match `^[A-Za-z0-9_-]+$`, while the registry
groups with dots (`Tasks.Start`, `Shell.advanced`). `Registry` therefore
advertises the sanitized alias (`Specs()`), resolves calls back through
`InternalName`, and rewrites dotted names in prompt text
(`RewriteToolNamesForPrompt`), so the system prompt's `Tasks.Start`
example and the spec's `Tasks_Start` are the same tool. The tool
documentation the model actually reads is `TasksStartTool.PromptSection`
in `internal/tools/context_tools.go`; the other four tools document
through it.

## Configuration

```yaml
tools:
  tasks:
    enabled: true             # registers the five tools (needs the tasks manager)
    task_warn_turns: 5        # nudge to finish past this many turns in a task
    task_critical_turns: 10   # stronger warning past this many
    context_capacity_warn: 80 # reserved; no warning reads it yet
    max_nested_depth: 2       # warn past this nesting depth
    notify_file_changes: true # append "N files modified in this task"
```

`config.example.yaml` leaves Tasks out (off for a fresh install).
`configuration.md` lists every key above under `tools`.
