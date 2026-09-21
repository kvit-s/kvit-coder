# Security

## What this program does to a machine

kvit-coder runs shell commands and edits files on the machine it is started on,
under the account that started it, with whatever that account can reach. The
commands come from a language model reading a conversation that includes the
contents of files in the workspace. Treat it as you would treat running a script
somebody else wrote: on work you can afford to lose, in a directory you have
backed up or committed, and not as a user with more access than the task needs.

Two things constrain it, and both are configuration rather than a sandbox.

**Paths.** `workspace.path_safety_mode` decides what happens when a tool touches
a path outside the workspace: `allow`, `block`, `warn`, `ask_once` or
`ask_always`. The asking modes prompt on `/dev/tty` and remember the answer for
the run. Containment is computed with `filepath.Rel` rather than string
prefixing, so `/home/x/project-notes` does not count as inside `/home/x/project`.
Symbolic links are not resolved before the check.

**Commands.** A shell command is parsed into a syntax tree and every simple
command in the line is judged separately, so `git diff && rm -rf /` is two
decisions rather than one. Built-in rules cover the destructive cases; the
`allowed_commands` and `disallowed_commands` lists in `tools.shell` add to them.
A refusal is returned to the model as a tool result saying what the command
would have done, so the turn continues rather than stopping.

`--yolo` turns off both kinds of asking for a run. It means what it says.

## What it is not

There is no sandbox, no container, no syscall filter and no network policy. A
command that is allowed to run can do anything the account can do, including
reading credentials in the environment and making network requests. The
permission system decides what to run, not what a running command may do.

Conversations, including file contents the model was shown, are sent to whatever
endpoint `llm.base_url` names. With a hosted endpoint that means the contents of
your workspace leave the machine.

## Reporting something

Open an issue at https://github.com/kvit-s/kvit-coder/issues for anything that
is already public or harmless to describe. For a hole that is not — a way past
the path check or the command rules, or anything that would let a model reach
outside what the configuration allows — use GitHub's private vulnerability
reporting on the same repository, under Security, rather than an issue.

This is a personal project rather than a product with an on-call rotation. There
is no response-time commitment; reports are read and answered when there is time
to do them justice.
