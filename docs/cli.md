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
  changed a file), the session's title, and the context in use with
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
| `-setup` | Open [`:setup`](#setting-up-models) before the first prompt, to add models or turn on web search | false |

#### Setting up models

`:setup` adds models without editing a file. It lists the providers
kvit-coder-ui knows: OpenCode Zen, OpenCode Go, a local server (llama.cpp,
vLLM, Ollama or LM Studio), and any other endpoint that speaks chat
completions, responses or messages. After you pick one it goes through four
steps:

1. **The key.** It asks for the provider's API key, unless the environment
   variable the key is read from (`OPENCODE_API_KEY` for both OpenCode
   gateways) is set or a key is already saved under that name. For a local
   server it asks for the address instead, and for another endpoint its name,
   address and key variable.
2. **The model list.** It asks the endpoint which models it serves
   (`GET /models`). For OpenCode it also fetches the
   [models.dev](https://models.dev) catalog, kept for a day in
   `~/.kvit-coder/cache/models-dev.json`, which says each model's protocol,
   context size, effort levels and price.
3. **Choosing.** The list shows those details beside each model. Space ticks
   models, Enter goes on, and typing filters the list.
4. **Checking.** It shows the rows it will save, which can be changed first
   (effort levels, context size, name, profile, protocol), and can send each
   model one short test request before saving. The test is the first choice
   when models.dev does not describe a model: when the endpoint refuses the
   protocol the row guessed, the other two are tried, and the one that answers
   is kept.

Saving writes the rows to `~/.kvit-coder/models.yaml` and a key you typed to
`~/.kvit-coder/credentials.json`, which only you can read, then switches to the
first new model. `config.yaml` is never written. Both files, and how they
combine with `config.yaml`, are described in
[configuration.md](configuration.md#models-and-keys-saved-beside-the-config).

Below the providers, `:setup` lists **web search**, which shows whether the
agent may search the web (`Web.search`, through the Brave Search API) and read
pages (`Web.fetch`), and switches each on or off. Turning search on asks for a
Brave Search API key when none is found, and opens the page that issues them;
the key goes to `credentials.json` and page fetch is turned on with search,
because a search result is only a title, an address and a sentence or two. The
switches are saved to `~/.kvit-coder/tools.yaml` and apply from the next
prompt. A switch that `config.yaml` sets itself is shown as set there and
cannot be changed from kvit-coder-ui. See
[web search and page fetch](configuration.md#web-search-and-page-fetch).

kvit-coder-ui opens `:setup` by itself when no model is configured, and once a
model is saved there it asks whether to turn on web search too. When the
starting model reads its key from a variable that is not set and has no saved
key, it offers to save one; a model at an address on this machine is not asked
about, because a local server rarely needs a key.

`:models` lists every model with its protocol, context size and the file it
came from. Enter switches to one, and Delete removes one that `:setup` added
(a model written in `config.yaml` is removed there). `:keys` lists each
variable the models and web search read a key from and where its value comes
from: the environment, a saved key (shown as `sk-ab…wxyz`), or nowhere. It
saves or forgets a key, and after the web search key is entered while search
is off it offers to turn search on. An exported variable is always used before
a saved key.

kvit-coder-ui remembers the model and effort you last chose in
`~/.kvit-coder/kcu-state.json` and starts on them next time, ahead of
`default_model`; `-m` comes before both. The headless agent does not read that
file and starts on `default_model`.

#### Update commands

At most once a day the front end asks GitHub for the latest release and
prints one line when a newer one exists (`:version` shows the current build,
the agent it spawns, and the newest known release). `:update` installs the
latest release after asking, `:update --check` only reports it, and
`:update <tag> --apply` installs a specific release without asking. See
[`updates`](configuration.md#updates) for the check interval and opt-outs.

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
