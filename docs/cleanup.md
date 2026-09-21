# Preparing the public repository for release

## What this document is

kvit-coder is the coding agent in this repository: a Go program that sends a
conversation to an OpenAI-compatible model endpoint, receives requests to run
tools, runs them against a directory of source code, feeds the results back, and
repeats until the model answers without asking for another tool. The repository
is public at `github.com/kvit-s/kvit-coder` under the MIT licence.

This is a review of what that public repository currently shows a visitor, what
is wrong with it, and what to do before the project is promoted to an audience.
It was written on 20 September 2026 against local commit `bc1867e`. Every number
below was measured rather than estimated; the commands are listed in the last
section.

Nothing in the repository was changed when this was written. The work described
here is a proposal, not a record of work done.

## The public repository is eight months behind the code

Local `main` is 129 commits ahead of `origin/main`. The last push was
`667cd2e`, dated 4 January 2026; local `HEAD` is dated 20 September 2026. The
entire redesign — one operating-system process per turn, the session directory,
steering a running turn, background processes, structured reports, tool groups,
Model Context Protocol support, the web tools, PDF reading — exists only
locally. A visitor to GitHub is reading a January program.

Three further things are visible on the repository page:

- **The most recent CI run failed.** Workflow run `20698313249` (`Test`, 4
  January 2026) ended in failure, as did the four runs before it, so the newest
  public commit shows a red cross.
- **There are no tags and no releases**, on the remote or locally, although
  `.goreleaser.yml` and `.github/workflows/release.yml` are both written and
  ready. Installing the program currently requires a Go toolchain and a clone.
- **The repository description is stale**: "A minimal coding agent in Go with
  OpenAI-compatible LLM and pluggable tools." The program is 73,500 lines of Go
  with three benchmark harnesses. There are no topics, no homepage link and no
  social preview image.

Until the current code is pushed with a passing build, the rest of this document
has no effect on anyone.

## What the project can claim

The README opens with an accurate description that would fit any coding agent
written since 2024. The distinguishing properties are recorded in
`docs/redesign.md` and in `CLAUDE.md`, and most of them come with a number:

1. **The agent process does not outlive a turn.** One turn is one operating-system
   process: it starts, reads the session from disk, runs one instruction, appends
   what happened, and exits. Everything that must survive lives in
   `~/.kvit-coder/sessions/<name>/`. A wedged turn ends when its process does,
   and the session directory can be opened and read months later.
2. **A running turn can be steered.** Lines typed at the terminal and files
   dropped into the session's `inbox/` reach the model at its next iteration,
   and pressing Enter on an empty line pauses the turn at the next safe point.
3. **Shell command permissions are decided from a parsed syntax tree**, using
   `mvdan.cc/sh/v3`, rather than by matching patterns against the command
   string (`internal/permissions`). For a program that executes arbitrary shell
   commands on a user's machine, this is the argument for trusting it.
4. **Tool groups reduce what Model Context Protocol servers cost in every
   request.** Advertising Playwright's 24 browser tools costs about 7,400 tokens
   of schema and 43 tools per request; behind the `Web.browsing` group the same
   configuration costs about 3,400 tokens and 20 tools, and the members are
   described to the model only when it opens the group.
5. **Small local models are supported deliberately.** Setting `agent.profile:
   weak` turns on loop detection, backtracking, recovery of tool calls written
   into prose, fuzzy matching and indentation repair — machinery written for
   models that mis-format tool calls and miscount line numbers.
6. **Three benchmark harnesses are in the repository**, including a vendored,
   held-out grader suite, so any claim about model behaviour can be rerun by a
   reader.

Measured results would strengthen this. `docs/tools-stats.md` records which
tools the model actually calls, counted from saved sessions, and the thinkbench
harness produces scores; a short results table on the front page is worth more
to a reader than another paragraph of description.

## The README

**Rewritten on 21 September 2026**, from 847 lines and 6,125 words to 182 lines
and 1,472 words, along the lines described below. The reference material moved
rather than being deleted: `docs/cli.md`, `docs/configuration.md`,
`docs/sessions.md` and `docs/mcp.md` now hold it, and the benchmarking section
joined `benchmarks/README.md`. Every relative link in the documentation tree
resolves. What is still missing from the front page is the terminal recording;
the place it goes is marked in the file.

The description below is what the rewrite was aiming at, kept because the demo
is still outstanding and because it says why the shape was chosen.

It was 847 lines and 6,125 words in a single file, with no images and no status
badges. Reference material takes up close to half of it: the CLI flags section
is 787 words, Sessions is 1,640, the configuration reference is 1,225 and
Benchmarking is 577.

A front page of roughly 200 lines would hold: a one-sentence description, a
recording of the terminal interface, installation from a release binary, a
quickstart that works in under a minute, the six properties above with their
numbers, a short section on what the program is allowed to do to a machine and
how `workspace.path_safety_mode` and the command permission system constrain it,
and links to everything else. The CLI flags belong in `docs/cli.md`, the
configuration reference in `docs/configuration.md`, and the benchmarking section
in `benchmarks/README.md`, which already covers the same ground.

For the recording, `charmbracelet/vhs` produces a GIF from a scripted `.tape`
file and the project already depends on Bubble Tea. Committing the tape means
the demo can be regenerated whenever the interface changes.

## Repository hygiene

| Item | Current state | Proposed |
|---|---|---|
| Broken links | `README.md` points at `docs/review.md` and `docs/bench-refactor.md`; `docs/README.md` points at `review.md`, `bench-refactor.md`, `images.md` and `oh-my-pi.md`. The first three moved into `docs/archive/`; `oh-my-pi.md` is not in the repository at all | Fix the five paths, write or drop the sixth |
| Another program's config | `.clauding/config/settings.json` is tracked. It sets `"testCommand": "npm test"` in a Go repository and passes `--dangerously-skip-permissions` to a different agent | `git rm --cached`; `.clauding` is already in `.gitignore` |
| Dead directory | `eval/` holds four files pointing at `/home/sk/msn1` and `/home/sk/cag-mcp`, a private project | Delete, or move a sanitized copy under `benchmarks/` |
| Personal paths | `/home/sk/...` appears in `eval/`, `docs/agents-ram.md` and eight files under `docs/archive/` | Rewrite as `~/...` or as generic names |
| Local network addresses | `192.168.8.20` and `192.168.138.186` appear in the root `config.yaml` and in about twenty benchmark configs | Replace with `localhost` at least in the files a newcomer reads |
| Benchmark configs | 25 `config-*.yaml` files for local models nobody else has: `config-6.6b`, `config-dasd-4b`, `config-iquest-q8` and so on | Keep two or three representative ones |
| Documentation weight | 28 tracked files, 620 KB, most of it design history | Keep the four current documents indexed from `docs/README.md` and leave the rest in `archive/` under a line saying it is history |
| Stray file | `kvit-coder.log`, 968 KB, sits in the checkout; the default log path is `~/.kvit-coder/logs` | Delete it |

Nothing sensitive is exposed. A scan of the tracked tree and of every commit in
the history for API keys, tokens and hardcoded credentials found none.

## Defects to fix before a release

Each of these is small, and each is something a new user meets in the first
minute.

**A failed headless turn exits 0.** `repl.RunExec` (`internal/repl/repl.go:28`)
returns nothing, and `cmd/kvit-coder/main.go:855` calls it as the last statement
of `main`. Running `kvit-coder -p "say hi"` with no API key set prints
`[error] llm call failed: API error 401` and exits 0. The README describes this
binary as the one to script against, so a wrapper or CI job cannot currently
tell a failed turn from a successful one.

**The authentication error does not say what to set.** The configuration names
the variable in `api_key_env` (`OPENCODE_API_KEY` by default) and the 401
message should name it too.

**The banner does not say what the program is.** `cmd/kvit-coder/main.go:844`
prints `Agent REPL v0.1` and `internal/tui/ui.go:184` prints
`Agent REPL UI v0.1`. Meanwhile `scripts/build.sh` stamps a version into
`main.version`, that variable is shadowed by a local at
`cmd/kvit-coder/main.go:181`, and golangci-lint reports the package-level one as
unused. The `-version` flag prints only the commit date and hash, currently
`20260920-7b7ec2f`.

**CI will fail on the first push.** `gofmt -l` lists nine files:
`internal/benchmark/ansi_strip.go`, `internal/benchmark/config.go`,
`internal/benchmark/progress.go`, `internal/benchmark/types.go`,
`internal/prompt/funcs.go`, `internal/prompt/templates_test.go`,
`internal/safety/context.go`, `internal/safety/redactor.go` and
`internal/ui/report_card.go`. `golangci-lint run ./...` reports 19 findings,
among them an `if` statement with an empty body in `internal/safety/parser.go:160`
and an assignment that is never read in `internal/agent/runner.go:318`. The lint
job in `.github/workflows/test.yml` also pins `version: latest`, so a new
linter release can turn the build red with no commit; pin an exact version.

The test suite and `go vet` are clean: 97 test files, 841 test functions, all
passing.

**Installing needed a clone and a Go toolchain.** Fixed on 21 September 2026.
`install.sh` at the repository root downloads the release build for the
machine it runs on, checks it against the release's checksums and installs
`kc`, `kcu` and the two long names into `~/.local/bin`, seeding
`~/.kvit-coder/config.yaml` when there is none. The release archives now carry
`config.example.yaml`, without which a downloaded binary started by saying it
could not find a configuration and left nothing to copy. Windows was dropped
from `.goreleaser.yml` because the program does not compile for it —
`syscall.Flock` in `internal/workspace` and `Setsid` in `internal/procs` — so
the first tag would have failed the release job. goreleaser is pinned to the
version the config was validated against.

All of it was verified against archives built locally with
`goreleaser release --snapshot`: the install script was pointed at them over
HTTP and installed, seeded a configuration, and refused a tampered checksum.
What it cannot do until a tag exists is find a release, so **the tag has to come
before anyone reads the README**.

**The shipped configuration is the author's.** The root `config.yaml` has 178
keys, enables a Playwright server over the Model Context Protocol, sets
`reasoning_effort: xhigh`, and lists a model served from a machine on the
author's local network. Because `scripts/install.sh` links the binaries from
`~/.local/bin` back into the checkout, and configuration discovery resolves
symlinks before looking beside the binary, that file becomes the effective
default for every installation. A 15-line `config.example.yaml`, a gitignored
`config.yaml`, and an `install.sh` that seeds `~/.kvit-coder/config.yaml` from
the example when none exists would each solve a different part of this.

## Clone weight, and one decision left open

The tracked tree is 7.2 MB. The history holds 67.3 MB of file content across
1,662 blobs, and 51.1 MB of that, in 154 blobs, is benchmark output committed
before `.gitignore` started excluding it: run transcripts, result JSON and
Markdown reports, the interrogation logs, and the haystack corpus that
`scripts/amalgamate-go.sh` regenerates. The largest single file is
`benchmarks/terminal-haystack-6.6b-20260113-060433.txt` at 20 MB. Nothing of the
kind is tracked at `HEAD`; every copy is in the past.

Those figures are uncompressed. Git packs the history before sending it, and
transcripts compress well, so a fresh clone today transfers 6.6 MB rather than
67 MB. Removing the benchmark output from every commit takes that to 2.1 MB,
measured on a rewritten copy rather than estimated.

The rewrite was prepared and verified on 21 September 2026 in a scratch clone,
against `git-filter-repo` with the output paths inverted. The tree at `HEAD` came
out byte-identical to the original (tree `8343be2`), the build and the full test
suite pass on it, all 432 tracked files under `benchmarks/` survive, including
the vendored Thinkwright suite, and one commit disappeared because it added
nothing but benchmark output: `36b95f6`, "non-thinking", seven files of
thinkbench transcripts and results. Commit count goes from 162 to 161, and every
commit hash changes.

What remains is the decision to publish it. Rewriting the history means a force
push, which replaces the public history; the repository has no stars and no
forks, so no existing clone breaks. It is the only step described in this
document that cannot be undone.

## Files a visitor expects and will not find

There is no `CONTRIBUTING.md`, no `SECURITY.md`, no `CHANGELOG.md` and no issue
templates. For a program that executes shell commands on the user's machine,
`SECURITY.md` is more than a formality: it is where the path-safety modes, the
command permission model and the process for reporting a hole are written down.

`CLAUDE.md` at the repository root is worth keeping as it is. It states plainly
which parts of the program are on the default path and which are not, and it
signals that the project is used together with other coding agents.

## A suggested order

1. ~~Fix the formatting and lint findings, pin the linter version, make a failed
   headless turn exit non-zero, put the program's name and stamped version in
   the banner, and name the API key variable in the 401 message.~~ Done on
   21 September 2026; `go test`, `go vet`, `golangci-lint` and `gofmt` are all
   clean. The push has not happened yet.
2. Rewrite the README around the six properties above, record the demo, move the
   reference material into `docs/`, and fix the broken links. Done on
   21 September 2026 except the demo recording.
3. Remove `.clauding`, `eval/`, the stale benchmark configs and the stray log
   file; sanitize the personal paths; add `config.example.yaml`, `CONTRIBUTING.md`
   and `SECURITY.md`.
4. Settle the history rewrite.
5. Set the GitHub description, topics and social preview image, then tag
   `v0.1.0` and let the release workflow publish binaries.

## How this was checked

Against local commit `bc1867e` on 20 September 2026:

- `go build ./...`, `go test ./...` (all packages pass), `go vet ./...` (clean).
- `golangci-lint run ./...` (19 findings) and `gofmt -l .` (9 files).
- A cold first run: the headless binary invoked with an empty `HOME`, no
  `KVIT_CODER_CONFIG` and no API key, once with a config beside the binary and
  once with none reachable, recording the output and the exit status of each.
- `git rev-list --left-right --count origin/main...main` for the 129-commit gap,
  and `gh run list` for the state of the public CI.
- Every relative Markdown link in `README.md`, `docs/README.md`,
  `docs/archive/README.md` and `benchmarks/README.md` resolved against the file
  system.
- `git rev-list --objects --all` piped through `git cat-file --batch-check` for
  the blob sizes, and a scan of every commit's added lines for API keys, tokens
  and hardcoded credentials.
