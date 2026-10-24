package repl

import (
	"context"
	"errors"
	"fmt"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/compaction"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// compactAtTurnStart estimates the turn's first request and, when it has
// reached the tier 1 point, rewrites the context before the turn writes
// anything (docs/compaction.md). It fills in start with what the loop needs to
// go on compacting during the turn, and returns the messages to send.
//
// When the last turn ended with an answer, the turn in progress has nothing in
// it yet and the rewrite is done by code alone; when it did not, that turn is
// the turn in progress, and the model compacts it.
func compactAtTurnStart(ctx context.Context, runner *agent.Runner, writer *ui.Writer, cfg *config.Config,
	messages []llm.Message, tl *session.Timeline, start *agent.CompactionStart,
	promptText string, wake bool, instructions *ProjectInstructions, quietMode bool) []llm.Message {

	var prompt []llm.Message
	if !wake {
		prompt = []llm.Message{{Role: llm.RoleUser, Content: promptWithProjectInstructions(promptText, instructions)}}
	}
	promptTokens := compaction.EstimateTokens(prompt)
	sameModel := func(recorded string) bool { return config.WireID(recorded) == cfg.LLM.Model }
	comp := runner.Compactor()
	size, turn := compaction.StartEstimate(tl, messages, specTokens(comp), sameModel)
	start.PromptTokens = size + promptTokens
	start.TurnTokens = turn + promptTokens
	if instructions != nil {
		start.PromptPrefix = projectInstructionsPrefix(instructions)
	}

	if comp == nil {
		return messages
	}
	limits := comp.Settings()
	if !limits.Auto || !limits.Tier1Due(start.PromptTokens, start.TurnTokens, 0, 0) {
		return messages
	}
	res, err := comp.Compact(ctx, compaction.Request{
		Messages:     messages,
		TailIndex:    start.TailIndex,
		TailStart:    start.TailStart,
		TokensBefore: start.PromptTokens,
	})
	if err != nil {
		if !errors.Is(err, compaction.ErrNothingToCompact) {
			writer.Warn(fmt.Sprintf("compaction failed: %v", err))
		}
		return messages
	}
	start.TailIndex, start.TailStart = res.TailIndex, res.TailStart
	start.PromptTokens = res.TokensAfter + promptTokens
	if !quietMode {
		writer.Info(fmt.Sprintf("context compacted: %s → %s tokens", ui.FormatTokens(res.TokensBefore), ui.FormatTokens(res.TokensAfter)))
	}
	return res.Messages
}

// CompactSession rewrites a session's context on request — :compact in the
// interactive front end, kvit-coder --compact on the command line — without
// running a turn. It summarizes the oldest turns whatever the size of the
// result; focus, when given, is what the summaries should concentrate on.
func CompactSession(ctx context.Context, runner *agent.Runner, writer *ui.Writer, cfg *config.Config, systemPrompt string, sess *session.Session, focus string) error {
	comp := runner.Compactor()
	if comp == nil || sess == nil {
		return errors.New("compaction needs a session")
	}
	loaded, tl, err := sess.LoadContext()
	if err != nil {
		return err
	}
	messages := []llm.Message{{Role: llm.RoleSystem, Content: systemPrompt}}
	for _, m := range loaded.Messages {
		if m.Role == llm.RoleSystem {
			return errors.New("this session predates session directories and cannot be compacted")
		}
		messages = append(messages, m)
	}
	sameModel := func(recorded string) bool { return config.WireID(recorded) == cfg.LLM.Model }
	size, _ := compaction.StartEstimate(tl, messages, specTokens(comp), sameModel)
	res, err := comp.Compact(ctx, compaction.Request{
		Messages:     messages,
		TailIndex:    1 + loaded.TailIndex,
		TailStart:    loaded.TailStart,
		TokensBefore: size,
		Force:        true,
		Focus:        focus,
	})
	if errors.Is(err, compaction.ErrNothingToCompact) {
		writer.Info("nothing to compact: the context is as short as compaction makes it")
		return nil
	}
	if err != nil {
		return err
	}
	writer.Info(fmt.Sprintf("context compacted: %s → %s tokens", ui.FormatTokens(res.TokensBefore), ui.FormatTokens(res.TokensAfter)))
	return nil
}

// specTokens is the compactor's estimate of the tool specs every request
// carries, when it can give one.
func specTokens(comp agent.Compactor) int {
	if c, ok := comp.(interface{ SpecTokens() int }); ok {
		return c.SpecTokens()
	}
	return 0
}
