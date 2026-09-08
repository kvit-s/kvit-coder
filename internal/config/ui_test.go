package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUIMarkdownParses: ui.markdown arrives from YAML; absent it stays
// empty, which the writer reads as auto.
func TestUIMarkdownParses(t *testing.T) {
	tmpDir := t.TempDir()

	with := filepath.Join(tmpDir, "with-ui.yaml")
	content := "llm:\n  model: \"m\"\n\nworkspace:\n  root: \"/tmp/workspace\"\n\nui:\n  markdown: \"never\"\n"
	if err := os.WriteFile(with, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}
	cfg, err := Load(with)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.UI.Markdown != "never" {
		t.Errorf("UI.Markdown = %q, want %q", cfg.UI.Markdown, "never")
	}

	without := filepath.Join(tmpDir, "without-ui.yaml")
	plain := "llm:\n  model: \"m\"\n\nworkspace:\n  root: \"/tmp/workspace\"\n"
	if err := os.WriteFile(without, []byte(plain), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}
	cfg, err = Load(without)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.UI.Markdown != "" {
		t.Errorf("UI.Markdown = %q, want empty (auto)", cfg.UI.Markdown)
	}
}
