package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

func TestLoadProjectInstructionsDefaultCLAUDEMD(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(path, []byte("Use project conventions.\n"), 0644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	inst, err := LoadProjectInstructions(&config.Config{}, tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectInstructions() error = %v", err)
	}
	if inst == nil {
		t.Fatal("LoadProjectInstructions() = nil, want instructions")
	}
	if inst.Path != path {
		t.Errorf("Path = %q, want %q", inst.Path, path)
	}
	if inst.Content != "Use project conventions.\n" {
		t.Errorf("Content = %q", inst.Content)
	}
}

func TestLoadProjectInstructionsCustomPath(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(path, []byte("Use AGENTS instructions."), 0644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.ProjectInstructions.Path = "AGENTS.md"

	inst, err := LoadProjectInstructions(cfg, tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectInstructions() error = %v", err)
	}
	if inst == nil {
		t.Fatal("LoadProjectInstructions() = nil, want instructions")
	}
	if inst.Path != path {
		t.Errorf("Path = %q, want %q", inst.Path, path)
	}
}

func TestLoadProjectInstructionsDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("Do not load."), 0644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	enabled := false
	cfg := &config.Config{}
	cfg.Agent.ProjectInstructions.Enabled = &enabled

	inst, err := LoadProjectInstructions(cfg, tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectInstructions() error = %v", err)
	}
	if inst != nil {
		t.Fatalf("LoadProjectInstructions() = %#v, want nil", inst)
	}
}

func TestPromptWithProjectInstructionsAndStrip(t *testing.T) {
	inst := &ProjectInstructions{
		Path:    "/repo/CLAUDE.md",
		Content: "Use project conventions.",
	}

	prompt := promptWithProjectInstructions("Refactor auth.", inst)
	if !strings.Contains(prompt, "Use project conventions.") {
		t.Fatalf("prompt missing instructions: %q", prompt)
	}
	if !strings.HasSuffix(prompt, "Refactor auth.") {
		t.Fatalf("prompt should end with user request: %q", prompt)
	}

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "system"},
		{Role: llm.RoleUser, Content: prompt},
		{Role: llm.RoleAssistant, Content: "done"},
	}
	stripped := stripProjectInstructions(messages, inst)
	if stripped[1].Content != "Refactor auth." {
		t.Errorf("stripped user content = %q", stripped[1].Content)
	}
	if messages[1].Content == "Refactor auth." {
		t.Error("stripProjectInstructions mutated input messages")
	}
}
