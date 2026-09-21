# Documentation

Reference for using kvit-coder. The root [`README.md`](../README.md) describes
what the program is and how to install it; these pages are the detail behind it.

| File | What it covers |
|---|---|
| [`cli.md`](cli.md) | Every command-line flag both binaries take. |
| [`configuration.md`](configuration.md) | Every key in `config.yaml`, grouped by the section it sits in. |
| [`sessions.md`](sessions.md) | Sessions on disk, steering a running turn, batched calls, how a shell command is judged before it runs, background processes, and the tool the model uses to ask a question. |
| [`mcp.md`](mcp.md) | Tools from Model Context Protocol servers, per-project instructions, and the agent file. |
| [`agents-ram.md`](agents-ram.md) | Measured memory cost of one session of each coding agent on one machine, September 2026, with the method. |

The design notes that explain why the program is shaped the way it is are not in
this repository. They were working documents — proposals, reviews and stage
plans, some of them describing things that were never built — and reading them
as current guidance sends you the wrong way.
