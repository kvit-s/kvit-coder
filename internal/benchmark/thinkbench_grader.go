package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// gradeTask drops the held-out grade.py into the workspace, runs it with the uv
// interpreter (cwd = workspace, with the injected uv env), parses the JSON
// scorecard from stdout, and removes the grader afterward. The grader is
// contracted to exit 0 and print a single JSON scorecard; any other outcome is
// recorded as a grading failure (Score 0, ImportOK false).
//
// The grader is run directly by the harness (not through the agent's sandboxed
// shell tool) — it is trusted code and needs to import the agent's produced
// package from the workspace cwd.
func gradeTask(ctx context.Context, task TBTask, workspace string, env *thinkbenchEnv, timeout time.Duration) (*TBScorecard, []string) {
	graderDst := filepath.Join(workspace, "grade.py")

	// Copy the held-out grader in.
	if err := copyFile(task.GraderPath, graderDst, 0644); err != nil {
		return failedScorecard(task), []string{fmt.Sprintf("failed to copy grader: %v", err)}
	}
	// Always remove it after grading so it never lingers in a persisted workspace.
	defer os.Remove(graderDst)

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, env.UVPy, "grade.py")
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), env.InjectEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	card, parseErr := parseScorecard(stdout.String())
	if card != nil {
		return card, nil
	}

	// No parseable scorecard — record a diagnostic failure.
	var errs []string
	if runErr != nil {
		errs = append(errs, fmt.Sprintf("grader exec error: %v", runErr))
	}
	if parseErr != nil {
		errs = append(errs, fmt.Sprintf("grader scorecard parse error: %v", parseErr))
	}
	if s := strings.TrimSpace(stderr.String()); s != "" {
		errs = append(errs, "grader stderr: "+truncateString(s, 500))
	}
	if len(errs) == 0 {
		errs = []string{"grader produced no scorecard"}
	}
	return failedScorecard(task), errs
}

// parseScorecard extracts the JSON scorecard from grader stdout. The grader
// prints a single JSON object on its own line; we scan from the last non-empty
// line backward to be robust to stray prints.
func parseScorecard(stdout string) (*TBScorecard, error) {
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || line[0] != '{' {
			continue
		}
		var card TBScorecard
		if err := json.Unmarshal([]byte(line), &card); err == nil && card.Total > 0 {
			return &card, nil
		}
	}
	return nil, fmt.Errorf("no JSON scorecard found in %d bytes of stdout", len(stdout))
}

// summarizeFailedChecks returns a "check_id: detail" line for each grader check
// that did not pass, so the report can show exactly where a run fell short.
func summarizeFailedChecks(card *TBScorecard) []string {
	if card == nil {
		return nil
	}
	var out []string
	for _, c := range card.Checks {
		if !c.Passed {
			detail := strings.TrimSpace(c.Detail)
			if detail == "" {
				out = append(out, c.ID)
			} else {
				out = append(out, fmt.Sprintf("%s: %s", c.ID, truncateString(detail, 160)))
			}
		}
	}
	return out
}

// failedScorecard returns a zero-score scorecard with the fixed denominator
// from the manifest, used when grading itself fails.
func failedScorecard(task TBTask) *TBScorecard {
	return &TBScorecard{
		Task:     task.Slug,
		ImportOK: false,
		Passed:   0,
		Total:    task.NumChecks,
		Score:    0,
	}
}
