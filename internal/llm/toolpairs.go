package llm

import (
	"errors"
	"net/http"
	"strings"
)

// StoppedCallNote is the result given to a tool call that has none. A turn
// writes the model's tool calls to the session before it runs them and writes
// each result after, so a turn whose process stops in between (a crash, a
// killed process, a power cut) leaves calls with no result. Every API this
// program speaks refuses a history like that: OpenAI's Responses API answers
// "No tool output found for function call <id>", and so does every later
// request in the session. The note is what the model reads in place of the
// output; it says the tool may have run in part, because a process killed
// while a tool ran leaves the same record as one that stopped before it.
const StoppedCallNote = "Not run: kvit-coder stopped before this tool call returned, so the tool did not run or did not finish, and none of its output was kept. Call it again if it is still needed."

// StoppedCallResult is the StoppedCallNote result for tc.
func StoppedCallResult(tc ToolCall) Message {
	return Message{Role: RoleTool, Name: tc.Function.Name, ToolCallID: tc.ID, Content: StoppedCallNote}
}

// LastCallsUnanswered is a StoppedCallNote result for each tool call of the
// conversation's last assistant message that has no result, when only tool
// results follow that message; otherwise nil. Appending them to the
// conversation answers every call, in the order the model made them.
func LastCallsUnanswered(msgs []Message) []Message {
	i := len(msgs) - 1
	for i >= 0 && msgs[i].Role == RoleTool {
		i--
	}
	if i < 0 || msgs[i].Role != RoleAssistant || len(msgs[i].ToolCalls) == 0 {
		return nil
	}
	return unanswered(msgs[i], answeredIDs(msgs[i+1:]))
}

// AnswerUnansweredCalls returns msgs with a StoppedCallNote result after each
// tool call that no tool message answers, placed after the tool results that
// directly follow the call's assistant message. It returns msgs itself when
// every call is answered, which is every request a turn builds.
//
// A session repaired when it is opened (Session.AnswerStoppedCalls) only ever
// needs this for calls followed by later messages, which the history file
// being append-only cannot put a result after: a session that went on, and was
// refused, after a crash left a call unanswered. Doing it as the request is
// built leaves the conversation and the positions compaction counts in it as
// they are, and inserts the same text at the same place in every request, so
// the prompt cache still matches.
func AnswerUnansweredCalls(msgs []Message) []Message {
	answered := answeredIDs(msgs)
	var out []Message
	copied := 0 // msgs[:copied] is in out
	for i := 0; i < len(msgs); i++ {
		if msgs[i].Role != RoleAssistant || len(msgs[i].ToolCalls) == 0 {
			continue
		}
		end := i + 1
		for end < len(msgs) && msgs[end].Role == RoleTool {
			end++
		}
		if missing := unanswered(msgs[i], answered); len(missing) > 0 {
			out = append(append(out, msgs[copied:end]...), missing...)
			copied = end
		}
		i = end - 1
	}
	if out == nil {
		return msgs
	}
	return append(out, msgs[copied:]...)
}

// unanswered is a StoppedCallNote result for each of call's tool calls whose id
// is not in answered, and records those ids there, so an id the model repeated
// gets one result. A call without an id cannot be answered and is left alone.
func unanswered(call Message, answered map[string]bool) []Message {
	var out []Message
	for _, tc := range call.ToolCalls {
		if tc.ID == "" || answered[tc.ID] {
			continue
		}
		answered[tc.ID] = true
		out = append(out, StoppedCallResult(tc))
	}
	return out
}

func answeredIDs(msgs []Message) map[string]bool {
	ids := make(map[string]bool)
	for _, m := range msgs {
		if m.Role == RoleTool && m.ToolCallID != "" {
			ids[m.ToolCallID] = true
		}
	}
	return ids
}

// unpairedPhrases are the ways endpoints refuse a history whose tool calls
// and tool results do not pair up. OpenAI's Responses API says "No tool output
// found for function call" and, the other way round, "No tool call found for
// function call output"; chat completions says a message with tool_calls "must
// be followed by tool messages responding to each 'tool_call_id'", or that a
// tool message "must be a response to a preceeding message with
// 'tool_calls'" (their spelling); Anthropic says "tool_use ids were found
// without tool_result blocks" or "unexpected tool_use_id".
var unpairedPhrases = []string{
	"no tool output found",
	"no tool call found",
	"must be followed by tool messages",
	"must be a response to a preceeding message",
	"must be a response to a preceding message",
	"did not have response messages",
	"without tool_result",
	"unexpected tool_use_id",
}

// IsUnpairedToolCall reports whether err is the endpoint refusing a request
// because a tool call in it has no result, or a result has no call. Sending
// the same history again is refused the same way, so the error is not one to
// retry.
func IsUnpairedToolCall(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	var status *StatusError
	if errors.As(err, &status) {
		if status.Code != http.StatusBadRequest {
			return false
		}
		text = string(status.Body)
	}
	// Anthropic quotes the field names: "`tool_use` ids were found without
	// `tool_result` blocks".
	text = strings.ToLower(strings.ReplaceAll(text, "`", ""))
	for _, phrase := range unpairedPhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}
