package agent

// Subagent child loop — setup/teardown for a nested Runner driven
// synchronously inside the turn's process. The tool file
// (internal/tools/subagent.go) owns schema, allowlists and result shaping;
// this file owns the loop. The split mirrors maki's primitives/policy divide
// in Go form.
//
// Import direction is one-way: agent imports tools, never the reverse. The
// tool reaches this driver through the tools.SubRunner interface injected at
// setup, so this file satisfies that interface without tools importing agent.

import (
	"context"
	"errors"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// subagentSessionState is the driver's private half of a
// tools.SubagentSession: the child runner plus the history follow-ups
// re-enter. tools only carries the opaque handle; everything here stays
// in-process and is never persisted.
type subagentSessionState struct {
	child    *Runner
	messages []llm.Message
}

// RunSubagent drives a child loop synchronously inside the turn's process.
// The child's messages are never appended to history.jsonl and never merged
// into the parent messages: the delegation contract is the summary text in
// the parent's Subagent tool result.
func (r *Runner) RunSubagent(ctx context.Context, p tools.SubagentRunParams) (tools.SubagentRunResult, error) {
	sess, out, err := r.StartSubagent(ctx, p)
	if err != nil {
		return out, err
	}
	_ = sess
	return out, nil
}

// StartSubagent runs the child loop to completion and returns a session
// handle for follow-ups (structured_output or summary nudges) alongside
// the result.
func (r *Runner) StartSubagent(ctx context.Context, p tools.SubagentRunParams) (*tools.SubagentSession, tools.SubagentRunResult, error) {
	maxIters := p.MaxIters
	if maxIters <= 0 {
		maxIters = r.cfg.Tools.Subagent.ResolvedMaxChildIterations()
	}
	// The child loop is bounded by its own budget, not the parent's
	// max_tool_iterations: a shallow config copy carries the cap without
	// mutating the turn's config.
	childCfg := shallowSubagentConfig(r.cfg, maxIters)

	child := NewRunner(RunnerOptions{
		Cfg:       childCfg,
		LLMClient: r.llmClient,
		Registry:  p.Registry,
		Writer:    r.writer,
		Logger:    r.logger,
		ToolCtx:   p.ToolCtx,
		// Nil procs/inbox: no background processes, no steering inside a
		// child. Nil persist/rollback: the child transcript is never
		// persisted; only the parent's Subagent tool result reaches the
		// session.
	})

	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: p.System},
		{Role: llm.RoleUser, Content: p.Prompt},
	}
	sess := &subagentSessionState{child: child}
	res, err := child.Run(ctx, RunConfig{Messages: msgs, QuietMode: true})
	if err != nil {
		// A failure to run at all is an error. Anything the loop produced
		// (including a timeout/budget label) comes back as a result.
		if res == nil {
			return nil, tools.SubagentRunResult{}, err
		}
	}
	if res == nil {
		return nil, tools.SubagentRunResult{}, errors.New("subagent run produced no result")
	}
	sess.messages = res.FinalMessages
	return tools.NewSubagentSession(sess), mapSubagentResult(ctx, r.cfg.LLM.Model, res), nil
}

// ContinueSubagent re-enters the session's loop with one more user prompt
// and runs it to completion again. History persists across calls, so a
// nudge sees everything the child said and did before.
func (r *Runner) ContinueSubagent(ctx context.Context, sess *tools.SubagentSession, prompt string) (tools.SubagentRunResult, error) {
	state, ok := sess.Unwrap().(*subagentSessionState)
	if !ok || state == nil || state.child == nil {
		return tools.SubagentRunResult{}, errors.New("invalid subagent session")
	}
	msgs := append(append([]llm.Message(nil), state.messages...), llm.Message{Role: llm.RoleUser, Content: prompt})
	res, err := state.child.Run(ctx, RunConfig{Messages: msgs, QuietMode: true})
	if err != nil {
		if res == nil {
			return tools.SubagentRunResult{}, err
		}
	}
	if res == nil {
		return tools.SubagentRunResult{}, errors.New("subagent run produced no result")
	}
	state.messages = res.FinalMessages
	return mapSubagentResult(ctx, r.cfg.LLM.Model, res), nil
}

// mapSubagentResult converts one loop run into the parent-facing result.
// Per-run usage is reported as-is; the caller sums across continuations.
// (Prompt tokens overlap across runs since history persists; that double
// counting is the honest cost of the extra requests, the same way the
// parent's own retries each cost a full request.)
func mapSubagentResult(ctx context.Context, model string, res *RunResult) tools.SubagentRunResult {
	out := tools.SubagentRunResult{
		Text:            finalAnswerText(res.FinalMessages),
		BudgetExhausted: res.BudgetExhausted,
		TimedOut:        res.TimedOut,
		Model:           model,
	}
	if res.Stats != nil {
		out.PromptTokens = res.Stats.TotalPromptTokens
		out.CompletionTokens = res.Stats.TotalCompletionTokens
		out.Cost = res.Stats.TotalCost
		out.Iterations = res.Stats.Steps
	}
	// A child that hit the context deadline surfaces as timed out even when
	// the loop reports it as a plain failure: the parent sees a labeled
	// partial result, not a bare error.
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.TimedOut = true
	}
	// Budget exhaustion without a final answer still returns what the child
	// said last, labeled by the caller. Empty stays empty: the label is the
	// contract, not invented prose.
	return out
}

// shallowSubagentConfig copies the turn's config with the child's iteration
// budget. It delegates to tools.FloorChildConfig so the child tools and the
// child loop see the same config: maps the child must not share are re-made
// while workspace root, path-safety lists, shell allowlists and
// model/endpoint stay shared for identical containment; read-before-edit is
// floored to the budget and the preview handshake is forced off (confirm
// tools are structurally absent from the child handle).
func shallowSubagentConfig(cfg *config.Config, maxIters int) *config.Config {
	return tools.FloorChildConfig(cfg, maxIters)
}

// finalAnswerText is the child's summary: the last assistant message with
// text and no tool calls. Anything else (tool results, budget notice) is not
// the answer.
func finalAnswerText(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role == llm.RoleAssistant && m.Content != "" && len(m.ToolCalls) == 0 {
			// Skip the loop's own budget notice: it is a marker, not a
			// summary. The caller labels exhaustion itself.
			if strings.HasPrefix(m.Content, "[iteration budget exhausted") {
				continue
			}
			return m.Content
		}
	}
	// Fall back to the last non-empty assistant text (thinking attached to a
	// tool call) so a budget-exhausted child still returns something to label.
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role == llm.RoleAssistant && m.Content != "" {
			return m.Content
		}
	}
	return ""
}
