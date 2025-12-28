package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true, // Disable connection reuse to avoid EOF issues
			},
		},
	}
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

func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Prepare request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Retry configuration
	const maxRetries = 10
	baseDelay := 1 * time.Second
	maxDelay := 128 * time.Second

	var lastErr error
	var lastStatusCode int
	var lastRespBody []byte

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before attempting
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Wait before retry (exponential backoff)
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1)) // 1s, 2s, 4s, 8s, 16s
			if delay > maxDelay {
				delay = maxDelay
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		// Create HTTP request (must create new one each attempt)
		httpReq, err := http.NewRequestWithContext(
			ctx,
			"POST",
			c.baseURL+"/chat/completions",
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		// Set headers
		httpReq.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		// Execute request
		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("execute request: %w", err)
			lastStatusCode = 0
			if isRetryableError(0, err) && attempt < maxRetries {
				continue // retry
			}
			return nil, lastErr
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
			return nil, lastErr
		}

		if len(respBody) == 0 {
			lastErr = fmt.Errorf("empty response body")
			if isRetryableError(resp.StatusCode, lastErr) && attempt < maxRetries {
				continue // retry
			}
			return nil, lastErr
		}

		lastStatusCode = resp.StatusCode
		lastRespBody = respBody

		// Check status code - retry on retryable errors
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error %d: %s", resp.StatusCode, respBody)
			// Don't retry on permanent 500 errors (validation, template errors)
			if resp.StatusCode == 500 && isPermanent500Error(respBody) {
				return nil, lastErr
			}
			if isRetryableError(resp.StatusCode, nil) && attempt < maxRetries {
				continue // retry
			}
			return nil, lastErr
		}

		// Parse response
		var chatResp ChatResponse
		parseErr := json.Unmarshal(respBody, &chatResp)

		// If parse failed and we had a read error, try adding missing closing brace
		// This works around a llama.cpp bug where Content-Length is incorrect
		if parseErr != nil && readErr != nil {
			fixedBody := append(respBody, '}')
			parseErr = json.Unmarshal(fixedBody, &chatResp)
		}

		if parseErr != nil {
			// Log first 500 chars of response for debugging
			preview := string(respBody)
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			return nil, fmt.Errorf("decode response: %w (body preview: %s)", parseErr, preview)
		}

		return &chatResp, nil
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
	}
	return nil, fmt.Errorf("after %d retries: API error %d: %s", maxRetries, lastStatusCode, lastRespBody)
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
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, respBody)
	}

	// Parse response
	var stats GenerationStats
	if err := json.Unmarshal(respBody, &stats); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &stats, nil
}
