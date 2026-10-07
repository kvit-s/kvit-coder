package modelsetup

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// Candidate is one model :setup can offer: an id the endpoint listed, joined
// with what models.dev says about it.
type Candidate struct {
	ID      string
	Listed  ListedModel
	Catalog *CatalogModel // nil when models.dev does not describe the model
	// Backend is the protocol the model is served over, from models.dev; for
	// a model models.dev does not describe it is the provider's usual one,
	// and a test request finds the real one (see Probe).
	Backend string
	// Unsupported says why the model cannot be used, such as a protocol
	// kvit-coder does not speak. Empty when it can.
	Unsupported string
}

// Described reports whether models.dev describes the model, which is what
// makes its protocol, context size and effort levels known without asking.
func (c Candidate) Described() bool { return c.Catalog != nil }

// BackendForNPM maps the client library models.dev names for a model to the
// protocol kvit-coder speaks to it. OpenCode serves one gateway over three
// protocols and models.dev records which by naming the library: the OpenAI
// library speaks the responses protocol, the Anthropic one the messages
// protocol, and the generic OpenAI-compatible one (or none named) chat
// completions. Google's library speaks a protocol kvit-coder does not have.
func BackendForNPM(npm string) (string, bool) {
	switch npm {
	case "@ai-sdk/openai":
		return llm.BackendResponses, true
	case "@ai-sdk/anthropic":
		return llm.BackendMessages, true
	case "", "@ai-sdk/openai-compatible":
		return llm.BackendChatCompletions, true
	}
	return "", false
}

// Candidates joins an endpoint's model list with the catalog, in the
// endpoint's order.
func Candidates(p Provider, listed []ListedModel, cat *Catalog) []Candidate {
	out := make([]Candidate, 0, len(listed))
	for _, l := range listed {
		c := Candidate{ID: l.ID, Listed: l, Backend: llm.BackendChatCompletions}
		if p.CatalogID != "" {
			c.Catalog = cat.Model(p.CatalogID, l.ID)
		}
		if c.Catalog != nil {
			npm := cat.ProviderNPM(p.CatalogID)
			if c.Catalog.Provider != nil && c.Catalog.Provider.NPM != "" {
				npm = c.Catalog.Provider.NPM
			}
			backend, ok := BackendForNPM(npm)
			if !ok {
				c.Unsupported = protocolName(npm) + ", which kvit-coder does not speak"
			}
			c.Backend = backend
		}
		out = append(out, c)
	}
	return out
}

func protocolName(npm string) string {
	if npm == "@ai-sdk/google" {
		return "Google's Gemini protocol"
	}
	return "the protocol of " + npm
}

// BaseURLFor is where a row for this provider sends requests over backend:
// the messages address for the messages protocol, the usual one otherwise.
func (p Provider) BaseURLFor(backend string) string {
	if backend == llm.BackendMessages && p.MessagesURL != "" {
		return p.MessagesURL
	}
	return p.BaseURL
}

// Row builds the models: row for a candidate. taken reports whether an id is
// already in use, so the row gets one that is not ("kimi-k3-go", then
// "kimi-k3-go-2").
func Row(p Provider, c Candidate, taken func(string) bool) config.ModelEntry {
	name := c.ID
	if c.Catalog != nil && c.Catalog.Name != "" {
		name = c.Catalog.Name
	}
	if p.Short != "" {
		name += " (" + p.Short + ")"
	}
	e := config.ModelEntry{
		ID:         uniqueID(Slug(c.ID)+"-"+p.Key, taken),
		Name:       name,
		Model:      c.ID,
		BaseURL:    p.BaseURLFor(c.Backend),
		APIBackend: c.Backend,
		APIKeyEnv:  p.KeyEnv,
		Context:    c.Listed.Context,
		Efforts:    Efforts(c.Catalog),
		Headers:    append([]string(nil), p.Headers...),
	}
	if c.Catalog != nil && c.Catalog.Limit.Context > 0 {
		e.Context = c.Catalog.Limit.Context
	}
	if c.Backend == llm.BackendChatCompletions {
		e.EffortField = p.EffortField
	}
	return e
}

// SetBackend points a row at another protocol of the same provider, as when
// a test request found the one that answers.
func SetBackend(p Provider, e *config.ModelEntry, backend string) {
	e.APIBackend = backend
	e.BaseURL = p.BaseURLFor(backend)
	e.EffortField = ""
	if backend == llm.BackendChatCompletions {
		e.EffortField = p.EffortField
	}
}

func uniqueID(base string, taken func(string) bool) string {
	if base == "" || base == "-" {
		base = "model"
	}
	id := base
	for n := 2; taken != nil && taken(id); n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

// Efforts is the :eN menu for a model from its models.dev effort levels, in
// kvit-coder's order from none to max, starting at high when it is offered
// and at the highest level otherwise. A model that does not reason, or whose
// reasoning is only switched on and off, gets no menu.
func Efforts(m *CatalogModel) []config.EffortOption {
	if m == nil || !m.Reasoning {
		return nil
	}
	for _, ro := range m.ReasoningOptions {
		if ro.Type != "effort" {
			continue
		}
		var values []string
		seen := map[string]bool{}
		for _, v := range ro.Values {
			v = strings.ToLower(strings.TrimSpace(v))
			if config.IsCanonicalEffort(v) && !seen[v] {
				seen[v] = true
				values = append(values, v)
			}
		}
		if len(values) == 0 {
			return nil
		}
		sort.SliceStable(values, func(i, j int) bool { return effortRank(values[i]) < effortRank(values[j]) })
		def := len(values) - 1
		for i, v := range values {
			if v == "high" {
				def = i
			}
		}
		opts := make([]config.EffortOption, len(values))
		for i, v := range values {
			opts[i] = config.EffortOption{Value: v, Default: i == def}
		}
		return opts
	}
	return nil
}

func effortRank(v string) int {
	for i, e := range config.CanonicalEfforts {
		if e == v {
			return i
		}
	}
	return len(config.CanonicalEfforts)
}

// EffortSummary is a menu as one phrase: "minimal–xhigh", "max", or "" for
// none.
func EffortSummary(opts []config.EffortOption) string {
	switch len(opts) {
	case 0:
		return ""
	case 1:
		return opts[0].Value
	}
	return opts[0].Value + "–" + opts[len(opts)-1].Value
}

// ProtocolLabel is how a protocol is written for a person.
func ProtocolLabel(backend string) string {
	switch backend {
	case llm.BackendResponses:
		return "responses"
	case llm.BackendMessages:
		return "messages"
	case llm.BackendChatCompletions, "":
		return "chat completions"
	}
	return backend
}

// Tokens writes a context size the way people say it: 1048576 is "1.05M",
// 262144 is "262K".
func Tokens(n int) string {
	switch {
	case n <= 0:
		return "?"
	case n >= 1_000_000:
		s := strconv.FormatFloat(float64(n)/1e6, 'f', 2, 64)
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		return s + "M"
	case n >= 1000:
		return strconv.Itoa((n+500)/1000) + "K"
	}
	return strconv.Itoa(n)
}

// Detail is the line the model list shows beside a candidate's id.
func (c Candidate) Detail() string {
	if c.Unsupported != "" {
		return "uses " + c.Unsupported
	}
	var parts []string
	if c.Catalog == nil {
		parts = append(parts, "not on models.dev")
		if c.Listed.Context > 0 {
			parts = append(parts, Tokens(c.Listed.Context)+" context")
		}
		return strings.Join(parts, " · ")
	}
	parts = append(parts, ProtocolLabel(c.Backend))
	if ctx := c.Catalog.Limit.Context; ctx > 0 {
		parts = append(parts, Tokens(ctx)+" context")
	}
	if s := EffortSummary(Efforts(c.Catalog)); s != "" {
		parts = append(parts, "effort "+s)
	}
	if cost := c.Catalog.Cost; cost != nil {
		if cost.Input == 0 && cost.Output == 0 {
			parts = append(parts, "free")
		} else {
			parts = append(parts, fmt.Sprintf("$%s/$%s per M", price(cost.Input), price(cost.Output)))
		}
	}
	if c.Catalog.Status != "" {
		parts = append(parts, c.Catalog.Status)
	}
	return strings.Join(parts, " · ")
}

func price(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	return strings.TrimSuffix(s, ".00")
}
