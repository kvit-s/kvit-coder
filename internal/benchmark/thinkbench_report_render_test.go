package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTBReportMarkdownRichSections(t *testing.T) {
	tasks := []TBTask{
		{Slug: "base62", Type: TBBugFix, NumChecks: 16},
		{Slug: "cronsim", Type: TBImplement, NumChecks: 14},
		{Slug: "auditlog", Type: TBAmbiguousSpec, Observed: true},
	}
	raw := []TBRunResult{
		{Slug: "base62", Type: "bug-fix", Run: 1, Score: 1.0, Passed: 16, Total: 16, FullPass: true, ImportOK: true, DurationMS: 45000, Tokens: 30000, PromptTokens: 28000, CachedTokens: 21000},
		{Slug: "cronsim", Type: "implement", Run: 1, Score: 0.857, Passed: 12, Total: 14, ImportOK: true, DurationMS: 200000, Tokens: 320000, PromptTokens: 300000, CachedTokens: 240000,
			FailedChecks: []string{"dow_wraparound: cron('0 0 * * 7') should match Sunday; got no match", "step_ranges: */15 in minute field off by one"},
			WorkspaceDir: "/x/failures/cronsim/run-1"},
		{Slug: "auditlog", Type: "ambiguous-spec", Run: 1, Observed: true, DurationMS: 90000, Tokens: 50000, WorkspaceDir: "/x/observed/auditlog/run-1"},
	}
	res := BuildTBResults("qwen-test", 1700000000, 1, tasks, raw, map[string]any{"provider": "local", "endpoint": "http://x/v1"})
	dir := t.TempDir()
	md := filepath.Join(dir, "r.md")
	if err := WriteTBResultsMarkdown(md, "qwen-test", res, raw, "llm:\n  model: qwen-test\n"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(md)
	s := string(b)
	for _, want := range []string{
		"avg time", "avg tokens", // per-task timing cols
		"## Per-run results (graded)",                        // per-run table
		"| cronsim | implement | 1 | 0.857 | 12/14 |",        // a per-run row
		"## Failures (graded runs below full pass)",          // failures section
		"dow_wraparound:",                                    // a failed-check detail
		"Workspace kept at:",                                 // failure workspace pointer
		"## Configuration (config file)", "model: qwen-test", // embedded config
		"## Ambiguous-spec (observed", // observed section
	} {
		if !strings.Contains(s, want) {
			t.Errorf("report missing %q", want)
		}
	}
}
