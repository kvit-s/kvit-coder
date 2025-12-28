package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `llm:
  base_url: "http://localhost:8080/v1"
  api_key: "test-key"
  api_key_env: "TEST_API_KEY"
  model: "test-model"
  temperature: 0.5
  max_output_tokens: 1024

workspace:
  root: "/tmp/workspace"

agent:
  max_tool_iterations: 5

tools:
  shell:
    enabled: true
    allowed_commands:
      - "go"
      - "git"
  read:
    enabled: true
    max_file_size_kb: 128
  edit:
    enabled: true
    max_file_size_kb: 128
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Test loading config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify LLM config
	if cfg.LLM.BaseURL != "http://localhost:8080/v1" {
		t.Errorf("LLM.BaseURL = %q, want %q", cfg.LLM.BaseURL, "http://localhost:8080/v1")
	}
	if cfg.LLM.APIKey != "test-key" {
		t.Errorf("LLM.APIKey = %q, want %q", cfg.LLM.APIKey, "test-key")
	}
	if cfg.LLM.Model != "test-model" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "test-model")
	}
	if cfg.LLM.Temperature != 0.5 {
		t.Errorf("LLM.Temperature = %f, want %f", cfg.LLM.Temperature, 0.5)
	}
	if cfg.LLM.MaxTokens != 1024 {
		t.Errorf("LLM.MaxTokens = %d, want %d", cfg.LLM.MaxTokens, 1024)
	}

	// Verify workspace config
	if cfg.Workspace.Root != "/tmp/workspace" {
		t.Errorf("Workspace.Root = %q, want %q", cfg.Workspace.Root, "/tmp/workspace")
	}

	// Verify agent config
	if cfg.Agent.MaxIterations != 5 {
		t.Errorf("Agent.MaxIterations = %d, want %d", cfg.Agent.MaxIterations, 5)
	}

	// Verify tools config
	if !cfg.Tools.Shell.Enabled {
		t.Error("Tools.Shell.Enabled = false, want true")
	}
	if len(cfg.Tools.Shell.AllowedCommands) != 2 {
		t.Errorf("len(Tools.Shell.AllowedCommands) = %d, want 2", len(cfg.Tools.Shell.AllowedCommands))
	}
	if !cfg.Tools.Read.Enabled {
		t.Error("Tools.Read.Enabled = false, want true")
	}
	if cfg.Tools.Read.MaxFileSizeKB != 128 {
		t.Errorf("Tools.Read.MaxFileSizeKB = %d, want 128", cfg.Tools.Read.MaxFileSizeKB)
	}
	if !cfg.Tools.Edit.Enabled {
		t.Error("Tools.Edit.Enabled = false, want true")
	}
	if cfg.Tools.Edit.MaxFileSizeKB != 128 {
		t.Errorf("Tools.Edit.MaxFileSizeKB = %d, want 128", cfg.Tools.Edit.MaxFileSizeKB)
	}
}

func TestLoadEnvironmentOverride(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `llm:
  base_url: "http://localhost:8080/v1"
  api_key: "original-key"
  api_key_env: "TEST_API_KEY_OVERRIDE"
  model: "test-model"
  temperature: 0.5
  max_output_tokens: 1024
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Set environment variable
	os.Setenv("TEST_API_KEY_OVERRIDE", "env-override-key")
	defer os.Unsetenv("TEST_API_KEY_OVERRIDE")

	// Test loading config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify API key was overridden
	if cfg.LLM.APIKey != "env-override-key" {
		t.Errorf("LLM.APIKey = %q, want %q (from env)", cfg.LLM.APIKey, "env-override-key")
	}
}

func TestLoadNoEnvironmentOverride(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `llm:
  base_url: "http://localhost:8080/v1"
  api_key: "original-key"
  api_key_env: "NONEXISTENT_ENV_VAR"
  model: "test-model"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Test loading config (environment variable doesn't exist)
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify API key was NOT overridden
	if cfg.LLM.APIKey != "original-key" {
		t.Errorf("LLM.APIKey = %q, want %q (original)", cfg.LLM.APIKey, "original-key")
	}
}

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() with invalid path should return error")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	// Create a temporary invalid YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `llm:
  base_url: "http://localhost:8080/v1"
  invalid yaml content [[[
`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create invalid config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() with invalid YAML should return error")
	}
}
