package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileApprovalsSurviveANewProcess(t *testing.T) {
	// The point of the store: one process per turn means the second turn is a
	// different process, and ask_once has to still mean once.
	path := filepath.Join(t.TempDir(), "mcp-approvals.json")

	first, err := OpenApprovals(path)
	if err != nil {
		t.Fatalf("OpenApprovals: %v", err)
	}
	if first.Approved("mcp.playwright.browser_click") {
		t.Error("nothing was approved yet")
	}
	if err := first.Approve("mcp.playwright.browser_click"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// A different store over the same file stands in for the next turn.
	second, err := OpenApprovals(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	if !second.Approved("mcp.playwright.browser_click") {
		t.Error("the approval did not survive; ask_once would prompt again next turn")
	}
	if second.Approved("mcp.playwright.browser_navigate") {
		t.Error("approving one tool must not approve another")
	}
}

func TestFileApprovalsAreIdempotentAndSorted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.json")
	a, err := OpenApprovals(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"z.tool", "a.tool", "z.tool"} {
		if err := a.Approve(tool); err != nil {
			t.Fatalf("Approve(%s): %v", tool, err)
		}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"approved\": [\n    \"a.tool\",\n    \"z.tool\"\n  ]\n}\n"
	if string(body) != want {
		t.Errorf("file is\n%s\nwant\n%s", body, want)
	}
}

func TestFileApprovalsSeeAnotherAgentsApproval(t *testing.T) {
	// Turns of one session can overlap. Re-reading rather than caching means a
	// concurrent agent's answer is honoured instead of prompted for again.
	path := filepath.Join(t.TempDir(), "a.json")
	mine, err := OpenApprovals(path)
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := OpenApprovals(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := theirs.Approve("shared.tool"); err != nil {
		t.Fatal(err)
	}
	if !mine.Approved("shared.tool") {
		t.Error("an approval made elsewhere should be visible without reopening")
	}
}

func TestOpenApprovalsRejectsAnUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenApprovals(path); err == nil {
		t.Error("a corrupt approvals file should be reported, not silently treated as empty")
	}
}

func TestConfirmerAskOnceConsultsTheStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.json")
	store, err := OpenApprovals(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Approve("mcp.x.y"); err != nil {
		t.Fatal(err)
	}

	c := newConfirmer()
	c.setApprovals(store)
	// Simulate "no one at the terminal": refuse without touching /dev/tty,
	// which would sit waiting for input whenever the test runs with a
	// controlling terminal (e.g. plain `go test` in a terminal) and hang
	// the suite until the go test timeout.
	var prompted bool
	SetLinePrompter(func(string) (string, bool) { prompted = true; return "", false })
	t.Cleanup(func() { SetLinePrompter(nil) })

	// Already approved: allowed without any prompt, which matters because
	// there is no one to answer in a test and asking would refuse.
	if err := c.Confirm("mcp.x.y", "x", ConfirmAskOnce, ""); err != nil {
		t.Errorf("an approved tool should not be asked about again: %v", err)
	}
	if prompted {
		t.Error("an approved tool must not prompt at all")
	}
	// Not approved, no one answering: refused rather than hanging.
	if err := c.Confirm("mcp.x.other", "x", ConfirmAskOnce, ""); err == nil {
		t.Error("an unapproved tool with no one answering should be refused")
	}
}
