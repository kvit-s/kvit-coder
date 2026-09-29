package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// BackendMessages is the Anthropic Messages API (POST /v1/messages).
const BackendMessages = "messages"

// messagesDefaultMaxTokens is sent when the caller did not set a limit.
// The messages API rejects a request that omits max_tokens.
const messagesDefaultMaxTokens = 16384

// anthropicThinkingID marks a ReasoningBlock that is a Claude thinking block
// to be replayed, not a Responses-API encrypted item. The signature has to
// go back with the thinking text on the next request of the same tool loop,
// or Claude rejects the call.
const anthropicThinkingID = "anthropic-thinking"

// anthropicRedactedID is the same for a redacted thinking block, whose
// payload is opaque and has to be replayed unchanged.
const anthropicRedactedID = "anthropic-redacted"

func (c *Client) chatViaMessages(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(c.buildMessagesRequest(req))
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	respBody, _, err := c.postJSON(ctx, "/v1/messages", body)
	if err != nil {
		return nil, err
	}
	var resp messagesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w (body preview: %s)", err, bodyPreview(respBody))
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return nil, fmt.Errorf("API error: %s", resp.Error.Message)
	}
	return messagesToChat(&resp), nil
}

type messagesRequest struct {
	Model        string          `json:"model"`
	MaxTokens    int             `json:"max_tokens"`
	System       string          `json:"system,omitempty"`
	Messages     []messagesOut   `json:"messages"`
	Tools        []messagesTool  `json:"tools,omitempty"`
	ToolChoice   *messagesChoice `json:"tool_choice,omitempty"`
	Temperature  *float32        `json:"temperature,omitempty"`
	Thinking     *messagesThink  `json:"thinking,omitempty"`
	OutputConfig *messagesEffort `json:"output_config,omitempty"`
	Stream       bool            `json:"stream"`
}

type messagesOut struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type messagesTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type messagesChoice struct {
	Type string `json:"type"`
}

type messagesThink struct {
	Type    string `json:"type"`
	Display string `json:"display,omitempty"`
}

type messagesEffort struct {
	Effort string `json:"effort"`
}

type messagesBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	Source    *messagesImage  `json:"source,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Signature string          `json:"signature,omitempty"`
	Data      string          `json:"data,omitempty"`
}

type messagesImage struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type messagesResponse struct {
	Content []struct {
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		Thinking  string          `json:"thinking"`
		Signature string          `json:"signature"`
		Data      string          `json:"data"`
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Input     json.RawMessage `json:"input"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) buildMessagesRequest(req ChatRequest) messagesRequest {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = messagesDefaultMaxTokens
	}
	out := messagesRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		System:    systemText(req.Messages),
		Messages:  messagesHistory(req.Messages),
		Tools:     messagesTools(req.Tools),
		Stream:    false,
	}
	if choice := messagesToolChoice(req.ToolChoice, len(out.Tools) > 0); choice != nil {
		out.ToolChoice = choice
	}
	// Adaptive thinking and a temperature are not accepted together, and a
	// title call that only wants a few words leaves the effort empty so it
	// does not pay for thinking.
	if effort := c.reasoningEffort; effort != "" && !strings.EqualFold(effort, "none") {
		out.Thinking = &messagesThink{Type: "adaptive", Display: "summarized"}
		out.OutputConfig = &messagesEffort{Effort: effort}
	} else if req.Temperature != 0 {
		t := req.Temperature
		out.Temperature = &t
	}
	return out
}

func systemText(msgs []Message) string {
	var parts []string
	for _, m := range msgs {
		if m.Role == RoleSystem && strings.TrimSpace(m.Content) != "" {
			parts = append(parts, m.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}

// messagesHistory converts a chat-completions history into the alternating
// user/assistant list the messages API requires. Tool results are user
// messages, so a tool result followed by the next user prompt is one user
// message: two user messages in a row are rejected.
func messagesHistory(msgs []Message) []messagesOut {
	var out []messagesOut
	for _, m := range msgs {
		var next *messagesOut
		switch m.Role {
		case RoleSystem:
			continue
		case RoleTool:
			block := messagesBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}
			if block.Content == "" {
				block.Content = " "
			}
			next = &messagesOut{Role: "user", Content: []messagesBlock{block}}
		case RoleAssistant:
			blocks := append(replayThinking(m), assistantBlocks(m)...)
			if len(blocks) == 0 {
				continue
			}
			next = &messagesOut{Role: "assistant", Content: encodeBlocks(blocks)}
		default:
			blocks := userBlocks(m)
			if len(blocks) == 0 {
				continue
			}
			next = &messagesOut{Role: "user", Content: encodeBlocks(blocks)}
		}
		if len(out) > 0 && out[len(out)-1].Role == next.Role {
			out[len(out)-1].Content = mergeContent(out[len(out)-1].Content, next.Content)
			continue
		}
		out = append(out, *next)
	}
	return out
}

func replayThinking(m Message) []messagesBlock {
	var blocks []messagesBlock
	for _, b := range m.ReasoningBlocks {
		switch b.ID {
		case anthropicThinkingID:
			text := ""
			if len(b.Summary) > 0 {
				text, _ = b.Summary[0].(string)
			}
			blocks = append(blocks, messagesBlock{Type: "thinking", Thinking: text, Signature: b.EncryptedContent})
		case anthropicRedactedID:
			blocks = append(blocks, messagesBlock{Type: "redacted_thinking", Data: b.EncryptedContent})
		}
	}
	return blocks
}

func assistantBlocks(m Message) []messagesBlock {
	var blocks []messagesBlock
	if m.Content != "" {
		blocks = append(blocks, messagesBlock{Type: "text", Text: m.Content})
	}
	for i, call := range m.ToolCalls {
		id := call.ID
		if id == "" {
			id = fmt.Sprintf("toolu_%d", i)
		}
		blocks = append(blocks, messagesBlock{
			Type:  "tool_use",
			ID:    id,
			Name:  call.Function.Name,
			Input: toolInput(call.Function.Arguments),
		})
	}
	return blocks
}

func userBlocks(m Message) []messagesBlock {
	var blocks []messagesBlock
	if m.Content != "" {
		blocks = append(blocks, messagesBlock{Type: "text", Text: m.Content})
	}
	for _, img := range imagesWithData(m.Images) {
		blocks = append(blocks, messagesBlock{
			Type: "image",
			Source: &messagesImage{
				Type:      "base64",
				MediaType: img.MediaType,
				Data:      base64.StdEncoding.EncodeToString(img.Data),
			},
		})
	}
	return blocks
}

func encodeBlocks(blocks []messagesBlock) any {
	if len(blocks) == 1 && blocks[0].Type == "text" {
		return blocks[0].Text
	}
	return blocks
}

func mergeContent(a, b any) any {
	return append(asBlocks(a), asBlocks(b)...)
}

func asBlocks(v any) []messagesBlock {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return []messagesBlock{{Type: "text", Text: t}}
	case []messagesBlock:
		return t
	default:
		return nil
	}
}

func toolInput(arguments string) json.RawMessage {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return json.RawMessage(`{}`)
	}
	if json.Valid([]byte(arguments)) {
		return json.RawMessage(arguments)
	}
	b, _ := json.Marshal(map[string]string{"arguments": arguments})
	return b
}

func messagesTools(tools []ToolSpec) []messagesTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]messagesTool, 0, len(tools))
	for _, tool := range tools {
		schema := tool.Function.Parameters
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, messagesTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: schema,
		})
	}
	return out
}

func messagesToolChoice(choice string, hasTools bool) *messagesChoice {
	if !hasTools {
		return nil
	}
	switch strings.ToLower(choice) {
	case "", "auto":
		return &messagesChoice{Type: "auto"}
	case "none":
		return &messagesChoice{Type: "none"}
	case "required", "any":
		return &messagesChoice{Type: "any"}
	default:
		return &messagesChoice{Type: "auto"}
	}
}

func messagesToChat(resp *messagesResponse) *ChatResponse {
	msg := Message{Role: RoleAssistant}
	var text []string
	var thinking []string
	for _, part := range resp.Content {
		switch part.Type {
		case "text":
			text = append(text, part.Text)
		case "thinking":
			thinking = append(thinking, part.Thinking)
			msg.ReasoningBlocks = append(msg.ReasoningBlocks, ReasoningBlock{
				ID:               anthropicThinkingID,
				EncryptedContent: part.Signature,
				Summary:          []any{part.Thinking},
			})
		case "redacted_thinking":
			msg.ReasoningBlocks = append(msg.ReasoningBlocks, ReasoningBlock{
				ID:               anthropicRedactedID,
				EncryptedContent: part.Data,
			})
		case "tool_use":
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:   part.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      part.Name,
					Arguments: string(part.Input),
				},
			})
		}
	}
	msg.Content = strings.Join(text, "")
	msg.ReasoningContent = strings.Join(thinking, "\n")
	finish := resp.StopReason
	switch resp.StopReason {
	case "end_turn", "stop_sequence":
		finish = "stop"
	case "tool_use":
		finish = "tool_calls"
	case "max_tokens":
		finish = "length"
	case "refusal":
		finish = "content_filter"
	}
	return &ChatResponse{
		Choices: []Choice{{
			Message:      msg,
			FinishReason: finish,
		}},
		Usage: Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}
