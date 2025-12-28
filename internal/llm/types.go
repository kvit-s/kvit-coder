package llm

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type Message struct {
	Role             MessageRole `json:"role"`
	Content          string      `json:"content,omitempty"`
	ReasoningContent string      `json:"reasoning_content,omitempty"` // For models that return thinking/reasoning
	Name             string      `json:"name,omitempty"`
	ToolCalls        []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID       string      `json:"tool_call_id,omitempty"` // For tool role messages
}

type ToolCall struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

type ChatRequest struct {
	Model       string     `json:"model"`
	Messages    []Message  `json:"messages"`
	Temperature float32    `json:"temperature,omitempty"`
	MaxTokens   int        `json:"max_tokens,omitempty"`
	Tools       []ToolSpec `json:"tools,omitempty"`
	ToolChoice  string     `json:"tool_choice,omitempty"`
	Stream      bool       `json:"stream,omitempty"`
}

// ChoiceError represents an error returned in a choice (e.g., upstream provider errors)
type ChoiceError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int          `json:"index"`
		Message      Message      `json:"message"`
		FinishReason string       `json:"finish_reason"`
		Error        *ChoiceError `json:"error,omitempty"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type ToolSpec struct {
	Type     string `json:"type"` // always "function"
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

// GenerationStats contains detailed generation statistics from the LLM server
type GenerationStats struct {
	Data struct {
		ID                     string  `json:"id"`
		CreatedAt              string  `json:"created_at"`
		Model                  string  `json:"model"`
		Streamed               bool    `json:"streamed"`
		FinishReason           string  `json:"finish_reason"`
		TokensPrompt           int     `json:"tokens_prompt"`
		TokensCompletion       int     `json:"tokens_completion"`
		NativeTokensPrompt     int     `json:"native_tokens_prompt"`
		NativeTokensCompletion int     `json:"native_tokens_completion"`
		NativeTokensCached     int     `json:"native_tokens_cached"`
		TotalCost              float64 `json:"total_cost"`
		CacheDiscount          float64 `json:"cache_discount"`
		ProviderName           string  `json:"provider_name"`
		InternalProvider       string  `json:"internal_provider"`
		// Timing fields (in milliseconds)
		Latency        float64 `json:"latency"`         // Time to process (native_tokens_prompt - native_tokens_cached)
		GenerationTime float64 `json:"generation_time"` // Time to generate native_tokens_completion
	} `json:"data"`
}
