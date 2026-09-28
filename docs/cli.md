# Command-line flags

Every flag both binaries take. `kvit-coder` runs one turn and exits and is what
a script drives; `kvit-coder-ui` is the interactive front end, which spawns
`kvit-coder` once per turn. The root `README.md` covers what the two programs
are for; this file is the reference.


## kvit-coder

| Flag | Description | Default |
|------|-------------|---------|
| `-p <prompt>` | Run with prompt and exit | - |
| `-pq <prompt>` | Quiet mode: only print final response | - |
| `-image <path>` | Attach an image file to the prompt (repeatable) | - |
| `-config <path>` | Config file path | first found (see [Configure](#configure)) |
| `-model <name>` | Override model | from config |
| `-base-url <url>` | Override LLM endpoint | from config |
| `-agent-file <path>` | Append file content to system prompt | - |
| `-log <path>` | Log file (empty to disable) | `~/.kvit-coder/logs/kvit-coder.log` (`%USERPROFILE%\.kvit-coder\logs\kvit-coder.log` on Windows) |
| `--json` | Structured JSON output to stderr | false |
| `--structured` | End the turn with a structured report instead of prose (a validated JSON report; see `Report` in the tool list) | false |
| `-s <name>` | Continue or create named session | - |
| `-c` | Continue the most recent session | - |
| `--yolo` | Read and write anywhere, and run anything that would have asked | false |
| `--sessions` | List sessions | - |
| `--session-show <name>` | Show session history | - |
| `--session-delete <name>` | Delete a session | - |
| `--version` | Show version info | - |

### kvit-coder copilot models

`kvit-coder copilot models [-config path] [-host hostname]` prints the models
the signed-in GitHub Copilot account can call, and the API host the token
exchange selected (personal, business, or enterprise). Each line is the model
id, the wire protocol, the context window, and the effort levels when the
model publishes any. Models the account's policy has turned off are left out.
`-host` is a GitHub Enterprise hostname such as `company.ghe.com`. The command
reads the same sign-in Copilot CLI uses; see [GitHub Copilot](configuration.md#github-copilot)
for the token order and the `provider: github-copilot` model entry.

### kvit-coder acp

`kvit-coder acp [-config <path>] [-log <path>]` runs the agent for an editor or
another program that speaks the Agent Client Protocol, version 1: JSON-RPC
messages, one per line, on standard input and standard output. Nothing else is
written to standard output. What a `-p` run prints for a person goes to
standard error, and a command the model runs reads nothing from standard input
and writes its output into its tool result.

- Sessions are kvit-coder sessions, and a session's id is its name.
  `session/new` makes one whose workspace is the `cwd` the client sends;
  `session/resume` reopens one; `session/load` reopens it and first sends its
  history as updates; `session/list` gives the sessions whose workspace is the
  directory asked about; `session/close` stops its prompt if one runs.
- Each session has a `model` setting, from the config's `models:` catalogue,
  and a `thought_level` setting, the chosen model's effort menu, changed with
  `session/set_config_option` as `--model` and `--effort` change them.
- Each `session/prompt` is one turn, set up as a `-p` turn is. Its steps arrive
  as `session/update` notifications: the model's text and reasoning, each tool
  call with its kind, file and arguments, its result (with a diff when it
  changed a file), the plan, the session's title, and the context in use with
  the session's cost so far. The answer gives `end_turn`, `max_turn_requests`
  when the iteration budget ran out, or `cancelled` after `session/cancel`; a
  turn that failed is answered with an error giving the reason.
- A shell command that needs permission, a path outside the workspace, and a
  Model Context Protocol tool not yet allowed are asked about with
  `session/request_permission`, offering "Allow once", "Allow for this
  session" and "Refuse". The answers that would write a permanent grant are
  not offered.
- When the client declares `elicitation.form` in `initialize`, the `Question`
  tool puts each call's questions to it as one `elicitation/create` form, a
  field per question (`q01`, `q02`, …): a choice for a question with options,
  several choices for one that takes several, text otherwise, and beside each
  choice a `<field>_other` text field for an answer in the person's own words.
  It waits for the answer with no time limit. `decline` tells the model the
  person chose not to answer, and `cancel` is taken as the turn being stopped.
  A client that shows no forms gets the behaviour with no terminal: the
  question waits `tools.question.timeout` seconds in the session's inbox.
- `_meta.systemPrompt` on `session/new`, `session/resume` or `session/load`,
  a string or an object with `append`, is added to the system prompt as
  `-agent-file` is. `_meta.kvit.policy` holds what the session may touch:
  `readable` and `writable` roots, `denied` paths (each with `read` and
  `write`), and `shell`. It replaces the config's path lists for that session,
  refuses any path outside them, makes the workspace read-only unless a
  writable root covers it, and switches off the editing tools when nothing is
  writable and the shell and background process tools when `shell` is false.

## kvit-coder-ui

| Flag | Description | Default |
|------|-------------|---------|
| `-config <path>` | Config file path | first found (see [Configure](#configure)) |
| `-agent-path <path>` | Path to kvit-coder binary | auto-detected |
| `-s <name>` | Continue or create named session | - |
| `-c` | Continue the most recent session | - |
| `--yolo` | Read and write anywhere, and run anything that would have asked (passed to each turn) | false |
| `--structured` | End each turn with a structured report, drawn as a card at the prompt (`--structured=false` for prose) | true |

#### Window title

While it runs, `kvit-coder-ui` keeps the terminal's window title — which a
tabbed terminal such as VS Code's shows as the tab label — saying what is
happening, so a window that is not on screen still tells you whether it needs
you:

| Title | State |
|---|---|
| `⏳ <session title>` | a turn is running |
| `💬 <session title>` | the turn has stopped to ask you something and is waiting for the answer: a [`Question`](#questions), a path or MCP confirmation, or the pause prompt |
| `💤 <session title>` | nothing is running and the composer is open |

The session title is the short summary of the first prompt that `--sessions`
lists; until the first turn has produced one the session's directory name
stands in.

The front end sets the title around each turn. The `💬` state is set by the
agent process itself, because the front end is blocked waiting for the turn to
finish and cannot see that it has stopped to ask something; the agent hands
the title back when the answer arrives. A headless `kvit-coder` run never
touches the title — there is no long-lived process to put it back afterwards.

The terminal has to be willing to take a title from the program running in it.
Most of them already are — Windows Terminal, iTerm2, GNOME Terminal, Alacritty
and Kitty need nothing — but two need a setting:

**VS Code** labels each terminal tab with the shell's name and ignores the
title the program set, until you tell it otherwise. Open the command palette
(`Ctrl+Shift+P`), run *Preferences: Open User Settings (JSON)*, and add:

```json
"terminal.integrated.tabs.title": "${sequence}"
```

`${sequence}` means "the title the running program set". You can combine it
with the others to keep a fallback for terminals running something that sets no
title: `${process}` is the shell's name (what the default shows), and
`${separator}` is a dash that appears only when there is something on both
sides of it, so `"${sequence}${separator}${process}"` gives you the agent's
title when there is one and the shell's name when there is not.

When VS Code is connected to a container, an SSH host or WSL, this one belongs
in the *local* settings file rather than the remote one, because the tab is
drawn by the window on your own machine. That is what *Open User Settings
(JSON)* gives you by default; on Windows the file is at
`%APPDATA%\Code\User\settings.json`, and from inside WSL the same file is
`/mnt/c/Users/<you>/AppData/Roaming/Code/User/settings.json`.

**tmux** needs `set -g set-titles on` in `~/.tmux.conf` before it passes a
title through to the terminal around it.

All three icons are configurable and the whole thing can be turned off; see
[`ui`](#ui) below. Nothing is written when output is redirected, and
`KVIT_CODER_NO_TITLE=1` in the environment turns it off without a config edit.
