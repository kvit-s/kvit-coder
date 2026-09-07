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

## Outputs

A run writes `benchmark-{name}-{timestamp}.md` (or `haystack-`/`thinkbench-`),
`terminal-{name}-{timestamp}.txt` with the full agent transcript, and for
thinkbench a matching `.json`. Runtime scratch goes under
`.kvit-coder-benchmark/` and `thinkbench/runs/`. None of it is tracked; copy
anything you want to keep somewhere outside the checkout.
