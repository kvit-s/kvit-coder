# Contributing

## What this project is

kvit-coder is written for its author's own use and shared because it may be
useful to somebody else. It is not a product, there is no roadmap you can rely
on, and there is no commitment to respond within any particular time. That is
worth knowing before you spend an afternoon on a pull request.

Bug reports are welcome, particularly from anyone running it against a model on
their own hardware, which is the case the program is built for and the one with
the fewest people testing it. Say which model, which endpoint, and what the
agent did.

## Before a pull request

Open an issue first for anything beyond a fix. A change that fits how the
program is put together is much likelier to be taken than one that is merely
correct, and that is easier to establish in a paragraph than in a diff.

## Working on it

```bash
scripts/build.sh          # both binaries into the repository root
go test ./...             # all green; keep it that way
go vet ./...
gofmt -l .                # should print nothing
golangci-lint run ./...   # should print nothing; CI pins the version
```

There is no `config.yaml` in the repository: copy `config.example.yaml` to
`~/.kvit-coder/config.yaml` and point it at an endpoint you have.
`scripts/install.sh` links the binaries you build into `~/.local/bin`, so a
later `scripts/build.sh` is all it takes to update them.

`CLAUDE.md` describes how the program is organised and names the things in it
that mislead a newcomer. It is written for a coding agent working in the
repository and is just as useful to a person.

## What the code looks like

Commit messages are lowercase, with an area prefix and a body saying why:
`llm: talk to endpoints that only serve the Responses API`. The diff already
shows what changed.

Comments explain why something is the way it is, not what the line does. Several
of the decisions here look wrong until you know what they are avoiding — tool
specifications are sorted deterministically so a prompt cache keeps hitting,
a tool group's members arrive as a tool result rather than as new tool
specifications for the same reason — so when you change one of those, say so in
the comment.

Tests go next to what they test and are expected to pass on Linux. macOS should
work and is not tested; Windows does not compile.

## Licence

By contributing you agree that your work is published under the MIT licence in
[`LICENSE`](LICENSE).
