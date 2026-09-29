# Documentation

Reference for using kvit-coder. The root [`README.md`](../README.md) describes
what the program is and how to install it; these pages are the detail behind it.

| File | What it covers |
|---|---|
| [`tools.md`](tools.md) | Every tool the model can call, from reading files to background processes and external servers. |
| [`cli.md`](cli.md) | Every command-line flag both binaries take. |
| [`configuration.md`](configuration.md) | Every key in `config.yaml`, grouped by the section it sits in. |
| [`models.md`](models.md) | How to add a model from OpenCode Zen (including the free models) and from the OpenCode Go subscription. |
| [`sessions.md`](sessions.md) | Sessions on disk, steering a running turn, batched calls, how a shell command is judged before it runs, background processes, and the tool the model uses to ask a question. |
| [`mcp.md`](mcp.md) | Tools from Model Context Protocol servers, per-project instructions, and the agent file. |
| [`architecture.md`](architecture.md) | Why the program is built the way it is: one process per turn, sessions as directories, input to a running turn, shell permissions, tool groups, and PDF reading. |
| [`agents-ram.md`](agents-ram.md) | Measured memory cost of one session of each coding agent on one machine, September 2026, with the method. |

[`architecture.md`](architecture.md) describes the design as it is now. The
working notes kept while designing it are not in this repository: they were
proposals, reviews and stage plans, some of them describing things that were
never built, and reading them as current guidance sends you the wrong way.
