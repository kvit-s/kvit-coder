package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/markdown"
)

const markdownTestMsg = "# Done\n\nFixed **two bugs** in `writer.go`.\n"

// newAnswerWriter builds a headless writer whose stdout is a buffer and
// whose terminal answer is given explicitly, so the styled and raw paths
// are exercised without a pseudo-terminal.
func newAnswerWriter(stdoutTTY bool) (*Writer, *bytes.Buffer) {
	var buf bytes.Buffer
	w := NewWriter(0)
	w.SetHeadless(true)
	w.SetStdout(&buf)
	w.SetStdoutIsTerminal(stdoutTTY)
	return w, &buf
}

// cleanEnv pins the environment knobs the gate reads, so ambient NO_COLOR
// or TERM values cannot flip a test.
func cleanEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm")
}

// TestAssistantPipedStaysRaw: piped stdout keeps byte-identical markdown,
// which is what scripts and logs have always received.
func TestAssistantPipedStaysRaw(t *testing.T) {
	cleanEnv(t)
	w, buf := newAnswerWriter(false)
	w.Assistant(markdownTestMsg)
	if out := buf.String(); out != markdownTestMsg+"\n" {
		t.Errorf("piped answer = %q, want raw markdown %q", out, markdownTestMsg+"\n")
	}
}

// TestAssistantTTYStyles: on a terminal the markdown source is replaced by
// styled output with the text intact.
func TestAssistantTTYStyles(t *testing.T) {
	cleanEnv(t)
	w, buf := newAnswerWriter(true)
	w.Assistant(markdownTestMsg)
	out := buf.String()
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("terminal answer has no ANSI codes: %q", out)
	}
	if strings.Contains(out, "# Done") {
		t.Errorf("terminal answer kept its markdown source: %q", out)
	}
	for _, want := range []string{"Done", "two bugs", "writer.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("terminal answer lost %q", want)
		}
	}
}

// TestAssistantNeverStaysRaw: the opt-out restores exactly today's behavior
// even on a terminal.
func TestAssistantNeverStaysRaw(t *testing.T) {
	cleanEnv(t)
	w, buf := newAnswerWriter(true)
	w.SetMarkdownMode("never")
	w.Assistant(markdownTestMsg)
	if out := buf.String(); out != markdownTestMsg+"\n" {
		t.Errorf("never answer = %q, want raw markdown", out)
	}
}

// TestAssistantAlwaysStylesWhenPiped: the explicit opt-in styles even
// without a terminal.
func TestAssistantAlwaysStylesWhenPiped(t *testing.T) {
	cleanEnv(t)
	w, buf := newAnswerWriter(false)
	w.SetMarkdownMode("always")
	w.Assistant(markdownTestMsg)
	out := buf.String()
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("always answer has no ANSI codes when piped: %q", out)
	}
	if strings.Contains(out, "# Done") {
		t.Errorf("always answer kept its markdown source: %q", out)
	}
}

// TestAssistantNoColorDisables: NO_COLOR wins over everything but JSON mode
// keeps its own path (tested below); raw markdown goes through.
func TestAssistantNoColorDisables(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm")
	w, buf := newAnswerWriter(true)
	w.SetMarkdownMode("always")
	w.Assistant(markdownTestMsg)
	if out := buf.String(); out != markdownTestMsg+"\n" {
		t.Errorf("NO_COLOR answer = %q, want raw markdown", out)
	}
}

// TestAssistantTermDumbDisables: a dumb terminal gets source, not escapes.
func TestAssistantTermDumbDisables(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	w, buf := newAnswerWriter(true)
	w.SetMarkdownMode("always")
	w.Assistant(markdownTestMsg)
	if out := buf.String(); out != markdownTestMsg+"\n" {
		t.Errorf("dumb-terminal answer = %q, want raw markdown", out)
	}
}

// TestAssistantQuietTTYStyles: -pq still styles on a terminal; quiet only
// suppresses progress, never the answer itself.
func TestAssistantQuietTTYStyles(t *testing.T) {
	cleanEnv(t)
	w, buf := newAnswerWriter(true)
	w.SetQuiet(true)
	w.Assistant(markdownTestMsg)
	if out := buf.String(); !strings.Contains(out, "\x1b[") {
		t.Errorf("quiet terminal answer has no ANSI codes: %q", out)
	}
}

// TestAssistantJSONStaysRaw: the JSON document carries the raw markdown the
// model wrote, never ANSI escapes.
func TestAssistantJSONStaysRaw(t *testing.T) {
	cleanEnv(t)
	var buf bytes.Buffer
	w := NewWriter(0)
	w.SetJSONMode(true)
	w.SetStdout(&buf)
	w.SetStdoutIsTerminal(true)
	w.SetMarkdownMode("always")
	w.Assistant(markdownTestMsg)
	w.WriteJSONOutput(&JSONStats{})
	var doc struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("answer JSON does not parse: %v (%q)", err, buf.String())
	}
	if doc.Content != markdownTestMsg {
		t.Errorf("JSON content = %q, want raw markdown %q", doc.Content, markdownTestMsg)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("JSON document contains ANSI codes: %q", buf.String())
	}
}

// TestAssistantRenderFallbackMatchesLegacy: when rendering is skipped (here
// an oversized answer), a terminal gets exactly what it got before
// rendering existed.
func TestAssistantRenderFallbackMatchesLegacy(t *testing.T) {
	cleanEnv(t)
	msg := strings.Repeat("plain line\n", markdown.MaxInputBytes/11+10)
	w, buf := newAnswerWriter(true)
	w.Assistant(msg)
	if out := buf.String(); out != msg+"\n" {
		t.Errorf("fallback answer differs from legacy output (%d vs %d bytes)", len(out), len(msg)+1)
	}
}

// TestAssistantEmptyPrintsBlankLine: an empty answer keeps its legacy
// single newline rather than going near the renderer.
func TestAssistantEmptyPrintsBlankLine(t *testing.T) {
	cleanEnv(t)
	w, buf := newAnswerWriter(true)
	w.Assistant("")
	if out := buf.String(); out != "\n" {
		t.Errorf("empty answer = %q, want a single newline", out)
	}
}
