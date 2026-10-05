package session

import (
	"os"
	"testing"
)

// TestLastSettingsModel: the most recent settings event's model is what the
// tail of the history was produced by; a session with no turns reports "".
func TestLastSettingsModel(t *testing.T) {
	mgr := setupTestManager(t)
	sess, err := mgr.Open("last-settings")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got, err := sess.LastSettingsModel(); err != nil || got != "" {
		t.Fatalf("fresh session LastSettingsModel = %q, %v; want empty, nil", got, err)
	}
	if err := sess.Settings("model-a:xhigh", true, []string{"Read"}); err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if err := sess.Settings("model-b", true, []string{"Read"}); err != nil {
		t.Fatalf("Settings: %v", err)
	}
	got, err := sess.LastSettingsModel()
	if err != nil {
		t.Fatalf("LastSettingsModel: %v", err)
	}
	if got != "model-b" {
		t.Errorf("LastSettingsModel = %q, want the most recent settings model", got)
	}
}

// A malformed line is still an error: only two fields are decoded, but the
// whole line has to be valid JSON.
func TestLastSettingsModelRejectsAMalformedLine(t *testing.T) {
	mgr := setupTestManager(t)
	sess, err := mgr.Open("last-settings-bad")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := sess.Settings("model-a", true, []string{"Read"}); err != nil {
		t.Fatalf("Settings: %v", err)
	}
	f, err := os.OpenFile(sess.HistoryPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{\"kind\":\"message\",\"message\":{\"role\":\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := sess.LastSettingsModel(); err == nil {
		t.Errorf("LastSettingsModel = %q, nil; want an error for the malformed line", got)
	}
}
