# thinkbench

A reproducible benchmark of autonomous coding agents on **diverse, real-shaped tasks**
(not homogeneous puzzles): 72 tasks across five types —
implement (15), bug-fix (15), feature-add (15), repair-to-green (15), ambiguous-spec (12).

It has two modes. **Graded** tasks (implement, bug-fix, feature-add, repair-to-green)
are scored by a held-out behavioral grader. **Ambiguous-spec** tasks are deliberately
underspecified and **not graded** — they exist to be *observed*: there is no single
right answer, so we look at how each model interprets the same vague brief and report
the divergence qualitatively.

## How a graded task is run

Each model runs each task in a fresh, isolated workspace through an autonomous agent
loop with `read_file` / `write_file` / `run_command` tools. The model sees only
`brief.txt` (and `setup/` starter code for non-greenfield tasks). After the model
stops, the held-out grader `grade.py` is dropped in and run; it prints a JSON
scorecard. Scoring uses a **fixed denominator** — an empty/broken solution scores
0.0, never a misleading partial — so a per-task score is `passed / total` checks.

## How an ambiguous-spec task is run

Same agent loop, but the task ships only `brief.txt` and there is no grader. We keep
the model's solution and run metrics and compare interpretations across models; the
score is not defined.

## Layout

- `manifest.json` — machine-readable task index (slug, type, brief, graded behaviors).
- `TASKS.md` — human-readable catalog grouped by type.
- `tasks/<slug>/`
  - `brief.txt` — the prompt the model saw.
  - `setup/` — starter code copied into the workspace (diverse tasks only).
  - `grade.py` — the held-out behavioral grader (never shown to the model).
  - `reference/` — a reference solution (never shown to the model; used to self-test the grader).
- `results.json` — per-task model results (added after a benchmark run).

## Reproducing

Place the model's solution as a package `<slug>` in a working dir, run `python3 grade.py`
from that dir, and read the JSON scorecard. Each `grade.py` is standalone (stdlib only).

## Download hygiene

This public bundle contains task briefs, starter code, held-out graders, reference
solutions, the task catalog, and aggregate result files. It does not include model
transcripts, saved workspaces, raw runner directories, provider credentials, environment
dumps, private host paths, or local machine metadata. Example names, addresses, emails,
hosts, and domains inside task fixtures are synthetic.

## License

Licensed under the Apache License, Version 2.0 — see `LICENSE`. Copyright 2026 Thinkwright.
If you use this benchmark, attribution to Thinkwright is appreciated.
