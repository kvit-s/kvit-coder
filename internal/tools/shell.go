package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/permissions"
	"github.com/kvit-s/kvit-coder/internal/procutil"
	"github.com/kvit-s/kvit-coder/internal/safety"
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
	return "Execute a shell command. Takes a command string; working_dir and timeout are also accepted and behave exactly like Shell.advanced."
}

func (t *ShellTool) JSONSchema() map[string]any {
	// Shell accepts the same options as Shell.advanced so a model sending
	// {"command": ..., "timeout": ...} to Shell does not fail and retry.
	// The prompt still steers timeout/working_dir use toward Shell.advanced.
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
				"description": "Timeout in seconds (default: 30, max: 600)",
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
	// toolCtx carries the grants made so far and the inbox a permission
	// question is answered through. Nil in a test, which leaves only the
	// builtin and config rules in force.
	toolCtx *ToolContext
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

Runs in workspace root (%s). working_dir and timeout are also accepted here
and behave like Shell.advanced, e.g. Shell({"command": "make build", "timeout": 120}).%s`, t.advanced.workspaceRoot, warningLine)
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

// shellParsedArgs is the tolerant result of parsing Shell/Shell.advanced
// arguments: bare strings, numeric strings and "120s" timeouts all land here
// so the model gets an actionable message instead of a Go type error.
type shellParsedArgs struct {
	Command    string
	WorkingDir string
	Timeout    int
}

const shellArgsExample = `Example: {"command": "go vet ./...", "timeout": 120}`

// shellArgsError formats a field-level error the model can act on.
func shellArgsError(toolName, msg string) error {
	return fmt.Errorf("Invalid arguments for %s: %s %s", toolName, msg, shellArgsExample)
}

// parseShellArgs accepts what LLMs actually send:
//   - {"command": "..."} plus optional working_dir/timeout,
//   - a bare JSON string "ls -la" meaning {"command": "ls -la"},
//   - timeout as 120, 120.0, "120" or "120s" ("" and null mean default).
func parseShellArgs(toolName string, args json.RawMessage) (shellParsedArgs, error) {
	trimmed := strings.TrimSpace(string(args))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return shellParsedArgs{}, shellArgsError(toolName, `"command" is required (a shell command string).`)
	}
	// Bare string: Shell("go vet ./...") instead of Shell({"command": "..."}).
	var bare string
	if err := json.Unmarshal(args, &bare); err == nil {
		if strings.TrimSpace(bare) == "" {
			return shellParsedArgs{}, shellArgsError(toolName, `"command" must not be empty.`)
		}
		return shellParsedArgs{Command: bare}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		// Covers arrays, numbers, and malformed JSON: tell the model the
		// shape instead of leaking "cannot unmarshal array into Go value".
		return shellParsedArgs{}, shellArgsError(toolName,
			`arguments must be a JSON object with a "command" string.`)
	}
	var out shellParsedArgs
	// command (required)
	cmdRaw, ok := raw["command"]
	if !ok {
		return shellParsedArgs{}, shellArgsError(toolName, `"command" is required (a shell command string).`)
	}
	// Null command is missing, not empty.
	if strings.TrimSpace(string(cmdRaw)) == "null" {
		return shellParsedArgs{}, shellArgsError(toolName, `"command" is required (a shell command string).`)
	}
	var cmd string
	if err := json.Unmarshal(cmdRaw, &cmd); err != nil {
		// Be lenient when the normalizer did not run (a bare number/bool):
		// numbers become their string form, anything else is an error.
		var generic any
		if gerr := json.Unmarshal(cmdRaw, &generic); gerr == nil {
			if coerced, ok := coerceToString(generic); ok && strings.TrimSpace(coerced) != "" {
				cmd = coerced
			} else {
				return shellParsedArgs{}, shellArgsError(toolName,
					fmt.Sprintf(`"command" must be a string, got %s.`, strings.TrimSpace(string(cmdRaw))))
			}
		} else {
			return shellParsedArgs{}, shellArgsError(toolName,
				fmt.Sprintf(`"command" must be a string, got %s.`, strings.TrimSpace(string(cmdRaw))))
		}
	}
	if strings.TrimSpace(cmd) == "" {
		return shellParsedArgs{}, shellArgsError(toolName, `"command" must not be empty.`)
	}
	out.Command = cmd
	// working_dir (optional)
	if wdRaw, ok := raw["working_dir"]; ok && strings.TrimSpace(string(wdRaw)) != "" && strings.TrimSpace(string(wdRaw)) != "null" {
		var wd string
		if err := json.Unmarshal(wdRaw, &wd); err != nil {
			var generic any
			if gerr := json.Unmarshal(wdRaw, &generic); gerr == nil {
				if coerced, ok := coerceToString(generic); ok {
					wd = coerced
				} else {
					return shellParsedArgs{}, shellArgsError(toolName,
						fmt.Sprintf(`"working_dir" must be a directory path string, got %s.`, strings.TrimSpace(string(wdRaw))))
				}
			} else {
				return shellParsedArgs{}, shellArgsError(toolName,
					fmt.Sprintf(`"working_dir" must be a directory path string, got %s.`, strings.TrimSpace(string(wdRaw))))
			}
		}
		out.WorkingDir = wd
	}
	// timeout (optional): 120, 120.0, "120", "120s" all mean 120 seconds.
	if tRaw, ok := raw["timeout"]; ok && strings.TrimSpace(string(tRaw)) != "" && strings.TrimSpace(string(tRaw)) != "null" {
		var generic any
		if err := json.Unmarshal(tRaw, &generic); err != nil {
			return shellParsedArgs{}, shellArgsError(toolName,
				fmt.Sprintf(`"timeout" must be seconds as a number (e.g. 120), got %s.`, strings.TrimSpace(string(tRaw))))
		}
		switch v := generic.(type) {
		case float64:
			out.Timeout = int(v)
		case string:
			coerced, empty, ok := parseFlexibleIntString(v)
			if empty {
				out.Timeout = 0
			} else if !ok {
				return shellParsedArgs{}, shellArgsError(toolName,
					fmt.Sprintf(`"timeout" must be seconds as a number (e.g. 120), got %q.`, v))
			} else {
				out.Timeout = coerced
			}
		case bool:
			return shellParsedArgs{}, shellArgsError(toolName,
				fmt.Sprintf(`"timeout" must be seconds as a number (e.g. 120), got %s.`, strings.TrimSpace(string(tRaw))))
		default:
			return shellParsedArgs{}, shellArgsError(toolName,
				fmt.Sprintf(`"timeout" must be seconds as a number (e.g. 120), got %s.`, strings.TrimSpace(string(tRaw))))
		}
	}
	return out, nil
}

// Check performs validation - delegates to Shell.advanced
func (t *ShellTool) Check(ctx context.Context, args json.RawMessage) error {
	params, err := parseShellArgs("Shell", args)
	if err != nil {
		return err
	}
	workDir := t.advanced.workspaceRoot
	if params.WorkingDir != "" {
		resolvedDir, err := t.advanced.validateWorkingDir(params.WorkingDir)
		if err != nil {
			return fmt.Errorf("invalid working_dir: %w", err)
		}
		workDir = resolvedDir
	}
	// Use workspace root as effective working directory
	return t.advanced.validateCommand(params.Command, workDir)
}

// ReadOnlyCall reports whether the command only reads, so a turn that ran
// nothing but ls, grep or git diff is not treated as one that changed files.
func (t *ShellTool) ReadOnlyCall(args json.RawMessage) bool {
	return t.advanced.ReadOnlyCall(args)
}

// ReadOnlyCall reports whether the command only reads. The working directory
// and timeout do not change the answer.
func (t *ShellAdvancedTool) ReadOnlyCall(args json.RawMessage) bool {
	params, err := parseShellArgs("Shell", args)
	if err != nil {
		return false
	}
	return permissions.ReadOnly(params.Command)
}

// Call executes command - delegates to Shell.advanced (working_dir/timeout behave the same)
func (t *ShellTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	// Pass through to advanced - it parses the same tolerant arguments
	return t.advanced.Call(ctx, args)
}

// Check performs validation for ShellAdvancedTool
func (t *ShellAdvancedTool) Check(ctx context.Context, args json.RawMessage) error {
	params, err := parseShellArgs("Shell.advanced", args)
	if err != nil {
		return err
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
	params, err := parseShellArgs("Shell.advanced", args)
	if err != nil {
		return nil, err
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
	// Fail fast when no POSIX shell exists (Windows without Git for Windows)
	// rather than surfacing a bare exec "sh not found".
	if _, err := procutil.ResolveShell(); err != nil {
		return nil, err
	}

	// Create output buffer for managing large outputs
	outputBuf := NewOutputBuffer(t.tempFileMgr)
	defer outputBuf.Close()

	// Execute command with process group for proper cleanup.
	// Optionally wrap in an OS sandbox (ExecPrefix) and/or override the child
	// environment (InjectEnv) — used by the thinkbench harness for uv + bwrap.
	cmd := t.buildCommand(command, workDir)
	// Under verbose output the command's stream also goes to the terminal as it
	// arrives, so a long build is something to watch rather than a cursor that
	// sits there for two minutes. The buffer still gets everything, so what the
	// model is told is unchanged.
	sink := io.Writer(outputBuf)
	if t.cfg.LLM.Verbose > 0 {
		sink = io.MultiWriter(outputBuf, os.Stderr)
	}
	cmd.Stdout = sink
	cmd.Stderr = sink

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

	var timedOut, interrupted bool
	var cmdErr error

	select {
	case <-ctx.Done():
		// The turn was cancelled — the user pressed ctrl-c, or the caller's
		// time budget expired. Kill the whole process group so nothing this
		// command started outlives the turn.
		t.killProcessGroup(cmd)
		<-done // Wait for process to exit
		interrupted = true
	case <-timer.C:
		// Timeout - kill the entire process group
		t.killProcessGroup(cmd)
		<-done // Wait for process to exit
		timedOut = true
	case cmdErr = <-done:
		// Command completed normally
	}

	// Either way the command was killed part-way, so return what it had
	// produced. Which of the two it was decides what to tell the model: a
	// timeout can be retried with a longer one, an interrupt cannot.
	if interrupted {
		partialOutput, _ := outputBuf.FormatForLLM()
		return map[string]any{
			"stdout":    partialOutput,
			"exit_code": -1,
			"error":     "interrupted",
			"hint":      "The command was killed part-way when the turn was cancelled. The output above is everything it produced before that.",
		}, nil
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

	// The agent's shell is sh everywhere: on Windows this resolves to Git for
	// Windows' sh.exe (PATH plus the standard install locations), failing
	// fast with a message naming Git for Windows when none is found.
	shell, err := procutil.ResolveShell()
	if err != nil {
		shell = "sh"
	}

	var cmd *exec.Cmd
	if len(prefix) > 0 {
		// Substitute {workdir} in any prefix arg (e.g. bwrap --chdir {workdir}).
		args := make([]string, 0, len(prefix)+3)
		for _, a := range prefix {
			args = append(args, strings.ReplaceAll(a, "{workdir}", workDir))
		}
		args = append(args, shell, "-c", command)
		cmd = exec.Command(args[0], args[1:]...)
	} else {
		cmd = exec.Command(shell, "-c", command)
	}
	cmd.Dir = workDir

	if injected := t.cfg.Tools.Shell.InjectEnv; len(injected) > 0 {
		cmd.Env = append(os.Environ(), injected...)
	}

	// Create a new process group so we can kill all child processes on timeout
	// (Setpgid on unix, CREATE_NEW_PROCESS_GROUP on Windows).
	procutil.DetachProcessGroup(cmd)
	return cmd
}

// killProcessGroup kills the entire process group of the command
func (t *ShellAdvancedTool) killProcessGroup(cmd *exec.Cmd) {
	procutil.KillProcessGroup(cmd)
}

// validateCommand validates a shell command for safety
// baseDir is the effective working directory for resolving relative paths
func (t *ShellAdvancedTool) validateCommand(cmd string, baseDir string) error {
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

	// Decide from the command's syntax tree what it actually wants to do, and
	// judge each simple command in it on its own. "git diff && rm -rf /" has
	// two commands and the second one is the problem; a check that looked at
	// the line as text never saw it.
	if err := t.checkPermissions(cmd); err != nil {
		return err
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

// SetToolContext gives the tool the grants and the inbox it needs to ask about
// a command that needs permission.
func (t *ShellAdvancedTool) SetToolContext(toolCtx *ToolContext) { t.toolCtx = toolCtx }

// SetToolContext passes the context through to the tool that does the work.
func (t *ShellTool) SetToolContext(toolCtx *ToolContext) { t.advanced.SetToolContext(toolCtx) }

// extractPaths returns the arguments of a command that name a file or a
// directory, taken from the parsed command rather than by matching the line
// against a regular expression. A pattern over the raw text cannot tell a path
// from a flag, a grep pattern, or a word inside a quoted string; the parser
// knows which word is which.
func (t *ShellAdvancedTool) extractPaths(cmd string) []string {
	scopes, err := permissions.Parse(cmd)
	if err != nil {
		return nil
	}
	var paths []string
	seen := map[string]bool{}
	for _, s := range scopes {
		for _, p := range s.Paths {
			if !seen[p] {
				paths = append(paths, p)
				seen[p] = true
			}
		}
	}
	return paths
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
