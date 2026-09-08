package session

import "testing"

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
