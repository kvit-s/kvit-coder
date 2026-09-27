# Tools

The tools the model can call. Which of them are switched on, and their options,
is set under `tools:` in `config.yaml`; every key is in
[`configuration.md`](configuration.md).

| Tool | Description |
|------|-------------|
| **Read** | File contents or a directory listing, with partial reads for large files. A PDF is detected from its bytes and its text extracted a page range at a time. Other non-text files — images, archives, databases, compiled output — are named, with the tool or command that opens them, rather than read as bytes. |
| **ReadImage** | Read an image (PNG, JPEG, GIF) so the model can see it. Large images are downscaled. |
| **Edit** | Three modes: **lines** (line ranges), **searchreplace** (find and replace, tolerant of whitespace differences), **patch** (unified diffs). Optional preview mode with confirm or cancel. |
| **Write** | Create a file or overwrite one. |
| **Search** | Regular-expression search with glob filters, through ripgrep. |
| **Shell** | Run a command, with timeouts, a working directory, and permission decided from the parsed command line. |
| **Shell.start / Shell.output / Observe.wait** | Background processes that keep running while the turn continues: a dev server, a long test run, a build. |
| **Batch** | Several independent tool calls in one request. |
| **Question** | Ask the person a question rather than guessing, when there is somebody at the terminal. |
| **Web.search / Web.fetch** | Search through the Brave API, and fetch one page as markdown. Both off by default; the search needs a key. |
| **Web.browsing** | A real browser through a Playwright server, behind a tool group that hides its tools until opened. |
| **Plan.\* / Checkpoint.\* / Tasks.\*** | Plan management, file versions per turn kept in a separate git repository, and shortening of old history to fit the context window. Plan and Checkpoint are one tool group and Tasks is the other; enable one or the other. |
| **mcp.\*** | Tools from external [Model Context Protocol](https://modelcontextprotocol.io) servers, over stdio or HTTP. See [`docs/mcp.md`](mcp.md). |

Sending input mid-turn, grouping independent calls, shell command permissions,
background processes and the question tool are covered in
[`docs/sessions.md`](sessions.md).
