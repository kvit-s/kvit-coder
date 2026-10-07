// Package modelsetup holds what kvit-coder-ui needs to add a model without
// the person editing a file: the providers it knows, the models.dev catalog
// that says how each model is served, the endpoint's own list of models, the
// model row built from those, and a test request that checks a row works.
// It draws nothing on the terminal; internal/tui asks the questions.
//
// The design is spec/configing.md. The rows built here are written to
// ~/.kvit-coder/models.yaml by internal/config, which is where the agent finds
// them on its next turn.
package modelsetup

import (
	"strings"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// Kind is how a provider is set up.
type Kind int

const (
	// Hosted is a gateway with a fixed address, a key, and an entry on
	// models.dev that describes its models.
	Hosted Kind = iota
	// Local is a server the person runs, such as llama.cpp, vLLM, Ollama or
	// LM Studio: its address is asked, it needs no key, and models.dev knows
	// nothing about what it serves.
	Local
	// Other is any endpoint that speaks one of the three protocols. Its name,
	// address, protocol and key variable are all asked.
	Other
)

// Provider is one entry of the list :setup opens with.
type Provider struct {
	// Key is the short name put at the end of a row's id ("kimi-k3-go"), so
	// the same model from two providers gets two ids.
	Key string
	// Name is what the list shows, and Short is what goes in brackets after
	// a model's name ("Kimi K3 (Go)").
	Name, Short string
	Kind        Kind
	// BaseURL is where requests go. MessagesURL is the address for models
	// served over Claude's messages protocol, for which kvit-coder appends
	// /v1/messages itself, so it stops before /v1.
	BaseURL, MessagesURL string
	// KeyEnv is the environment variable a row reads its key from; a key
	// typed into :setup is saved under the same name in credentials.json.
	KeyEnv string
	// KeyPage is the web page where a key is issued.
	KeyPage string
	// CatalogID is the provider's id on models.dev.
	CatalogID string
	// EffortField is where a chat-completions request carries the effort for
	// this provider; see llm.effort_field in docs/configuration.md.
	EffortField string
	// Headers are put on every row for this provider.
	Headers []string
}

// openCodeHeaders are what OpenCode asks of a client: a name, and one session
// id per conversation so that a conversation's prompt cache stays on one
// backend. ${KVIT_RUN_ID} is derived from the session name.
var openCodeHeaders = []string{
	"User-Agent=kvit-coder",
	"x-opencode-session=kvit-coder-${KVIT_RUN_ID}",
}

// Providers is the list :setup offers, in the order shown. Only endpoints
// kvit-coder has been used with are listed by name; "Other endpoint" covers
// the rest. Anthropic's own API is not among them because it expects the key
// in an x-api-key header, and the messages client sends Authorization: Bearer,
// which OpenCode accepts.
var Providers = []Provider{
	{
		Key: "zen", Name: "OpenCode Zen", Short: "Zen", Kind: Hosted,
		BaseURL:     "https://opencode.ai/zen/v1",
		MessagesURL: "https://opencode.ai/zen",
		KeyEnv:      "OPENCODE_API_KEY",
		KeyPage:     "https://opencode.ai/zen",
		CatalogID:   "opencode",
		EffortField: llm.EffortFieldReasoningEffort,
		Headers:     openCodeHeaders,
	},
	{
		Key: "go", Name: "OpenCode Go", Short: "Go", Kind: Hosted,
		BaseURL:     "https://opencode.ai/zen/go/v1",
		MessagesURL: "https://opencode.ai/zen/go",
		KeyEnv:      "OPENCODE_API_KEY",
		KeyPage:     "https://opencode.ai/zen",
		CatalogID:   "opencode-go",
		EffortField: llm.EffortFieldReasoningEffort,
		Headers:     openCodeHeaders,
	},
	{
		Key: "local", Name: "Local server", Short: "local", Kind: Local,
		BaseURL: "http://localhost:8080/v1",
	},
	{
		Key: "other", Name: "Other endpoint", Kind: Other,
	},
}

// CatalogIDs are the models.dev providers the list above uses, which is all
// of models.dev that is kept in the cache.
func CatalogIDs() []string {
	var ids []string
	for _, p := range Providers {
		if p.CatalogID != "" {
			ids = append(ids, p.CatalogID)
		}
	}
	return ids
}

// OtherProvider describes an endpoint the person named in :setup. name is
// what they typed, baseURL the address, and keyEnv the variable for its key
// (empty when it needs none).
func OtherProvider(name, baseURL, keyEnv string) Provider {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return Provider{
		Key:         Slug(name),
		Name:        name,
		Short:       name,
		Kind:        Other,
		BaseURL:     base,
		MessagesURL: strings.TrimSuffix(base, "/v1"),
		KeyEnv:      keyEnv,
	}
}

// LocalProvider is the local server entry pointed at baseURL.
func LocalProvider(baseURL string) Provider {
	var p Provider
	for _, q := range Providers {
		if q.Kind == Local {
			p = q
		}
	}
	p.BaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	p.MessagesURL = strings.TrimSuffix(p.BaseURL, "/v1")
	return p
}

// KeyEnvFor suggests a variable name for an endpoint's key: "My Proxy" gives
// MY_PROXY_API_KEY.
func KeyEnvFor(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "_"):
			b.WriteByte('_')
		}
	}
	s := strings.Trim(b.String(), "_")
	if s == "" {
		s = "MODEL"
	}
	return s + "_API_KEY"
}

// Slug turns a name or model id into the lowercase letters, digits, dots and
// dashes a row id is made of: "hf:zai-org/GLM-4.7" gives "hf-zai-org-glm-4.7".
func Slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-.")
}
