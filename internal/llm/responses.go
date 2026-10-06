package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// This file lets the client talk to endpoints that serve a model only through
// OpenAI's Responses API (POST /responses) instead of chat completions. The
// rest of kvit-coder keeps working in chat-completions terms: a ChatRequest
// goes in, a ChatResponse comes out, and the translation happens here.
//
// The two protocols differ in three ways that matter:
//
//   - Messages become a flat list of items. A tool call and its result are
//     separate items ("function_call", "function_call_output") rather than an
//     assistant message with tool_calls plus a tool-role message.
//   - Tool schemas are flat: name/description/parameters sit on the tool
//     object itself instead of under a "function" key.
//   - A reasoning model returns its thinking as opaque encrypted blocks and
//     expects them handed back on the next request of the same tool loop.
//     Those blocks ride along on Message.ReasoningBlocks.

type responsesRequest struct {
	Model           string           `json:"model"`
	Input           []responsesItem  `json:"input"`
	Tools           []responsesTool  `json:"tools,omitempty"`
	ToolChoice      string           `json:"tool_choice,omitempty"`
	MaxOutputTokens int              `json:"max_output_tokens,omitempty"`
	Temperature     float32          `json:"temperature,omitempty"`
	Reasoning       *responsesEffort `json:"reasoning,omitempty"`
	Include         []string         `json:"include,omitempty"`
	Store           bool             `json:"store"`
	Stream          bool             `json:"stream"`
}

type responsesEffort struct {
	Effort string `json:"effort,omitempty"`
	// Summary asks the provider for readable text describing the model's
	// thinking. Without it the reasoning items come back with an empty summary
	// and only encrypted content, which kvit-coder can replay but nobody can
	// read — so merge_thinking has nothing to merge. Values are the
	// provider's: auto, concise, detailed.
	Summary string `json:"summary,omitempty"`
}

type responsesTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// responsesItem is one element of the input or output list. Which fields are
// meaningful depends on Type, so everything but Type is optional.
type responsesItem struct {
	Type             string `json:"type,omitempty"`
	ID               string `json:"id,omitempty"`
	Role             string `json:"role,omitempty"`
	Content          any    `json:"content,omitempty"`
	CallID           string `json:"call_id,omitempty"`
	Name             string `json:"name,omitempty"`
	Arguments        string `json:"arguments,omitempty"`
	Output           string `json:"output,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty"`
	// Summary is a pointer so a reasoning item can carry an explicitly empty
	// list: the provider rejects a reasoning item that has no summary field at
	// all, while every other item type must leave it out entirely.
	Summary *[]any `json:"summary,omitempty"`
	Status  string `json:"status,omitempty"`
}

type responsesContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesResponse struct {
	ID     string          `json:"id"`
	Model  string          `json:"model"`
	Status string          `json:"status"`
	Output []responsesItem `json:"output"`
	Usage  struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

// chatViaResponses answers a ChatRequest by calling /responses.
func (c *Client) chatViaResponses(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body, err := c.responsesRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, _, err := c.postJSON(ctx, "/responses", body)
	if err != nil {
		return nil, err
	}

	var resp responsesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w (body preview: %s)", err, bodyPreview(respBody))
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return nil, fmt.Errorf("API error: %s", resp.Error.Message)
	}

	return responsesToChat(&resp), nil
}

// responsesRequestBody is the /responses request for req, with its input
// items encoded as it is sent.
func (c *Client) responsesRequestBody(req ChatRequest) (*requestBody, error) {
	request := c.buildResponsesRequest(req)
	input := request.Input
	if len(request.Input) > 0 {
		request.Input = []responsesItem{}
	}
	return newRequestBody(request, "input", len(input), func(i int) any { return &input[i] })
}

func (c *Client) buildResponsesRequest(req ChatRequest) responsesRequest {
	out := responsesRequest{
		Model:           req.Model,
		Input:           messagesToResponsesInput(req.Messages),
		Tools:           toolsToResponsesTools(req.Tools),
		ToolChoice:      req.ToolChoice,
		MaxOutputTokens: req.MaxTokens,
		Temperature:     req.Temperature,
		// Nothing is kept server-side: kvit-coder sends the whole conversation
		// every turn, and asking for the encrypted thinking back is what makes
		// that replay possible for a reasoning model.
		Store:   false,
		Include: []string{"reasoning.encrypted_content"},
		Stream:  false,
	}
	if c.reasoningEffort != "" || c.reasoningSummary != "" {
		out.Reasoning = &responsesEffort{Effort: c.reasoningEffort, Summary: c.reasoningSummary}
	}
	return out
}

// messagesToResponsesInput flattens a chat-completions history into the item
// list /responses expects.
func messagesToResponsesInput(msgs []Message) []responsesItem {
	items := make([]responsesItem, 0, len(msgs)+2)

	for _, msg := range msgs {
		switch msg.Role {
		case RoleTool:
			items = append(items, responsesItem{
				Type:   "function_call_output",
				CallID: msg.ToolCallID,
				Output: msg.Content,
			})

		case RoleAssistant:
			// The model's own thinking goes back first, in the order it came.
			for _, block := range msg.ReasoningBlocks {
				summary := block.Summary
				if summary == nil {
					summary = []any{}
				}
				items = append(items, responsesItem{
					Type:             "reasoning",
					ID:               block.ID,
					EncryptedContent: block.EncryptedContent,
					Summary:          &summary,
				})
			}
			if msg.Content != "" {
				items = append(items, responsesItem{
					Type: "message",
					Role: string(RoleAssistant),
					Content: []responsesContentPart{{
						Type: "output_text",
						Text: msg.Content,
					}},
				})
			}
			for _, tc := range msg.ToolCalls {
				items = append(items, responsesItem{
					Type:      "function_call",
					ID:        msg.ToolCallItemIDs[tc.ID],
					CallID:    tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}

		default: // system, user, and anything else that is plain text
			// Images ride on user messages as input_text plus input_image
			// parts. Text-only messages keep the bare string, so the prompt
			// prefix stays byte-identical and the server-side cache hits.
			if len(imagesWithData(msg.Images)) == 0 {
				if msg.Content == "" {
					continue
				}
				items = append(items, responsesItem{
					Type:    "message",
					Role:    string(msg.Role),
					Content: msg.Content,
				})
				continue
			}
			items = append(items, responsesItem{
				Type:    "message",
				Role:    string(msg.Role),
				Content: toResponsesContent(msg),
			})
		}
	}

	return items
}

func toolsToResponsesTools(tools []ToolSpec) []responsesTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]responsesTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, responsesTool{
			Type:        "function",
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}
	return out
}

// responsesToChat folds a /responses answer back into the ChatResponse shape
// the agent loop understands.
func responsesToChat(resp *responsesResponse) *ChatResponse {
	msg := Message{Role: RoleAssistant}
	var text, thinking strings.Builder

	for _, item := range resp.Output {
		switch item.Type {
		case "reasoning":
			var summary []any
			if item.Summary != nil {
				summary = *item.Summary
			}
			msg.ReasoningBlocks = append(msg.ReasoningBlocks, ReasoningBlock{
				ID:               item.ID,
				EncryptedContent: item.EncryptedContent,
				Summary:          summary,
			})
			// A summary is the only readable part; encrypted blocks have none.
			for _, s := range summary {
				if part, ok := s.(map[string]any); ok {
					if t, ok := part["text"].(string); ok && t != "" {
						if thinking.Len() > 0 {
							thinking.WriteString("\n")
						}
						thinking.WriteString(t)
					}
				}
			}

		case "message":
			text.WriteString(extractOutputText(item.Content))

		case "function_call":
			tc := ToolCall{ID: item.CallID, Type: "function"}
			tc.Function.Name = item.Name
			tc.Function.Arguments = item.Arguments
			msg.ToolCalls = append(msg.ToolCalls, tc)
			if item.ID != "" {
				if msg.ToolCallItemIDs == nil {
					msg.ToolCallItemIDs = map[string]string{}
				}
				msg.ToolCallItemIDs[item.CallID] = item.ID
			}
		}
	}

	msg.Content = text.String()
	msg.ReasoningContent = thinking.String()

	finishReason := "stop"
	if len(msg.ToolCalls) > 0 {
		finishReason = "tool_calls"
	}
	if resp.IncompleteDetails != nil && resp.IncompleteDetails.Reason == "max_output_tokens" {
		finishReason = "length"
	}

	out := &ChatResponse{ID: resp.ID, Model: resp.Model}
	out.Choices = append(out.Choices, Choice{Message: msg, FinishReason: finishReason})
	out.Usage.PromptTokens = resp.Usage.InputTokens
	out.Usage.CompletionTokens = resp.Usage.OutputTokens
	out.Usage.TotalTokens = resp.Usage.TotalTokens
	return out
}

// extractOutputText pulls the text out of a message item's content, which is
// a list of typed parts but may also arrive as a bare string.
func extractOutputText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, part := range v {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t != "output_text" && t != "text" {
				continue
			}
			if text, ok := m["text"].(string); ok {
				b.WriteString(text)
			}
		}
		return b.String()
	}
	return ""
}
