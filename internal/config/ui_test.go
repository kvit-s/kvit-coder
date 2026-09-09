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

// TestTerminalTitleParses: ui.terminal_title arrives from YAML, an absent
// section leaves the front end's defaults in place, and an icon set to the
// empty string is a deliberate "no icon" rather than an unset one.
func TestTerminalTitleParses(t *testing.T) {
	tmpDir := t.TempDir()
	write := func(name, body string) *Config {
		t.Helper()
		path := filepath.Join(tmpDir, name)
		content := "llm:\n  model: \"m\"\n\nworkspace:\n  root: \"/tmp/workspace\"\n" + body
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test config: %v", err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		return cfg
	}

	bare := write("bare.yaml", "")
	if !bare.UI.TerminalTitle.On() {
		t.Error("with no terminal_title section the title should still be set")
	}
	if r, w, a := bare.UI.TerminalTitle.Icons("R", "W", "A"); r != "R" || w != "W" || a != "A" {
		t.Errorf("icons = %q/%q/%q, want the defaults R/W/A", r, w, a)
	}

	set := write("set.yaml", "\nui:\n  terminal_title:\n    enabled: false\n    running: \"*\"\n    waiting: \"\"\n    asking: \"?\"\n")
	if set.UI.TerminalTitle.On() {
		t.Error("enabled: false should turn the title off")
	}
	if r, w, a := set.UI.TerminalTitle.Icons("R", "W", "A"); r != "*" || w != "" || a != "?" {
		t.Errorf("icons = %q/%q/%q, want */\"\"/?", r, w, a)
	}
}
