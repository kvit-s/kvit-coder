package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/safety"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// Anomaly trigger names (must match config.DiagnosticsConfig.InterrogateTriggers values).
const (
	TriggerDuplicateCall   = "duplicate_call"
	TriggerPendingBlocked  = "pending_blocked"
	TriggerAlternatingLoop = "alternating_loop"
	TriggerFatal           = "fatal"
	TriggerBacktrack       = "backtrack"
)

// interrogateHistoryExcerpt is how many trailing history messages to capture in a log record.
const interrogateHistoryExcerpt = 8

// interrogateTimeout bounds a single interrogation round-trip.
const interrogateTimeout = 30 * time.Second

// Episode describes one detected anomaly to (maybe) interrogate and log.
type Episode struct {
	Trigger       string // one of the Trigger* constants
	Key           string // dedup key - interrogation fires once per episode
	TriggerCount  int    // e.g. how many consecutive identical calls
	OffendingTool string
	OffendingArgs string // raw JSON arguments of the offending call
	PriorResult   string // the previous tool result the model saw
	PendingDiff   string // for pending-edit triggers: the pending diff
	Turn          int
	ContextTokens int
}

// Interrogator asks the model to explain anomalous behavior and logs the Q&A to a
// side channel. It is observability-only: callers resume their normal recovery path
// unchanged. Construction returns nil when diagnostics are disabled, so all call sites
// can treat a nil *Interrogator as "do nothing".
type Interrogator struct {
	client   *llm.Client
	cfg      *config.Config
	writer   *ui.Writer
	logger   *Logger
	runID    string
	redactor *safety.Redactor
}

// NewInterrogator builds an Interrogator, or returns nil when interrogation is disabled.
func NewInterrogator(cfg *config.Config, client *llm.Client, writer *ui.Writer, logger *Logger, runID string) *Interrogator {
	if cfg == nil || !cfg.Diagnostics.InterrogateOnAnomaly {
		return nil
	}
	var redactor *safety.Redactor
	if cfg.Safety.Audit.RedactSecrets {
		redactor = safety.NewRedactor()
	}
	return &Interrogator{
		client:   client,
		cfg:      cfg,
		writer:   writer,
		logger:   logger,
		runID:    runID,
		redactor: redactor,
	}
}

// Enabled reports whether interrogation is active (nil-safe).
func (it *Interrogator) Enabled() bool {
	return it != nil && it.cfg.Diagnostics.InterrogateOnAnomaly
}

// Interrogate runs one interrogation episode if all gates pass: feature enabled, this
// trigger enabled, the per-task cap not yet hit, and this episode not already seen.
// It never mutates production history and never returns an error - failures are logged
// and swallowed so the caller's recovery path is unaffected.
func (it *Interrogator) Interrogate(ctx context.Context, ep Episode, state *runState) {
	if !it.Enabled() {
		return
	}
	if !it.cfg.Diagnostics.TriggerEnabled(ep.Trigger) {
		return
	}
	if state.seenInterrogations == nil {
		state.seenInterrogations = make(map[string]bool)
	}
	// Fire once per anomaly episode, and respect the per-task hard cap.
	if ep.Key != "" && state.seenInterrogations[ep.Key] {
		return
	}
	if state.interrogationCount >= it.cfg.Diagnostics.GetInterrogateMaxPerTask() {
		return
	}
	if ep.Key != "" {
		state.seenInterrogations[ep.Key] = true
	}
	state.interrogationCount++

	question := buildInterrogationQuestion(ep)

	it.writer.Warn(fmt.Sprintf("🔎 Interrogation [%s] - asking the model to explain the anomaly (diagnostic, will not change the task)", ep.Trigger))
	it.writer.Info("Interrogation Q:\n" + question)

	answer, err := it.ask(ctx, state.messages, question)
	if err != nil {
		if it.logger != nil {
			it.logger.Error("interrogation failed", err)
		}
		it.writer.Warn(fmt.Sprintf("Interrogation A: <failed: %v>", err))
		// Still log the episode with an empty answer so the anomaly is recorded.
	} else {
		answerForScreen := answer
		if answerForScreen == "" {
			answerForScreen = "<empty response>"
		}
		it.writer.Info("Interrogation A:\n" + answerForScreen)
	}

	it.log(ep, question, answer, state)
}

// ask sends history plus the diagnostic question to the model and returns the model's
// textual explanation. It works on a copy of history and sends no tools (so the model
// answers in prose). The history is sanitized so a trailing assistant message with
// unanswered tool_calls doesn't make strict chat templates reject the follow-up turn.
func (it *Interrogator) ask(ctx context.Context, history []llm.Message, question string) (string, error) {
	askCtx, cancel := context.WithTimeout(ctx, interrogateTimeout)
	defer cancel()

	msgs := sanitizeForFollowup(history)
	msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: question})

	// No Tools and no ToolChoice: a plain chat completion is maximally compatible across
	// OpenAI-style servers and the model has nothing to call, so it must answer in prose.
	resp, err := it.client.Chat(askCtx, llm.ChatRequest{
		Model:       it.cfg.LLM.Model,
		Messages:    msgs,
		Temperature: it.cfg.LLM.Temperature,
		MaxTokens:   it.cfg.LLM.MaxTokens,
		Stream:      false,
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in interrogation response")
	}
	msg := resp.Choices[0].Message
	answer := msg.Content
	if answer == "" {
		answer = msg.ReasoningContent
	}
	return strings.TrimSpace(answer), nil
}

// interrogationRecord is one JSON-lines record per anomaly episode.
type interrogationRecord struct {
	TS             string   `json:"ts"`
	Model          string   `json:"model"`
	RunID          string   `json:"run_id"`
	Trigger        string   `json:"trigger"`
	TriggerCount   int      `json:"trigger_count"`
	OffendingCall  any      `json:"offending_call,omitempty"`
	PriorResult    string   `json:"prior_result,omitempty"`
	PendingDiff    string   `json:"pending_diff,omitempty"`
	HistoryExcerpt []string `json:"history_excerpt,omitempty"`
	Question       string   `json:"question"`
	Answer         string   `json:"answer"`
	ContextTokens  int      `json:"context_tokens"`
	Turn           int      `json:"turn"`
}

// log appends a self-contained JSON-lines record for the episode to the side channel.
func (it *Interrogator) log(ep Episode, question, answer string, state *runState) {
	dir := it.cfg.Diagnostics.GetInterrogateLogDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		if it.logger != nil {
			it.logger.Error("interrogation log mkdir failed", err)
		}
		return
	}

	var offending any
	if ep.OffendingTool != "" {
		var args any
		if json.Unmarshal([]byte(ep.OffendingArgs), &args) != nil {
			args = ep.OffendingArgs
		}
		offending = map[string]any{"tool": ep.OffendingTool, "args": args}
	}

	rec := interrogationRecord{
		TS:             time.Now().UTC().Format(time.RFC3339),
		Model:          it.cfg.LLM.Model,
		RunID:          it.runID,
		Trigger:        ep.Trigger,
		TriggerCount:   ep.TriggerCount,
		OffendingCall:  offending,
		PriorResult:    it.redact(ep.PriorResult),
		PendingDiff:    it.redact(ep.PendingDiff),
		HistoryExcerpt: it.historyExcerpt(state.messages),
		Question:       it.redact(question),
		Answer:         it.redact(answer),
		ContextTokens:  state.totalTokens,
		Turn:           state.agentStats.Steps,
	}

	line, err := json.Marshal(&rec)
	if err != nil {
		return
	}

	path := filepath.Join(dir, it.runID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		if it.logger != nil {
			it.logger.Error("interrogation log open failed", err)
		}
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

// historyExcerpt returns the last few messages, redacted, as compact strings.
func (it *Interrogator) historyExcerpt(messages []llm.Message) []string {
	start := len(messages) - interrogateHistoryExcerpt
	if start < 0 {
		start = 0
	}
	out := make([]string, 0, len(messages)-start)
	for _, m := range messages[start:] {
		content := m.Content
		if content == "" && len(m.ToolCalls) > 0 {
			var calls []string
			for _, tc := range m.ToolCalls {
				calls = append(calls, tc.Function.Name+" "+tc.Function.Arguments)
			}
			content = "[tool_calls] " + strings.Join(calls, "; ")
		}
		out = append(out, fmt.Sprintf("%s: %s", m.Role, it.redact(content)))
	}
	return out
}

// redact applies secret redaction when audit redaction is configured.
func (it *Interrogator) redact(s string) string {
	if it.redactor == nil || s == "" {
		return s
	}
	return it.redactor.Redact(s)
}

// sanitizeForFollowup returns a copy of history that is safe to append a user message to.
// Anomalies are often detected mid-execution - before the offending call's tool result is
// recorded - so history can end with an assistant message whose tool_calls have no matching
// tool responses. Strict chat templates reject a following user turn in that state
// ("tool_calls must be followed by tool messages"), which would fail every interrogation.
// We append a synthetic tool response for each dangling tool_call to keep the request valid.
func sanitizeForFollowup(history []llm.Message) []llm.Message {
	msgs := make([]llm.Message, len(history))
	copy(msgs, history)
	if len(msgs) == 0 {
		return msgs
	}
	last := msgs[len(msgs)-1]
	if last.Role != llm.RoleAssistant || len(last.ToolCalls) == 0 {
		return msgs
	}
	for _, tc := range last.ToolCalls {
		msgs = append(msgs, llm.Message{
			Role:       llm.RoleTool,
			Name:       tc.Function.Name,
			ToolCallID: tc.ID,
			Content:    "[diagnostic interrogation: tool result omitted]",
		})
	}
	return msgs
}

// lastToolResult returns the content of the most recent tool-role message, or empty.
func lastToolResult(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llm.RoleTool {
			return messages[i].Content
		}
	}
	return ""
}

// buildInterrogationQuestion renders a templated, trigger-specific diagnostic question.
// Every template states the answer is for diagnostics and will not be graded, to reduce
// the model performing rather than reporting.
func buildInterrogationQuestion(ep Episode) string {
	const diagnosticNote = "\n\nThis question is for diagnostics only. Your answer will NOT be graded and will NOT change the task. Answer concisely and honestly."

	switch ep.Trigger {
	case TriggerPendingBlocked:
		diff := ep.PendingDiff
		if diff == "" {
			diff = "(diff unavailable)"
		}
		return fmt.Sprintf("An edit is pending with this diff:\n%s\n\n"+
			"You are required to call `Edit.confirm` or `Edit.cancel` and nothing else. "+
			"You instead called `%s`. Explain why - did you not see the pending state, "+
			"did you expect the edit was already applied, or something else?%s",
			diff, ep.OffendingTool, diagnosticNote)

	case TriggerAlternatingLoop:
		return fmt.Sprintf("You appear to be stuck in an alternating loop involving `%s`: "+
			"issuing the same action and then cancelling/undoing it, repeatedly. "+
			"Explain: (1) what you believe the current file/tool state is, (2) why the cycle "+
			"keeps repeating, (3) what different action would actually make progress.%s",
			ep.OffendingTool, diagnosticNote)

	default: // duplicate_call, fatal, backtrack - all "same call again" shaped
		prior := ep.PriorResult
		if prior == "" {
			prior = "(previous result unavailable)"
		}
		return fmt.Sprintf("You just issued this tool call again with identical arguments:\n`%s %s`\n\n"+
			"The previous result was:\n%s\n\n"+
			"Explain: (1) why you expected a different outcome, (2) what you believe the current "+
			"file/tool state is, (3) what you think you must do next to make progress.%s",
			ep.OffendingTool, ep.OffendingArgs, prior, diagnosticNote)
	}
}
