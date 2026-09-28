package config

import (
	"context"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/copilot"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// UseAPIBackend records a wire protocol chosen on the command line. It wins
// over the protocol a Copilot model list would otherwise select.
func (c *Config) UseAPIBackend(backend string) {
	c.LLM.APIBackend = backend
	c.copilotBackendPinned = true
}

// Authorizer is the Copilot request authorizer installed by
// PrepareActiveEndpoint, or nil when the active model is an ordinary endpoint.
func (c *Config) Authorizer() llm.RequestAuthorizer { return c.copilotAuth }

// PrepareActiveEndpoint exchanges a GitHub Copilot token when the active
// model uses that provider, and records the API host, session token and wire
// protocol on cfg.LLM. Other providers return immediately. The authorizer
// kept for the client refreshes the session token on later requests.
func (c *Config) PrepareActiveEndpoint(ctx context.Context) error {
	if c.LLM.Provider != copilot.Provider {
		c.copilotAuth = nil
		return nil
	}
	// A legacy config with no models: list states its protocol in the llm:
	// block. A catalog entry that leaves api_backend empty does not: the
	// block's value belongs to the default endpoint, and Copilot's own model
	// list decides. --api-backend sets copilotBackendPinned either way.
	pinned := c.copilotBackendPinned
	if len(c.Models) == 0 && strings.TrimSpace(c.LLM.APIBackend) != "" {
		pinned = true
	}
	res, err := copilot.Prepare(ctx, copilot.Options{
		Model:           c.LLM.Model,
		GitHubToken:     c.LLM.APIKey,
		Host:            c.LLM.CopilotHost,
		BaseURL:         c.LLM.BaseURL,
		BackendPinned:   pinned,
		InteractionType: "conversation-agent",
	})
	if err != nil {
		return err
	}
	c.LLM.BaseURL = res.BaseURL
	c.LLM.APIKey = res.APIKey
	// The value now in APIKey is a short-lived session token, not the
	// environment variable the GitHub token was read from.
	c.LLM.APIKeyEnv = ""
	if res.Backend != "" {
		c.LLM.APIBackend = res.Backend
	}
	// Claude's API requires an output cap. When the config does not set one,
	// use the cap the account's model list advertised. Chat and responses
	// models are left alone: Copilot rejects max_tokens on some of them.
	if res.Backend == llm.BackendMessages && c.LLM.MaxTokens == 0 && res.MaxOutput > 0 {
		c.LLM.MaxTokens = res.MaxOutput
	}
	c.copilotAuth = res.Auth
	return nil
}

// PrepareCopilotEntry is PrepareActiveEndpoint for a catalog row that is not
// the active model — the session-title model, when that row is Copilot and
// the turn itself is on something else. title forces the request to be
// counted as agent-initiated and skips thinking.
func (c *Config) PrepareCopilotEntry(ctx context.Context, entry ModelEntry, title bool) (copilot.Result, error) {
	if entry.Provider != copilot.Provider {
		return copilot.Result{}, nil
	}
	host := entry.CopilotHost
	if host == "" {
		host = c.fileCopilotHost
	}
	opt := copilot.Options{
		Model:         entry.Model,
		GitHubToken:   EntryAPIKey(entry),
		Host:          host,
		BaseURL:       entry.BaseURL,
		BackendPinned: entry.APIBackend != "",
	}
	if title {
		opt.ForceInitiator = "agent"
		opt.InteractionType = "agent-session-name-generation"
	}
	return copilot.Prepare(ctx, opt)
}
