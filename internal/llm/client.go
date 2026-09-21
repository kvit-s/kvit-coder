package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Backend names the wire protocol the endpoint speaks.
const (
	// BackendChatCompletions is OpenAI's /chat/completions, what almost every
	// local server and gateway offers. It is the default.
	BackendChatCompletions = "chat_completions"
	// BackendResponses is OpenAI's /responses. Some hosted models are served
	// only there and answer /chat/completions with an error.
	BackendResponses = "responses"
)

// EffortField names where a chat-completions endpoint reads the reasoning
// effort from. The two spellings are not interchangeable: an endpoint that
// wants one ignores the other silently, so the effort a person picked with
// :eN does nothing and nothing says so.
const (
	// EffortFieldChatTemplateKwargs puts the effort in
	// `chat_template_kwargs.reasoning_effort`, which is how a chat template
	// running on a local llama.cpp or vLLM server reads it (the Qwen template
	// does). This is the default, because that is where the setting started.
	EffortFieldChatTemplateKwargs = "chat_template_kwargs"
	// EffortFieldReasoningEffort puts it in the top-level `reasoning_effort`
	// field, which is OpenAI's own spelling and what hosted gateways expect.
	// opencode.ai's /chat/completions wants this one and drops
	// chat_template_kwargs on the floor.
	EffortFieldReasoningEffort = "reasoning_effort"
)

// ValidEffortField reports whether f names a known effort field (or is empty,
// which means the chat-template default).
func ValidEffortField(f string) bool {
	return f == "" || f == EffortFieldChatTemplateKwargs || f == EffortFieldReasoningEffort
}

// defaultRequestTimeout bounds one HTTP request end to end. Without it a
// hung endpoint hangs the agent with no way out short of killing the process.
// It has to cover a slow reasoning model's whole answer, so it is generous.
const defaultRequestTimeout = 10 * time.Minute

// defaultMaxRetries is how many times a failed request is tried again before
// the error is returned. The ladder below doubles the wait each time, so ten
// retries is several minutes of patience — right for the model call a turn
// cannot continue without, and wrong for anything optional.
const defaultMaxRetries = 10

type Client struct {
	baseURL string
	apiKey  string
	// apiKeyEnv is the name of the environment variable the key was read
	// from, kept only so a rejected request can say which one to set.
	apiKeyEnv string
	client    *http.Client

	backend          string
	headers          map[string]string
	reasoningEffort  string
	effortField      string
	reasoningSummary string
	// onRetry, when set, is told that a request failed and is about to be
	// tried again. Without it a failing endpoint is indistinguishable from a
	// slow one: the caller's progress indicator ticks on either way, and a
	// request that times out and retries silently can hold a turn for the
	// timeout times the retry count.
	onRetry func(attempt, maxAttempts int, delay time.Duration, reason error)
	// maxRetries is how many further attempts a failed request gets. See
	// WithMaxRetries.
	maxRetries int
}

// WithMaxRetries caps how many times a failed request is tried again, for a
// caller that would rather fall back than wait. Zero means one attempt and no
// retry; a negative value leaves the default of defaultMaxRetries.
//
// The retry ladder waits 1s, 2s, 4s, 8s, 16s and so on between attempts, and
// an endpoint that refuses instantly — a 502 from a local server with no model
// loaded, say — spends all of that waiting. A request whose answer is optional
// should not: it holds up whatever is waiting on it and there is nothing on
// screen to say why.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

// WithRetryNotice installs a callback told about each failed attempt, so a
// caller can say so rather than leaving a stalled turn looking like a slow one.
func WithRetryNotice(fn func(attempt, maxAttempts int, delay time.Duration, reason error)) Option {
	return func(c *Client) { c.onRetry = fn }
}

// Option adjusts a Client at construction time.
type Option func(*Client)

// WithAPIKeyEnv records which environment variable the key came from, so a
// 401 can name it. An endpoint that needs no key leaves this empty.
func WithAPIKeyEnv(name string) Option {
	return func(c *Client) {
		c.apiKeyEnv = name
	}
}

// WithBackend selects the wire protocol. An empty or unknown value leaves the
// client on chat completions.
func WithBackend(backend string) Option {
	return func(c *Client) {
		if backend == BackendResponses {
			c.backend = BackendResponses
		}
	}
}

// WithHeaders adds request headers sent with every call, on top of
// Content-Type and Authorization. Use it for endpoints that demand something
// extra, such as a routing or session header.
func WithHeaders(headers map[string]string) Option {
	return func(c *Client) {
		for k, v := range headers {
			if c.headers == nil {
				c.headers = map[string]string{}
			}
			c.headers[k] = v
		}
	}
}

// WithTimeout bounds one HTTP request end to end, overriding the default of
// ten minutes. A non-positive value leaves the default in place.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.client.Timeout = d
		}
	}
}

// WithReasoningEffort sets how much thinking a reasoning model should do.
// The Responses backend sends it as `reasoning.effort`; the chat-completions
// backend sends it wherever WithEffortField points. Empty means "server
// default" on both.
func WithReasoningEffort(effort string) Option {
	return func(c *Client) { c.reasoningEffort = effort }
}

// WithEffortField picks which field of a chat-completions request carries the
// reasoning effort: EffortFieldChatTemplateKwargs (the default, for a model
// whose chat template reads it) or EffortFieldReasoningEffort (OpenAI's own
// top-level field, which hosted gateways want). An empty or unknown value
// leaves the default. The Responses backend ignores this.
func WithEffortField(field string) Option {
	return func(c *Client) {
		if field == EffortFieldReasoningEffort || field == EffortFieldChatTemplateKwargs {
			c.effortField = field
		}
	}
}

// WithReasoningSummary asks a reasoning model for readable text describing its
// thinking, which is the only part of its reasoning anyone can read: the rest
// comes back encrypted, to be replayed rather than understood. Only the
// Responses backend sends it; the accepted values are the provider's (commonly
// auto, concise, detailed). Without it merge_thinking has nothing to merge.
func WithReasoningSummary(summary string) Option {
	return func(c *Client) { c.reasoningSummary = summary }
}

func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	// Reusing a connection to a local llama.cpp server produced spurious EOFs,
	// which is why keep-alives were turned off. That workaround costs a fresh
	// TLS handshake on every request to a hosted endpoint, so apply it only to
	// the plain-HTTP servers it was written for.
	disableKeepAlives := strings.HasPrefix(baseURL, "http://")

	c := &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		backend:    BackendChatCompletions,
		maxRetries: defaultMaxRetries,
		client: &http.Client{
			Timeout: defaultRequestTimeout,
			Transport: &http.Transport{
				DisableKeepAlives: disableKeepAlives,
			},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// isRetryableError returns true if the error or status code should trigger a retry
func isRetryableError(statusCode int, err error) bool {
	// Network/connection errors are retryable
	if err != nil {
		return true
	}
	// 429 = rate limited, 5xx = server errors
	return statusCode == 429 || statusCode >= 500
}

// isPermanent500Error checks if a 500 error is permanent and should not be retried
// These are typically validation errors from chat templates, not transient server issues
func isPermanent500Error(respBody []byte) bool {
	// Template validation errors (e.g., role alternation)
	if bytes.Contains(respBody, []byte("conversation roles must alternate")) ||
		bytes.Contains(respBody, []byte("raise_exception")) {
		return true
	}
	// Schema/format validation errors
	if bytes.Contains(respBody, []byte("Invalid message")) ||
		bytes.Contains(respBody, []byte("invalid role")) {
		return true
	}
	// Jinja template errors (runtime errors in chat templates)
	if bytes.Contains(respBody, []byte("Value is not callable")) ||
		bytes.Contains(respBody, []byte("is undefined")) ||
		bytes.Contains(respBody, []byte("at row")) {
		return true
	}
	return false
}

// Chat sends one completion request and returns the model's answer. Which
// wire protocol it speaks is decided by the backend the client was built with:
// OpenAI chat completions by default, or the Responses API when the endpoint
// only serves the model there.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if c.backend == BackendResponses {
		return c.chatViaResponses(ctx, req)
	}

	// A selected effort rides in whichever field this endpoint reads. The
	// server's configured default applies when nothing is sent, so an empty
	// effort sends nothing either way.
	if c.reasoningEffort != "" {
		if c.effortField == EffortFieldReasoningEffort {
			// OpenAI's own top-level field, which is what a hosted gateway
			// such as opencode.ai reads.
			req.ReasoningEffort = c.reasoningEffort
		} else if req.ChatTemplateKwargs == nil {
			// The Qwen template's per-request override (see compose.yaml:
			// "chat_template_kwargs": {"reasoning_effort":
			// "low"|"medium"|"high"|"xhigh"}). An explicit per-request
			// ChatTemplateKwargs (e.g. the interrogator's
			// enable_thinking:false) wins over the client's default.
			req.ChatTemplateKwargs = map[string]any{
				"enable_thinking":  true,
				"reasoning_effort": c.reasoningEffort,
			}
		}
	}

	// Prepare request body. Messages are converted to their wire form, where
	// attachments become content parts; text-only messages serialize exactly
	// as before so the prompt cache keeps hitting.
	body, err := json.Marshal(toChatWireRequest(req))
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, truncated, err := c.postJSON(ctx, "/chat/completions", body)
	if err != nil {
		return nil, err
	}

	// Parse response
	var chatResp ChatResponse
	parseErr := json.Unmarshal(respBody, &chatResp)

	// If parse failed and we had a read error, try adding missing closing brace
	// This works around a llama.cpp bug where Content-Length is incorrect
	if parseErr != nil && truncated {
		fixedBody := append(respBody, '}')
		parseErr = json.Unmarshal(fixedBody, &chatResp)
	}

	if parseErr != nil {
		return nil, fmt.Errorf("decode response: %w (body preview: %s)", parseErr, bodyPreview(respBody))
	}

	return &chatResp, nil
}

// bodyPreview trims a response body down to something safe to put in an error.
func bodyPreview(body []byte) string {
	preview := string(body)
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	return preview
}

// postJSON posts body to path under the client's base URL and returns the
// response body, retrying network failures, 429s and 5xx with exponential
// backoff. The second return value reports that the body was read only
// partially, which the caller may want to repair before parsing.
func (c *Client) postJSON(ctx context.Context, path string, body []byte) ([]byte, bool, error) {
	// Retry configuration
	maxRetries := c.maxRetries
	baseDelay := 1 * time.Second
	maxDelay := 128 * time.Second

	var lastErr error
	var lastStatusCode int
	var lastRespBody []byte

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before attempting
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}

		// Wait before retry (exponential backoff)
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1)) // 1s, 2s, 4s, 8s, 16s
			if delay > maxDelay {
				delay = maxDelay
			}
			if c.onRetry != nil {
				reason := lastErr
				if reason == nil && lastStatusCode != 0 {
					reason = fmt.Errorf("API error %d", lastStatusCode)
				}
				c.onRetry(attempt, maxRetries, delay, reason)
			}
			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case <-time.After(delay):
			}
		}

		// Create HTTP request (must create new one each attempt)
		httpReq, err := http.NewRequestWithContext(
			ctx,
			"POST",
			c.baseURL+path,
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, false, fmt.Errorf("create request: %w", err)
		}

		// Set headers
		httpReq.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		for k, v := range c.headers {
			httpReq.Header.Set(k, v)
		}

		// Execute request
		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("execute request: %w", err)
			lastStatusCode = 0
			if isRetryableError(0, err) && attempt < maxRetries {
				continue // retry
			}
			return nil, false, lastErr
		}

		// Read response body (do this once for all paths)
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		// If we got an EOF error but have some data, try to use it anyway
		// llama.cpp sometimes has Content-Length issues
		if readErr != nil && readErr != io.EOF && readErr.Error() != "unexpected EOF" {
			lastErr = fmt.Errorf("read response: %w", readErr)
			if isRetryableError(resp.StatusCode, readErr) && attempt < maxRetries {
				continue // retry
			}
			return nil, false, lastErr
		}

		if len(respBody) == 0 {
			lastErr = fmt.Errorf("empty response body")
			if isRetryableError(resp.StatusCode, lastErr) && attempt < maxRetries {
				continue // retry
			}
			return nil, false, lastErr
		}

		lastStatusCode = resp.StatusCode
		lastRespBody = respBody

		// Check status code - retry on retryable errors
		if resp.StatusCode != http.StatusOK {
			lastErr = c.statusError(resp.StatusCode, respBody)
			// Don't retry on permanent 500 errors (validation, template errors)
			if resp.StatusCode == 500 && isPermanent500Error(respBody) {
				return nil, false, lastErr
			}
			if isRetryableError(resp.StatusCode, nil) && attempt < maxRetries {
				continue // retry
			}
			return nil, false, lastErr
		}

		return respBody, readErr != nil, nil
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, false, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
	}
	return nil, false, fmt.Errorf("after %d retries: %w", maxRetries, c.statusError(lastStatusCode, lastRespBody))
}

// statusError describes a request the endpoint refused. For 401 and 403 it
// adds what to do about it: the provider's own message says the key is missing
// or wrong in its words, not in terms of this program's configuration, so a
// first run against a fresh checkout ends in a message that does not say which
// variable was expected.
func (c *Client) statusError(status int, body []byte) error {
	err := fmt.Errorf("API error %d: %s", status, body)
	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		return err
	}
	switch {
	case c.apiKey == "" && c.apiKeyEnv != "":
		return fmt.Errorf("%w (no API key was sent: $%s is not set)", err, c.apiKeyEnv)
	case c.apiKey == "":
		return fmt.Errorf("%w (no API key was sent: name the variable holding it in llm.api_key_env)", err)
	case c.apiKeyEnv != "":
		return fmt.Errorf("%w (the key in $%s was refused)", err, c.apiKeyEnv)
	}
	return err
}

// GetGenerationStats queries the generation statistics for a given generation ID
func (c *Client) GetGenerationStats(ctx context.Context, generationID string) (*GenerationStats, error) {
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		c.baseURL+"/generation?id="+generationID,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	// Execute request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, c.statusError(resp.StatusCode, respBody)
	}

	// Parse response
	var stats GenerationStats
	if err := json.Unmarshal(respBody, &stats); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &stats, nil
}
