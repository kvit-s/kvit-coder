package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// TestAppendLoadRoundTrip: what a turn appends is what the next turn loads,
// and only the lines that carry a message become conversation.
func TestAppendLoadRoundTrip(t *testing.T) {
	mgr := setupTestManager(t)
	sess, err := mgr.Open("round-trip")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := sess.Settings("test-model", false); err != nil {
		t.Fatalf("Settings: %v", err)
	}
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "list the files"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID: "c1", Type: "function",
			Function: llm.ToolCallFunction{Name: "Shell", Arguments: `{"command":"ls"}`},
		}}},
		{Role: llm.RoleTool, Name: "Shell", ToolCallID: "c1", Content: "a.go\nb.go"},
		{Role: llm.RoleAssistant, Content: "two files"},
	}
	if err := sess.AppendMessages(messages); err != nil {
		t.Fatalf("AppendMessages: %v", err)
	}
	if err := sess.Notice("iteration budget reached"); err != nil {
		t.Fatalf("Notice: %v", err)
	}

	// A separate handle on the same directory, as the next turn's process has.
	reopened, err := mgr.Open("round-trip")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	loaded, err := reopened.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != len(messages) {
		t.Fatalf("loaded %d messages, want %d (the settings and notice lines carry none)", len(loaded), len(messages))
	}
	for i := range loaded {
		if loaded[i].Role != messages[i].Role || loaded[i].Content != messages[i].Content {
			t.Errorf("message %d is %s/%q, want %s/%q", i,
				loaded[i].Role, loaded[i].Content, messages[i].Role, messages[i].Content)
		}
	}
	if len(loaded[1].ToolCalls) != 1 || loaded[1].ToolCalls[0].Function.Name != "Shell" {
		t.Errorf("the assistant's tool call did not survive the round trip: %+v", loaded[1].ToolCalls)
	}
}

// TestHistoryIsATranscript: every line is a timestamped JSON object naming what
// it records, so the file can be read and grepped without kvit-coder.
func TestHistoryIsATranscript(t *testing.T) {
	mgr := setupTestManager(t)
	sess, err := mgr.Open("transcript")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = sess.AppendMessages([]llm.Message{
		{Role: llm.RoleUser, Content: "go"},
		{Role: llm.RoleTool, Name: "Read", Content: "contents"},
	})
	_ = sess.Notice("cancelled by user")

	data, err := os.ReadFile(sess.HistoryPath())
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("history has %d lines, want 3", len(lines))
	}

	wantKinds := []Kind{KindMessage, KindToolResult, KindNotice}
	for i, line := range lines {
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("line %d is not one JSON object: %v", i, err)
		}
		if ev.Kind != wantKinds[i] {
			t.Errorf("line %d has kind %q, want %q", i, ev.Kind, wantKinds[i])
		}
		if ev.TS.IsZero() {
			t.Errorf("line %d has no timestamp", i)
		}
	}

	var toolEvent Event
	_ = json.Unmarshal([]byte(lines[1]), &toolEvent)
	if toolEvent.Tool != "Read" {
		t.Errorf("the tool result line does not name its tool: %q", toolEvent.Tool)
	}
}

// TestMigrateFlatFile: a session written when a session was one flat file
// becomes a session directory, in order and without loss, and the old file is
// kept under a new name rather than deleted.
func TestMigrateFlatFile(t *testing.T) {
	mgr := setupTestManager(t)

	original := []llm.Message{
		{Role: llm.RoleSystem, Content: "you are an agent"},
		{Role: llm.RoleUser, Content: "first"},
		{Role: llm.RoleAssistant, Content: "second"},
		{Role: llm.RoleTool, Name: "Shell", Content: "third"},
	}
	var flat strings.Builder
	for _, m := range original {
		line, _ := json.Marshal(m)
		flat.Write(line)
		flat.WriteByte('\n')
	}
	flatPath := filepath.Join(mgr.baseDir, "legacy.jsonl")
	if err := os.WriteFile(flatPath, []byte(flat.String()), 0644); err != nil {
		t.Fatalf("write flat session: %v", err)
	}
	mtime := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(flatPath, mtime, mtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if !mgr.SessionExists("legacy") {
		t.Error("a flat session file is not reported as an existing session")
	}

	sess, err := mgr.Open("legacy")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	loaded, err := sess.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != len(original) {
		t.Fatalf("migrated session has %d messages, want %d", len(loaded), len(original))
	}
	for i := range loaded {
		if loaded[i].Role != original[i].Role || loaded[i].Content != original[i].Content {
			t.Errorf("message %d is %s/%q, want %s/%q", i,
				loaded[i].Role, loaded[i].Content, original[i].Role, original[i].Content)
		}
	}

	// The migrated lines are stamped with the old file's modification time,
	// which is the only date the flat format recorded.
	data, _ := os.ReadFile(sess.HistoryPath())
	var first Event
	_ = json.Unmarshal([]byte(strings.SplitN(string(data), "\n", 2)[0]), &first)
	if !first.TS.Equal(mtime) {
		t.Errorf("migrated event is stamped %s, want the flat file's mtime %s", first.TS, mtime)
	}

	if _, err := os.Stat(flatPath); !os.IsNotExist(err) {
		t.Error("the flat session file is still in place after migration")
	}
	if _, err := os.Stat(flatPath + ".migrated"); err != nil {
		t.Errorf("the flat session file was not kept as .migrated: %v", err)
	}

	// Opening again must not migrate a second time or lose anything.
	again, err := mgr.Open("legacy")
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	loadedAgain, _ := again.Load()
	if len(loadedAgain) != len(original) {
		t.Errorf("reopening the migrated session gives %d messages, want %d", len(loadedAgain), len(original))
	}
}

// TestSubdirectories: opening a session creates the places the later stages
// keep their state, so nothing has to guess whether a directory exists.
func TestSubdirectories(t *testing.T) {
	mgr := setupTestManager(t)
	sess, err := mgr.Open("dirs")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, dir := range []string{sess.CheckpointsDir(), sess.ProcDir(), sess.InboxDir(), sess.TmpDir()} {
		fi, err := os.Stat(dir)
		if err != nil {
			t.Errorf("%s was not created: %v", dir, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

// TestMetaRoundTrip: meta.json survives being reopened, which is what a later
// compaction step will read to resolve a name to the session that replaced it.
func TestMetaRoundTrip(t *testing.T) {
	mgr := setupTestManager(t)
	sess, err := mgr.Open("meta")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	m := sess.Meta()
	m.Workspace = "/tmp/project"
	m.Model = "test-model"
	m.FirstPrompt = "add a test"
	if err := sess.SaveMeta(); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}

	reopened, err := mgr.Open("meta")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got := reopened.Meta()
	if got.Workspace != "/tmp/project" || got.Model != "test-model" || got.FirstPrompt != "add a test" {
		t.Errorf("metadata did not survive reopening: %+v", got)
	}
	if got.Created.IsZero() || got.Updated.IsZero() {
		t.Error("metadata has no timestamps")
	}
}

// TestOpenFollowsSucceededBy: a name that compaction replaced resolves to the
// session that replaced it.
func TestOpenFollowsSucceededBy(t *testing.T) {
	mgr := setupTestManager(t)

	old, err := mgr.Open("old")
	if err != nil {
		t.Fatalf("Open old: %v", err)
	}
	newer, err := mgr.Open("newer")
	if err != nil {
		t.Fatalf("Open newer: %v", err)
	}
	_ = newer.AppendMessages([]llm.Message{{Role: llm.RoleUser, Content: "carried over"}})

	old.Meta().SucceededBy = "newer"
	if err := old.SaveMeta(); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}

	resolved, err := mgr.Open("old")
	if err != nil {
		t.Fatalf("reopen old: %v", err)
	}
	if resolved.Name() != "newer" {
		t.Errorf("opening %q resolved to %q, want \"newer\"", "old", resolved.Name())
	}
}

// TestRollbackIsAppliedOnLoad: the loop takes messages back by appending a
// record of the discard, never by rewriting the file. What Load returns is the
// conversation; the abandoned attempt stays in the file, where it says what the
// model tried.
func TestRollbackIsAppliedOnLoad(t *testing.T) {
	mgr := setupTestManager(t)
	sess, err := mgr.Open("rollback")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	_ = sess.AppendMessages([]llm.Message{{Role: llm.RoleUser, Content: "go"}})
	// An attempt the loop then abandons.
	_ = sess.AppendMessages([]llm.Message{
		{Role: llm.RoleAssistant, Content: "wrong turn"},
		{Role: llm.RoleTool, Name: "Edit", Content: "no such file"},
	})
	if err := sess.Rollback(2); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	_ = sess.AppendMessages([]llm.Message{{Role: llm.RoleAssistant, Content: "recovered"}})

	loaded, err := sess.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d messages, want the prompt and the recovered answer: %+v", len(loaded), loaded)
	}
	if loaded[0].Content != "go" || loaded[1].Content != "recovered" {
		t.Errorf("loaded %q then %q, want the discarded attempt skipped", loaded[0].Content, loaded[1].Content)
	}

	// The file still holds everything, including what was taken back.
	data, err := os.ReadFile(sess.HistoryPath())
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if !strings.Contains(string(data), "wrong turn") {
		t.Error("the abandoned attempt was removed from the file, which is meant to be append-only")
	}
	if !strings.Contains(string(data), `"kind":"rollback"`) {
		t.Error("the discard was not recorded as its own line")
	}
}

// TestRollbackCannotUnderflow: a discard larger than the history so far leaves
// an empty conversation rather than panicking.
func TestRollbackCannotUnderflow(t *testing.T) {
	mgr := setupTestManager(t)
	sess, _ := mgr.Open("underflow")
	_ = sess.AppendMessages([]llm.Message{{Role: llm.RoleUser, Content: "one"}})
	_ = sess.Rollback(50)

	loaded, err := sess.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("loaded %d messages, want none", len(loaded))
	}
}
