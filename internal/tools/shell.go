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
	"github.com/kvit-s/kvit-coder/internal/safety"
)

// cmdBlock is a dangerous binary blocked only when it appears in COMMAND
// POSITION — at the start of the command line or immediately after a shell
// separator (; | & ( ) { } < > newline or backtick), optionally with a path
// prefix (/usr/bin/curl, ./curl). Matching command position rather than a bare
// substring avoids the false-positive class where the word is a grep pattern, an
// echo string, a filename, or part of another word: "grep -r shutdown .",
// "man curl", `echo "adapt the layer"` (contains "apt "), "Hebrew" (contains
// "brew "), `grep reboot /var/log`. A plain space does NOT start a command, so an
// argument like the second token in "grep nc" is never matched.
type cmdBlock struct {
	name string
	re   *regexp.Regexp
}

func buildCmdBlocks(names ...string) []cmdBlock {
	const sep = "(?:^|[;|&(){}<>`\n])\\s*"   // start, or a shell separator + optional whitespace
	const pathPrefix = `(?:[\w./-]*/)?`       // optional /usr/bin/ or ./ prefix
	const trail = "(?:$|[\\s;|&(){}<>`\n])"   // token boundary: end or separator/whitespace
	out := make([]cmdBlock, len(names))
	for i, n := range names {
		out[i] = cmdBlock{
			name: n,
			re:   regexp.MustCompile(sep + pathPrefix + regexp.QuoteMeta(n) + trail),
		}
	}
	return out
}

// cmdBlockedAlways blocks privilege-escalation, package-manager, system-control,
// and network binaries — but only when actually invoked as a command (see
// cmdBlock). su/nc/ncat live here too; they were previously matched as standalone
// words, which still falsely blocked them as arguments ("find . -name nc").
var cmdBlockedAlways = buildCmdBlocks(
	"sudo", "chroot", "su", // privilege escalation
	"apt", "apt-get", "yum", "brew", // package managers (modify system)
	"shutdown", "reboot", // system control
	"curl", "wget", "netcat", "nc", "ncat", // network (data exfiltration)
)

// cmdBlockedEval blocks the `eval` shell builtin (in command position). Lifted
// when AllowInterpreters is set (thinkbench/benchmarks): inside that hard OS
// sandbox `eval` grants nothing beyond the already-allowed interpreter
// one-liners, while the token legitimately appears in coding tasks. Default
// keeps it blocked.
var cmdBlockedEval = buildCmdBlocks("eval")

// rmRootRe / rmHomeRe block `rm -rf /` and `rm -rf ~` ONLY when the target is
// the filesystem root or the bare home dir — i.e. `/`, `/*`, `~`, or root/home
// followed by a command terminator. A plain substring match blocked every
// absolute path (`rm -rf /testbed/foo` starts with "rm -rf /"), a false positive
// for ordinary recursive deletes. Deleting an actual system-critical path is
// still caught by safety.RmRule (IsSystemCritical covers /, /usr, $HOME, ...).
var rmRootRe = regexp.MustCompile(`rm\s+-rf\s+/(\s|;|&|\||\)|\*|$)`)
var rmHomeRe = regexp.MustCompile(`rm\s+-rf\s+~(\s|;|&|\||\)|$)`)

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
	safetyChecker *safety.Checker
}

func NewShellAdvancedTool(cfg *config.Config, timeout time.Duration, tempFileMgr *TempFileManager) *ShellAdvancedTool {
	// Convert config.SafetyConfig to safety.SafetyConfig
	safetyCfg := &safety.SafetyConfig{
		StrictMode:   cfg.Safety.StrictMode,
		ParanoidMode: cfg.Safety.ParanoidMode,
		Audit: safety.AuditConfig{
			Enabled:       cfg.Safety.Audit.Enabled,
			LogDir:        cfg.Safety.Audit.LogDir,
			RedactSecrets: cfg.Safety.Audit.RedactSecrets,
			RetentionDays: cfg.Safety.Audit.RetentionDays,
		},
		Git: safety.GitSafetyConfig{
			BlockPush:             cfg.Safety.Git.BlockPush,
			BlockHardReset:        cfg.Safety.Git.BlockHardReset,
			BlockCheckoutDiscard:  cfg.Safety.Git.BlockCheckoutDiscard,
			BlockStashDrop:        cfg.Safety.Git.BlockStashDrop,
			BlockCleanForce:       cfg.Safety.Git.BlockCleanForce,
			WarnBranchForceDelete: cfg.Safety.Git.WarnBranchForceDelete,
		},
		Rm: safety.RmSafetyConfig{
			AllowInTemp:         cfg.Safety.Rm.AllowInTemp,
			AllowInWorkspaceCwd: cfg.Safety.Rm.AllowInWorkspaceCwd,
			BlockWorkspaceRoot:  cfg.Safety.Rm.BlockWorkspaceRoot,
		},
		Interpreters: safety.InterpreterConfig{
			BlockOneLiners: cfg.Safety.Interpreters.BlockOneLiners,
			Allowed:        cfg.Safety.Interpreters.Allowed,
		},
	}

	return &ShellAdvancedTool{
		workspaceRoot: cfg.Workspace.Root,
		cfg:           cfg,
		timeout:       timeout,
		tempFileMgr:   tempFileMgr,
		safetyChecker: safety.NewChecker(safetyCfg),
	}
}

// maxTimeout is the ceiling a call's own timeout is clamped to, from
// tools.shell.max_timeout. Falls back to ten minutes when unconfigured, which
// is what a Config built by config.Load always has.
func (t *ShellAdvancedTool) maxTimeout() time.Duration {
	if t.cfg != nil && t.cfg.Tools.Shell.MaxTimeout > 0 {
		return time.Duration(t.cfg.Tools.Shell.MaxTimeout) * time.Second
	}
	return 10 * time.Minute
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
				"type": "integer",
				"description": fmt.Sprintf("Timeout in seconds (default: %d, max: %d)",
					int(t.timeout.Seconds()), int(t.maxTimeout().Seconds())),
			},
		},
		"required": []string{"command"},
	}
}

func (t *ShellTool) PromptCategory() string     { return "shell" }
func (t *ShellTool) PromptOrder() int           { return 10 }
func (t *ShellTool) PromptTemplateName() string { return "shell" }
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

func (t *ShellAdvancedTool) PromptCategory() string     { return "shell" }
func (t *ShellAdvancedTool) PromptOrder() int           { return 11 }
func (t *ShellAdvancedTool) PromptTemplateName() string { return "shell-advanced" }
func (t *ShellAdvancedTool) PromptSection() string {
	return fmt.Sprintf(`### Shell.advanced - Shell with Options

Use when you need working_dir or timeout. Call with JSON object:

Examples:
- Shell.advanced({"command": "npm test", "working_dir": "frontend/"})
- Shell.advanced({"command": "make build", "timeout": 120})

Parameters:
- command (required): The shell command
- working_dir (optional): Directory to run in (default: %s)
- timeout (optional): Seconds, default %d, max %d`,
		t.workspaceRoot, int(t.timeout.Seconds()), int(t.maxTimeout().Seconds()))
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

	// Determine timeout: use the provided value or the configured default,
	// capped at tools.shell.max_timeout.
	timeout := t.timeout
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Second
		if max := t.maxTimeout(); timeout > max {
			timeout = max
		}
	}

	return t.executeCommand(ctx, params.Command, workDir, timeout)
}

// executeCommand is the actual shell execution implementation
func (t *ShellAdvancedTool) executeCommand(ctx context.Context, command, workDir string, timeout time.Duration) (any, error) {

	// Create output buffer for managing large outputs
	outputBuf := NewOutputBuffer(t.tempFileMgr)
	defer outputBuf.Close()

	// Execute command with process group for proper cleanup.
	// Optionally wrap in an OS sandbox (ExecPrefix) and/or override the child
	// environment (InjectEnv) — used by the thinkbench harness for uv + bwrap.
	cmd := t.buildCommand(command, workDir)
	cmd.Stdout = outputBuf
	cmd.Stderr = outputBuf

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

// buildCommand constructs the *exec.Cmd for a shell command, applying the
// optional ExecPrefix wrapper (OS sandbox) and InjectEnv environment override
// configured for the thinkbench harness. With no prefix/env configured this is
// equivalent to exec.Command("sh", "-c", command) with the parent environment.
func (t *ShellAdvancedTool) buildCommand(command, workDir string) *exec.Cmd {
	prefix := t.cfg.Tools.Shell.ExecPrefix

	var cmd *exec.Cmd
	if len(prefix) > 0 {
		// Substitute {workdir} in any prefix arg (e.g. bwrap --chdir {workdir}).
		args := make([]string, 0, len(prefix)+3)
		for _, a := range prefix {
			args = append(args, strings.ReplaceAll(a, "{workdir}", workDir))
		}
		args = append(args, "sh", "-c", command)
		cmd = exec.Command(args[0], args[1:]...)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Dir = workDir

	if injected := t.cfg.Tools.Shell.InjectEnv; len(injected) > 0 {
		cmd.Env = append(os.Environ(), injected...)
	}

	// Create a new process group so we can kill all child processes on timeout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// killProcessGroup kills the entire process group of the command
func (t *ShellAdvancedTool) killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Kill the entire process group (negative PID)
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		// Fallback: kill just the process
		_ = cmd.Process.Kill()
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

	// Enhanced safety checks (if any features are enabled)
	if t.safetyChecker != nil && t.safetyChecker.IsEnabled() {
		ctx := safety.NewContext(t.safetyChecker.Config(), t.workspaceRoot, effectiveDir)
		result, err := t.safetyChecker.Check(cmd, ctx)
		if err != nil {
			return fmt.Errorf("safety check error: %w", err)
		}

		switch result.Action {
		case safety.Block:
			return fmt.Errorf("blocked by safety rule [%s]: %s", result.Rule, result.Message)
		case safety.Prompt:
			// Delegate to existing path safety prompting for outside-workspace paths
			for _, path := range result.Paths {
				if err := t.cfg.CheckPathSafety("shell", path); err != nil {
					return err
				}
			}
		case safety.Warn:
			fmt.Fprintf(os.Stderr, "⚠️  Warning [%s]: %s\n", result.Rule, result.Message)
		}
	}

	// Block dangerous commands. Privilege-escalation, package-manager,
	// system-control, and network binaries are matched in command position via
	// cmdBlockedAlways below (not here) to avoid substring false positives.
	// These two stay as substrings: they are specific arg patterns, not bare
	// binary names, and rarely collide with innocent text.
	blocked := []string{
		"mkfs", "dd if=", // disk formatting/writing (root/home rm handled by rmRootRe/rmHomeRe below)
	}

	// Interpreter one-liners (python -c, node -e, ...) are blocked by default to
	// keep the agent from bypassing the Edit tool, but the thinkbench harness
	// enables them: its tasks are interpreter-driven and run inside a hard OS
	// sandbox + isolated uv env, where `python3 -c` is the natural way to verify.
	if !t.cfg.Tools.Shell.AllowInterpreters {
		blocked = append(blocked,
			"python -c", "python2 -c", "python3 -c",
			"perl -e", "perl -E",
			"ruby -e",
			"node -e", "node --eval",
			"php -r",
		)
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

	// Block `rm -rf /` (root) and `rm -rf ~` (home) only when the target is
	// root/home itself, not a subdirectory like /testbed/foo or ~/.cache.
	if rmRootRe.MatchString(cmdLower) {
		return fmt.Errorf("blocked dangerous command containing 'rm -rf /'. If you need to run this command, explain why it's necessary and provide the exact command as a one-liner for the user to run manually")
	}
	if rmHomeRe.MatchString(cmdLower) {
		return fmt.Errorf("blocked dangerous command containing 'rm -rf ~'. If you need to run this command, explain why it's necessary and provide the exact command as a one-liner for the user to run manually")
	}

	for _, danger := range blocked {
		if strings.Contains(cmdLower, danger) {
			dangerName := strings.TrimSpace(danger)
			return fmt.Errorf("blocked dangerous command containing '%s'. If you need to run this command, explain why it's necessary and provide the exact command as a one-liner for the user to run manually", dangerName)
		}
	}

	// Privilege/network/system binaries are blocked only in command position
	// (sudo, curl, nc, eval, ...) — not as substrings — so grep patterns, echo
	// strings, filenames, and words like "shutdown"/"reboot"/"adapt" pass through.
	cmdBlocks := cmdBlockedAlways
	if !t.cfg.Tools.Shell.AllowInterpreters {
		cmdBlocks = append(append([]cmdBlock{}, cmdBlockedAlways...), cmdBlockedEval...)
	}
	for _, cb := range cmdBlocks {
		if cb.re.MatchString(cmdLower) {
			return fmt.Errorf("blocked dangerous command containing '%s'. If you need to run this command, explain why it's necessary and provide the exact command as a one-liner for the user to run manually", cb.name)
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
