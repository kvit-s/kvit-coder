package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// gptMajor matches the generation in a Copilot GPT model id (gpt-5.4, gpt-5-mini).
var gptMajor = regexp.MustCompile(`^gpt-(\d+)`)

// ListedModel is one model the signed-in Copilot account is allowed to call.
type ListedModel struct {
	ID        string
	Name      string
	Endpoints []string
	Protocol  string
	Context   int
	Efforts   []string
	Disabled  bool
}

type remoteModels struct {
	Data []remoteModel `json:"data"`
}

type remoteModel struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	SupportedEndpoints []string `json:"supported_endpoints"`
	ModelPickerEnabled *bool    `json:"model_picker_enabled"`
	Policy             struct {
		State string `json:"state"`
	} `json:"policy"`
	Capabilities struct {
		Limits struct {
			MaxContext int `json:"max_context_window_tokens"`
			MaxPrompt  int `json:"max_prompt_tokens"`
			MaxOutput  int `json:"max_output_tokens"`
		} `json:"limits"`
		Supports struct {
			ReasoningEffort []string `json:"reasoning_effort"`
			ToolCalls       *bool    `json:"tool_calls"`
		} `json:"supports"`
	} `json:"capabilities"`
}

func (m remoteModel) listed() ListedModel {
	protocol, known := protocolFor(m.ID, m.SupportedEndpoints)
	if !known {
		protocol = DefaultBackend(m.ID)
	}
	ctxTokens := m.Capabilities.Limits.MaxContext
	if ctxTokens == 0 {
		ctxTokens = m.Capabilities.Limits.MaxPrompt
	}
	return ListedModel{
		ID:        m.ID,
		Name:      m.Name,
		Endpoints: m.SupportedEndpoints,
		Protocol:  protocol,
		Context:   ctxTokens,
		Efforts:   m.Capabilities.Supports.ReasoningEffort,
		Disabled:  m.Policy.State == "disabled",
	}
}

// protocolFor chooses the wire protocol a Copilot model is actually served
// on. Claude on Copilot is the Anthropic messages API; GPT-5, Grok and
// Gemini are the Responses API; the rest are chat completions. An explicit
// supported_endpoints list wins over the name, except that a model which
// advertises the messages API is always called that way — that is how
// Copilot serves Claude, and chat completions then answers that the model
// is not supported.
func protocolFor(model string, endpoints []string) (string, bool) {
	hasMsg, hasResp, hasChat := false, false, false
	for _, e := range endpoints {
		switch strings.TrimSpace(e) {
		case "/v1/messages", "messages":
			hasMsg = true
		case "/responses", "responses":
			hasResp = true
		case "/chat/completions", "chat/completions":
			hasChat = true
		}
	}
	switch {
	case hasMsg:
		return llm.BackendMessages, true
	case hasResp && !hasChat:
		return llm.BackendResponses, true
	case hasChat && !hasResp:
		return llm.BackendChatCompletions, true
	case hasResp && hasChat:
		return DefaultBackend(model), true
	case len(endpoints) > 0:
		return DefaultBackend(model), true
	default:
		return "", false
	}
}

// DefaultBackend is the protocol used when the account's model list does not
// say, chosen from the model id the way Copilot routes it.
func DefaultBackend(model string) string {
	id := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(id, "claude"):
		return llm.BackendMessages
	case strings.HasPrefix(id, "grok-"), strings.HasPrefix(id, "gemini-"), strings.HasPrefix(id, "mai-code-"):
		return llm.BackendResponses
	case strings.HasPrefix(id, "gpt-5-mini"):
		return llm.BackendChatCompletions
	default:
		if m := gptMajor.FindStringSubmatch(id); len(m) == 2 {
			n, _ := strconv.Atoi(m[1])
			if n >= 5 {
				return llm.BackendResponses
			}
		}
		return llm.BackendChatCompletions
	}
}

func (g *gate) fetchModels(ctx context.Context, apiBase, bearer string) ([]remoteModel, error) {
	apiBase = strings.TrimRight(apiBase, "/")
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+bearer)
	header.Set("Accept", "application/json")
	header.Set("User-Agent", userAgent)
	header.Set("X-GitHub-Api-Version", chatAPIVersion)
	status, body, err := g.get(ctx, apiBase+"/models", header)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, &httpStatus{Status: status, URL: apiBase + "/models", Body: trimBody(body)}
	}
	var parsed remoteModels
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode Copilot model list: %w", err)
	}
	return parsed.Data, nil
}

type httpStatus struct {
	Status int
	URL    string
	Body   string
}

func (e *httpStatus) Error() string {
	return fmt.Sprintf("GitHub Copilot request to %s returned %d: %s", e.URL, e.Status, e.Body)
}

func authStatus(err error) bool {
	hs, ok := err.(*httpStatus)
	return ok && (hs.Status == http.StatusUnauthorized || hs.Status == http.StatusForbidden)
}

func findRemote(models []remoteModel, id string) (remoteModel, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return remoteModel{}, false
}

// IsEmbeddingModel reports whether a catalog model is an embedding model.
// Those have no chat endpoint, so they are useless as agent models. GitHub
// serves them as text-embedding-*, with context 0 and no chat protocol.
func IsEmbeddingModel(m ListedModel) bool {
	if strings.Contains(strings.ToLower(m.ID), "embed") {
		return true
	}
	if strings.Contains(strings.ToLower(m.Name), "embed") {
		return true
	}
	for _, e := range m.Endpoints {
		if strings.Contains(strings.ToLower(e), "embed") {
			return true
		}
	}
	return false
}
