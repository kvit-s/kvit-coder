package compaction

import (
	"encoding/json"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
)

// charsPerToken is the estimate used wherever the server's own count is not
// available. Code and JSON tokenize at about three characters per token, so
// three errs toward a request being larger than it is, which is the safe side
// for a threshold that keeps a request inside the window.
const charsPerToken = 3

// messageOverhead is what each message costs beyond its text: the role and
// the separators around it.
const messageOverhead = 4

// EstimateTokens estimates what msgs cost as prompt, from their text, tool
// calls and pictures. Encrypted thinking is not counted: it never leaves the
// process that received it, and the server's count covers it wherever it is
// sent.
func EstimateTokens(msgs []llm.Message) int {
	chars := 0
	for _, m := range msgs {
		chars += len(m.Content) + len(m.ReasoningContent)
		for _, tc := range m.ToolCalls {
			chars += len(tc.Function.Name) + len(tc.Function.Arguments)
		}
	}
	return chars/charsPerToken + messageOverhead*len(msgs) + llm.ImageTokensForMessages(msgs)
}

// EstimateSpecs estimates what tool specs cost as prompt. They go with every
// request but are not messages, so EstimateTokens does not see them.
func EstimateSpecs(specs []llm.ToolSpec) int {
	if len(specs) == 0 {
		return 0
	}
	raw, err := json.Marshal(specs)
	if err != nil {
		return 0
	}
	return len(raw) / charsPerToken
}

// StartEstimate estimates, at the start of a turn, the size of the request
// messages would make, and how much the turn in progress has already added.
//
// The request size starts from the server's count recorded on the last
// assistant message, when that count still describes this context: it came
// after the last compaction, and from the same model (a different model counts
// with a different tokenizer). What came after that message is estimated. When
// there is no such count, all of messages is estimated, plus overhead for what
// a request carries besides messages (the tool specs).
//
// The turn in progress includes the turns before this one that ended without
// an answer; what they added is estimated from their messages.
func StartEstimate(tl *session.Timeline, messages []llm.Message, overhead int, sameModel func(recorded string) bool) (prompt, turn int) {
	prompt = EstimateTokens(messages) + overhead
	floor := 0
	if c := tl.LatestCompaction(); c != nil {
		floor = c.At
	}
	for p := len(tl.Messages) - 1; p >= floor; p-- {
		m := tl.Messages[p]
		if m.Role != llm.RoleAssistant || m.PromptTokens <= 0 {
			continue
		}
		if sameModel != nil && !sameModel(modelAt(tl, p)) {
			break
		}
		prompt = m.PromptTokens + EstimateTokens(tl.Messages[p:])
		break
	}
	turn = EstimateTokens(tl.Messages[tl.InProgressFrom():])
	return prompt, turn
}

// modelAt is the model recorded for the turn that position p belongs to.
func modelAt(tl *session.Timeline, p int) string {
	model := ""
	for _, t := range tl.Turns {
		if t.At > p {
			break
		}
		model = t.Model
	}
	return model
}
