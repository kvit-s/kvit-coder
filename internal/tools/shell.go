package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// ShellTool - simple string-only interface, translates to Shell.advanced internally
type ShellTool struct {
	advanced *ShellAdvancedTool
}

func NewShellTool(cfg *config.Config, timeout time.Duration, tempFileMgr *TempFileManager) *ShellTool {
	return &ShellTool{
		advanced: NewShellAdvancedTool(cfg, timeout, tempFileMgr),
	}
}

func (t *ShellTool) Name() string {
	return "Shell"
}

func (t *ShellTool) Description() string {
	return "Execute a shell command. Takes a command string directly. For working_dir or timeout options, use Shell.advanced."
}

func (t *ShellTool) JSONSchema() map[string]any {
	// Same schema as Shell.advanced - accepts object, but ignores working_dir/timeout
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
		},
		"required": []string{"command"},
	}
}

// ShellAdvancedTool - full implementation with working_dir and timeout options
type ShellAdvancedTool struct {
	workspaceRoot string
	cfg           *config.Config
	timeout       time.Duration
	tempFileMgr   *TempFileManager
}

func NewShellAdvancedTool(cfg *config.Config, timeout time.Duration, tempFileMgr *TempFileManager) *ShellAdvancedTool {
	return &ShellAdvancedTool{
		workspaceRoot: cfg.Workspace.Root,
		cfg:           cfg,
		timeout:       timeout,
		tempFileMgr:   tempFileMgr,
	}
}

func (t *ShellAdvancedTool) Name() string {
	return "Shell.advanced"
}

func (t *ShellAdvancedTool) Description() string {
	return "Execute a shell command with options for working directory and timeout."
}

func (t *ShellAdvancedTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"working_dir": map[string]any{
				"type":        "string",
				"description": "Working directory (relative to workspace root or absolute)",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds (default: 30, max: 180)",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ShellTool) PromptCategory() string { return "shell" }
func (t *ShellTool) PromptOrder() int        { return 10 }
func (t *ShellTool) PromptSection() string {
	// Build file operations warning based on enabled tools
	var warnings []string

	if t.advanced.cfg.Tools.Read.Enabled {
		warnings = append(warnings, "Do NOT use cat/head/tail - use Read tool")
	}
	if t.advanced.cfg.Tools.Edit.Enabled {
		warnings = append(warnings, "Do NOT use sed/awk - use Edit tool")
	}

	var warningLine string
	if len(warnings) > 0 {
		warningLine = "\n\n" + strings.Join(warnings, ". ") + "."
	}

	return fmt.Sprintf(`### Shell - Execute Shell Commands

Shell({"command": "pytest -q"})

Examples: "go build ./...", "npm test", "git status", "ls -la"

Runs in workspace root (%s). For different directory or custom timeout, use Shell.advanced.%s`, t.advanced.workspaceRoot, warningLine)
}

func (t *ShellAdvancedTool) PromptCategory() string { return "shell" }
func (t *ShellAdvancedTool) PromptOrder() int        { return 11 }
func (t *ShellAdvancedTool) PromptSection() string {
	return fmt.Sprintf(`### Shell.advanced - Shell with Options

Use when you need working_dir or timeout. Call with JSON object:

Examples:
- Shell.advanced({"command": "npm test", "working_dir": "frontend/"})
- Shell.advanced({"command": "make build", "timeout": 120})

Parameters:
- command (required): The shell command
- working_dir (optional): Directory to run in (default: %s)
- timeout (optional): Seconds, default 30, max 180`, t.workspaceRoot)
}

// Check performs validation - delegates to Shell.advanced
func (t *ShellTool) Check(ctx context.Context, args json.RawMessage) error {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	// Use workspace root as effective working directory
	return t.advanced.validateCommand(params.Command, t.advanced.workspaceRoot)
}

// Call executes command - delegates to Shell.advanced (ignores working_dir/timeout)
func (t *ShellTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	// Pass through to advanced - it will use defaults for working_dir and timeout
	return t.advanced.Call(ctx, args)
}

// Check performs validation for ShellAdvancedTool
func (t *ShellAdvancedTool) Check(ctx context.Context, args json.RawMessage) error {
	var params struct {
		Command    string `json:"command"`
		WorkingDir string `json:"working_dir,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	// Determine effective working directory for path safety checks
	effectiveWorkDir := t.workspaceRoot
	if params.WorkingDir != "" {
		resolvedDir, err := t.validateWorkingDir(params.WorkingDir)
		if err != nil {
			return fmt.Errorf("invalid working_dir: %w", err)
		}
		effectiveWorkDir = resolvedDir
	}

	// Safety check: validate command with effective working directory
	return t.validateCommand(params.Command, effectiveWorkDir)
}

func (t *ShellAdvancedTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Command    string `json:"command"`
		WorkingDir string `json:"working_dir,omitempty"`
		Timeout    int    `json:"timeout,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Determine working directory
	workDir := t.workspaceRoot
	if params.WorkingDir != "" {
		resolvedDir, err := t.validateWorkingDir(params.WorkingDir)
		if err != nil {
			return nil, fmt.Errorf("invalid working_dir: %w", err)
		}
		workDir = resolvedDir
	}

	// Determine timeout: use provided value or default, capped at 3 minutes
	const maxTimeout = 3 * time.Minute
	timeout := t.timeout
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Second
		if timeout > maxTimeout {
			timeout = maxTimeout
		}
	}

	return t.executeCommand(ctx, params.Command, workDir, timeout)
}

// executeCommand is the actual shell execution implementation
func (t *ShellAdvancedTool) executeCommand(ctx context.Context, command, workDir string, timeout time.Duration) (any, error) {

	// Create output buffer for managing large outputs
	outputBuf := NewOutputBuffer(t.tempFileMgr)
	defer outputBuf.Close()

	// Execute command with process group for proper cleanup
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = workDir
	cmd.Stdout = outputBuf
	cmd.Stderr = outputBuf
	// Create a new process group so we can kill all child processes on timeout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Wait for command with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Create timeout timer
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var timedOut bool
	var cmdErr error

	select {
	case <-ctx.Done():
		// Parent context cancelled (e.g., user pressed ESC)
		t.killProcessGroup(cmd)
		<-done // Wait for process to exit
		timedOut = true
	case <-timer.C:
		// Timeout - kill the entire process group
		t.killProcessGroup(cmd)
		<-done // Wait for process to exit
		timedOut = true
	case cmdErr = <-done:
		// Command completed normally
	}

	if timedOut {
		// Get any partial output
		partialOutput, _ := outputBuf.FormatForLLM()
		timeoutSecs := int(timeout.Seconds())
		return map[string]any{
			"stdout":    partialOutput,
			"exit_code": -1,
			"error":     "timeout",
			"hint":      fmt.Sprintf("Command timed out after %ds. Use Shell.advanced with timeout=%d", timeoutSecs, timeoutSecs*2),
		}, nil
	}

	exitCode := 0
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("execution failed: %w", cmdErr)
		}
	}

	// Format output for LLM (with truncation if needed)
	formattedOutput, err := outputBuf.FormatForLLM()
	if err != nil {
		return nil, fmt.Errorf("failed to format output: %w", err)
	}

	return map[string]any{
		"stdout":    formattedOutput,
		"exit_code": exitCode,
	}, nil
}

// killProcessGroup kills the entire process group of the command
func (t *ShellAdvancedTool) killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Kill the entire process group (negative PID)
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		// Fallback: kill just the process
		cmd.Process.Kill()
	}
}

// validateCommand validates a shell command for safety
// baseDir is the effective working directory for resolving relative paths
func (t *ShellAdvancedTool) validateCommand(cmd string, baseDir string) error {
	cmdLower := strings.ToLower(cmd)
	cmdTrimmed := strings.TrimSpace(cmd)

	// Effective directory starts with baseDir, may be modified by cd
	effectiveDir := baseDir

	// Handle 'cd' commands - allow chained (cd /path && cmd), block standalone
	if cmdTrimmed == "cd" || strings.HasPrefix(cmdTrimmed, "cd ") {
		// Check if cd is chained with another command via && or ;
		hasChain := strings.Contains(cmdTrimmed, "&&") || strings.Contains(cmdTrimmed, ";")

		if !hasChain {
			// Standalone cd has no effect in stateless shell - block with helpful message
			parts := strings.Fields(cmdTrimmed)
			if len(parts) >= 2 {
				return fmt.Errorf("Standalone 'cd' has no effect (shell is stateless). Either:\n- Use Shell.advanced with working_dir=\"%s\"\n- Chain commands: cd %s && your_command", parts[1], parts[1])
			}
			return fmt.Errorf("Standalone 'cd' has no effect. Use Shell.advanced with working_dir parameter")
		}

		// Extract cd target and update effective directory for path safety checks
		if cdTarget := t.extractCdTarget(cmdTrimmed); cdTarget != "" {
			resolvedCdDir, err := t.resolveCdPath(cdTarget, baseDir)
			if err == nil {
				effectiveDir = resolvedCdDir
			}
			// If cd path can't be resolved, continue with baseDir (shell will fail at runtime)
		}
	}

	// Block dangerous commands
	blocked := []string{
		"sudo ", "sudo\t",
		"su ", "su\t",
		"rm -rf /", "rm -rf ~",
		"apt ", "apt-get ", "yum ", "brew ",
		"shutdown", "reboot",
		"chroot ",
		"mkfs", "dd if=", // disk formatting/writing
	}

	// Block file edit commands - only when Edit tool is available as an alternative
	if t.cfg.Tools.Edit.Enabled {
		if strings.Contains(cmdLower, "sed -i") {
			return fmt.Errorf("STOP: Do not use Shell to edit files. Call the Edit tool with {\"path\": \"<filepath>\", \"start_line\": N, \"end_line\": N, \"new_text\": \"<replacement>\"}")
		}

		if strings.Contains(cmdLower, "awk ") {
			return fmt.Errorf("STOP: Do not use awk. Call Read to read files, or Edit to modify files")
		}
	}

	for _, danger := range blocked {
		if strings.Contains(cmdLower, danger) {
			dangerName := strings.TrimSpace(danger)
			return fmt.Errorf("blocked dangerous command containing '%s'. If you need to run this command, explain why it's necessary and provide the exact command as a one-liner for the user to run manually", dangerName)
		}
	}

	// Check allowlist if configured
	if len(t.cfg.Tools.Shell.AllowedCommands) > 0 {
		allowed := false
		for _, allowedCmd := range t.cfg.Tools.Shell.AllowedCommands {
			if strings.HasPrefix(cmd, allowedCmd) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command not in allowlist: %s", cmd)
		}
	}

	// Check blocklist if configured
	if len(t.cfg.Tools.Shell.DisallowedCommands) > 0 {
		for _, disallowedCmd := range t.cfg.Tools.Shell.DisallowedCommands {
			if strings.HasPrefix(cmd, disallowedCmd) {
				return fmt.Errorf("command in blocklist: %s", disallowedCmd)
			}
		}
	}

	// Check for paths outside workspace using effective working directory
	if err := t.checkPathSafety(cmd, effectiveDir); err != nil {
		return err
	}

	return nil
}

// extractCdTarget extracts the target directory from a cd command
// Returns the first cd target found (e.g., "cd /foo && cmd" returns "/foo")
func (t *ShellAdvancedTool) extractCdTarget(cmd string) string {
	// Match "cd <path>" at start of command, handling quotes
	cdPattern := regexp.MustCompile(`^cd\s+["']?([^\s"';&]+)["']?`)
	if match := cdPattern.FindStringSubmatch(cmd); len(match) > 1 {
		return match[1]
	}
	return ""
}

// resolveCdPath resolves a cd target path to an absolute path
func (t *ShellAdvancedTool) resolveCdPath(cdTarget, baseDir string) (string, error) {
	// Handle home directory expansion
	path := cdTarget
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}

	// Resolve relative paths against baseDir
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}

	return filepath.Clean(path), nil
}

// extractPaths extracts potential file paths from a command
func (t *ShellAdvancedTool) extractPaths(cmd string) []string {
	// Pattern to match potential file paths (both absolute and relative)
	// Matches: /abs/path, ~/home/path, ../relative, ./file, filename
	// Fixed: simplified pattern to properly capture paths after whitespace
	pathPattern := regexp.MustCompile(`(?:^|\s)([~/.][\w\-./~]+|/[\w\-./~]+)`)
	matches := pathPattern.FindAllStringSubmatch(cmd, -1)

	var paths []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			path := strings.Trim(match[1], "\"'")
			// Skip common commands and flags
			if strings.HasPrefix(path, "-") || isCommonCommand(path) {
				continue
			}
			if !seen[path] {
				paths = append(paths, path)
				seen[path] = true
			}
		}
	}
	return paths
}

// isCommonCommand checks if a string is a common Unix command
func isCommonCommand(s string) bool {
	commands := map[string]bool{
		"ls": true, "cat": true, "grep": true, "find": true, "sed": true,
		"awk": true, "echo": true, "cd": true, "pwd": true, "mkdir": true,
		"rm": true, "cp": true, "mv": true, "touch": true, "chmod": true,
		"rg": true, "patch": true, "diff": true, "git": true, "make": true,
	}
	return commands[s]
}

// isPathOutsideWorkspace checks if a path resolves to outside the workspace
// baseDir is the directory to resolve relative paths against
// Uses shared utility function for consistent path validation across all tools
func (t *ShellAdvancedTool) isPathOutsideWorkspace(path string, baseDir string) (bool, string, error) {
	// First resolve the path relative to baseDir
	resolvedPath := path
	if !filepath.IsAbs(path) && !strings.HasPrefix(path, "~/") {
		resolvedPath = filepath.Join(baseDir, path)
	}
	// Then check if it's outside workspace
	normalized, outside, err := NormalizeAndValidatePath(t.workspaceRoot, resolvedPath)
	return outside, normalized, err
}

// checkPathSafety checks if command accesses paths outside workspace and prompts if needed
// baseDir is the effective working directory (considering cd commands or working_dir param)
func (t *ShellAdvancedTool) checkPathSafety(cmd string, baseDir string) error {
	paths := t.extractPaths(cmd)

	// Use unified safety check for individual paths
	for _, path := range paths {
		outside, absPath, err := t.isPathOutsideWorkspace(path, baseDir)
		if err != nil {
			continue // Skip paths we can't resolve
		}
		if outside {
			// Use unified safety check for individual paths
			if err := t.cfg.CheckPathSafety("shell", absPath); err != nil {
				return err
			}
		}
	}

	return nil
}


// validateWorkingDir validates and resolves a working directory path
func (t *ShellAdvancedTool) validateWorkingDir(dir string) (string, error) {
	// Use shared path normalization utility
	absDir, outside, err := NormalizeAndValidatePath(t.workspaceRoot, dir)
	if err != nil {
		return "", err
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		return "", fmt.Errorf("directory does not exist: %s", absDir)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absDir)
	}

	// If outside workspace, use unified safety check
	if outside {
		if err := t.cfg.CheckPathSafety("shell.workdir", absDir); err != nil {
			return "", err
		}
	}

	return absDir, nil
}
