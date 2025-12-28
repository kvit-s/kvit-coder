package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// ValidationResult holds the result of a single validation check.
type ValidationResult struct {
	Check   ValidationCheck
	Passed  bool
	Message string
}

// Validator validates benchmark results against expected conditions.
type Validator struct {
	workspaceDir string
	output       string // LLM final output
	toolCalls    []ToolCallLog
}

// NewValidator creates a new validator for a benchmark run.
func NewValidator(workspaceDir, output string, toolCalls []ToolCallLog) *Validator {
	return &Validator{
		workspaceDir: workspaceDir,
		output:       output,
		toolCalls:    toolCalls,
	}
}

// Validate runs all validation checks and returns results.
func (v *Validator) Validate(checks []ValidationCheck) (bool, []string) {
	var errors []string
	allPassed := true

	for _, check := range checks {
		result := v.runCheck(check)
		if !result.Passed {
			allPassed = false
			errors = append(errors, result.Message)
		}
	}

	return allPassed, errors
}

// runCheck executes a single validation check.
func (v *Validator) runCheck(check ValidationCheck) ValidationResult {
	var passed bool
	var message string

	switch check.Type {
	case "file_contains":
		passed, message = v.checkFileContains(check)
	case "file_not_contains":
		passed, message = v.checkFileNotContains(check)
	case "file_equals":
		passed, message = v.checkFileEquals(check)
	case "file_exists":
		passed, message = v.checkFileExists(check)
	case "file_not_exists":
		passed, message = v.checkFileNotExists(check)
	case "file_line_count":
		passed, message = v.checkFileLineCount(check)
	case "file_line_equals":
		passed, message = v.checkFileLineEquals(check)
	case "tool_called":
		passed, message = v.checkToolCalled(check)
	case "tool_called_with":
		passed, message = v.checkToolCalledWith(check)
	case "tool_not_called":
		passed, message = v.checkToolNotCalled(check)
	case "output_contains":
		passed, message = v.checkOutputContains(check)
	case "output_not_contains":
		passed, message = v.checkOutputNotContains(check)
	case "output_matches":
		passed, message = v.checkOutputMatches(check)
	case "multi_tool_calls":
		passed, message = v.checkMultiToolCalls(check)
	case "run_command":
		passed, message = v.checkRunCommand(check)
	default:
		passed = false
		message = fmt.Sprintf("unknown validation type: %s", check.Type)
	}

	// Handle negation
	if check.Negate {
		passed = !passed
		if passed {
			message = "check passed (negated)"
		}
	}

	return ValidationResult{
		Check:   check,
		Passed:  passed,
		Message: message,
	}
}

// checkFileContains checks if a file contains expected content.
func (v *Validator) checkFileContains(check ValidationCheck) (bool, string) {
	content, err := v.readFile(check.Target)
	if err != nil {
		return false, fmt.Sprintf("failed to read file %s: %v", check.Target, err)
	}

	if strings.Contains(string(content), check.Expected) {
		return true, ""
	}
	return false, fmt.Sprintf("file %s does not contain '%s'", check.Target, check.Expected)
}

// checkFileNotContains checks if a file does not contain specific content.
func (v *Validator) checkFileNotContains(check ValidationCheck) (bool, string) {
	content, err := v.readFile(check.Target)
	if err != nil {
		// File not existing means it doesn't contain the content
		return true, ""
	}

	if !strings.Contains(string(content), check.Expected) {
		return true, ""
	}
	return false, fmt.Sprintf("file %s contains '%s' but should not", check.Target, check.Expected)
}

// checkFileEquals checks if file content exactly matches expected.
// Trailing newlines are normalized - accepts both with and without.
func (v *Validator) checkFileEquals(check ValidationCheck) (bool, string) {
	content, err := v.readFile(check.Target)
	if err != nil {
		return false, fmt.Sprintf("failed to read file %s: %v", check.Target, err)
	}

	actual := string(content)

	// Normalize trailing newlines - accept both with and without
	actualNorm := strings.TrimRight(actual, "\n")
	expectedNorm := strings.TrimRight(check.Expected, "\n")

	if actualNorm == expectedNorm {
		return true, ""
	}

	// Generate diff to show what's different
	diff, _ := generateDiff(check.Expected, actual, check.Target)
	return false, fmt.Sprintf("file %s content does not match expected:\n%s", check.Target, diff)
}

// checkFileExists checks if a file exists.
func (v *Validator) checkFileExists(check ValidationCheck) (bool, string) {
	path := filepath.Join(v.workspaceDir, check.Target)
	if _, err := os.Stat(path); err == nil {
		return true, ""
	}
	return false, fmt.Sprintf("file %s does not exist", check.Target)
}

// checkFileNotExists checks if a file does not exist.
func (v *Validator) checkFileNotExists(check ValidationCheck) (bool, string) {
	path := filepath.Join(v.workspaceDir, check.Target)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true, ""
	}
	return false, fmt.Sprintf("file %s exists but should not", check.Target)
}

// checkFileLineCount checks if file has expected number of lines.
func (v *Validator) checkFileLineCount(check ValidationCheck) (bool, string) {
	content, err := v.readFile(check.Target)
	if err != nil {
		return false, fmt.Sprintf("failed to read file %s: %v", check.Target, err)
	}

	lines := strings.Split(string(content), "\n")
	// Handle trailing newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) == check.Count {
		return true, ""
	}
	return false, fmt.Sprintf("file %s has %d lines, expected %d", check.Target, len(lines), check.Count)
}

// checkFileLineEquals checks if a specific line equals expected content.
func (v *Validator) checkFileLineEquals(check ValidationCheck) (bool, string) {
	content, err := v.readFile(check.Target)
	if err != nil {
		return false, fmt.Sprintf("failed to read file %s: %v", check.Target, err)
	}

	lines := strings.Split(string(content), "\n")
	if check.Line < 1 || check.Line > len(lines) {
		return false, fmt.Sprintf("line %d out of range (file has %d lines)", check.Line, len(lines))
	}

	actual := lines[check.Line-1] // 1-indexed
	if actual == check.Expected {
		return true, ""
	}
	return false, fmt.Sprintf("line %d is '%s', expected '%s'", check.Line, actual, check.Expected)
}

// checkToolCalled checks if a tool was called.
func (v *Validator) checkToolCalled(check ValidationCheck) (bool, string) {
	for _, tc := range v.toolCalls {
		if strings.EqualFold(tc.Tool, check.Expected) {
			return true, ""
		}
	}
	return false, fmt.Sprintf("tool '%s' was not called", check.Expected)
}

// checkToolNotCalled checks if a tool was NOT called.
func (v *Validator) checkToolNotCalled(check ValidationCheck) (bool, string) {
	for _, tc := range v.toolCalls {
		if strings.EqualFold(tc.Tool, check.Expected) {
			return false, fmt.Sprintf("tool '%s' was called but should not have been", check.Expected)
		}
	}
	return true, ""
}

// checkToolCalledWith checks if a tool was called with specific arguments.
func (v *Validator) checkToolCalledWith(check ValidationCheck) (bool, string) {
	for _, tc := range v.toolCalls {
		if !strings.EqualFold(tc.Tool, check.Expected) {
			continue
		}

		argsStr := string(tc.Args)
		if check.Args == "" {
			return true, "" // Just checking tool was called
		}

		// Try regex match
		re, err := regexp.Compile(check.Args)
		if err != nil {
			// Fall back to substring match
			if strings.Contains(argsStr, check.Args) {
				return true, ""
			}
		} else if re.MatchString(argsStr) {
			return true, ""
		}
	}

	return false, fmt.Sprintf("tool '%s' was not called with args matching '%s'", check.Expected, check.Args)
}

// checkOutputContains checks if the LLM output contains expected text.
func (v *Validator) checkOutputContains(check ValidationCheck) (bool, string) {
	if strings.Contains(v.output, check.Expected) {
		return true, ""
	}
	// Try case-insensitive
	if strings.Contains(strings.ToLower(v.output), strings.ToLower(check.Expected)) {
		return true, ""
	}
	return false, fmt.Sprintf("output does not contain '%s'", check.Expected)
}

// checkOutputNotContains checks if the LLM output does NOT contain specific text.
func (v *Validator) checkOutputNotContains(check ValidationCheck) (bool, string) {
	if !strings.Contains(v.output, check.Expected) &&
		!strings.Contains(strings.ToLower(v.output), strings.ToLower(check.Expected)) {
		return true, ""
	}
	return false, fmt.Sprintf("output contains '%s' but should not", check.Expected)
}

// checkOutputMatches checks if the LLM output matches a regex pattern.
func (v *Validator) checkOutputMatches(check ValidationCheck) (bool, string) {
	re, err := regexp.Compile(check.Expected)
	if err != nil {
		return false, fmt.Sprintf("invalid regex pattern: %v", err)
	}

	if re.MatchString(v.output) {
		return true, ""
	}
	return false, fmt.Sprintf("output does not match pattern '%s'", check.Expected)
}

// checkMultiToolCalls checks if multiple tool calls were made.
func (v *Validator) checkMultiToolCalls(check ValidationCheck) (bool, string) {
	count := 0
	for _, tc := range v.toolCalls {
		if check.Expected == "" || strings.EqualFold(tc.Tool, check.Expected) {
			count++
		}
	}

	if count >= check.Count {
		return true, ""
	}
	return false, fmt.Sprintf("expected at least %d %s calls, got %d", check.Count, check.Expected, count)
}

// checkRunCommand runs a command in the workspace and checks output.
// Uses check.Command for the command to run, check.Expected for expected output.
func (v *Validator) checkRunCommand(check ValidationCheck) (bool, string) {
	if check.Command == "" {
		return false, "run_command requires 'command' field"
	}

	// Run command in workspace directory
	cmd := exec.Command("sh", "-c", check.Command)
	cmd.Dir = v.workspaceDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Sprintf("command failed: %v\noutput: %s", err, string(output))
	}

	// Check if output contains expected string
	if check.Expected != "" {
		if !strings.Contains(string(output), check.Expected) {
			return false, fmt.Sprintf("command output does not contain '%s'\nactual output: %s", check.Expected, string(output))
		}
	}

	return true, ""
}

// readFile reads a file from the workspace.
func (v *Validator) readFile(relativePath string) ([]byte, error) {
	path := filepath.Join(v.workspaceDir, relativePath)
	return os.ReadFile(path)
}

// generateDiff creates a unified diff between expected and actual content.
func generateDiff(expected, actual, filename string) (string, error) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(expected),
		B:        difflib.SplitLines(actual),
		FromFile: "expected",
		ToFile:   "actual",
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(diff)
}

// ExtractToolCalls extracts tool call logs from agent messages.
func ExtractToolCalls(messages []json.RawMessage) []ToolCallLog {
	var calls []ToolCallLog

	for _, msg := range messages {
		var m struct {
			Role      string `json:"role"`
			ToolCalls []struct {
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		}

		if err := json.Unmarshal(msg, &m); err != nil {
			continue
		}

		if m.Role != "assistant" {
			continue
		}

		for _, tc := range m.ToolCalls {
			calls = append(calls, ToolCallLog{
				Tool: tc.Function.Name,
				Args: tc.Function.Arguments,
			})
		}
	}

	return calls
}
