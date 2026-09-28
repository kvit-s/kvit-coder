package prompt

// Child prompt parity with maki's research_drops_conventions_but_keeps_efficient_extras:
// the child keeps role/environment/output-discipline/batch guidance and drops
// everything aimed at the driver.

import (
	"strings"
	"testing"

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
