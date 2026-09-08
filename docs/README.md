# Documents

Design notes and reviews for kvit-coder. The program itself is described in the
repository root's `README.md`; these are the documents behind its current shape.

## Current

| File | What it is |
|---|---|
| [`review.md`](review.md) | Architecture review of the program as it stood at commit `fe77715`, September 2026, and the case for the redesign. Section 2 describes the whole system and is the fastest way in. |
| [`redesign.md`](redesign.md) | What to build instead: one process per turn, a session directory, a steering inbox, background processes, a question tool, syntax-tree command permissions, batched calls, model profiles. Sixteen sections plus an appendix of ideas not taken. |
| [`redesign-plan.md`](redesign-plan.md) | How `redesign.md` was implemented, stage by stage. All eleven stages are committed; the header records what is left. |
| [`redesign-mcp.md`](redesign-mcp.md) | A proposal, not yet built. One process per turn tears down MCP server connections every turn, which costs more than the reconnect time for servers holding per-connection state. Proposes a separate long-running program that holds them. |
| [`web-tools.md`](web-tools.md) | Web search, page retrieval and browser control as three separate capabilities: a native `Web.search` and `Web.fetch` in the agent, a browser driven through an MCP server, and what that server adds to the daemon in `redesign-mcp.md`. `Web.search` and `Web.fetch` are built; the browser half is not. Records the measured state of the Brave API key, and what a browser costs in memory. |
| [`bench-refactor.md`](bench-refactor.md) | A proposal, not yet built. Moving the benchmark harness out of the agent binary into its own command. |
| [`tools-stats.md`](tools-stats.md) | Which tools the model actually calls, measured from the saved sessions, with the script that regenerates the numbers. A snapshot to decide from later, not a recommendation. |
| [`agents-ram.md`](agents-ram.md) | Measured memory cost of one session of each coding agent on this machine, September 2026. Reference for how many can run at once, not about kvit-coder's design. |
| [`images.md`](images.md) | Reading an image from a file and pasting one in the composer, built September 2026: message attachments, both wire protocols, session storage, then the TUI producers. |

## Archive

[`archive/`](archive/) holds plans and reviews for work that is finished or
abandoned. They describe states the program has left, so read them as history
rather than as guidance. `archive/README.md` says what each one was.
