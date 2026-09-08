# Reading and pasting images

## 1. What this document is

kvit-coder is a text-only agent today: the composer accepts text, the `Read`
tool returns text, a message is a string, and both wire protocols send strings.
There is no path by which pixels reach the model. A `grep` for
`image|base64|image_url|vision|png|jpeg` across `internal/` returns zero
matches.

This document says how to add one, in two phases: first read an image from a
file and send it to the model, then let the composer paste one. The first is a
prerequisite of the second, not a separate feature — paste without a working
file-to-model path has nothing to produce.

Nothing here is built. It is a proposal in the same sense as
[`redesign-mcp.md`](redesign-mcp.md) and [`bench-refactor.md`](bench-refactor.md):
the reasoning and the plan are written down so the work can be picked up later
without re-deriving them. The reader is assumed not to have the code open, so
the parts of it that matter are described where they come up, with file
references that were correct in September 2026 and will drift.

## 2. Where the text-only assumption lives

Five places, each of which has to change or be worked around. They are listed
in the order an image travels, from keystroke to wire.

### 2.1 A message is a string

`internal/llm/types.go:12` defines the conversation as the rest of the program
sees it:

```go
type Message struct {
    Role    MessageRole `json:"role"`
    Content string      `json:"content,omitempty"`
    // ... ToolCalls, ReasoningBlocks, ToolCallItemIDs
}
```

Every producer and consumer assumes `Content` is text: the runner
(`internal/agent/runner_llm.go`, `runner_tools.go`, `runner_iteration.go`),
the interrogator (`internal/agent/interrogate.go:lastToolResult`), the
metacommands preview (`internal/agent/metacommands.go:59`), steering and the
inbox (which append to `Content` with `+=`). Tool results are strings too:
`executeToolWithTimeout` at `internal/agent/runner_tools.go:350` returns
`(content string, ...)`, and the caller stores it as
`llm.Message{Role: RoleTool, Content: content}`. There is no channel for
binary or blobs anywhere in this chain.

### 2.2 Both wire paths assume text

`Client.Chat` (`internal/llm/client.go:165`) speaks one of two protocols. The
chat-completions path marshals the request struct directly. The Responses path
— the default, and what the shipped config uses for Muse Spark through
opencode.ai — translates through `messagesToResponsesInput`
(`internal/llm/responses.go:145`): a tool call and its result become
`function_call` / `function_call_output` items, and every system/user message
becomes `{type: "message", content: <string>}` (lines 191–199). A multimodal
message needs structured content parts on both paths, and neither path has
them.

### 2.3 `Read` returns text

`ReadFileTool.Call` (`internal/tools/filesystem.go:390`) does line mode by
default and seek-based `char_mode` for large files, truncating at 150 lines or
24KB with a ~128KB ceiling. There is no MIME sniffing and no binary guard.
Pointing it at a PNG today returns the bytes decoded as text inside a JSON
string tool result — the model sees mojibake, never pixels. Its schema
(`JSONSchema`, line 342) offers `path`, `char_mode`, `start`, `limit`: no way
to ask for an image even if one were supported underneath.

### 2.4 The UI hands the agent a string over `argv`

`kvit-coder-ui` does not link the agent; it spawns one `kvit-coder` process per
turn (`internal/tui/ui.go:303 runAgent`):

```go
args := []string{"-p", prompt}
cmd := exec.Command(u.agentPath, args...)
```

The prompt travels as a command-line argument, with `-s`, `-config`, `-yolo`
alongside it. Binary or multi-megabyte base64 cannot ride there (argument size
limits, quoting, and prompt-cache churn — see 4.3). The headless binary builds
its user message the same way (`internal/repl/repl.go:27 RunExec`:
`Content: promptWithProjectInstructions(promptText, ...)`). Any image the
composer collects needs its own transport next to `-p`, not inside it.

The composer itself (`internal/ui/input.go:InputModel` around
`bubbles/textarea`) is a text widget. Terminals do not deliver pasted image
*bytes* to it in the first place (see 5.1), so paste support is input plumbing
around the widget, not a widget feature.

### 2.5 There is no image dependency

`go.mod` has no imaging library. The standard library can decode PNG and JPEG
but cannot resize; downscaling (required — see 4.4) wants something like
`golang.org/x/image/draw`, which is pure Go and keeps `scripts/build.sh`
(`--race`, `--release`, `-trimpath`) free of cgo. The indirect
`github.com/atotto/clipboard` (via bubbles) is text-only and does not help.

## 3. Order of work: file first, paste second

Paste depends on read-from-file working, so phase 1 is the whole file →
message → wire → session path, exercised headlessly. Phase 2 adds producers
that feed that path. Concretely:

- **Phase 1:** `llm.Message` carries image attachments; both backends send
  them; a tool puts them there; the session persists them; logs redact them.
  Verified with `kvit-coder -p ... -image foo.png` against the real endpoint,
  no TUI involved.
- **Phase 2:** the composer collects image paths — `@path` references,
  drag-drop paths, clipboard bytes saved to a file — and hands them to the
  existing phase-1 transport.

This order keeps each phase testable without the other, and means the TUI work
never blocks on protocol questions.

## 4. Phase 1: read an image from a file and send it

### 4.1 Carry attachments on `Message`, following the `ReasoningBlocks` precedent

`ReasoningBlocks` and `ToolCallItemIDs` (`types.go:24-27`) are the model for
data that rides along on a message but is serialized per-backend rather than
as JSON: tagged `json:"-"`, kept in memory, expanded at request-build time.
Add the same:

```go
type ImagePart struct {
    MediaType     string `json:"media_type"` // image/png, image/jpeg, image/webp
    Path          string `json:"path,omitempty"` // session tmp ref, see 4.2
    Width, Height int
}
type Message struct {
    // ... existing fields
    Images []ImagePart `json:"images,omitempty"`
}
```

Old histories (no `images` field) must load unchanged — the field is additive,
and `Session.Load` (`internal/session/session.go:342`) should treat its
absence as no attachments.

Out of scope for the struct: inline `DataB64`. See next section for why the
bytes live on disk.

### 4.2 Store bytes in the session directory, encode at request time

Two options for where the pixels live between turns:

- (a) **inline base64 in `history.jsonl`** — simple, but every turn re-sends
  megabytes, the append-only history grows without bound, and the
  byte-identical prompt prefix the server-side prompt cache needs (see 4.3)
  churns on every image turn;
- (b) **copy the normalized file into the session's `tmp/`** (the directory
  that already holds tool output too large for a message;
  `internal/session/session.go:41,204 TmpDir`), reference it by path from the
  message, and base64-encode only when building the HTTP body.

(b) is the one that fits the program's shaping fact: one process per turn, so
nothing that must outlive a turn lives in memory — it goes in the session
directory (`history.jsonl`, `meta.json`, `checkpoints/`, `proc/`, `inbox/`,
`tmp/`). The tool result itself stays a small JSON summary
(`{success, path, media_type, width, height, size}`); the *runner* attaches the
pixels to the following `RoleTool` message at execution time (see 4.5), and
subsequent turns re-encode from `tmp/` the same way.

### 4.3 Translate once per backend, keep text messages byte-identical

Both backends need the translation, in the two functions that already own it:

- **chat-completions** (`client.go:Chat`): a message with images sends
  `content` as an array —
  `[{type: "text", text: ...}, {type: "image_url", image_url: {url:
  "data:<mime>;base64,..."}}]`. Messages *without* images must keep sending a
  bare string.
- **Responses** (`responses.go:messagesToResponsesInput`): a user message with
  images sends content parts — `input_text` plus `input_image` (exact part
  names vary by provider; verify against the opencode.ai endpoint with a `curl`
  probe before coding, not from memory).

The "keep sending a bare string" clause matters beyond tidiness. Tool specs are
sorted deterministically and the prompt prefix must stay byte-identical
between requests so the provider's prompt cache keeps hitting; `KVIT_RUN_ID`
(`session.RunIDFor`, derived from the session name) pins the backend route for
the same reason. An image turn necessarily starts a new cache prefix from that
message on — unavoidable — but text-only turns before and after it must
serialize exactly as they do today, or every message pays for the feature.

Unit tests belong next to the existing ones (`responses_test.go`,
`client_test.go`): build a `Message` with an `ImagePart` pointing at
`testdata/`, run both translators, assert the JSON shape. Fixtures go under
`testdata/`, not `benchmarks/` — benchmark outputs are untracked by policy and
must stay that way.

### 4.4 A new tool, not a flag on `Read`

Two options:

- **Extend `Read`** with sniffing (`net/http.DetectContentType` on the first
  512 bytes): if image, return an attachment instead of text. Risk: silently
  changes an existing tool's contract, and the model may pull megabytes of
  pixels when it wanted a directory listing or a text peek.
- **A separate tool (recommended)**, e.g. `ReadImage {"path": "..."}`. Its own
  schema, its own permission prompt, its own prompt template
  (`PromptTemplateName: "read_image"`, registered in
  `internal/tools/setup.go` next to the `Read` registration at line 60), no
  existing behavior disturbed.

Either way, reuse the path machinery, not new checks: `CheckPathPermission`
then `NormalizeAndValidatePath` (containment, never `strings.HasPrefix` — see
CLAUDE.md), then `CheckPathSafety` for outside-workspace paths, exactly as
`ReadFileTool.Call` lines 401–419 do. Validate MIME (png/jpeg/webp; decide on
gif — first frame only — during implementation), cap input size (10–20MB is a
sane starting offer, as config, see 4.6), sniff content rather than trusting
the extension, and **downscale before storing**: max ~1568px on the long edge,
re-encode JPEG quality ~0.85 into `tmp/`. Full-resolution screenshots cost
thousands of tokens for no additional model accuracy on code-adjacent images;
the stored file is what gets re-encoded every turn, so normalize once.

Record the read in `ReadTracker` for consistency, but read-before-edit stays
text-only: an image read must not satisfy the edit gate.

This path is profile-independent. `agent.profile: strong` (the default) skips
the weak-model machinery (backtracking, duplicate-call kill switch, fuzzy
matching, interrogation); none of it needs to know about images except the two
text consumers in 4.6.

### 4.5 The runner must return more than a string

`executeToolWithTimeout` returns `content string` today. The minimal change is
to let an image-capable tool return `(text string, images []llm.ImagePart)`
— either a second return value or a small result struct — and have
`executeTools` attach them to the `RoleTool` message it already appends. The
`Batch` tool (`internal/tools/batch.go`) runs tools concurrently and joins
their results; its join must carry attachments through, preserving per-call
association (which image came from which call).

### 4.6 Session, logging, tokens, config

- **Session round-trip**: `AppendMessages` (`session.go:291`) persists the
  `images` references; `Load` rehydrates them; `internal/context`'s
  `AppendMessages` follows. A `tmp/` file deleted out from under a session
  must degrade to a text notice (`[image foo.png unavailable]`), not a failed
  turn.
- **Log redaction**: `ShowSession` (`internal/session/manager.go:204`), the
  transcript writer, and any debug logging print a one-line summary —
  `[image foo.png 800x600 120KB]` — never base64. Same for the loop detector's
  recorded content and `interrogate.go:lastToolResult`, which consume text
  summaries.
- **Token accounting**: image tokens are real tokens. Server-reported usage
  already counts them, but any client-side context-window guard
  (`runner_llm.go`) should estimate per image (provider rule, roughly
  base + per-tile) so a turn with several screenshots fails early with a clear
  error instead of mid-loop.
- **Config**: a small section, e.g. `tools.images: {enabled, max_size_mb,
  max_dimension}`, next to `ReadToolConfig
  <internal/config/config.go:452>`. Disabled by default until the wire path is
  verified, like `mcp.enabled` was.

## 5. Phase 2: paste in the composer

### 5.1 Terminals do not deliver image bytes on paste

Ctrl+V over an image arrives at the TUI as one of: nothing, a file path or
`file://` URL (drag-drop — most terminals insert a quoted path), or clipboard
content readable only through OS-specific tools. So "paste" is really two or
three producers of the same phase-1 attachment:

1. **`@path/to/img.png` references and auto-detected image paths in the input
   line** (cheapest, works everywhere, also covers drag-drop). Parsed in the
   TUI before `runAgent`, or as a `:image foo.png` command alongside `:new`,
   `:switch`, `:sessions`. The stored input history stays text (paths), so the
   history file does not bloat.
2. **True clipboard image** (Ctrl+V with pixels in the clipboard): a TUI
   keybinding that shells out to `wl-paste --type image/png` (Wayland),
   `xclip -selection clipboard -t image/png -o` (X11),
   `osascript`/`pbpaste` (macOS), or PowerShell (Windows), saves the bytes to
   the session's `inbox/` or `/tmp`, and attaches the saved file. Probe per OS
   at keypress time and report "no image in clipboard" plainly — the common
   case on a remote SSH session, where there is no clipboard to read.
3. Deliberately **not** OSC 52 / Kitty graphics-protocol inline paste:
   terminal support is spotty, and both end at "bytes the TUI must then save
   to a file anyway" — which is producer 2 with more escape sequences.

`atotto/clipboard` cannot serve producer 2; it is text-only.

### 5.2 Transport: a repeatable `-image` flag next to `-p`

`runAgent` builds `["-p", prompt, "-s", session, ...]` today. Add a repeatable
`-image <path>` flag carried the same way; `RunExec` (`internal/repl/repl.go`)
copies each into the session's `tmp/`, builds the multimodal user message
(`Content: promptText` plus `Images: [...]`), and persists both halves through
the existing `AppendMessages` + `SetPersist` hooks. Flags keep the one-process-
per-turn shape untouched: the UI still holds no agent state, and a wedged turn
still ends with its process.

Display: echo a summary line under the submitted prompt
(`[image 1: foo.png (800x600, 120KB)]`), mirroring how the submitted text is
echoed with its gray background today.

## 6. What this costs and what it does not disturb

- **Prompt cache**: one new cache prefix per image turn (unavoidable); text
  turns serialize byte-identically (by construction, 4.3), so nothing else
  invalidates.
- **History size**: bounded — references in `history.jsonl`, normalized bytes
  in `tmp/`, input history stays paths. The failure mode to avoid is inline
  base64 in the JSONL; 4.2 rules it out.
- **Dependencies**: one pure-Go resize (`golang.org/x/image/...`), no cgo, no
  new beverage in `scripts/build.sh` or `scripts/release.sh`.
- **Safety prompting**: image reads go through the same path-permission and
  `/dev/tty` prompting as any read (`internal/safety`); outside-workspace
  images ask before they are attached.
- **Weak-model machinery**: untouched. Backtracking, the duplicate-call
  detector, interrogation, and fuzzy edit matching stay text consumers; images
  bypass them by design.
- **Benchmarks**: no new tracked outputs. Test with `testdata/` fixtures and
  the existing `go test ./...` / `go vet ./...` gates, green before and after.

## 7. Build order and how to verify each step

1. `llm.Message.Images` + both translators + unit tests (no tool yet). Verify
   with a hardcoded attachment against a fake-server test, then a `curl`
   probe of the real Responses endpoint shape.
2. Headless `-image` flag through `RunExec` to both backends, against the real
   endpoint with a real PNG. This is the first moment pixels demonstrably
   reach the model; stop here if the provider's part names surprise.
3. `ReadImage` tool + runner attachment plumbing + `Batch` join +
   session persistence + log redaction. Verify: model calls `ReadImage`,
   next request carries the parts, `:history` shows the summary line,
   reopening the session re-sends from `tmp/`.
4. Downscale/caps/config + prompt template + README tools-table row.
   Verify: 4K screenshot stores normalized, oversize input errors clearly,
   disabled-by-default config skips registration.
5. TUI: `@path` / `:image` → `-image` flag; then the clipboard keybinding per
   OS. Verify each producer headlessly first (producer 1 is string parsing and
   needs no terminal).

## 8. Open questions, deferred to implementation

- Accepted MIME set and gif policy (first frame vs. reject).
- Size caps and long-edge default (10–20MB in, ~1568px stored are starting
  offers, not measurements — tune against token cost on real screenshots).
- Whether tool results other than `ReadImage` may carry images (e.g. a
  `Shell` screenshot helper later — no, not in this proposal).
- Steering and inbox attachments mid-turn: the inbox carries text lines today;
  pasting an image *during* a turn wants a file drop in `inbox/` plus a drain
  that understands it. Left out; the prompt-time path covers the ask.
- PDF pages and multi-frame images: reject with a clear error for now rather
  than half-supporting them.
