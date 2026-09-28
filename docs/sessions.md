# Sessions, steering and background work

A session is a directory under `~/.kvit-coder/sessions/<name>/` (`%USERPROFILE%\.kvit-coder\sessions\<name>\` on Windows) holding
everything that has to outlive a turn, because the agent runs as one process per
turn and keeps nothing in memory between them. This file covers working with
sessions from the command line, talking to a turn while it runs, how several
tool calls travel in one request, how a shell command is judged before it runs,
processes that outlive the turn, and the tool the model uses to ask a question.


Conversation history persists across runs via named sessions:

```bash
# Start or continue a session
./kvit-coder -p "set up the project" -s my-feature
./kvit-coder -p "now add tests" -s my-feature

# Manage sessions
./kvit-coder --sessions              # list all
./kvit-coder --session-show my-feature   # view history
./kvit-coder --session-delete my-feature # delete
```

Each session is a directory under `~/.kvit-coder/sessions/` (`%USERPROFILE%\.kvit-coder\sessions\` on Windows):

```
~/.kvit-coder/sessions/my-feature/
    history.jsonl   append-only transcript, one timestamped event per line
    meta.json       when it was created and last used, workspace, model, first prompt
    checkpoints/    the shadow git repository the checkpoint tools commit into
    proc/           which process is running a turn
    inbox/          messages waiting for the running turn
    tmp/            tool output too large to fit in a message
```

Everything that outlives a turn lives there, so a checkpoint made in one run
can be restored in the next and a temp file the model was given the path to is
still readable later. A session left over from when a session was a single
`<name>.jsonl` file is converted the first time it is opened; the old file is
kept as `<name>.jsonl.migrated`.

## Steering a running turn

A message can reach the model between iterations of a turn that is already
running. Type a line at the terminal while the agent works, or send one from
another terminal:

```bash
./kvit-coder steer "actually, check the tests first"
./kvit-coder steer -s my-feature "use the other library"
```

Without `-s` it delivers to the one session that has a turn running, and says
so if there is none or more than one. The message arrives as a user message
tagged `<user-steering>` at the model's next iteration.

Typing at the same terminal competes with the agent's own output, so a
half-typed line can be hard to read while the turn streams past it. For a
clean line, hit `Enter` on an empty line instead: the turn finishes its
current step, pauses at the next iteration boundary, and prompts for steering
— type it, hit `Enter` again, and the turn continues. `Enter` on the empty
prompt resumes with nothing added, and `Enter` while a question is waiting
still answers the question rather than pausing.

## Batching calls

With `tools.batch.enabled`, several independent calls go in one request:

```
Batch({"calls": [
  {"tool": "Read", "args": {"path": "main.go"}},
  {"tool": "Read", "args": {"path": "config.go"}},
  {"tool": "Search", "args": {"pattern": "func main"}}
]})
```

One request and one round of thinking instead of one each. Calls that only read
run at the same time, so four reads take about as long as one; calls that
change the workspace run afterwards, in order. Each call comes back with its own
result or its own error, so one failure does not lose the others. At most ten
calls, and no `Batch` inside a `Batch`.

## Command permissions

Whether a shell command may run is decided from its syntax tree, not from
matching its text. Every simple command in the line is found and judged on its
own, so `git diff && rm -rf /` is refused for its second command, `ps aux | awk
'{print $2}'` runs, and a word like `shutdown` in a grep pattern is an argument
rather than a program.

Refused outright: `sudo`, `su`, `chroot`, package managers, `shutdown`/`reboot`,
and deleting `/` or your home directory. Deleting anything *under* those
directories is ordinary cleanup and is allowed. Under `agent.profile: weak`,
`sed -i` is refused too when the Edit tool is enabled, since a weak model will
otherwise edit files with it instead of calling Edit; the strong profile allows
it, and sed is an ordinary shell command there.

`--yolo` answers these questions for you, which is what it is for in a run with
no terminal: there an unanswered question comes back as a refusal rather than a
pause, so without it a headless `--yolo` run fails on the first `curl` it needs.
Two things it does not answer. What is refused outright stays refused, since a
refusal was never a question. And `dd` and the `mkfs` family keep asking, so
with no terminal they are still refused — everything else on the list can be
undone or lived with, whereas an overwritten disk cannot. Each command `--yolo`
allows is named on the terminal as it happens, so the run records what the flag
decided rather than only that it was passed. The same switch is
`tools.shell.allow_without_asking` in the config file.

None of this is a sandbox. A denied command is generally reachable some other
way — through an interpreter, or by writing a script and running it — and no
rule here tries to close that off. What the rules catch is a command that would
do damage without anyone having decided to.

Needing permission — dangerous in general, ordinary in context — are `curl`,
`wget`, `nc`, `dd` and the `mkfs` family. Under `agent.profile: weak`, `eval`
and interpreter one-liners such as `python -c` are added to that list; the
strong profile runs them without asking, since the same code written to a file
and run is refused by nothing. Whichever way a command is refused, the message
says in one line what the command would do rather than which list it is on,
since that is all a person has to decide on and all the model is told when
nobody is there to ask. At a
terminal you are asked, with four answers: just this once, for the rest of this
session, always for this project, or always everywhere. The last three are
written to permission files under `~/.kvit-coder/permissions/` and the session
directory — never inside the workspace, where the agent could edit them. With
no terminal the command is refused and the model is told which pattern to add
to `tools.shell.allowed_commands`.

`allowed_commands` and `disallowed_commands` still work, now as patterns
matched against each command in the line rather than as a prefix of the whole
string. A pattern is a list of words: `curl` is every curl, `python3 -c` is only
the one-liner form, and a program name ending in `*` covers a family of programs
that differ only by suffix, so `mkfs.*` is `mkfs.ext4`, `mkfs.xfs` and the rest.
Setting `allowed_commands` still makes everything else a denial, and a grant can
never open something that is refused outright.

## Background processes

With `tools.procs.enabled`, a command can outlive the turn's iterations (and, if persistent, the turn itself):

```
Shell.start({"command": "npm run dev", "name": "dev server"})   → {"id": "bg1"}
Shell.start({"command": "npm run dev", "persistent": true})   → {"id": "bg1"} (survives the turn, red)
Shell.start({"command": "npm run dev", "until": "Listening on", "remind_every": 60})
Observe.wait({"id": "bg1", "until": "Listening on", "report": "match"})
Shell.output({"id": "bg1", "cursor": 4096})
Shell.list({}) / Shell.status({"id": "bg1"}) / Shell.kill({"id": "bg1"}) / Shell.tune({"id": "bg1", "remind_every": 30})
Observe.add({"command": "git status --short", "every": 60})
```

`Shell` is unchanged: it runs a command to completion and dies with the turn.
`Shell.start` detaches into its own session so it survives the turn's
iterations, and its output and exit status are recorded in `<session>/proc/`
where the next iteration reads them. It is ephemeral by default: stopped when
the turn ends, so abandoned sessions leak nothing. Pass `persistent: true`
only for what the next turn still needs, like a dev server — it survives the
turn, is shown red (`★N!`, blue `★N` is ephemeral), and is listed at turn end
with how to stop it. Persistent still dies on interrupt (unless
`tools.procs.kill_on_exit` is false), on `Shell.kill`, and on reboot
(reported as `gone`): for a service that must outlive those, write a script
and ask the user to run it. Say when to be told
up front — `report` (`exit` by default: nothing until it ends), `until` (a
pattern that reports on match), `remind_every` (a "still running" tick every
that many seconds) — and the reminders arrive on their own between iterations,
with no round trip to another tool. `Shell.tune` adjusts the same policy
(including `persistent`) on a running process without restarting it.
Reap leftovers any time: `kvit-coder --list-background <session|all>`,
`kvit-coder --kill-background <session|all>`. Deleting a session stops its
processes first, so it never orphans them.

`Observe.wait` blocks until the process ends, prints something matching a
pattern, or `max_wait` passes — one tool call however long it takes, where
checking with `Shell.output` in a loop costs a full round of thinking each
time. It also returns early if you type something, so you are not left waiting
for a process whose result no longer matters.

Between iterations the agent asks the registry what has happened and tells the
model about anything it asked to hear — an ending, a pattern match, new
output, or a tick of the clock — so watching costs nothing until there is
something to say. Ctrl-C stops everything the session started, unless
`tools.procs.kill_on_exit` is false.

## Questions

With `tools.question.enabled`, the model can ask you something and wait for the
answer, which comes back as the tool's result:

```
── retry strategy ──
Reuse the existing retry wrapper, or write a new one?
  1) Reuse internal/http.Retry  — same backoff, already tested
  2) New wrapper in this package  — no shared state
  [1-2, or type]
```

A bare number chooses, `1,3` chooses several, and anything else comes back as
free text — so answering and steering are the same keystrokes and you never
have to decide which you are doing before you type. A line typed before the
question appeared is treated as steering rather than silently taken as the
answer. Ctrl-C dismisses the question and ends the turn; the model is refused
if it asks the same question again.

With a terminal the call waits as long as it takes. With no terminal it waits
`tools.question.timeout` seconds for an answer dropped in the session inbox
(`kvit-coder steer` writes there), then tells the model to proceed on its own
judgement and say what it assumed. The default of zero falls back at once, so a
benchmark or scripted run never hangs on a question.
