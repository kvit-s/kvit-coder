# Incorporating thinkbench into kvit-coder

A review of the **thinkbench** eval suite and example results, and an assessment of
what it would take to run it as another benchmark family inside kvit-coder (alongside
the existing tool-usage benchmarks and haystack/needle-retrieval benchmarks).

---

## 1. What thinkbench is

thinkbench (by "Thinkwright", Apache-2.0) is a benchmark of **autonomous coding agents
on diverse, real-shaped tasks** — small but realistic Python libraries/CLIs, not
homogeneous algorithm puzzles. The bundle we were given has two parts:

- `thinkbench-eval-suite.zip` — the **task suite** (briefs, starter code, held-out graders, reference solutions).
- `thinkbench-results.zip` — an **example results bundle** (what a finished run produces).

### Task suite layout

```
thinkbench-eval-suite/
  manifest.json        # machine-readable index: slug, type, brief, num_checks, has_setup, observed
  TASKS.md             # human-readable catalog grouped by type
  README.md, LICENSE, NOTICE
  tasks/<slug>/
    brief.txt          # the ONLY prompt the model sees
    setup/             # starter package copied into the workspace (non-greenfield tasks only)
    grade.py           # held-out behavioral grader — dropped in AFTER the agent stops; never shown
    reference/         # reference solution — never shown; used to self-test the grader
```

### The 72 tasks, by type

| type | count | graded? | shape |
|---|--:|---|---|
| `implement` | 15 | ✅ behavioral grader | greenfield — build a project from scratch from a spec |
| `bug-fix` | 15 | ✅ | inherit a working-ish package with subtle defects; fix to contract |
| `feature-add` | 15 | ✅ | extend an existing package with a new behavior |
| `repair-to-green` | 15 | ✅ | make a broken/failing project pass |
| `ambiguous-spec` | 12 | ❌ **observed, not scored** | deliberately vague brief; compare interpretations |
| **total** | **72** | 60 graded / 12 observed | |

- **45** tasks ship a `setup/` starter package (all non-greenfield types); the 15 `implement`
  tasks and ambiguous greenfield ones start empty.
- Graders range from **8 to 48 checks** per task (avg grader ≈ 13.5 KB of standalone stdlib Python).

### How a graded task is scored

Each model runs the task in a **fresh, isolated workspace** through an autonomous agent loop
with `read_file` / `write_file` / `run_command` tools. The model sees only `brief.txt` (plus
`setup/` for non-greenfield). After the agent stops:

1. The held-out `grade.py` is dropped into the workspace.
2. `python3 grade.py` runs from the workspace dir and prints **a single JSON scorecard** to stdout.
3. Scoring uses a **fixed denominator** — every check is registered up front, so an import
   failure or empty solution scores `0.0` (never a misleading partial). A per-task score is
   `passed / total` and is **continuous**, not all-or-nothing.

Example scorecard (`tasks/base62/grade.py`):

```json
{"task":"fix_base62","import_ok":true,"passed":16,"total":16,"score":1.0,
 "checks":[{"id":"encode_zero","desc":"...","passed":true,"detail":"..."}, ...]}
```

Key properties of the graders, relevant to integration:
- **Standalone, stdlib-only.** Each `grade.py` runs with `python3` and no deps.
- **Self-contained behavioral oracle.** The grader recomputes expected values itself and
  imports only the agent's produced package (e.g. `base62.public`, falling back to `base62`).
  It never imports the agent's own tests.
- **Never raises out.** Exit code is `0` whenever grading completed (even at score `0.0`);
  the JSON scorecard on stdout is the contract.

### How an ambiguous-spec task is "run"

Same agent loop, but only `brief.txt` ships and **there is no grader**. The solution and run
metrics are kept and interpretations are compared **qualitatively** across models (see the
example `OBSERVATIONS.md`). There is no score.

### What the results bundle looks like

`thinkbench-results.zip` is what a completed run emits — a model-comparison report, not part
of the harness:

- `results.json` — the aggregate data model: `run_ids`, `models`, `trials`, `num_graded_tasks`,
  `num_observed_tasks`, `config` (provider/endpoint/pricing), plus rollups:
  `overall_graded`, `by_type`, `by_task`, and `observed` — each keyed by model with
  `{n, solved, solve_rate, mean_score, avg_secs, avg_tokens, cached_frac, total_cost_usd}`.
- `RESULTS.md` — human-readable tables (overall, by-type, per-task mean score over trials).
- `OBSERVATIONS.md` — qualitative writeup of the 12 ambiguous tasks.

Note the two distinct success metrics they report: **mean score** (avg of `passed/total`) and
**full-pass / solve_rate** (fraction of runs scoring exactly `1.0`).

---

## 2. How kvit-coder benchmarks work today

The existing harness lives in `internal/benchmark/`:

| file | role |
|---|---|
| `types.go` | `BenchmarkDef`, `ValidationCheck`, `RunResult`, `AggregatedStats`, `Report` |
| `config.go` | loads `benchmarks.yaml` / `haystack.yaml`; filtering; file discovery |
| `setup.go` | `Environment` — creates per-run workspace from inline `Setup` files |
| `executor.go` | runs the agent loop (or an external `BenchmarkCmd`) and validates |
| `validator.go` | the validation check implementations |
| `runner.go` | orchestrates N runs × benchmarks, resume-from-CSV, progress |
| `report.go`, `csv.go`, `progress.go` | reporting and live output |

Defining shape of the current model:

- **Tasks are inline YAML** (`benchmarks/benchmarks.yaml`). A `BenchmarkDef` has `id`, `name`,
  `category`, `goal`, `task` (the prompt), `setup` (list of `{file, content}` written into the
  workspace), and `validation` (list of checks). Categories today: `search`, `read`, `write`,
  `edit`, `compound` (+ `needle_retrieval` for haystack).
- **Validation is binary.** `Validator.Validate` returns `(allPassed bool, errors)`. A run is
  `Success: true` only if **every** check passes. Check types: `file_contains`, `file_equals`,
  `file_exists`, `file_line_count`, `tool_called(_with)`, `output_contains`, `multi_tool_calls`,
  `run_command`, etc. — all string/substring/regex assertions.
- **`run_command` already exists** (`validator.go:314`): runs `sh -c <command>` in the workspace
  dir and asserts its combined output **contains** an expected substring.
- **The agent loop matches thinkbench's model.** `executor.go` sends the system prompt + `task`
  as the user message and runs `agent.Runner` with the file/command tools, in a fresh workspace
  cleaned before each run. There's also an **external-command mode** (`cfg.LLM.BenchmarkCmd`,
  `{prompt}` substitution) to benchmark any external agent CLI.
- **Reporting** aggregates over runs: success rate, tokens, cost, duration, percentiles. The
  unit of success is the boolean per run; there is **no partial-score concept**.
- **Invocation:** `kvit-coder --benchmark <suffix> -n <runs>` (config `config-<suffix>.yaml`),
  with `--benchmark-category`, `--benchmark-id`, `--benchmark-list`, `--no-resume`, `-o`.
  Haystack benchmarks are a parallel family behind `--bench-haystack`.

---

## 3. Fit analysis — where thinkbench matches and where it doesn't

| dimension | kvit-coder today | thinkbench needs | gap |
|---|---|---|---|
| Agent loop (read/write/run + fresh workspace) | ✅ exactly this | same | **none** — direct fit |
| Prompt = single brief | ✅ `task` field | `brief.txt` | trivial |
| Starter code in workspace | ✅ `setup: [{file, content}]` | a `setup/` **directory tree** | mechanical: flatten dir → file list |
| Grading | **binary**, inline string asserts | **partial score** from an external `grade.py` JSON scorecard | **core gap** |
| Held-out grader file | no concept (checks are inline) | drop `grade.py` in *after* run, execute, parse JSON | **core gap** |
| Score model | `Success bool` | `score ∈ [0,1]` + full-pass | **schema gap** (`RunResult` has no `Score`) |
| Task type / by-type rollup | `category` exists but no per-type score table | `by_type`, `by_task`, mean-score vs full-pass | reporting work |
| Ungraded "observed" tasks | none — every run is pass/fail | keep workspace, no score, compare | **new mode** |
| Python in environment | used ad-hoc via `run_command` | `python3` required for every grader | env requirement |

**Bottom line:** the *execution* side is a near-perfect fit — thinkbench was designed for exactly
the autonomous read/write/run loop kvit-coder already drives. The *grading* side is the real work:
kvit-coder's binary inline-assert model has to gain (a) the ability to run a held-out grader
dropped in post-run and (b) a continuous score plumbed through results and reports.

---

## 4. Two integration paths

### Path A — Minimal, reuses the existing harness (binary full-pass only)

Treat each graded thinkbench task as one `BenchmarkDef` and lean on the existing `run_command`
validation to invoke the grader.

- A converter script walks `tasks/<slug>/`, reads `brief.txt` → `task`, flattens `setup/**` into
  `setup: [{file, content}]` entries, and emits a `thinkbench.yaml` in the `BenchmarksFile` shape.
- For validation, point a `run_command` check at the held-out grader (kept **outside** the
  workspace so the agent never sees it) and assert full-pass:

  ```yaml
  validation:
    - type: run_command
      command: "python3 /abs/path/to/tasks/base62/grade.py"
      expected: '"score": 1.0'
  ```

  This works because `grade.py` does `ROOT = os.getcwd()` and imports the produced package from
  the cwd — and `run_command` runs with `cmd.Dir = workspaceDir`. The grader file itself can live
  anywhere.

**Pros:** zero Go changes; usable almost immediately; reuses resume/CSV/reporting.
**Cons:** collapses the continuous score to **full-pass-or-not** (you lose `passed/total`, the
headline `mean_score` metric, and the per-check detail). The 12 ambiguous tasks **can't** be
represented (they have no grader). Pasting 45 starter trees + briefs inline into YAML is bulky and
loses fidelity vs. reading the suite directly. Good for a quick "can our agent full-pass these?"
smoke test; **not** a faithful reproduction of thinkbench's numbers.

### Path B — A first-class `thinkbench` benchmark family (recommended)

Mirror how `haystack` is a parallel family (`--bench-haystack`, its own loader/types). Add a
`thinkbench` family that reads the suite **directly** and grades with partial scores.

Concretely:

1. **New flag + entry point.** `--bench-thinkbench <suffix>` (+ `--bench-thinkbench-id`), wired in
   `cmd/kvit-coder/main.go` next to the haystack flags, selecting `config-<suffix>.yaml` the same way.

2. **Loader** (`internal/benchmark/thinkbench.go`). Read `manifest.json` and, per task, load
   `brief.txt` and the `setup/` tree from disk — no YAML conversion. Carry `type`, `num_checks`,
   `observed`, and the path to the held-out `grade.py`. A `--thinkbench-suite <dir>` flag (or config
   key) points at the unzipped suite.

3. **Setup from a directory.** `Environment.SetupBenchmark` already creates nested files
   (`MkdirAll` on parents); add a sibling that copies an on-disk `setup/` tree into the workspace
   instead of writing inline `Setup` content.

4. **Grader-based scoring** (the core addition). After the agent stops:
   - Copy `grade.py` into the workspace (or run it with cwd=workspace, as the grader expects).
   - `python3 grade.py`, capture stdout, parse the JSON scorecard.
   - Extract `score`, `passed`, `total`, `import_ok`, and the per-check list.

   Add fields to `RunResult` (`types.go`): `Score float64`, `Passed int`, `Total int`, and keep
   `Success` defined as `score == 1.0` (full-pass) for back-compat with existing aggregation.
   This is purely additive — existing benchmarks leave `Score` zero/unused.

5. **Observed (ambiguous) tasks.** Mark them `observed: true`; skip grading, persist the workspace
   under a results dir, record only run metrics. Optionally an LLM-judge comparison later — out of
   scope for v1; matching thinkbench, just keep the artifacts.

6. **Reporting.** A thinkbench report that mirrors `results.json` / `RESULTS.md`: `overall_graded`,
   `by_type`, `by_task`, plus both metrics (**mean score** and **full-pass rate**). Most of the
   token/cost/latency aggregation in `report.go` is reusable; the new axes are score-weighting and
   the per-type grouping (the `type` field, analogous to today's `category`).

7. **Environment requirement.** `python3` (stdlib only) must be on PATH where benchmarks run.
   Add a preflight check and document it. (Graders import nothing beyond stdlib.)

**Pros:** faithful reproduction — continuous scores, by-type/by-task tables, observed tasks, and
directly comparable numbers to the example `results.json`. Reads the suite as shipped (no lossy
inlining). **Cons:** real Go work (new loader, grader-runner, `RunResult`/report extensions, flag
wiring), plus a Python runtime dependency in the bench environment.

---

## 5. Recommended plan

1. **Spike with Path A** on ~3 tasks (one `bug-fix`, one `implement`, one `feature-add`) to confirm
   the agent loop + `python3 grade.py` flow end-to-end and that our agent can actually drive these
   multi-file tasks within a sane timeout. Low cost, high signal.
2. **Build Path B** as the real integration:
   - `internal/benchmark/thinkbench.go` — loader (manifest + on-disk brief/setup) and grader-runner
     (run `grade.py`, parse JSON scorecard).
   - Extend `RunResult` with `Score/Passed/Total` (additive); define `Success = score==1.0`.
   - Directory-tree setup copy in `setup.go`.
   - `--bench-thinkbench` wiring in `cmd/kvit-coder/main.go`, parallel to `--bench-haystack`.
   - Observed-task path: skip grading, keep workspace + metrics.
   - thinkbench report (`overall_graded` / `by_type` / `by_task`, mean-score + full-pass).
3. **Operational notes:**
   - Bump `timeout_per_run` well above the 120 s default — `implement` tasks build whole projects
     (the example run shows ~150 s avg, and they're the slowest).
   - Keep the suite **out of the repo** (or in a gitignored path) so graders/references never leak
     into the agent's view; load it from `--thinkbench-suite`.
   - Honor the Apache-2.0 license / `NOTICE` and attribute Thinkwright if results are published.

### Effort estimate

| item | size |
|---|---|
| Path A spike (converter + `run_command` grader) | a few hours |
| Path B loader + grader-runner | ~1 day |
| `RunResult`/report extensions for partial scores + by-type | ~1 day |
| Observed-task handling | ~half day |
| Flag wiring, docs, preflight (`python3`) | ~half day |

Roughly **2–3 focused days** for a faithful Path-B integration, with Path A available same-day as a
smoke test.
