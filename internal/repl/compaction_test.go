package repl

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/compaction"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// noModel fails the test if compaction asks a model anything.
type noModel struct{ t *testing.T }

func (n noModel) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	n.t.Fatalf("compaction at turn start after an answered turn asked the model")
	return nil, nil
}

func turnStartFixture(t *testing.T, window int) (*agent.Runner, *session.Session, *config.Config, *ui.Writer, *bytes.Buffer) {
	t.Helper()
	sess, err := session.OpenDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.LLM.Context = window
	var out bytes.Buffer
	writer := ui.NewWriter(0)
	writer.SetStdout(&out)
	writer.SetStderr(&out)
	writer.SetHeadless(true)
	logger, err := agent.NewLogger("", false)
	if err != nil {
		t.Fatal(err)
	}
	runner := agent.NewRunner(agent.RunnerOptions{
		Cfg: cfg, Registry: tools.NewRegistry(), Writer: writer, Logger: logger, ToolCtx: tools.NewToolContext(),
	})
	runner.SetCompactor(&compaction.Compactor{
		Sess:   sess,
		Client: noModel{t},
		Model:  "test-model",
		Limits: compaction.Resolve(config.CompactionConfig{}, window),
	})
	return runner, sess, cfg, writer, &out
}

func loadStart(t *testing.T, sess *session.Session) ([]llm.Message, *session.Timeline, *agent.CompactionStart) {
	t.Helper()
	ctx, tl, err := sess.LoadContext()
	if err != nil {
		t.Fatal(err)
	}
	msgs := append([]llm.Message{{Role: llm.RoleSystem, Content: "system"}}, ctx.Messages...)
	return msgs, tl, &agent.CompactionStart{TailIndex: 1 + ctx.TailIndex, TailStart: ctx.TailStart}
}

// TestCompactAtTurnStartRewritesAFullContext: when the recorded size of the
// last request puts the new turn past the tier 1 point, the earlier turns are
// reduced to their summaries before the turn writes anything, without asking
// a model.
func TestCompactAtTurnStartRewritesAFullContext(t *testing.T) {
	runner, sess, cfg, writer, out := turnStartFixture(t, 100_000)
	for i, text := range []string{"one", "two"} {
		if err := sess.Settings("test-model", false, nil); err != nil {
			t.Fatal(err)
		}
		answer := llm.Message{Role: llm.RoleAssistant, Content: "done " + text, PromptTokens: 50_000 + 45_000*i}
		if err := sess.AppendMessages([]llm.Message{{Role: llm.RoleUser, Content: text}, answer}); err != nil {
			t.Fatal(err)
		}
	}
	msgs, tl, start := loadStart(t, sess)

	got := compactAtTurnStart(context.Background(), runner, writer, cfg, msgs, tl, start, "three", false, nil, false)

	if len(got) != 5 || got[1].Content != "one" || !strings.HasPrefix(got[2].Content, "done one") {
		t.Fatalf("context after compacting = %d messages starting %q", len(got), got[1].Content)
	}
	if start.TailIndex != 5 || start.TailStart != 4 {
		t.Errorf("start = %+v, want the tail after both summaries", start)
	}
	if start.PromptTokens >= 95_000 || start.PromptTokens <= 0 {
		t.Errorf("first request estimated at %d tokens after compacting", start.PromptTokens)
	}
	if !strings.Contains(out.String(), "context compacted") {
		t.Errorf("no compaction line in the output:\n%s", out.String())
	}
	tl2, err := sess.Timeline()
	if err != nil {
		t.Fatal(err)
	}
	if rec := tl2.LatestCompaction(); rec == nil || rec.Cut != 4 || rec.From != 4 {
		t.Errorf("record = %+v", rec)
	}
}

// TestCompactAtTurnStartLeavesASmallContext: below the tier 1 point nothing is
// rewritten, but the loop is still told where the messages stand.
func TestCompactAtTurnStartLeavesASmallContext(t *testing.T) {
	runner, sess, cfg, writer, _ := turnStartFixture(t, 1_000_000)
	if err := sess.Settings("test-model", false, nil); err != nil {
		t.Fatal(err)
	}
	if err := sess.AppendMessages([]llm.Message{{Role: llm.RoleUser, Content: "one"}, {Role: llm.RoleAssistant, Content: "done", PromptTokens: 5_000}}); err != nil {
		t.Fatal(err)
	}
	msgs, tl, start := loadStart(t, sess)
	got := compactAtTurnStart(context.Background(), runner, writer, cfg, msgs, tl, start, "two", false,
		&ProjectInstructions{Path: "AGENTS.md", Content: "be careful"}, false)
	if len(got) != len(msgs) {
		t.Errorf("a small context was rewritten: %d → %d messages", len(msgs), len(got))
	}
	if start.PromptTokens < 5_000 || start.TurnTokens <= 0 || !strings.Contains(start.PromptPrefix, "be careful") {
		t.Errorf("start = %+v", start)
	}
	tl2, _ := sess.Timeline()
	if len(tl2.Compactions) != 0 {
		t.Errorf("a record was written for a small context")
	}
}
