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

	// ReasoningBlocks holds the opaque thinking blocks a Responses-API model
	// returned with this assistant turn. The model expects them back verbatim
	// on the next request of the same tool loop, so they ride along in the
	// history but are never sent on the wire by the chat-completions path.
	ReasoningBlocks []ReasoningBlock `json:"-"`
	// ToolCallItemIDs maps a tool call ID to the Responses-API item ID the
	// model gave that call, so a replayed function_call keeps its identity.
	ToolCallItemIDs map[string]string `json:"-"`
	// Images are pictures attached to the message. The pixels live on disk
	// (the session's tmp/ once normalized) and Path points at the copy;
	// bytes are loaded into Data by HydrateImages before a request is built,
	// so history stores references, never base64. Images ride on user
	// messages; other roles ignore them.
	Images []ImagePart `json:"images,omitempty"`
}

// ReasoningBlock is one "reasoning" item from a Responses-API answer. The
// content is encrypted by the provider: kvit-coder cannot read it, it only
// stores it and hands it back.
type ReasoningBlock struct {
	ID               string `json:"id,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty"`
	Summary          []any  `json:"summary,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction is the function half of a tool call: which tool, and the
// arguments as the JSON string the model produced.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type ChatRequest struct {
	Model       string     `json:"model"`
	Messages    []Message  `json:"messages"`
	Temperature float32    `json:"temperature,omitempty"`
	MaxTokens   int        `json:"max_tokens,omitempty"`
	Tools       []ToolSpec `json:"tools,omitempty"`
	ToolChoice  string     `json:"tool_choice,omitempty"`
	Stream      bool       `json:"stream,omitempty"`
	// ChatTemplateKwargs passes extra args to the server's chat template (llama.cpp /
	// vLLM style), e.g. {"enable_thinking": false} to suppress reasoning for a call.
	// Omitted from the body when nil; ignored by templates that don't use the key.
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
	// ReasoningEffort is OpenAI's own top-level effort field, which hosted
	// gateways read instead of ChatTemplateKwargs. Which of the two a client
	// fills is WithEffortField; see the EffortField constants.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// ChoiceError represents an error returned in a choice (e.g., upstream provider errors)
type ChoiceError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice is one candidate answer. Every endpoint kvit-coder talks to returns
// exactly one, so the loop only ever reads Choices[0].
type Choice struct {
	Index        int          `json:"index"`
	Message      Message      `json:"message"`
	FinishReason string       `json:"finish_reason"`
	Error        *ChoiceError `json:"error,omitempty"`
}

// Usage is the token accounting the endpoint reports for one request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ToolSpec struct {
	Type     string           `json:"type"` // always "function"
	Function ToolSpecFunction `json:"function"`
}

// ToolSpecFunction is the schema half of a tool spec, as the API expects it.
type ToolSpecFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
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
