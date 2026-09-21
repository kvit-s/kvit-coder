# Moving the benchmark harness out of the agent binary

## 1. What this document is

kvit-coder is a coding agent written in Go. It also contains three benchmark
harnesses — a tool-use suite, a large-context retrieval suite, and an autonomous
coding suite scored by a held-out grader — and all three are compiled into the
agent binary and reached through its flags. This document argues that they
should be a second command instead, and says how to get there.

Nothing here is built. It is a proposal in the same sense as
[`redesign-mcp.md`](../redesign-mcp.md): the reasoning and the plan are written
down so the work can be picked up later without re-deriving them. The
[repository cleanup of September 2026](../README.md) deliberately left this
alone, because it changes how you invoke benchmarks and cleanup should not do
that.

The reader is assumed not to have the code open, so the parts of it that matter
are described where they come up.

## 2. What the situation is

### 2.1 The harness is a seventh of the program

`internal/benchmark` is 5,313 lines of non-test Go across 22 files, which is
about 15% of the program's non-test source and second only to `internal/tools`.
It holds three families:

| Family | Files | What it does |
|---|---|---|
| Tool benchmarks | `cli.go`, `runner.go`, `executor.go`, `validator.go`, `report.go`, `csv.go`, `setup.go`, `progress.go`, `types.go` | Runs the tasks in `benchmarks/benchmarks.yaml` against a fresh workspace and validates the result |
| Haystack | `cli.go` (`RunHaystack`) | Plants a fact in a large file and asks for it back |
| Thinkbench | nine `thinkbench_*.go` files | Runs the vendored Thinkwright suite in a sandbox and scores it with the suite's grader |

Its imports are clean: `internal/agent`, `internal/config`, `internal/llm`, and
nothing else. The package does not reach into the tool registry, the session
layer, the prompt generator, or the transport. That matters, because it means
the difficulty of splitting it is not in the package.

### 2.2 The coupling is in `main.go`

`cmd/kvit-coder/main.go` is 732 lines, and benchmark concerns are interleaved
into the shared startup path in five places rather than sitting at the end of
it:

1. **Flag declaration** (lines 58–76). Twelve of the roughly twenty-five flags
   are benchmark flags. Two of them, `-n` and `-o`, are the most obvious short
   names in the program and are spent on "runs per benchmark" and "benchmark
   output file".
2. **Config path resolution** (lines 184–213). Each family accepts an optional
   suffix that redirects the config: `--benchmark muse-spark` loads
   `config-muse-spark.yaml`. Three near-identical branches implement this.
3. **An early exit** (lines 224–230). `--benchmark-list` prints the suite and
   returns before anything else is constructed.
4. **Workspace redirection** (lines 243–283), which has to happen *before* the
   tool registry is built, because the tools capture the workspace root. Plain
   benchmark mode points the root at `.kvit-coder-benchmark/workspace-{suffix}`;
   thinkbench points it at a sandbox under `benchmarks/thinkbench/runs/`, kept
   physically apart from the suite so the graders and reference solutions stay
   out of the model's reach.
5. **Session resolution** (lines 318–336). A benchmark run has no session to
   continue, so it is given an ephemeral one in a temp directory that is removed
   on exit.

Between points 4 and 5 and the call into the harness sit about 200 lines that
every mode needs: the LLM client, the checkpoint manager, the context manager,
the steering inbox, the background-process registry, the MCP manager, the tool
registry, the prompt generator, and the runner. The harness is handed the last
two of these as `*agent.Runner` and `systemPrompt`.

### 2.3 Why this is worth changing

**The agent binary carries code it never runs.** A daily driver links, ships and
loads a report generator, a CSV writer, a scorecard parser and a Python sandbox
manager that a working session never touches.

**Startup reads benchmark decisions before it reads yours.** The first thing
`main.go` does after parsing flags is work out which of three benchmark suffixes
might redirect the config file. That is the wrong first question for a program
whose common case is running one turn.

**Two short flags are spent.** `-n` and `-o` mean nothing to an agent invocation
and cannot be reused while the harness lives here.

**The two things change for different reasons and on different schedules.** The
agent changes when the model or the session model changes. The harness changes
when a suite is added or a grader moves. Keeping them in one command means every
harness change risks the binary you use all day.

Against that: the harness needs a fully constructed agent, so a second command
duplicates the construction unless the construction is extracted first. That
extraction is the bulk of the work, and it is worth doing on its own merits —
`main.go` at 732 lines with five interleaved concerns is the part of the program
hardest to change safely.

## 3. What to build

Three commands where there are now two:

| Command | Role |
|---|---|
| `kvit-coder` | One turn of the agent, headless. Unchanged except that the benchmark flags leave. |
| `kvit-coder-ui` | The interactive front end. Unchanged; it already spawns `kvit-coder` as a child rather than linking the agent. |
| `kvit-bench` | The three benchmark families. Links `internal/benchmark` and the shared setup, and nothing else new. |

`internal/benchmark` itself does not move and its exported functions do not
change signature. The work is entirely in `cmd/`.

### 3.1 Extract the shared setup

Add `internal/setup` (or `internal/bootstrap`; the name matters less than that
it is one package) exposing roughly:

```go
// Options are the decisions a caller makes before anything is constructed.
type Options struct {
    ConfigPath     string
    WorkspaceRoot  string  // empty means take it from the config
    Session        *session.Session
    ModelOverride  string
    BaseURLOverride string
    AgentFile      string
    Writer         ui.Writer
    Logger         *log.Logger
}

// Agent is everything a caller needs to run turns.
type Agent struct {
    Runner       *agent.Runner
    SystemPrompt string
    Config       *config.Config
    Registry     *tools.Registry
    Cleanup      func() error
}

func Build(ctx context.Context, opts Options) (*Agent, error)
```

`Build` is the 200 lines currently between the workspace override and the
benchmark dispatch in `main.go`, moved verbatim and returning errors instead of
calling `log.Fatalf`. Both commands then read as: decide the config path, decide
the workspace, open or create a session, call `Build`, and dispatch.

This step alone is worth committing on its own. It shortens `main.go`
substantially, it makes the startup path testable for the first time, and it
leaves the program behaving identically.

### 3.2 Move the benchmark-specific pre-steps

Each of the five interleavings in §2.2 moves to `cmd/kvit-bench`:

- The twelve flags become that command's flag set, and can drop their
  `--bench-`/`--benchmark-` prefixes once they are not sharing a namespace.
  `--benchmark-category` becomes `--category`, `--bench-thinkbench-types`
  becomes `--types`, and so on.
- Suffix-to-config resolution becomes one function, called once, rather than
  three branches. The three families differ only in which flag supplied the
  suffix.
- `--benchmark-list` becomes `kvit-bench list`.
- Workspace redirection becomes the `WorkspaceRoot` field of `setup.Options`,
  computed by whichever family is running before `Build` is called. This is the
  step that most needs the extraction: it exists as an in-place mutation of
  `cfg` today precisely because the tool registry is built later in the same
  function.
- The ephemeral session becomes `kvit-bench`'s only session behaviour. It never
  continues a named session, so the `-s`, `--sessions`, `--session-show` and
  `--session-delete` flags do not follow it.

### 3.3 Shape of the command

```
kvit-bench tools      [--config C] [--suffix S] [-n RUNS] [--category C] [--id ID] [-o FILE] [--no-resume]
kvit-bench haystack   [--config C] [--suffix S] [--id ID]
kvit-bench thinkbench [--config C] [--suffix S] [--id SLUG] [--types T] [--resume] [--suite DIR]
kvit-bench list
```

Subcommands rather than a flag per family, because the families are mutually
exclusive and the current flags only express that by convention. `--suffix` is
separated from the family selector, which removes the `"."` and `"true"`
sentinel values that `--benchmark .` and `--bench-haystack true` currently need.

### 3.4 Build and release

`.goreleaser.yml` gains a third binary. `.github/workflows/test.yml` needs no
change, since it already builds and tests `./...`.

## 4. Order of work

Each step leaves a program that builds, passes its tests, and can be used.

1. **Extract `internal/setup`.** No behaviour change, no flags move, both
   binaries call `Build`. `main.go` should lose roughly 250 lines.
2. **Add `cmd/kvit-bench` alongside the existing flags.** Both routes work; the
   new one is the one you use. This is where the workspace and session pre-steps
   get written properly and where any difference in behaviour will show up,
   with the old path still there to compare against.
3. **Verify against the suites.** Run all three families both ways on the same
   config and compare reports. Thinkbench is the one that matters most, because
   its sandbox is the most delicate part of the move: the run workspace must
   stay physically separate from `benchmarks/thinkbench/suite/`, or the model
   can read the graders and reference solutions it is being scored against.
4. **Delete the benchmark flags from `kvit-coder`,** and with them the
   `internal/benchmark` import. Free `-n` and `-o`.
5. **Update** the root `README.md` benchmarking section, `benchmarks/README.md`,
   and `.goreleaser.yml`.

Steps 1 and 2 are the work; 3 is the care; 4 and 5 are small.

## 5. What this is not

It is not a rewrite of the harness. The report formats, the CSV resume logic,
the validator and the thinkbench grader are untouched, and `internal/benchmark`
keeps its current API.

It is not a case for dropping any suite. `redesign.md` §15 keeps all three, and
thinkbench in particular, on the grounds that it is the only way to tell whether
a change to the agent helped. That argument is unaffected by which binary runs
it.

It does not depend on, and is not depended on by, `redesign-mcp.md`.
