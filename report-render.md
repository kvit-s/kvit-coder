# Rendering regular markdown reports nicely

Status: implemented (September 2026). Scope is **regular free-form markdown**
final answers, not a structured `submit_report`-style schema (per steering).

## 1. Problem

The final answer today is raw markdown printed verbatim, so a long report
reads as source rather than as a report: `#`/`**`/fences/tables/links all
visible, no heading hierarchy, no code styling, wrapping left to the
terminal.

Current path in this repo:

- `internal/agent/runner_iteration.go:handleFinalAnswer` prints a `Divider`
  (timing) then calls `Writer.Assistant(assistantMsg.Content)`.
- `internal/ui/writer.go:Assistant` does `whiteColor.Printf("%s\n\n", msg)`
  in interactive mode, or `fmt.Fprintf(w.stdout, "%s\n", msg)` in headless
  mode. No markdown parsing, no styling, no width handling.
- Output routing: headless (`-p`) sends progress to stderr and the final
  answer to stdout (`Writer.SetHeadless`, `progressTarget`, `out()`).
  `--json` stores the raw string in `jsonContent` and emits it inside
  `JSONOutput.content`. `-pq` (quiet) prints only the final answer.
- `cmd/kvit-coder-ui` + `internal/tui/ui.go:runAgent` spawn `kvit-coder -p`
  per turn with `cmd.Stdout/Stderr` inherited. Whatever the child prints is
  what the user sees; the UI does no re-rendering.
- `go.mod` already has `lipgloss`/`bubbletea`/`fatih/color`, but no markdown
  renderer. `termWidth()` (`COLUMNS`, clamped 40–250, default 100) exists
  for step lines and is reusable for report wrapping.

So there is exactly one integration point that fixes every surface at once:
`Writer.Assistant`.

## 2. How krok renders regular markdown

References below are to `~/krok` (`/home/sk/krok`). This is about krok's
normal assistant-message markdown, not `structured-report.md` /
`submit_report`.

### 2.1 Crates and entry points

- `crates/codegen/xai-grok-markdown/src/lib.rs`: public API.
  `render_markdown(text, style, pretty, syntect) -> (String, SourceMap)` for
  ANSI strings; `render_markdown_ratatui_full` /
  `render_markdown_ratatui_with_buffers[_width]` for `ratatui::Line`s.
- `crates/codegen/xai-grok-markdown-core/src/lib.rs`: the shared
  `pulldown-cmark` config (`parser_options`: GFM + strikethrough + math +
  tasklists + tables) and `offset_events`, which demotes single-`~` to
  literal text so only `~~…~~` counts as strikethrough.
- `crates/codegen/xai-grok-markdown/src/{parse,render,streaming,style,
  colors,output,buffers,hyperlinks,source_map}.rs`: parser, two render
  targets, streaming layer, styling, link/URL handling.
- `crates/codegen/xai-grok-pager-render/src/theme/md_style.rs`: builds the
  `MarkdownStyle` from the active theme (already quantized to terminal
  capability). This is the theme bridge.
- `crates/codegen/xai-grok-pager/src/scrollback/blocks/markdown_content.rs`:
  `MarkdownContent` wraps `StreamingMarkdownRenderer` plus a word-wrap cache
  keyed on `(width, generation, theme)`. Used by both `AgentMessageBlock`
  and `ThinkingBlock`.
- `crates/codegen/xai-grok-pager/src/headless.rs`: headless `plain` mode
  buffers `text_buffer` and writes it out raw at end of turn; there is no
  ANSI-markdown pass on that path. Nice rendering lives in the TUI
  scrollback, not in piped output. JSON/streaming modes carry the raw text
  plus a separate `report` event.

### 2.2 Two modes, two targets

- `pretty: bool` is the key switch. Interactive scrollback and question
  cards always use `pretty = true` (hide markdown syntax, paint headings,
  code, quotes, lists, tables). `false` shows raw source. The renderer
  supports both from the same parse.
- Target 1 — `ratatui::Line`s for the full-screen TUI, with hyperlink
  ranges (`HyperlinkTarget`, OSC 8, wrap-aware grouping), `CodeBlockSpan`
  metadata per closed fence, table copy metadata, and a `line_source_map`
  back to source lines for copy/select.
- Target 2 — ANSI string for terminals (`render_ansi(pretty)`), with
  `anstyle` styles adapted to terminal capability (`colors.rs`:
  truecolor/256/16 detection, polarity-safe syntax colors).

### 2.3 What gets styled

`style.rs:MarkdownStyle` has per-element styles: 6 heading levels, strong,
emphasis, strikethrough, inline code, fenced code (language tag, background),
blockquote bar, task checkboxes, list markers, rules, links (text/url/title),
tables (box/ASCII/double borders, max table width), plus body default. Code
fences go through `syntect` (with an incremental highlighter for the still-
open trailing fence on the streaming path). Extra passes handle plain-URL
detection, `(url)` suffixes in pretty link mode, LaTeX `$…$`/`$$…$$` to
Unicode, HTML entities, and mermaid blocks (vendored diagram stack).

### 2.4 Streaming and caching (TUI-only complexity)

- `StreamingMarkdownRenderer::push_and_render` per token, with
  checkpoint-based freezing: only the tail after the last stable boundary
  re-renders. One-shot `render_markdown*` calls are byte-identical to
  streamed ones (tested both ways in `streaming.rs`).
- `MarkdownContent` caches wrapped output; repeated `output()` calls at the
  same width/theme are free. Table width is capped to the viewport.

### 2.5 What to copy, what not to

Copy the contract, not the code:

- Do copy: pretty-vs-raw switch; TTY-gated ANSI with graceful fallback;
  raw markdown inside JSON/piped output; theme-driven style table;
  width-aware wrapping; never fail the turn on a render error.
- Do not port: Rust `pulldown-cmark`+`syntect`+`ratatui` stack, streaming
  checkpoints, source maps, OSC 8 link grouping, LaTeX/mermaid — none of it
  is needed for a one-shot final report in Go, and the Go ecosystem already
  has a lipgloss-native equivalent.

## 3. Proposal for kvit-coder

### 3.1 Goals / non-goals

Goals: final answers readable in a normal terminal (headings, lists, code,
tables, quotes, links distinct; sane wrapping); zero breakage for scripts
(piped stdout stays raw markdown); `--json` stays raw markdown inside
`content`; works through `kvit-coder-ui` with no UI change.

Non-goals: no report schema or validation; no fullscreen viewer, no
collapsible sections, no follow-up/decision machinery; no streaming
re-render of the report mid-turn; no OSC 8 / source-map / LaTeX / mermaid
in v1.

### 3.2 Library choice

Use `charmbracelet/glamour` (plus its `muesli/termenv` color detection,
already adjacent to our `lipgloss` stack):

- Renders GitHub-flavored markdown to ANSI, lipgloss-style aware, handles
  headings, lists, task lists, code fences (chroma highlighting), tables,
  blockquotes, rules, links — i.e. everything our reports actually contain.
- Word wrap built in (`WithWordWrap(width)`); style via built-in dark/light
  themes or a small custom JSON style matching our palette.
- Actively maintained, pure Go, no new native deps.

Considered and rejected:

- `yuin/goldmark` + hand-rolled lipgloss renderer: more control, but we
  would reimplement what glamour already does (tables, wrapping, themes).
- Minimal hand-rolled regex highlighter: cheap but wrong (nested emphasis,
  fences, tables) and becomes a maintenance trap.
- Porting krok's renderer: wrong language, ~15k lines with syntect/ratatui,
  solves streaming-TUI problems we do not have.

### 3.3 Integration point

New package `internal/markdown` (thin wrapper, easily tested/mocked):

```go
// Render returns ANSI-styled markdown, or the input unchanged when
// styling is disabled or rendering fails (never fail the turn here).
func Render(src string, width int) string
func Enabled(w *ui.Writer) bool // tty + config + env gate
```

Hook only `Writer.Assistant`:

- `--json`: unchanged — keep storing raw `msg` in `jsonContent`.
- Headless/interactive with TTY: `Render(msg, termWidth())`, print result.
- Piped stdout / `NO_COLOR` / `TERM=dumb` / explicit opt-out: print raw `msg`
  exactly as today.
- On render error: print raw `msg`.

Because `kvit-coder-ui` inherits the child's stdout, this single change
fixes `-p`, `-pq` (on a TTY), and interactive UI sessions at once. No
`internal/tui` change needed in v1.

### 3.4 Output contract

| Case | stdout | stderr/progress | Notes |
|---|---|---|---|
| `-p` on TTY | ANSI-rendered markdown | unchanged (stderr) | colors via termenv detection |
| `-p` piped | raw markdown (byte-identical to today) | unchanged | scripts unaffected |
| `-pq` on TTY | ANSI-rendered | quiet as today | same gate as `-p` |
| `-pq` piped | raw markdown | quiet as today | `... \| head` keeps working |
| `--json` | JSON doc, `content` = raw markdown | unchanged | never put ANSI in JSON |
| `NO_COLOR`/`TERM=dumb`/not-a-tty | raw markdown | unchanged | standard opt-outs |
| render error | raw markdown | unchanged | log at debug, never error the turn |

### 3.5 Style and width

- Width: reuse existing `termWidth()` (`COLUMNS`, 40–250, default 100) and
  pass to glamour word wrap. No new width plumbing.
- Style: start from glamour's dark style; tune headings/bold to HiWhite,
  secondary text to our gray, code background subtle, links underlined.
  Keep the existing pre-report `Divider` and step palette untouched.
- Keep `fatih/color` behavior: rendering must respect `color.NoColor` the
  same way `useColor()` does, so existing flags/pipes keep working.

### 3.6 Config (minimal)

```yaml
ui:
  markdown: auto   # auto | always | never; auto = TTY-gated as above
  # markdown_style: dark  # optional later: dark | light | <glamour JSON path>
```

`auto` is the default; `never` restores today's behavior exactly.
Environment (`NO_COLOR`, `TERM`, `COLUMNS`) overrides style/width as usual.

### 3.7 Implementation (as built)

1. `internal/markdown` (`markdown.go`): `Mode` (`auto`/`always`/`never`),
   `ParseMode` (empty/unknown → auto), and `Render(src, width)` on
   `glamour@v0.9.1` with the dark style and word wrap. v0.9.1 was chosen
   because it matches the repo's `lipgloss v1.1.0` / `termenv v0.16.0`
   exactly; v0.10.0/v1.0.0 would drag lipgloss to a pseudo-version.
   `Render` returns the input unchanged on empty input, input over
   `MaxInputBytes` (256 KiB), or any renderer error — it never fails a turn.
2. `internal/ui/writer.go`: `Writer` gained `markdownMode`,
   `stdoutTTY` (test override), `SetMarkdownMode` / `SetStdoutIsTerminal`,
   and the gate `shouldRenderMarkdown` + `answerTTY`. `Assistant` renders
   with `termWidth()` on a TTY only; `--json` still stores raw `msg`;
   piped / `NO_COLOR` / `TERM=dumb` / `never` print raw `msg` byte-identical
   to before; a styled answer bypasses the `whiteColor` wrapper (which would
   only reset the styling) in interactive mode. Deviations from the sketch
   in 3.3: the gate lives on `Writer` rather than as `markdown.Enabled`
   (which would have imported `ui` into `markdown`), and the stdout check
   mirrors `progressTarget` but for the answer stream (`w.stdout` headless,
   `color.Output` otherwise).
3. `internal/config`: `UI.UIConfig.Markdown` (`yaml:"markdown"`), empty by
   default (= auto); wired in `cmd/kvit-coder/main.go` next to `SetVerbose`;
   sample in `config.yaml`.
4. Tests: `internal/markdown/markdown_test.go` (headings, emphasis, fences,
   tables, lists, quotes, empty/oversized input, width clamps, wrap
   behavior); `internal/ui/writer_markdown_test.go` (pipe/TTY ×
   auto/always/never × `NO_COLOR`/`TERM=dumb` × quiet × JSON, oversized
   fallback byte-identical to legacy, empty answer); `internal/config/ui_test.go`
   (YAML parsing, absent section). `go test ./...` green (17 packages),
   `go vet ./...` clean. One test lesson: glamour styles spans separately
   (chroma code tokens, quote bars), so assertions check single words, not
   phrases.
5. Docs: this file; `README.md` usage note; `config.yaml` comments. The
   `markdown_style` custom-style option from the sketch was deferred — the
   stock dark style matches the palette well enough for v1.

### 3.8 Risks

- ANSI in stdout breaking scripts: mitigated by TTY gate + raw-on-pipe +
  `never` escape hatch; verify with `... | cat` and `--json` tests.
- Glamour style clash with our palette: mitigate with a small custom style
  JSON rather than forking rendering.
- Very long reports / wide tables: glamour wraps and truncates sanely, but
  add a size cap test (e.g. 200KB report still renders fast or falls back
  to raw on timeout/error).

## 4. References

- kvit-coder: `internal/ui/writer.go` (`Assistant`, `Divider`, `useColor`,
  `termWidth`, `progressTarget`), `internal/agent/runner_iteration.go`
  (`handleFinalAnswer`), `internal/repl/repl.go` (`RunExec`),
  `cmd/kvit-coder/main.go` (`-p`/`-pq`/`--json`), `internal/tui/ui.go`
  (`runAgent`), `go.mod`, `config.yaml`.
- krok: `crates/codegen/xai-grok-markdown/src/{lib,parse,render,
  streaming,style,colors,output}.rs`,
  `crates/codegen/xai-grok-markdown-core/src/lib.rs`,
  `crates/codegen/xai-grok-pager-render/src/theme/md_style.rs`,
  `crates/codegen/xai-grok-pager/src/scrollback/blocks/markdown_content.rs`,
  `crates/codegen/xai-grok-pager/src/headless.rs`.
