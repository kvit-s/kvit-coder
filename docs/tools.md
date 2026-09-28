# Tools

The tools the model can call. Which of them are switched on, and their options,
is set under `tools:` in `config.yaml`; every key is in
[`configuration.md`](configuration.md).

| Tool | Description |
|------|-------------|
| **Read** | File contents, with partial reads for large files. Files only: a directory path is refused with a pointer to Glob. A PDF is detected from its bytes and its text extracted a page range at a time. Other non-text files — images, archives, databases, compiled output — are named, with the tool or command that opens them, rather than read as bytes. |
| **ReadImage** | Read an image (PNG, JPEG, GIF) so the model can see it. Large images are downscaled. |
| **Edit** | Three modes: **lines** (line ranges), **searchreplace** (find and replace, tolerant of whitespace differences), **patch** (unified diffs). Optional preview mode with confirm or cancel. |
| **Write** | Create a file or overwrite one. |
| **Glob** | Find files by glob pattern (`**/*.go`), or list a directory when called without a pattern. Sorted, capped, build and VCS directories excluded. |
| **Search** | Regular-expression search with glob filters, through ripgrep. |
| **Shell** | Run a command, with timeouts, a working directory, and permission decided from the parsed command line. |
| **Shell.start / Shell.output / Shell.status / Shell.list / Shell.kill / Shell.tune / Observe.wait / Observe.add** | Background processes that keep running while the turn continues: a dev server, a long test run, a build. |
| **Batch** | Several independent tool calls in one request. |
| **Subagent** | Delegate research to a child agent with its own context and a read-only tool set (research-only, sequential, fresh context — inline every path/symbol/constraint; the child sees no parent history). |
| **Question** | Ask the person a question rather than guessing, when there is somebody at the terminal. |
| **Web.search / Web.fetch** | Search through the Brave API, and fetch one page as markdown. Both off by default; the search needs a key. |
| **Web.browsing** | A real browser through a Playwright server, behind a tool group that hides its tools until opened. |
| **mcp.\*** | Tools from external [Model Context Protocol](https://modelcontextprotocol.io) servers, over stdio or HTTP. See [`docs/mcp.md`](mcp.md). |

Sending input mid-turn, grouping independent calls, shell command permissions,
background processes and the question tool are covered in
[`sessions.md`](sessions.md).
