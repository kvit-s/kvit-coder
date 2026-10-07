package modelsetup

import (
	"context"
	"errors"
	"net"
	"net/url"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// probeTimeout bounds one test request. A reasoning model at its lowest
// effort answers a one-word question in a few seconds.
const probeTimeout = 60 * time.Second

// probeMaxTokens caps the answer. The question needs one word; the room is
// for a reasoning model that thinks briefly first, and for endpoints that
// refuse a smaller cap.
const probeMaxTokens = 64

// ProbeResult is what a test request found out about a row.
type ProbeResult struct {
	// Backend is the protocol that answered; it differs from the row's when
	// the row's own was refused and another one answered.
	Backend string
	// Err is why no protocol answered; nil when one did.
	Err error
	// KeyRefused is set when the endpoint answered 401 or 403.
	KeyRefused bool
}

// OK reports whether the model answered.
func (r ProbeResult) OK() bool { return r.Err == nil }

// Probe sends the model in entry one short question through the same client
// the agent uses, with key and headers, at the lowest effort the row offers.
//
// When tryOthers is set and the endpoint refuses the row's protocol for any
// reason other than the key, a rate limit or the network, the other two
// protocols are tried and the first that answers is reported. That is how
// the protocol of a model models.dev does not describe is found: a 200 from
// one of them is good evidence, where a guess is not.
func Probe(ctx context.Context, p Provider, entry config.ModelEntry, key string, headers map[string]string, tryOthers bool) ProbeResult {
	order := []string{entry.APIBackend}
	if order[0] == "" {
		order[0] = llm.BackendChatCompletions
	}
	if tryOthers {
		for _, b := range []string{llm.BackendResponses, llm.BackendChatCompletions, llm.BackendMessages} {
			if b != order[0] {
				order = append(order, b)
			}
		}
	}
	var first ProbeResult
	for i, backend := range order {
		err := probeOnce(ctx, p, entry, backend, key, headers)
		res := classify(backend, err)
		if res.OK() || res.KeyRefused || !protocolMaybeWrong(err) {
			if i > 0 && !res.OK() {
				return first
			}
			return res
		}
		if i == 0 {
			first = res
		}
	}
	return first
}

func probeOnce(ctx context.Context, p Provider, entry config.ModelEntry, backend, key string, headers map[string]string) error {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	base := entry.BaseURL
	if backend != entry.APIBackend && p.BaseURL != "" {
		base = p.BaseURLFor(backend)
	}
	effortField := entry.EffortField
	if backend == llm.BackendChatCompletions && effortField == "" {
		effortField = p.EffortField
	}
	client := llm.NewClient(base, key,
		llm.WithBackend(backend),
		llm.WithHeaders(headers),
		llm.WithAPIKeyEnv(entry.APIKeyEnv),
		llm.WithReasoningEffort(lowestEffort(entry)),
		llm.WithEffortField(effortField),
		llm.WithTimeout(probeTimeout),
		llm.WithMaxRetries(0),
	)
	_, err := client.Chat(ctx, llm.ChatRequest{
		Model: entry.Model,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Reply with the single word OK."},
		},
		MaxTokens: probeMaxTokens,
	})
	return err
}

func classify(backend string, err error) ProbeResult {
	res := ProbeResult{Backend: backend, Err: err}
	if err == nil {
		return res
	}
	var status *llm.StatusError
	if errors.As(err, &status) {
		res.KeyRefused = status.Code == 401 || status.Code == 403
		return res
	}
	if !isNetworkError(err) {
		// The endpoint answered 200 and the client could not use the answer,
		// such as an empty reply from a model that spent its few tokens
		// thinking. The key and the protocol both worked, which is all the
		// test is for.
		res.Err = nil
	}
	return res
}

// protocolMaybeWrong reports whether a failure could be the endpoint not
// serving this model over this protocol, which is worth trying another for.
// A refused key, a rate limit and a network failure are the same whatever
// the protocol.
func protocolMaybeWrong(err error) bool {
	var status *llm.StatusError
	if !errors.As(err, &status) {
		return false
	}
	switch status.Code {
	case 401, 403, 429:
		return false
	}
	return true
}

func isNetworkError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var urlErr *url.Error
	var netErr net.Error
	return errors.As(err, &urlErr) || errors.As(err, &netErr)
}

func lowestEffort(e config.ModelEntry) string {
	if len(e.Efforts) == 0 {
		return ""
	}
	low := e.Efforts[0].Value
	for _, o := range e.Efforts[1:] {
		if effortRank(o.Value) < effortRank(low) {
			low = o.Value
		}
	}
	return low
}
