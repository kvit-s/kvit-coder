package prompt

// Child prompt parity with maki's research_drops_conventions_but_keeps_efficient_extras:
// the child keeps role/environment/output-discipline/batch guidance and drops
// everything aimed at the driver.

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/kvit-s/kvit-coder/internal/tools"
)

func TestResearchDropsConventionsButKeepsEfficientExtras(t *testing.T) {
	for _, typ := range []string{"research", "general"} {
		child := tools.BuildSubagentSystemPrompt(".", typ)
		lower := strings.ToLower(child)

		// Keeps: role line, environment, output-discipline, batch guidance.
		for _, wanted := range []string{
			"# SUBAGENT ROLE",
			"# ENVIRONMENT",
			"Working directory:",
			"Platform:",
			"Today:",
			"# OUTPUT DISCIPLINE",
			"injected into the parent",
			"file:line",
			"batch",
		} {
			if !strings.Contains(lower, strings.ToLower(wanted)) {
				t.Errorf("%s child prompt is missing %q:\n%s", typ, wanted, child)
			}
		}
		// Drops: the parent's identity/tone/conventions slots,
		// project-instructions after blocks, and any ask-the-user affordance.
		// The child says it *cannot* ask the user — that prohibition is
		// required; an invitation ("you can ask") is what must be absent.
		for _, unwanted := range []string{
			"# ROLE\n",
			"# MAIN TASKS",
			"# WORKFLOW",
			"# EXAMPLE",
			"# GUIDELINES",
			"# HOW THIS SESSION WORKS",
			"project-instructions",
			"conventions",
			"/dev/tty",
		} {
			if strings.Contains(child, unwanted) {
				// "# SUBAGENT ROLE" contains "ROLE" but not "# ROLE\n".
				t.Errorf("%s child prompt should not contain %q:\n%s", typ, unwanted, child)
			}
		}
		if strings.Contains(lower, "you can ask") {
			t.Errorf("%s child prompt tells it it can ask the user:\n%s", typ, child)
		}
		if !strings.Contains(lower, "cannot ask") {
			t.Errorf("%s child prompt should say it cannot ask the user:\n%s", typ, child)
		}
	}
	// Only general may touch files.
	if strings.Contains(tools.BuildSubagentSystemPrompt(".", "research"), "Only touch files") {
		t.Error("research child prompt should not invite edits")
	}
	if !strings.Contains(tools.BuildSubagentSystemPrompt(".", "general"), "Only touch files") {
		t.Error("general child prompt should scope edits to the task")
	}
}

// TestSubagentTemplatesMatchBuilder: the canonical templates render to
// exactly what the tools-side builder produces (modulo the template
// variables), so the two copies cannot drift.
func TestSubagentTemplatesMatchBuilder(t *testing.T) {
	vars := map[string]string{
		"WorkspaceRoot": "/repo",
		"Platform":      "linux/amd64",
		"Today":         "2026-09-28",
	}
	for _, typ := range []string{"research", "general"} {
		raw, err := os.ReadFile("prompts/subagents/" + typ + ".tmpl")
		if err != nil {
			t.Fatalf("read %s tmpl: %v", typ, err)
		}
		tmpl, err := template.New(typ).Parse(string(raw))
		if err != nil {
			t.Fatalf("parse %s tmpl: %v", typ, err)
		}
		var sb strings.Builder
		if err := tmpl.Execute(&sb, vars); err != nil {
			t.Fatalf("render %s tmpl: %v", typ, err)
		}
		got := tools.BuildSubagentSystemPrompt("/repo", typ)
		// The builder stamps the live platform and date; pin those for the
		// comparison so the test asserts the words, not the clock.
		got = strings.Replace(got, "Platform: "+runtime.GOOS+"/"+runtime.GOARCH, "Platform: linux/amd64", 1)
		got = strings.Replace(got, "Today: "+time.Now().Format("2006-01-02"), "Today: 2026-09-28", 1)
		if sb.String() != got {
			t.Errorf("%s tmpl does not match builder:\ntmpl:\n%s\nbuilder:\n%s", typ, sb.String(), got)
		}
	}
}
