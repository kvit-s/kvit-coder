package safety

import (
	"os"
	"path/filepath"
	"strings"
)

// SafetyConfig holds safety mode configuration
type SafetyConfig struct {
	StrictMode   bool              `yaml:"strict_mode"`   // Fail-closed on parse errors
	ParanoidMode bool              `yaml:"paranoid_mode"` // Aggressive restrictions
	Audit        AuditConfig       `yaml:"audit"`
	Git          GitSafetyConfig   `yaml:"git"`
	Rm           RmSafetyConfig    `yaml:"rm"`
	Interpreters InterpreterConfig `yaml:"interpreters"`
}

// AuditConfig configures audit logging
type AuditConfig struct {
	Enabled       bool   `yaml:"enabled"`
	LogDir        string `yaml:"log_dir"`
	RedactSecrets bool   `yaml:"redact_secrets"`
	RetentionDays int    `yaml:"retention_days"`
}

// GitSafetyConfig configures git operation safety
type GitSafetyConfig struct {
	BlockPush            bool `yaml:"block_push"`
	BlockHardReset       bool `yaml:"block_hard_reset"`
	BlockCheckoutDiscard bool `yaml:"block_checkout_discard"`
	BlockStashDrop       bool `yaml:"block_stash_drop"`
	BlockCleanForce      bool `yaml:"block_clean_force"`
	WarnBranchForceDelete bool `yaml:"warn_branch_force_delete"`
}

// RmSafetyConfig configures rm command safety
type RmSafetyConfig struct {
	AllowInTemp         bool `yaml:"allow_in_temp"`
	AllowInWorkspaceCwd bool `yaml:"allow_in_workspace_cwd"`
	BlockWorkspaceRoot  bool `yaml:"block_workspace_root"`
}

// InterpreterConfig configures interpreter one-liner blocking
type InterpreterConfig struct {
	BlockOneLiners bool     `yaml:"block_one_liners"`
	Allowed        []string `yaml:"allowed"`
}

// Context provides runtime context for safety checks
type Context struct {
	Config       *SafetyConfig
	WorkspaceDir string   // The workspace root directory
	WorkingDir   string   // Effective cwd (may differ from workspace)
	TempDirs     []string // Recognized temp directories
}

// NewContext creates a new safety context
func NewContext(cfg *SafetyConfig, workspaceDir, workingDir string) *Context {
	tempDirs := []string{"/tmp", "/var/tmp"}
	if tmpDir := os.TempDir(); tmpDir != "" && tmpDir != "/tmp" {
		tempDirs = append(tempDirs, tmpDir)
	}

	return &Context{
		Config:       cfg,
		WorkspaceDir: workspaceDir,
		WorkingDir:   workingDir,
		TempDirs:     tempDirs,
	}
}

// IsTempPath returns true if path is within a recognized temp directory
func (c *Context) IsTempPath(path string) bool {
	absPath := c.resolvePath(path)
	for _, tempDir := range c.TempDirs {
		if strings.HasPrefix(absPath, tempDir+string(filepath.Separator)) || absPath == tempDir {
			return true
		}
	}
	return false
}

// IsWithinWorkspace returns true if path is within workspace
func (c *Context) IsWithinWorkspace(path string) bool {
	absPath := c.resolvePath(path)
	absWorkspace := c.resolvePath(c.WorkspaceDir)

	return strings.HasPrefix(absPath, absWorkspace+string(filepath.Separator)) || absPath == absWorkspace
}

// IsWorkspaceRoot returns true if path equals workspace root
func (c *Context) IsWorkspaceRoot(path string) bool {
	absPath := c.resolvePath(path)
	absWorkspace := c.resolvePath(c.WorkspaceDir)

	return absPath == absWorkspace
}

// resolvePath resolves a path to an absolute path
func (c *Context) resolvePath(path string) string {
	// Handle home directory expansion
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Convert to absolute path
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.WorkingDir, path)
	}

	return filepath.Clean(path)
}

// SystemCriticalPaths are paths that should never be deleted
var SystemCriticalPaths = []string{
	"/",
	"/bin",
	"/boot",
	"/dev",
	"/etc",
	"/lib",
	"/lib64",
	"/proc",
	"/root",
	"/sbin",
	"/sys",
	"/usr",
	"/var",
}

// IsSystemCritical returns true if path is a critical system path
func IsSystemCritical(path string) bool {
	normalized := filepath.Clean(path)

	// Check exact match with system paths
	for _, critical := range SystemCriticalPaths {
		if normalized == critical {
			return true
		}
	}

	// Check home directory
	if home, err := os.UserHomeDir(); err == nil {
		if normalized == home {
			return true
		}
	}

	return false
}
