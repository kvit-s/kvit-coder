# thinkbench integration — implementation plan (Path B)

Detailed plan to add **thinkbench** as a first-class benchmark family in kvit-coder,
parallel to the existing haystack family. See `thinkbench.md` for the suite overview and
fit analysis. This document is the build spec.

## Goals & constraints (from requirements)

1. **Suite lives inside the repo** — not loaded from an external path.
2. **Hard workspace sandbox** — each agent run is confined to its own per-run subfolder,
   for **reads as well as writes**. The agent must not be able to see other tasks, the
   held-out graders, or the reference solutions.
3. **`uv`-managed Python** — instantiate a `uv` environment in the thinkbench folder and use
   it as the default interpreter for **all runs** (both the agent's `run_command` and grading).
4. **Path B** — faithful reproduction: continuous `passed/total` scores, by-type/by-task
   rollups, and the 12 observed (ungraded) tasks.

---

## 1. Architecture overview

Add a `thinkbench` family that mirrors `haystack`:

- New CLI entry: `--bench-thinkbench <suffix>` (+ `--bench-thinkbench-id`), wired in
  `cmd/kvit-coder/main.go` next to the haystack flags, selecting `config-<suffix>.yaml` the
  same way (`internal/benchmark/cli.go:Run` / `RunHaystack` are the templates).
- New files under `internal/benchmark/`:
  - `thinkbench_types.go` — task/result/report structs.
  - `thinkbench_loader.go` — read the in-repo suite (manifest + on-disk brief/setup).
  - `thinkbench_grader.go` — uv env management + drop-and-run `grade.py`, parse the JSON scorecard.
  - `thinkbench_executor.go` — per-task: sandboxed setup → agent loop → grade → result.
  - `thinkbench_report.go` — emit `results.json` + `RESULTS.md` (+ observed artifacts).
  - `thinkbench_cli.go` — `RunThinkbench(...)` orchestrator (resume, progress, output paths).
- Reuse as-is: `agent.Runner`, the progress tracker (`progress.go`), CSV resume (`csv.go`),
  and the token/cost aggregation helpers in `report.go`.

The existing YAML `BenchmarkDef` path is **not** reused — thinkbench tasks are read directly
from the suite directory tree to avoid lossy inlining.

---

## 2. In-repo suite layout

Vendor the unzipped suite under the repo, gitignored from the agent's reach by *physical
separation* (it is never inside a run workspace):

```
benchmarks/thinkbench/
  suite/                      # the unzipped thinkbench-eval-suite, committed
    manifest.json
    TASKS.md  README.md  LICENSE  NOTICE
    tasks/<slug>/
      brief.txt
      setup/                  # starter tree (non-greenfield only)
      grade.py                # HELD OUT — never copied into a run workspace pre-grade
      reference/              # HELD OUT — never copied in at all
  runs/                       # per-run sandboxed workspaces (gitignored)
    <slug>/run-<n>/...
  .uv/                        # the shared uv environment (gitignored)
  results/                    # results.json, RESULTS.md, OBSERVATIONS artifacts
```

- Add `benchmarks/thinkbench/runs/`, `benchmarks/thinkbench/.uv/`, and
  `benchmarks/thinkbench/results/` to `.gitignore`.
- The suite path is fixed/relative to the repo root (discovered like `FindBenchmarksFile`),
  honoring requirement #1. Add a `--thinkbench-suite` override only as an escape hatch.
- Keep Thinkwright's `LICENSE`/`NOTICE`; attribute on published results (Apache-2.0).

**Sandbox invariant:** a run workspace (`runs/<slug>/run-<n>/`) ever contains only
`brief`-derived context + the `setup/` starter tree. `grade.py` is copied in *after* the
agent stops and removed after grading; `reference/` is never copied. So even a full sandbox
escape finds nothing sensitive *inside* the workspace.

---

## 3. Sandbox design (reads + writes confined to the run subfolder)

kvit-coder already has the primitives — we compose three layers, weakest-to-strongest:

### Layer 1 — workspace root + `block` mode (in-harness, primary)

All file tools (`read*`, `edit*`, `glob`, `search`, `shell`) route through
`Config.CheckPathSafety` / `NormalizeAndValidatePath` against `cfg.Workspace.Root`
(`internal/config/config.go:432`, `internal/tools/path_utils.go`). With
`path_safety_mode: "block"` (`config.go:450`) any path resolving outside the root is **hard-
rejected, including reads** — no prompt, no warning.

Per run, set:
- `cfg.Workspace.Root = runs/<slug>/run-<n>/` (the executor already assigns
  `env.WorkspaceDir = cfg.Workspace.Root`, `cli.go:156`).
- `cfg.Workspace.PathSafetyMode = "block"` (force it for thinkbench regardless of the
  user's `config-<suffix>.yaml`, so a run can't be silently un-sandboxed).
- Clear `cfg.Workspace.AllowedPaths` / `AllowedReadPaths` / `AllowOutsideWorkspace`.

This covers the **structured** file tools completely.

### Layer 2 — physical separation (defense in depth)

As above: held-out `grade.py`/`reference/`/other tasks live under `suite/`, never inside the
run workspace. The suite dir is *outside* `cfg.Workspace.Root`, so Layer 1 blocks it; and even
if bypassed there is nothing sensitive in the workspace to read.

### Layer 3 — OS-level sandbox for `run_command` (hard guarantee, recommended)

The shell tool's path safety is *best-effort string parsing* of the command, and
`executeCommand` runs `sh -c` with `cmd.Dir = workDir` but **does not set `cmd.Env`**
(`internal/tools/shell.go:260`), so a crafted command (`python3 -c "open('/etc/...')"`,
`cd /`, symlinks) can read outside the root despite Layer 1. For a true "even reads" guarantee
on arbitrary shell, wrap the agent's command execution in an OS sandbox restricted to the run
workspace (+ the uv env, read-only):

- Preferred: **bubblewrap** (`bwrap --ro-bind <uv> ... --bind <workspace> ... --chdir <workspace> --unshare-all`).
- Alternatives: `firejail`, a throwaway container, or a per-run unprivileged user + `chroot`/bind-mounts.

Implementation hook: thread an optional **command wrapper** + **env** through the executor into
the shell tool. This needs two small, additive changes to `internal/tools/shell.go`:
1. Allow injecting `cmd.Env` (today it inherits the parent env) — required anyway for uv (§4).
2. Allow an optional exec prefix/wrapper (e.g. `bwrap ... -- sh -c <cmd>`).

Layer 3 is **on by default**: `thinkbench.sandbox: "auto"` (the default) auto-detects the
strongest wrapper available on the host at preflight and uses it for every agent `run_command`.
Detection order: **bubblewrap (`bwrap`) → `firejail` → none**. If none is found, the harness
falls back to Layers 1+2 and prints a loud warning that arbitrary-shell reads are not airtight
(and refuses to fall back silently only if `sandbox: "require"` is set — a strict mode that hard-
fails when no OS sandbox is available). Explicit `sandbox: "bwrap" | "firejail" | "none"` pins a
specific choice (`"none"` disables Layer 3 intentionally).

**Net:** Layers 1+2 are sufficient for honest agents and block all structured-tool reads;
Layer 3 closes the arbitrary-shell hole and is what makes "restricted even for reads" a hard
guarantee.

---

## 4. `uv` environment design

Requirement: one `uv` environment in the thinkbench folder, used as the default interpreter for
all runs (agent shell + grader). The graders are stdlib-only, so the env mainly guarantees a
known `python3` and isolates anything the agent `pip install`s during `implement`/`repair` tasks.

### Provisioning (once per benchmark invocation, at preflight)

1. Preflight: ensure `uv` is on PATH (else fail with an install hint).
2. Create a shared env under `benchmarks/thinkbench/.uv/` via `uv venv benchmarks/thinkbench/.uv`
   (Python pinned, e.g. `uv venv --python 3.11`). Created once; reused across all runs (this is
   the "use it as default for all runs" reading — one env, many runs — which also avoids paying
   `uv venv` per run).

### Making it the default interpreter without breaking the sandbox

The venv lives **outside** the per-run workspace (so it isn't listable/readable as a path under
the root, and isn't wiped by the per-run clean). The agent must use it *without naming an outside
path string* (which `block` mode would reject). Do this by **environment injection**, not path
references:

- Set, for every agent `run_command` and for the grader process:
  - `VIRTUAL_ENV = benchmarks/thinkbench/.uv`
  - `PATH = <.uv>/bin:$PATH`
  - `UV_PROJECT_ENVIRONMENT = <.uv>` (so `uv run`/`uv pip` target it)
- Then `python`, `python3`, and `pip` resolve to the uv env purely through `PATH` — the agent
  types `python3 foo.py`, no outside path appears in the command string, Layer 1 stays intact.

This requires the `cmd.Env` injection change in `internal/tools/shell.go` noted in §3. Thread a
`map[string]string` (or full env slice) from the thinkbench executor → shell tool. For normal
(non-thinkbench) runs, env stays nil = inherit parent (unchanged behavior).

- The **grader** is invoked by the harness (not the agent) as
  `<.uv>/bin/python grade.py` with `cwd = run workspace` and the same injected env.
- If Layer 3 (bwrap) is on, bind-mount `<.uv>` **read-only** into the sandbox so the agent can
  execute the interpreter but not mutate the shared env; per-run `pip install`s then need either
  a per-run overlay or an allowance — default to read-only env + per-run `--target` installs into
  the workspace. (Document this tradeoff; most graded tasks are stdlib-only.)

---

## 5. Data model (`thinkbench_types.go`)

```go
type TBType string // "implement" | "bug-fix" | "feature-add" | "repair-to-green" | "ambiguous-spec"

type TBTask struct {
    Slug      string   // from manifest
    Type      TBType
    Brief     string   // brief.txt contents
    Observed  bool     // ambiguous-spec → true (ungraded)
    NumChecks int      // from manifest (0 for observed)
    HasSetup  bool
    SetupDir  string   // abs path to suite/tasks/<slug>/setup (copied in)
    GraderPath string  // abs path to suite/tasks/<slug>/grade.py (held out)
}

// One graded run's scorecard (parsed from grade.py stdout).
type TBScorecard struct {
    Task      string       `json:"task"`
    ImportOK  bool         `json:"import_ok"`
    Passed    int          `json:"passed"`
    Total     int          `json:"total"`
    Score     float64      `json:"score"`
    Checks    []TBCheck    `json:"checks"`
}
type TBCheck struct{ ID, Desc, Detail string; Passed bool }

// Per-run record (graded or observed).
type TBRunResult struct {
    Slug, Type   string
    Run          int
    Observed     bool
    Score        float64   // 0..1; 0 for observed
    Passed, Total int
    FullPass     bool      // Score == 1.0
    ImportOK     bool
    // metrics reused from agent stats:
    DurationMS   int64
    Tokens, PromptTokens, GeneratedTokens, CachedTokens, ContextUsed int
    Cost         float64
    Errors       []string
    WorkspaceDir string    // kept for observed tasks / failures
}
```

**Scoring/`RunResult` decision:** rather than overload the existing
`benchmark.RunResult` (binary), thinkbench gets its own `TBRunResult` with a continuous
`Score`. Reuse CSV plumbing by adding `score`, `passed`, `total`, `full_pass`, `observed`
columns. Keep `FullPass` as the binary "solve" analogue so existing aggregation idioms map over.

---

## 6. Loader (`thinkbench_loader.go`)

- `LoadThinkbenchSuite(suiteDir) ([]TBTask, error)`:
  - Read `manifest.json` (array of `{slug,type,brief,observed,num_checks,has_setup}`).
  - For each task, resolve `SetupDir` and `GraderPath`; load `brief.txt` from disk (prefer the
    on-disk brief over the manifest copy to avoid drift).
  - Validate: graded tasks (`num_checks>0`) must have a readable `grade.py`; observed tasks must
    not be graded.
- `FilterThinkbench(tasks, ids, types)` — mirror `FilterBenchmarks` (by slug and/or type).
- `FindThinkbenchSuite(repoRoot)` — check `benchmarks/thinkbench/suite/` (+ `--thinkbench-suite`).

---

## 7. Executor (`thinkbench_executor.go`)

The run loop is `for run := 1; run <= N; run++ { for each task { ... } }` — i.e. **N trials per
task**, identical to the existing harness (`runner.go:87`). `N` is set by `-n` (see §9). Per
`(task, runID)`:

1. **Sandbox setup.**
   - `workspace = benchmarks/thinkbench/runs/<slug>/run-<n>/`; clean + recreate (extend
     `Environment.SetupBenchmark` with a directory-tree copy: today it only writes inline
     `Setup` content; add `copyTree(task.SetupDir, workspace)` for non-greenfield tasks).
   - Override config for this run: `cfg.Workspace.Root = workspace`,
     `PathSafetyMode = "block"`, clear allow-lists (§3 Layer 1).
2. **Agent loop.** Build messages `[system, user=task.Brief]` and run `agent.Runner.Run`
   (same as `Executor.Execute`, `executor.go:79`), with:
   - injected env (uv, §4) + optional OS-sandbox wrapper (§3 Layer 3) threaded into the shell tool;
   - a thinkbench timeout (config; default well above 120s — `implement` tasks averaged ~150s
     in the example run).
3. **Grade (skip if `task.Observed`).**
   - Copy `task.GraderPath` → `workspace/grade.py`.
   - Run `<.uv>/bin/python grade.py` with `cwd=workspace` + injected env; capture stdout.
   - Parse `TBScorecard` (JSON, last line of stdout to be safe). On parse/exec failure →
     `Score=0, ImportOK=false`, record stderr in `Errors` (grader is contracted to exit 0 and
     never raise, so a non-JSON result is a harness/agent-env problem).
   - Remove `grade.py` from the workspace.
4. **Observed tasks.** No grader. Persist the workspace under `results/observed/<slug>/run-<n>/`
   and record only run metrics (for later qualitative comparison / optional LLM-judge, out of scope v1).
5. **Result.** Populate `TBRunResult` (metrics from `agentResult.Stats` as in `executor.go:119`).
   Keep the workspace on grading failure for debugging; clean on full-pass.

External-command mode (`cfg.LLM.BenchmarkCmd`) can be supported the same way as
`executeExternalCommand` (run external agent in the sandboxed workspace, then grade) — useful
for benchmarking other agent CLIs. Mark as a phase-2 nicety.

---

## 8. Reporting (`thinkbench_report.go`)

Reproduce the example bundle so numbers are directly comparable:

- `results.json` with: `run_ids`, `models`, `trials`, `num_graded_tasks`, `num_observed_tasks`,
  `num_runs`, `config` (provider/endpoint/pricing from `config-<suffix>.yaml`), and rollups
  `overall_graded`, `by_type`, `by_task`, `observed` — each keyed by model with
  `{n, solved, solve_rate, mean_score, avg_secs, avg_tokens, cached_frac, total_cost_usd}`.
  - `mean_score` = mean of `Score`; `solved`/`solve_rate` = count/fraction with `FullPass`.
- `RESULTS.md` — overall table, by-type table (mean score / full-pass%), per-task mean-score
  table (over trials). Mirror the headings in the example `RESULTS.md`.
- Observed: write an `OBSERVATIONS` index pointing at the saved `results/observed/<slug>/...`
  workspaces; no scores. (Auto qualitative diff is out of scope for v1.)

Reuse the token/cost/latency/percentile math already in `report.go`; the new axes are
score-weighting and grouping by `Type`.

---

## 9. CLI & config wiring

### Run count (trials) — same UX as the other benches

`-n` controls **how many times each task runs through the benchmark** (trials per task), exactly
as it does for the tool and haystack benches: `cmd/kvit-coder/main.go` already defines
`benchmarkRuns := flag.Int("n", 10, ...)`, and the runner loops `RunsPerTask` times
(`runner.go:87`). Thinkbench reuses the **same `-n` flag** — `RunThinkbench` sets the loop bound
from it. So:

```bash
kvit-coder --bench-thinkbench <suffix> -n 3      # 3 trials per task
```

- Precedence: `-n` (if passed) overrides a `thinkbench.trials` config value; if neither is set,
  default to the harness default (10), or 3 to match the thinkbench example bundle — pick one and
  document it. (Recommend defaulting to **3** for thinkbench since its tasks are long/expensive,
  while still honoring an explicit `-n`.)
- `--no-resume` and CSV resume work per `(slug, run)` just like the other benches (`runner.go:99`,
  `csv.go`), so an interrupted N-trial sweep continues where it left off.
- `mean_score` and `solve_rate` in the report are aggregated **across the N trials** per task
  (and per type), mirroring the example `results.json` ("3 trials/model").

### Flags & orchestrator

- `cmd/kvit-coder/main.go`: add `--bench-thinkbench <suffix>`, `--bench-thinkbench-id`,
  `--thinkbench-suite` (optional), reuse `-n`, `-o`, `--no-resume`. Branch to
  `benchmark.RunThinkbench(...)` like the haystack branch.
- `internal/benchmark/thinkbench_cli.go: RunThinkbench(...)` — model on `RunHaystack`
  (`cli.go:207`): resolve output/terminal paths under the repo root, load suite, build the
  config (force `path_safety_mode=block`), preflight (uv + sandbox), provision the uv env, run
  all `(task × trials)` via the runner with resume/progress, then write reports.
- Config block in `config-<suffix>.yaml`:
  ```yaml
  thinkbench:
    suite_dir: "benchmarks/thinkbench/suite"   # optional; default discovered
    timeout_per_run: 600                          # seconds; implement tasks are slow
    uv_python: "3.11"
    sandbox: "auto"   # default: auto-detect bwrap→firejail→none. Also: "bwrap"|"firejail"|"none"|"require"
    types: ["implement","bug-fix","feature-add","repair-to-green"]  # exclude observed by default
    trials: 3            # default trials per task; overridden by -n on the command line
  ```
  Add a matching `ThinkbenchConfig` struct to `internal/config/config.go`.

---

## 10. Required changes to shared code (additive, low-risk)

1. `internal/tools/shell.go` — allow per-tool **env injection** (set `cmd.Env`) and an optional
   **exec wrapper** (OS sandbox prefix). Default nil → current behavior. Thread from the
   thinkbench executor only. *(Needed for uv default interpreter + Layer-3 sandbox.)*
2. `internal/benchmark/setup.go` — add `copyTree(src, dst)` and a setup path that copies an
   on-disk `setup/` tree (today only inline `Setup` content is supported).
3. `internal/config/config.go` — add `ThinkbenchConfig`; allow the harness to force
   `PathSafetyMode="block"` and clear allow-lists for a run.
4. `.gitignore` — ignore `benchmarks/thinkbench/{runs,.uv,results}/`.

No changes to existing benchmark/haystack behavior; all new code paths are behind the new flag.

---

## 11. Preflight checks

Before the first run, fail fast (or warn) on:
- `uv` present on PATH (hard fail with install hint).
- OS sandbox detection (`sandbox: "auto"`, the default): probe `bwrap` then `firejail`; record the
  chosen wrapper and `--version`. If none found → warn loudly and downgrade to Layers 1+2 (hard-fail
  instead if `sandbox: "require"`). If a wrapper is pinned but missing → hard-fail.
- Smoke-test the chosen wrapper once at preflight: run a trivial sandboxed command and assert a read
  of a path outside the run workspace is denied (proves the bind/mount config is correct before any task runs).
- Suite dir exists and `manifest.json` parses; every graded task has a `grade.py`.
- The uv env builds and `python -c "import sys"` works inside it.

---

## 12. Phasing / milestones

1. **M1 — loader + full sandbox (Layers 1–3) + uv (no grading), 1 graded task.** Run one
   `bug-fix` task end-to-end: sandboxed setup, agent loop with uv `python3` under the auto-detected
   OS sandbox, confirm both `block` mode (structured tools) **and** the OS wrapper (arbitrary shell:
   `cat ../grade.py`, `python3 -c "open('../grade.py')"`) deny reads outside the run workspace.
   Validates the hard parts (§3 all layers, §4) up front, since Layer 3 ships on by default.
2. **M2 — grader-runner + scoring.** Drop-and-run `grade.py`, parse scorecard, produce
   `TBRunResult`. Verify against the suite's own `reference/` solutions (each grader self-tests
   to 1.0 on its reference) as a harness sanity check.
3. **M3 — full run loop + resume + progress + CSV** across all 60 graded tasks × trials.
4. **M4 — reporting** (`results.json` + `RESULTS.md`); diff structure against the example bundle.
5. **M5 — observed tasks** (skip grading, persist workspaces, OBSERVATIONS index).
6. **M6 — external-command mode** (benchmark other agent CLIs in the sandboxed workspace) —
   optional extensibility. (Layer-3 OS sandbox is no longer here; it moved into M1 as a default.)

## 13. Testing

- **Harness sanity:** for each task, copy `reference/` into a workspace and run the grader → must
  score `1.0` (proves grader-runner + uv env are correct, independent of any model).
- **Sandbox tests:** assert that, under `block` mode, `read_file ../grade.py`,
  `glob ../**`, and `search` outside the root are rejected; with Layer 3 on, assert a shell
  `cat ../grade.py` / `python3 -c "open('../grade.py')"` also fails.
- **uv tests:** assert agent `run_command` `which python3` resolves into `.uv/bin`.
- **Scoring tests:** feed canned grader stdout (full-pass, partial, import-fail, non-JSON) to the
  parser and check `TBRunResult` mapping + `FullPass`.

## 14. Open decisions

- **Shared vs per-run uv env.** Plan uses one shared env (matches "use as default for all runs",
  fastest). If a task's `pip install` must not leak across runs, switch to read-only shared env +
  per-run `--target` installs into the workspace (already the Layer-3 default).
- **Layer-3 default = on.** `sandbox: "auto"` uses the strongest OS sandbox available
  (bwrap→firejail) on every run, since only Layer 3 makes "even reads" airtight against arbitrary
  shell. It downgrades to Layers 1+2 (with a loud warning) only when no sandbox tool is present;
  set `sandbox: "require"` to hard-fail instead, or `"none"` to opt out. Because Layer 3 is now
  default, build it in **M1** (not M6) so the sandbox guarantee is validated from the first task.
- **Observed tasks v1.** Persist artifacts only; defer automated qualitative/LLM-judge comparison.

### Effort

~3–4 focused days: M1–M2 (full sandbox incl. OS Layer 3, uv, grader — the real work) ~2d;
M3–M4 (loop + reports) ~1d; M5 ~0.5d; M6 (external-cmd mode) ~0.5d if pursued.
