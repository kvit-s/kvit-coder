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

// TestProfileStrongTurnsOffTheWeakModelMachinery: the profile is applied once,
// at load, so nothing downstream has to consult it.
func TestProfileStrongTurnsOffTheWeakModelMachinery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `
llm:
  model: test
workspace:
  root: "."
backtrack:
  enabled: true
diagnostics:
  interrogate_on_anomaly: true
tools:
  edit:
    enabled: true
    preview_mode: true
    fuzzy_threshold: 0.8
    smart_first_line_indent: true
    max_autoindent_fix: 3
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Agent.IsStrong() {
		t.Fatal("the default profile is not strong")
	}
	if cfg.Backtrack.Enabled {
		t.Error("backtracking is on under the strong profile")
	}
	if cfg.Tools.Edit.PreviewMode {
		t.Error("the edit confirm handshake is on under the strong profile")
	}
	if cfg.Tools.Edit.FuzzyThreshold != 0 || !cfg.Tools.Edit.ExactMatchOnly {
		t.Errorf("fuzzy matching is on under the strong profile: threshold=%v exactOnly=%v",
			cfg.Tools.Edit.FuzzyThreshold, cfg.Tools.Edit.ExactMatchOnly)
	}
	if cfg.Tools.Edit.SmartFirstLineIndent || cfg.Tools.Edit.MaxAutoindentFix != 0 {
		t.Error("indentation repair is on under the strong profile")
	}
	if cfg.Diagnostics.InterrogateOnAnomaly {
		t.Error("anomaly interrogation is on under the strong profile")
	}
}

// TestProfileWeakChangesNothing: "weak" is what the loop did before the profile
// existed, so the config must come through exactly as written.
func TestProfileWeakChangesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `
agent:
  profile: weak
llm:
  model: test
workspace:
  root: "."
backtrack:
  enabled: true
diagnostics:
  interrogate_on_anomaly: true
tools:
  edit:
    enabled: true
    preview_mode: true
    fuzzy_threshold: 0.8
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Agent.IsStrong() {
		t.Fatal("profile: weak was read as strong")
	}
	if !cfg.Backtrack.Enabled || !cfg.Tools.Edit.PreviewMode ||
		cfg.Tools.Edit.FuzzyThreshold != 0.8 || cfg.Tools.Edit.ExactMatchOnly ||
		!cfg.Diagnostics.InterrogateOnAnomaly {
		t.Errorf("profile: weak changed the configuration: %+v", cfg.Tools.Edit)
	}
}

// TestAllowedPathsSuppressThePrompt: a path the configuration already permits
// must not be queried every time it is touched. The check consulted only the
// workspace root, so allowed_read_paths bought nothing here — and once spilled
// tool output moved into the session directory, which sits outside the
// workspace, the model was asked to confirm every read of its own output.
func TestAllowedPathsSuppressThePrompt(t *testing.T) {
	cfg := &Config{}
	cfg.Workspace.Root = "/work/project"
	cfg.Workspace.PathSafetyMode = "block"
	cfg.Workspace.AllowedReadPaths = []string{"/home/u/.kvit-coder/sessions/s1/tmp"}
	cfg.Workspace.AllowedPaths = []string{"/srv/shared"}
	cfg.Tools.SafetyConfirmations = map[string]SafetyConfirmation{}

	// Reading spilled output from the session's own directory is permitted.
	if err := cfg.CheckPathSafety("read", "/home/u/.kvit-coder/sessions/s1/tmp/shell-123"); err != nil {
		t.Errorf("reading an allowed_read_paths file was refused: %v", err)
	}
	// A read-write allowance covers a writing tool too.
	if err := cfg.CheckPathSafety("edit", "/srv/shared/notes.txt"); err != nil {
		t.Errorf("editing an allowed_paths file was refused: %v", err)
	}
	// A read-only allowance does not license writing there.
	if err := cfg.CheckPathSafety("edit", "/home/u/.kvit-coder/sessions/s1/tmp/shell-123"); err == nil {
		t.Error("a read-only allowance permitted an edit")
	}
	// Nor a shell command, which is why the session directory itself has to be
	// on allowed_paths: a shell command can write, so it never qualifies for
	// the read-only list, and reading spilled output with cat asked every time.
	if err := cfg.CheckPathSafety("shell", "/home/u/.kvit-coder/sessions/s1/tmp/shell-123"); err == nil {
		t.Error("a read-only allowance permitted a shell command")
	}
	if err := cfg.CheckPathSafety("shell", "/srv/shared/build.sh"); err != nil {
		t.Errorf("a shell command in an allowed_paths directory was refused: %v", err)
	}
	// Anything not allowed is still caught.
	if err := cfg.CheckPathSafety("read", "/etc/shadow"); err == nil {
		t.Error("a path outside every allowance was permitted")
	}
	// And a sibling that merely shares a name prefix is not inside.
	if err := cfg.CheckPathSafety("read", "/srv/shared-secrets/keys"); err == nil {
		t.Error("a directory sharing an allowed path's name prefix was permitted")
	}
}

// TestWorkspaceLockIsOptIn: one agent at a time per working directory is a
// choice, not the default. The lock is held for a whole turn, so a turn
// waiting on a permission question used to block every other agent in the
// directory, and the one that could not get the lock exited rather than waited.
func TestWorkspaceLockIsOptIn(t *testing.T) {
	dir := t.TempDir()

	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		return path
	}

	cfg, err := Load(write("silent.yaml", "llm:\n  model: test\nworkspace:\n  root: \".\"\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Workspace.Lock {
		t.Error("a config that says nothing about locking got the lock anyway")
	}

	cfg, err = Load(write("locked.yaml", "llm:\n  model: test\nworkspace:\n  root: \".\"\n  lock: true\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Workspace.Lock {
		t.Error("workspace.lock: true did not turn the lock on")
	}
}

// TestYoloAllowsEverything: --yolo sets path_safety_mode to "allow", which
// removes the workspace boundary for every tool -- no prompt, no warning, no
// difference between a tool that reads and one that writes. An explicit
// denied_paths entry still wins, because that list was written on purpose.
func TestYoloAllowsEverything(t *testing.T) {
	cfg := &Config{}
	cfg.Workspace.Root = "/work/project"
	cfg.Workspace.PathSafetyMode = "allow"
	cfg.Tools.SafetyConfirmations = map[string]SafetyConfirmation{}

	for _, tool := range []string{"read", "edit", "search", "glob", "shell", "write"} {
		if err := cfg.CheckPathSafety(tool, "/etc/hosts"); err != nil {
			t.Errorf("%s was refused a path outside the workspace: %v", tool, err)
		}
	}
	if got, err := cfg.CheckPathPermission("/var/lib/anything", AccessWrite); got != PermissionGranted {
		t.Errorf("writing outside the workspace gave %v (%v), want granted", got, err)
	}

	cfg.Workspace.DeniedPaths = []string{"/etc/ssh"}
	if got, _ := cfg.CheckPathPermission("/etc/ssh/sshd_config", AccessRead); got != PermissionDenied {
		t.Errorf("a denied_paths entry gave %v, want denied even under --yolo", got)
	}
}

// TestModelDisplayAppendsEffort: the startup banner shows "model:effort"
// when llm.reasoning_effort is set, so the banner reflects what the
// Responses backend is actually asked for, and plain "model" otherwise.
func TestModelDisplayAppendsEffort(t *testing.T) {
	cfg := &Config{}
	cfg.LLM.Model = "muse-spark-1.3-contributor"
	cfg.LLM.ReasoningEffort = "xhigh"
	if got := cfg.ModelDisplay(); got != "muse-spark-1.3-contributor:xhigh" {
		t.Errorf("ModelDisplay() = %q, want %q", got, "muse-spark-1.3-contributor:xhigh")
	}

	cfg.LLM.ReasoningEffort = ""
	if got := cfg.ModelDisplay(); got != "muse-spark-1.3-contributor" {
		t.Errorf("ModelDisplay() = %q, want %q", got, "muse-spark-1.3-contributor")
	}
}
