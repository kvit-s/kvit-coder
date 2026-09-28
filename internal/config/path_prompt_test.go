package config

// WouldPromptForPath must mirror CheckPathSafety's prompting decision
// without performing any I/O (spec/subagents-phase2.md section 3, path 2).

import (
	"path/filepath"
	"testing"
)

func promptTestConfig(root, mode string) *Config {
	cfg := &Config{}
	cfg.Workspace.Root = root
	cfg.Workspace.PathSafetyMode = mode
	cfg.Tools.SafetyConfirmations = map[string]SafetyConfirmation{}
	return cfg
}

func TestWouldPromptForPathModes(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root+"-notes", "f.txt")
	inside := filepath.Join(root, "f.txt")

	// Modes that never prompt pass through, even outside and unconfirmed.
	for _, mode := range []string{"allow", "block", "warn"} {
		cfg := promptTestConfig(root, mode)
		if cfg.WouldPromptForPath("read", outside) {
			t.Errorf("mode %s prompts for %s", mode, outside)
		}
	}

	// ask_once prompts until confirmed.
	cfg := promptTestConfig(root, "ask_once")
	if !cfg.WouldPromptForPath("read", outside) {
		t.Errorf("ask_once does not prompt for unconfirmed %s", outside)
	}
	abs, _, err := NormalizeAndValidatePath(root, outside)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	cfg.Tools.SafetyConfirmations["read:"+abs] = SafetyConfirmation{ToolName: "read", Path: abs}
	if cfg.WouldPromptForPath("read", outside) {
		t.Errorf("ask_once still prompts after confirmation")
	}

	// ask_always prompts even when confirmed.
	cfg = promptTestConfig(root, "ask_always")
	cfg.Tools.SafetyConfirmations["read:"+abs] = SafetyConfirmation{ToolName: "read", Path: abs}
	if !cfg.WouldPromptForPath("read", outside) {
		t.Error("ask_always does not prompt despite confirmation")
	}

	// Inside the workspace never prompts, in any mode.
	for _, mode := range []string{"ask_once", "ask_always"} {
		cfg := promptTestConfig(root, mode)
		if cfg.WouldPromptForPath("read", inside) {
			t.Errorf("mode %s prompts for inside path %s", mode, inside)
		}
	}

	// A path the configuration already allows never prompts.
	cfg = promptTestConfig(root, "ask_once")
	cfg.Workspace.AllowedPaths = []string{root + "-notes"}
	if cfg.WouldPromptForPath("read", outside) {
		t.Error("pre-permitted allowed_paths entry still prompts")
	}

	// Empty mode behaves like ask_once (CheckPathSafety's default branch).
	cfg = promptTestConfig(root, "")
	if !cfg.WouldPromptForPath("read", outside) {
		t.Error("empty mode does not prompt like ask_once")
	}

	// Non-filesystem tool names skip containment and follow the mode,
	// exactly like CheckPathSafety does.
	cfg = promptTestConfig(root, "ask_once")
	if !cfg.WouldPromptForPath("Question", "anything") {
		t.Error("non-filesystem tool does not follow the ask mode")
	}
	if promptTestConfig(root, "block").WouldPromptForPath("Question", "anything") {
		t.Error("non-filesystem tool prompts under block")
	}

	// A zero Config mirrors CheckPathSafety's default branch (ask_once
	// behavior), so it reports a prompt for an outside path.
	if !(&Config{}).WouldPromptForPath("read", outside) {
		t.Error("zero Config does not mirror CheckPathSafety default")
	}
	var nilCfg *Config
	if nilCfg.WouldPromptForPath("read", outside) {
		t.Error("nil Config prompts")
	}
}

func TestWouldPromptForPathContainment(t *testing.T) {
	// The project-notes regression: a sibling directory sharing a string
	// prefix is outside, so it prompts; the project file itself does not.
	root := filepath.Join(t.TempDir(), "project")
	cfg := promptTestConfig(root, "ask_once")
	if !cfg.WouldPromptForPath("read", filepath.Join(root+"-notes", "f")) {
		t.Error("project-notes counted as inside project")
	}
	if cfg.WouldPromptForPath("read", filepath.Join(root, "f")) {
		t.Error("project/file counted as prompting inside its own project")
	}
	// Write-class tools do not consult allowed_read_paths.
	cfg.Workspace.AllowedReadPaths = []string{root + "-notes"}
	if !cfg.WouldPromptForPath("edit", filepath.Join(root+"-notes", "f")) {
		t.Error("edit of an allowed_read_paths entry should still prompt")
	}
	if cfg.WouldPromptForPath("read", filepath.Join(root+"-notes", "f")) {
		t.Error("read of an allowed_read_paths entry should not prompt")
	}
}
