package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LLM struct {
		BaseURL       string  `yaml:"base_url"`
		APIKey        string  `yaml:"api_key"`
		APIKeyEnv     string  `yaml:"api_key_env"`
		Model         string  `yaml:"model"`
		Temperature   float32 `yaml:"temperature"`
		MaxTokens     int     `yaml:"max_output_tokens"`
		Context       int     `yaml:"context"`        // Max context size for display (0 = don't show)
		MergeThinking bool    `yaml:"merge_thinking"` // Merge reasoning_content into content (default: false, discard thinking)
		Verbose       int     `yaml:"verbose"`        // 0 = off, >0 = show tool output up to N lines
		BenchmarkCmd  string  `yaml:"benchmark_cmd"`  // External command for benchmarks (use {prompt} placeholder)
	} `yaml:"llm"`

	Workspace struct {
		Root                  string   `yaml:"root"`
		PathSafetyMode        string   `yaml:"path_safety_mode"` // "block", "warn", "ask_once", "ask_always"
		AllowOutsideWorkspace bool     `yaml:"allow_outside_workspace"`
		AllowedPaths          []string `yaml:"allowed_paths"`
		AllowedReadPaths      []string `yaml:"allowed_read_paths"`
		DeniedPaths           []string `yaml:"denied_paths"`
	} `yaml:"workspace"`

	Agent struct {
		MaxIterations int `yaml:"max_tool_iterations"`
	} `yaml:"agent"`

	Backtrack BacktrackConfig `yaml:"backtrack"`

	Tools ToolsConfig `yaml:"tools"`
}

// ToolsConfig holds per-tool configuration with explicit enable/disable
type ToolsConfig struct {
	Read        ReadToolConfig        `yaml:"read"`
	Edit        EditToolConfig        `yaml:"edit"`
	RestoreFile RestoreFileToolConfig `yaml:"restore_file"`
	Search      SearchToolConfig      `yaml:"search"`
	Shell       ShellToolConfig       `yaml:"shell"`
	Plan        PlanToolsConfig       `yaml:"plan"`
	Checkpoint  CheckpointToolsConfig `yaml:"checkpoint"`
	Tasks       TasksToolsConfig      `yaml:"tasks"`

	// Safety confirmations (runtime only, not persisted)
	SafetyConfirmations map[string]SafetyConfirmation `yaml:"-"`
}

// ReadToolConfig configures the read tool
type ReadToolConfig struct {
	Enabled         bool  `yaml:"enabled"`
	MaxFileSizeKB   int   `yaml:"max_file_size_kb"`
	MaxReadSizeKB   int   `yaml:"max_read_size_kb"`
	MaxPartialLines int   `yaml:"max_partial_lines"`
	ShowLineNumbers *bool `yaml:"show_line_numbers"` // nil = default true, for backward compat
}

// EditToolConfig configures the edit tool
type EditToolConfig struct {
	Enabled               bool    `yaml:"enabled"`
	Mode                  string  `yaml:"mode"`                    // "lines" (default), "searchreplace", or "patch"
	MaxFileSizeKB         int     `yaml:"max_file_size_kb"`
	PreviewMode           bool    `yaml:"preview_mode"`            // enables edit.confirm/edit.cancel
	ReadBeforeEditMsgs    int     `yaml:"read_before_edit_msgs"`   // require read within N messages before edit (0 = disabled)
	PendingConfirmRetries int     `yaml:"pending_confirm_retries"` // max retries when LLM ignores confirm/cancel (0 = disabled, default 5)
	FuzzyThreshold        float64 `yaml:"fuzzy_threshold"`         // for searchreplace mode: 0 = exact only, 0.8 = fuzzy matching
}

// RestoreFileToolConfig configures the restore_file tool
type RestoreFileToolConfig struct {
	Enabled bool `yaml:"enabled"`
}

// SearchToolConfig configures the search tool
type SearchToolConfig struct {
	Enabled           bool `yaml:"enabled"`
	MaxSnippetResults int  `yaml:"max_snippet_results"` // Show full snippets up to this many (default: 20)
	MaxCompactResults int  `yaml:"max_compact_results"` // Show file:line:char up to this many (default: 100)
	// Above max_compact_results: save to temp file, show truncated
}

// ShellToolConfig configures the shell tool
type ShellToolConfig struct {
	Enabled            bool     `yaml:"enabled"`
	AllowedCommands    []string `yaml:"allowed_commands"`    // allowlist (empty = allow all)
	DisallowedCommands []string `yaml:"disallowed_commands"` // blocklist (checked after allowlist)
}

// PlanToolsConfig configures all plan.* tools as a group
type PlanToolsConfig struct {
	Enabled       bool   `yaml:"enabled"`        // group toggle for all plan.* tools
	InjectionMode string `yaml:"injection_mode"` // "none" or "every_step"
}

// CheckpointToolsConfig configures all checkpoint.* tools as a group
type CheckpointToolsConfig struct {
	Enabled          bool     `yaml:"enabled"`           // group toggle for all checkpoint.* tools
	MaxTurns         int      `yaml:"max_turns"`         // max checkpoints before rotating (default: 100)
	TempDir          string   `yaml:"temp_dir"`          // base directory for checkpoint storage
	MaxFileSizeKB    int      `yaml:"max_file_size_kb"`  // skip files larger than this (default: 1024)
	ExcludedPatterns []string `yaml:"excluded_patterns"` // don't track these files
}

// TasksToolsConfig configures Tasks.* tools for context compression
type TasksToolsConfig struct {
	Enabled  bool `yaml:"enabled"`  // Enable Tasks tools (disables Plan.* and Checkpoint.* tools)
	Collapse bool `yaml:"collapse"` // Stage 2: Enable Tasks.Collapse (requires enabled=true)
	Plan     bool `yaml:"plan"`     // Stage 3: Enable plan-based tools (requires enabled=true)

	// Runtime notice thresholds
	TaskWarnTurns       int  `yaml:"task_warn_turns"`       // Warn after N turns in task (default: 5)
	TaskCriticalTurns   int  `yaml:"task_critical_turns"`   // Critical warning after N turns (default: 10)
	ContextCapacityWarn int  `yaml:"context_capacity_warn"` // Warn at N% context capacity (default: 80)
	MaxNestedDepth      int  `yaml:"max_nested_depth"`      // Max task nesting depth (default: 2)
	NotifyFileChanges   bool `yaml:"notify_file_changes"`   // Notify about file changes in task (default: true)
}

// BacktrackConfig configures the backtrack error handling mode
type BacktrackConfig struct {
	Enabled           bool `yaml:"enabled"`             // Enable backtrack mode (default: true)
	MaxRetries        int  `yaml:"max_retries"`         // Max retries at same history point (default: 5)
	InjectUserMessage bool `yaml:"inject_user_message"` // On limit reached: backtrack + inject user message instead of error-in-history
}

// GetShowLineNumbers returns whether line numbers should be shown in Read output.
// Defaults to true for backward compatibility.
func (r *ReadToolConfig) GetShowLineNumbers() bool {
	if r.ShowLineNumbers == nil {
		return true // Default: show line numbers
	}
	return *r.ShowLineNumbers
}

// GetEditMode returns the edit mode, defaulting to "lines" for backward compatibility.
func (e *EditToolConfig) GetEditMode() string {
	if e.Mode == "" {
		return "lines"
	}
	return e.Mode
}

// SafetyConfirmation tracks user confirmations for path access
// This is memory-only and does not persist across sessions
type SafetyConfirmation struct {
	ToolName string    `yaml:"-"`
	Path     string    `yaml:"-"`
	Timestamp time.Time `yaml:"-"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply environment overrides
	if cfg.LLM.APIKeyEnv != "" {
		if key := os.Getenv(cfg.LLM.APIKeyEnv); key != "" {
			cfg.LLM.APIKey = key
		}
	}

	// Convert workspace root to absolute path
	if cfg.Workspace.Root != "" {
		absRoot, err := filepath.Abs(cfg.Workspace.Root)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve workspace root: %w", err)
		}
		cfg.Workspace.Root = absRoot
	}

	// Initialize runtime fields
	cfg.Tools.SafetyConfirmations = make(map[string]SafetyConfirmation)

	// Set default file size limits for read tool
	if cfg.Tools.Read.MaxFileSizeKB == 0 {
		cfg.Tools.Read.MaxFileSizeKB = 128
	}
	if cfg.Tools.Read.MaxReadSizeKB == 0 {
		cfg.Tools.Read.MaxReadSizeKB = 24
	}
	if cfg.Tools.Read.MaxPartialLines == 0 {
		cfg.Tools.Read.MaxPartialLines = 150
	}

	// Set default file size limit for edit tool
	if cfg.Tools.Edit.MaxFileSizeKB == 0 {
		cfg.Tools.Edit.MaxFileSizeKB = 128
	}

	// Set default checkpoint settings
	if cfg.Tools.Checkpoint.MaxFileSizeKB == 0 {
		cfg.Tools.Checkpoint.MaxFileSizeKB = 1024
	}
	if cfg.Tools.Checkpoint.MaxTurns == 0 {
		cfg.Tools.Checkpoint.MaxTurns = 100
	}

	// Set default path safety mode
	if cfg.Workspace.PathSafetyMode == "" {
		cfg.Workspace.PathSafetyMode = "ask_once"
	}

	// Set default backtrack settings
	// Backtracking is disabled by default; must be explicitly enabled in config
	if cfg.Backtrack.MaxRetries == 0 {
		cfg.Backtrack.MaxRetries = 5 // Default max retries if enabled
	}

	// Set default Tasks tools settings
	if cfg.Tools.Tasks.TaskWarnTurns == 0 {
		cfg.Tools.Tasks.TaskWarnTurns = 5
	}
	if cfg.Tools.Tasks.TaskCriticalTurns == 0 {
		cfg.Tools.Tasks.TaskCriticalTurns = 10
	}
	if cfg.Tools.Tasks.ContextCapacityWarn == 0 {
		cfg.Tools.Tasks.ContextCapacityWarn = 80
	}
	if cfg.Tools.Tasks.MaxNestedDepth == 0 {
		cfg.Tools.Tasks.MaxNestedDepth = 2
	}
	// NotifyFileChanges defaults to true (Go zero value is false, so we check if unset)
	// Since YAML unmarshals false as false, we need a different approach
	// For now, we'll leave it as the struct default behavior

	return &cfg, nil
}

// IsToolEnabled returns true if the tool is enabled in config
func (c *Config) IsToolEnabled(toolName string) bool {
	switch toolName {
	case "read":
		return c.Tools.Read.Enabled
	case "edit":
		return c.Tools.Edit.Enabled
	case "edit.confirm", "edit.cancel":
		return c.Tools.Edit.Enabled && c.Tools.Edit.PreviewMode
	case "restore_file":
		return c.Tools.RestoreFile.Enabled
	case "search":
		return c.Tools.Search.Enabled
	case "shell":
		return c.Tools.Shell.Enabled
	case "plan.create", "plan.add_step", "plan.complete_step", "plan.remove_step", "plan.move_step":
		// Plan tools are disabled when Tasks tools are enabled
		return c.Tools.Plan.Enabled && !c.Tools.Tasks.Enabled
	case "checkpoint.list", "checkpoint.restore", "checkpoint.diff", "checkpoint.undo":
		// User-facing checkpoint tools are disabled when Tasks tools are enabled
		return c.Tools.Checkpoint.Enabled && !c.Tools.Tasks.Enabled
	// Tasks.* tools
	case "Tasks.Start", "Tasks.Finish", "Tasks.AcceptDiff", "Tasks.DeclineDiff",
		"Tasks.RevertFile", "Tasks.RevertToTaskStart":
		return c.Tools.Tasks.Enabled
	case "Tasks.Collapse":
		return c.Tools.Tasks.Enabled && c.Tools.Tasks.Collapse
	case "Tasks.Plan", "Tasks.Add", "Tasks.Skip", "Tasks.Complete", "Tasks.Retry", "Tasks.Replace":
		return c.Tools.Tasks.Enabled && c.Tools.Tasks.Plan
	default:
		return false
	}
}

// CheckPathSafety performs unified path safety checks for all tools
// Behavior controlled by tools.safety.path_safety_mode
func (c *Config) CheckPathSafety(toolName, identifier string) error {
	// For filesystem tools, check if path is outside workspace
	if strings.HasPrefix(toolName, "read") || strings.HasPrefix(toolName, "edit") ||
		strings.HasPrefix(toolName, "restore") || toolName == "glob" || toolName == "shell" ||
		toolName == "search" {
		absPath, outside, err := NormalizeAndValidatePath(c.Workspace.Root, identifier)
		if err != nil || !outside {
			return nil // Not outside or invalid path
		}
		identifier = absPath // Use absolute path for key
	}

	// Handle based on safety mode
	mode := c.Workspace.PathSafetyMode
	switch mode {
	case "block":
		return fmt.Errorf("access to path outside workspace blocked (path_safety_mode=block): %s", identifier)

	case "warn":
		fmt.Fprintf(os.Stderr, "⚠️  Warning: %s accesses path outside workspace: %s\n", toolName, identifier)
		return nil

	case "ask_always":
		// Always prompt user
		if confirmed := c.promptForPathAccess(toolName, identifier); !confirmed {
			return fmt.Errorf("user rejected access to path outside workspace: %s", identifier)
		}
		return nil

	case "ask_once":
		fallthrough
	default:
		// Prompt user once per path (default behavior)
		key := fmt.Sprintf("%s:%s", toolName, identifier)
		if _, exists := c.Tools.SafetyConfirmations[key]; !exists {
			if confirmed := c.promptForPathAccess(toolName, identifier); !confirmed {
				return fmt.Errorf("user rejected access to path outside workspace: %s", identifier)
			}
			c.Tools.SafetyConfirmations[key] = SafetyConfirmation{
				ToolName:  toolName,
				Path:      identifier,
				Timestamp: time.Now(),
			}
		}
		return nil
	}
}

// promptForPathAccess prompts the user to confirm path access
func (c *Config) promptForPathAccess(toolName, path string) bool {
	// Print newline to clear any progress dots on the same line
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "⚠️  %s accesses path outside workspace:\n", toolName)
	fmt.Fprintf(os.Stderr, "   - %s\n", path)
	fmt.Fprintf(os.Stderr, "\nAllow this access? [y/N]: ")

	tty, err := os.Open("/dev/tty")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open /dev/tty: %v\n", err)
		return false
	}
	defer tty.Close()

	reader := bufio.NewReader(tty)
	responseByte, err := reader.ReadByte()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read confirmation: %v\n", err)
		return false
	}

	fmt.Fprintf(os.Stderr, "%c\n", responseByte)
	response := strings.ToLower(string(responseByte))
	return response == "y" || response == "yes"
}

// NormalizeAndValidatePath is a helper function to normalize and validate paths
// This should be moved to a shared utility package in a real implementation
func NormalizeAndValidatePath(workspaceRoot, path string) (string, bool, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path is outside workspace
	absWorkspace, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve workspace: %w", err)
	}

	// Normalize paths for comparison
	absPath = filepath.Clean(absPath)
	absWorkspace = filepath.Clean(absWorkspace)

	// Check if path is outside workspace
	if !strings.HasPrefix(absPath, absWorkspace+string(filepath.Separator)) && absPath != absWorkspace {
		return absPath, true, nil // Outside workspace
	}

	return absPath, false, nil // Inside workspace
}
