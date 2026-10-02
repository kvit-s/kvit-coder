# How kvit-coder is built

This page explains the design decisions that shape the program: why a turn is
a process of its own, why a session is a directory, how input reaches a turn
that is already running, how a shell command is judged, why some tools stay
hidden until they are needed, and how PDFs and Office documents are read. The reference detail for
each is in the other pages of [`docs/`](README.md), linked from each section.

## One process per turn

`kvit-coder` starts, reads the session from disk, runs one instruction, appends
what happened, and exits. If a turn gets stuck, ending its process ends the
turn. The front end, `kvit-coder-ui`, keeps no conversation in memory: it starts
`kvit-coder` again for every turn, and memory a turn used goes back to the
operating system when the process exits. Anything that has to outlive a turn is
therefore written to the session directory described next.

## A session is a directory

Sessions live under `~/.kvit-coder/sessions/<name>/` (`%USERPROFILE%\.kvit-coder\sessions\<name>\` on Windows): the transcript, the
session's metadata, background
process logs, the inbox, and tool output too large to put in a message. It is
plain files, so a session is also a record you can open months later and read.
The layout, entry by entry, is in [`sessions.md`](sessions.md).

## Sending input to a running turn

A line typed at the terminal, a file dropped by `kvit-coder steer` from another
shell, or an event from a background process all go to the same inbox
directory, which the loop checks once per iteration. Pressing Enter on an empty
line asks the turn to pause at the next point where it can stop safely, which
pauses it without ending it. How to steer a turn and what the model sees is in
[Steering a running turn](sessions.md#steering-a-running-turn).

## Shell permissions from the parsed command

Each shell line is parsed into separate commands and each one is checked on its
own against the built-in rules and the allow and deny lists in `tools.shell`.
`git diff && rm -rf /` counts as two commands and is checked as two, the name
of a program being run is treated differently from the same word appearing as a
`grep` pattern, and a blocked command can be allowed for one call, one session,
or permanently. The rules themselves are in
[Command permissions](sessions.md#command-permissions).

## External tools stay hidden until used

A tool group is one visible tool that stands in for a set the model cannot see
until it opens the group. Playwright's 24 browser tools add about 7,400 tokens
of schema and 43 tools to every request when advertised directly, while the
same configuration behind a group adds about 3,400 tokens and 20 tools. Nothing
behind the group starts until a call actually needs it.

When the model opens a group, the group's tools arrive as the result of that
call rather than as new tool definitions. The tool definitions come before the
whole conversation in each request, so adding tools partway through a session
would make the endpoint's prompt cache miss for every message after them, while
a tool result is appended like any other message. Groups are configured under
`tool_groups:` in `config.yaml`; `Web.browsing` is the one shipped.

## PDF reading without external programs

A PDF is read inside the agent process through PDFium compiled to WebAssembly
and run by wazero, with page ranges and a fallback that renders a page as an
image when the page holds only scanned images with no extractable text.

## Word, Excel and PowerPoint files

A .docx, .xlsx or .pptx file is a zip archive of XML. Read converts it to
Markdown with the code in `internal/office`: headings, lists, tables, one
table per worksheet, slide text and speaker notes. That code is copied from
docstomd-go, a Go port of the office converters in Firecrawl's anydoc. A Go
copy was chosen over anydoc itself, which is Rust and would have to run as a
second WebAssembly module, because on the documents both were tried on the
output was identical and the Go version used a fifth of the memory. Only the
office packages were copied; `internal/office/office.go` records the upstream
commit, the changes made to the copy, and how to take a newer version.

The converted Markdown is saved in the session's `tmp/` folder under a name
derived from the document's path, size and modification time, and every read
of the document is a line read of that file. Paging through a long spreadsheet
is therefore one conversion, and an edited document is converted again.
Conversion is refused above 16 MB of unpacked XML, because peak memory runs at
11 to 19 times that figure.

Only the zip-based formats Office has written since 2007 are converted. The
older binary .doc, .xls and .ppt formats, OpenDocument, RTF and EPUB are named
with a command that opens them, as before, because no maintained Go library
reads .doc or .ppt text and these formats were 2 of 95 office files on the
machine this was measured on.
