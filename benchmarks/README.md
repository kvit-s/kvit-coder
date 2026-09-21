# Benchmarks

This directory holds the inputs to kvit-coder's three benchmark families. The
outputs — reports, transcripts and result JSON — are written here too, but they
are not tracked: `.gitignore` excludes them, so a run leaves your checkout clean
and its files stay out of `git grep` and `ripgrep`.

## The three families

| Family | Flag | Definitions | What it measures |
|---|---|---|---|
| Tool benchmarks | `--benchmark` | `benchmarks.yaml` | Whether the model uses the tools correctly on small, checkable tasks |
| Haystack | `--bench-haystack` | `haystack.yaml` | Retrieval of a planted fact from a large context |
| Thinkbench | `--bench-thinkbench` | `thinkbench/suite/` | Autonomous coding tasks scored by a held-out grader |

`thinkbench/suite/` is a vendored third-party dataset from Thinkwright, licensed
Apache-2.0. See `thinkbench/suite/NOTICE` and `thinkbench/suite/LICENSE`.

## Per-model configs

`config-{name}.yaml` is a full kvit-coder config used for one benchmark run. All
three families take an optional suffix that selects one: `--benchmark muse-spark`
reads `config-muse-spark.yaml`. The suffix also names the run's workspace, so two
benchmark runs with different suffixes can proceed at the same time.

Most of these configs point at local endpoints from earlier work on small models.
`config-muse-spark.yaml` is the one that matches the current default in the
repository root's `config.yaml`.

## Running

```bash
# From the repository root
go build -o kvit-coder ./cmd/kvit-coder

./kvit-coder --benchmark muse-spark -n 10
./kvit-coder --bench-thinkbench muse-spark
```

The haystack family needs its corpus, which is an amalgamation of this repo's own
Go source. It is regenerated rather than tracked, because a committed copy would
put a stale second copy of the codebase into every search:

```bash
scripts/amalgamate-go.sh          # writes benchmarks/haystacks/kvit-coder.go.txt
./kvit-coder --bench-haystack muse-spark
```

## Tool benchmarks in detail

```bash
# Run all benchmarks (uses config-mymodel.yaml)
./kvit-coder --benchmark mymodel

# Custom runs, category filter, specific IDs
./kvit-coder --benchmark mymodel -n 5
./kvit-coder --benchmark mymodel --benchmark-category search,edit
./kvit-coder --benchmark mymodel --benchmark-id S1,S2,R1

# List available benchmarks
./kvit-coder --benchmark-list
```

## Categories

| Category | Description |
|----------|-------------|
| **search** | Code pattern search |
| **read** | File reading and directory listing |
| **edit** | File creation and modification |
| **shell** | Shell command execution |
| **compound** | Multi-step tasks combining multiple tools |

## Benchmarking another agent

Benchmark an external tool (e.g., Claude Code):

```yaml
llm:
  benchmark_cmd: "claude -p {prompt} --allowedTools Edit Bash Read"
```

## Haystack in detail

Needle retrieval in large context windows (1-5 hop reasoning, no tools required):

The corpus is an amalgamation of this repository's own Go source, regenerated
rather than tracked — a committed copy would put a stale second copy of the
codebase into every search. Build it first:

```bash
scripts/amalgamate-go.sh          # writes benchmarks/haystacks/kvit-coder.go.txt

./kvit-coder --bench-haystack mymodel
./kvit-coder --bench-haystack mymodel --bench-haystack-id 1H1,2H1
```

## Thinkbench in detail

[Thinkbench](thinkbench/suite/README.md) (by Thinkwright, Apache-2.0) is a suite of 72 autonomous coding-agent tasks across five types — `implement`, `bug-fix`, `feature-add`, `repair-to-green` (60 graded), and `ambiguous-spec` (12 observed). Each graded task runs the agent in a fresh, hard-sandboxed workspace from a single `brief.txt`; after the agent stops a held-out `grade.py` is dropped in and produces a continuous `passed/total` score.

```bash
# Run all graded thinkbench tasks (uses config-mymodel.yaml), 3 trials each
./kvit-coder --bench-thinkbench mymodel

# Specific tasks / types / trial count
./kvit-coder --bench-thinkbench mymodel --bench-thinkbench-id base62,backoff
./kvit-coder --bench-thinkbench mymodel --bench-thinkbench-types bug-fix,feature-add
./kvit-coder --bench-thinkbench mymodel -n 1
```

Outputs `results.json`, `RESULTS.md`, and a resumable CSV under `benchmarks/thinkbench/results/` (`overall_graded` / `by_type` / `by_task` / `observed` rollups, with both **mean score** and **full-pass rate**).

**Requirements & sandboxing:**

- **`uv`** must be on PATH. A shared environment is auto-provisioned at `benchmarks/thinkbench/.uv` (pinned via `thinkbench.uv_python`) and used as the default `python3`/`pip` for both the agent's `run_command` and the grader.
- Each run is confined to a per-run workspace for **reads as well as writes**: `path_safety_mode=block` covers the structured file tools, and an OS sandbox (auto-detected **bubblewrap** → **firejail**) confines arbitrary shell. A preflight smoke test proves a read outside the workspace is denied before any task runs. Held-out graders and reference solutions live under `suite/` and are never copied into a workspace pre-grade.

Config block (in `config-<suffix>.yaml`):

```yaml
thinkbench:
  suite_dir: "benchmarks/thinkbench/suite"   # optional; auto-discovered
  timeout_per_run: 600                        # seconds; implement tasks are slow
  uv_python: "3.11"
  sandbox: "auto"   # auto | bwrap | firejail | none | require
  types: ["implement", "bug-fix", "feature-add", "repair-to-green"]
  trials: 3          # default trials per task; overridden by -n
  include_observed: false   # also run the 12 ungraded ambiguous-spec tasks (persist-only)
```

## Adding your own

Define in `benchmarks/benchmarks.yaml`:

```yaml
benchmarks:
  - id: MY1
    name: "My Custom Benchmark"
    category: custom
    goal: "Test something specific"
    setup:
      - file: "test.go"
        content: |
          package main
          func myFunc() {}
    task: "Find the function named myFunc"
    validation:
      - type: output_contains
        expected: "test.go"
```

Validation types: `file_contains`, `file_equals`, `file_exists`, `file_not_exists`, `file_line_count`, `tool_called`, `tool_called_with`, `output_contains`, `output_not_contains`, `output_matches`, `multi_tool_calls`

## Outputs

A run writes `benchmark-{name}-{timestamp}.md` (or `haystack-`/`thinkbench-`),
`terminal-{name}-{timestamp}.txt` with the full agent transcript, and for
thinkbench a matching `.json`. Runtime scratch goes under
`.kvit-coder-benchmark/` and `thinkbench/runs/`. None of it is tracked; copy
anything you want to keep somewhere outside the checkout.
