package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestProjectInstructionsConfig(t *testing.T) {
	defaults := ProjectInstructionsConfig{}
	if !defaults.IsEnabled() {
		t.Error("project instructions should default to enabled")
	}
	if got := defaults.PathOrDefault(); got != "CLAUDE.md" {
		t.Errorf("PathOrDefault default = %q, want CLAUDE.md", got)
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	configContent := `agent:
  project_instructions:
    enabled: false
    path: AGENTS.md
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Agent.ProjectInstructions.Enabled == nil {
		t.Fatal("ProjectInstructions.Enabled = nil, want explicit false")
	}
	if cfg.Agent.ProjectInstructions.IsEnabled() {
		t.Error("ProjectInstructions.IsEnabled() = true, want false")
	}
	if got := cfg.Agent.ProjectInstructions.PathOrDefault(); got != "AGENTS.md" {
		t.Errorf("ProjectInstructions.PathOrDefault() = %q, want AGENTS.md", got)
	}
}

func TestDiagnosticsConfigDefaults(t *testing.T) {
	d := &DiagnosticsConfig{}
	if got := d.GetInterrogateIdenticalThreshold(); got != 2 {
		t.Errorf("GetInterrogateIdenticalThreshold default = %d, want 2", got)
	}
	if got := d.GetInterrogateMaxPerTask(); got != 3 {
		t.Errorf("GetInterrogateMaxPerTask default = %d, want 3", got)
	}
	if got := d.GetInterrogateLogDir(); got != "benchmarks/.kvit-coder-benchmark/interrogations" {
		t.Errorf("GetInterrogateLogDir default = %q", got)
	}
}

func TestDiagnosticsTriggerEnabled(t *testing.T) {
	// Empty list = all triggers enabled.
	empty := &DiagnosticsConfig{}
	if !empty.TriggerEnabled("duplicate_call") {
		t.Error("empty trigger list should enable all triggers")
	}
	// Explicit subset.
	subset := &DiagnosticsConfig{InterrogateTriggers: []string{"pending_blocked"}}
	if !subset.TriggerEnabled("pending_blocked") {
		t.Error("pending_blocked should be enabled")
	}
	if subset.TriggerEnabled("duplicate_call") {
		t.Error("duplicate_call should be disabled when not in subset")
	}
}

func TestLLMHeadersExpansion(t *testing.T) {
	t.Setenv("SESSION_SECRET", "s3cret")

	cfg := &Config{}
	cfg.LLM.Headers = []string{
		"x-opencode-session=kvit-coder-${KVIT_RUN_ID}",
		"x-token = ${SESSION_SECRET} ",
		"malformed-no-equals",
	}

	headers := cfg.LLMHeaders()

	if got := headers["x-token"]; got != "s3cret" {
		t.Errorf("x-token = %q, want s3cret", got)
	}
	if _, ok := headers["malformed-no-equals"]; ok {
		t.Error("entry without = should be skipped")
	}

	// The run ID has to be a real value, not an empty expansion, or the
	// endpoint sees a header it will reject.
	session := headers["x-opencode-session"]
	if session == "kvit-coder-" || !strings.HasPrefix(session, "kvit-coder-") {
		t.Errorf("x-opencode-session = %q, want a non-empty run ID suffix", session)
	}

	// It must stay the same for the life of the process, so every request of
	// one run routes together.
	if again := cfg.LLMHeaders()["x-opencode-session"]; again != session {
		t.Errorf("run ID changed between calls: %q then %q", session, again)
	}
}

// A run ID already in the environment is honored, so a caller can pin one
// across several runs.
func TestLLMHeadersRunIDFromEnv(t *testing.T) {
	t.Setenv("KVIT_RUN_ID", "pinned")

	cfg := &Config{}
	cfg.LLM.Headers = []string{"x-opencode-session=${KVIT_RUN_ID}"}

	if got := cfg.LLMHeaders()["x-opencode-session"]; got != "pinned" {
		t.Errorf("x-opencode-session = %q, want pinned", got)
	}
}

// TestPathWithinRejectsPrefixSiblings pins the containment rule: a directory
// whose name merely starts with an allowed or denied directory's name is not
// inside it. The old check was a plain string prefix, so /home/sk/kvit-coder-notes
// counted as inside /home/sk/kvit-coder and inherited its permissions.
func TestPathWithinRejectsPrefixSiblings(t *testing.T) {
	cases := []struct {
		parent, child string
		want          bool
	}{
		{"/home/sk/kvit-coder", "/home/sk/kvit-coder", true},
		{"/home/sk/kvit-coder", "/home/sk/kvit-coder/internal/tools", true},
		{"/home/sk/kvit-coder", "/home/sk/kvit-coder-notes", false},
		{"/home/sk/kvit-coder", "/home/sk/kvit-coder-notes/secrets.txt", false},
		{"/home/sk/kvit-coder", "/home/sk", false},
		{"/home/sk/kvit-coder", "/home/sk/kvit-coder/../other", false},
		{"/etc", "/etc/passwd", true},
	}
	for _, c := range cases {
		if got := pathWithin(c.parent, c.child); got != c.want {
			t.Errorf("pathWithin(%q, %q) = %v, want %v", c.parent, c.child, got, c.want)
		}
	}
}

// TestCheckPathPermissionPrefixSibling is the same rule seen through the config
// API a tool actually calls.
func TestCheckPathPermissionPrefixSibling(t *testing.T) {
	cfg := &Config{}
	cfg.Workspace.Root = "/home/sk/kvit-coder"
	cfg.Workspace.PathSafetyMode = "block"

	if res, _ := cfg.CheckPathPermission("/home/sk/kvit-coder/main.go", AccessWrite); res != PermissionGranted {
		t.Errorf("a file inside the workspace was not granted: %v", res)
	}
	if res, _ := cfg.CheckPathPermission("/home/sk/kvit-coder-notes/main.go", AccessWrite); res != PermissionDenied {
		t.Errorf("a sibling directory sharing the workspace's name prefix was treated as %v, want denied", res)
	}
}
