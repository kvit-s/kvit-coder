package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// Options selects one Copilot model and, optionally, forces the API origin
// or the wire protocol. An empty GitHubToken means "read whatever Copilot
// CLI would use on this machine".
type Options struct {
	Model           string
	GitHubToken     string
	Host            string
	BaseURL         string
	BackendPinned   bool
	ForceInitiator  string
	InteractionType string
}

// Result is what a chat client needs after the token exchange: the origin to
// call, the bearer token, the wire protocol, and an authorizer that refreshes
// the session token and sets Copilot's request headers.
type Result struct {
	BaseURL   string
	APIKey    string
	Backend   string
	Auth      llm.RequestAuthorizer
	APIBase   string
	SKU       string
	Exchanged bool
	// MaxOutput is the model's output cap from the account's model list, or
	// 0 when the list did not say. The messages API requires some cap;
	// chat and responses leave it to the caller.
	MaxOutput int
}

// Catalog is the model list for the signed-in account, plus which API origin
// and which kind of token produced it.
type Catalog struct {
	APIBase   string
	SKU       string
	Exchanged bool
	Models    []ListedModel
}

// Prepare exchanges the GitHub token and decides which protocol the named
// model speaks. A failure to fetch the model list does not fail the call
// when the token itself was accepted: the protocol then comes from the model
// id. An authorization failure does fail it, because the chat call would
// fail the same way.
func Prepare(ctx context.Context, opt Options) (Result, error) {
	return defaultGate.prepare(ctx, opt)
}

// List fetches the models this account can call. It is the `kvit-coder
// copilot models` command.
func List(ctx context.Context, opt Options) (Catalog, error) {
	return defaultGate.list(ctx, opt)
}

func (g *gate) githubToken(ctx context.Context, opt Options) (string, error) {
	token := strings.TrimSpace(opt.GitHubToken)
	if token != "" {
		if reason := rejectedToken(token); reason != "" {
			return "", fmt.Errorf("GitHub Copilot: the configured token is %s", reason)
		}
		return token, nil
	}
	token, _, err := g.finder().Token(ctx, opt.Host)
	return token, err
}

func (g *gate) prepare(ctx context.Context, opt Options) (Result, error) {
	githubToken, acct, err := g.session(ctx, opt)
	if err != nil {
		return Result{}, err
	}
	base := chooseBase(acct.apiBase, opt)
	backend := ""
	maxOutput := 0
	if !opt.BackendPinned && opt.Model != "" {
		backend, maxOutput, err = g.backendFor(ctx, base, acct.token, opt.Model)
		if err != nil {
			return Result{}, err
		}
	}
	return Result{
		BaseURL:   base,
		APIKey:    acct.token,
		Backend:   backend,
		APIBase:   base,
		SKU:       acct.sku,
		Exchanged: acct.exchanged,
		MaxOutput: maxOutput,
		Auth: &authorizer{
			g:               g,
			githubToken:     githubToken,
			host:            opt.Host,
			pin:             strings.TrimSpace(opt.BaseURL) != "",
			forceInitiator:  opt.ForceInitiator,
			interactionType: opt.InteractionType,
		},
	}, nil
}

func (g *gate) list(ctx context.Context, opt Options) (Catalog, error) {
	_, acct, err := g.session(ctx, opt)
	if err != nil {
		return Catalog{}, err
	}
	base := chooseBase(acct.apiBase, opt)
	models, err := g.fetchModels(ctx, base, acct.token)
	if err != nil {
		return Catalog{}, err
	}
	out := Catalog{APIBase: base, SKU: acct.sku, Exchanged: acct.exchanged}
	for _, m := range models {
		out.Models = append(out.Models, m.listed())
	}
	return out, nil
}

func chooseBase(discovered string, opt Options) string {
	if strings.TrimSpace(opt.BaseURL) != "" {
		return strings.TrimRight(opt.BaseURL, "/")
	}
	if discovered != "" {
		return discovered
	}
	return defaultAPI(opt.Host)
}

func (g *gate) session(ctx context.Context, opt Options) (string, account, error) {
	token, err := g.githubToken(ctx, opt)
	if err != nil {
		return "", account{}, err
	}
	acct, err := g.current(ctx, token, opt.Host)
	if err != nil {
		return "", account{}, err
	}
	return token, acct, nil
}

// backendFor picks the wire protocol for model and the output cap the list
// advertised. A disabled model is an error. A model missing from the list,
// or a list that could not be fetched for a reason other than authorization,
// falls back to the id and no cap.
func (g *gate) backendFor(ctx context.Context, base, bearer, model string) (string, int, error) {
	models, err := g.fetchModels(ctx, base, bearer)
	if err != nil {
		if authStatus(err) {
			return "", 0, err
		}
		return DefaultBackend(model), 0, nil
	}
	m, ok := findRemote(models, model)
	if !ok {
		return DefaultBackend(model), 0, nil
	}
	if m.Policy.State == "disabled" {
		return "", 0, fmt.Errorf("GitHub Copilot model %q is disabled by the organization's policy", model)
	}
	maxOut := m.Capabilities.Limits.MaxOutput
	if p, ok := protocolFor(m.ID, m.SupportedEndpoints); ok {
		return p, maxOut, nil
	}
	return DefaultBackend(model), maxOut, nil
}

// authorizer implements llm.RequestAuthorizer. It refreshes the session
// token and sets the headers Copilot's chat endpoint expects.
type authorizer struct {
	g               *gate
	githubToken     string
	host            string
	pin             bool
	forceInitiator  string
	interactionType string
}

func (a *authorizer) Authorize(ctx context.Context, req *http.Request, body []byte) error {
	acct, err := a.g.current(ctx, a.githubToken, a.host)
	if err != nil {
		return err
	}
	if !a.pin && acct.apiBase != "" {
		rewriteOrigin(req, acct.apiBase)
	}
	req.Header.Set("Authorization", "Bearer "+acct.token)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Openai-Intent", "conversation-edits")
	req.Header.Set("X-GitHub-Api-Version", chatAPIVersion)
	initiator := a.forceInitiator
	if initiator == "" {
		initiator = initiatorFromBody(body)
	}
	req.Header.Set("X-Initiator", initiator)
	if a.interactionType != "" {
		req.Header.Set("X-Interaction-Type", a.interactionType)
	}
	if id := os.Getenv("KVIT_RUN_ID"); id != "" {
		req.Header.Set("X-Interaction-Id", id)
	}
	if visionFromBody(body) {
		req.Header.Set("Copilot-Vision-Request", "true")
	}
	return nil
}

func (a *authorizer) Invalidate() {
	a.g.invalidate(a.githubToken, a.host)
}

// rewriteOrigin points req at base, keeping the path the client already
// built (/chat/completions, /responses, /v1/messages).
func rewriteOrigin(req *http.Request, base string) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" || req.URL == nil {
		return
	}
	req.URL.Scheme = u.Scheme
	req.URL.Host = u.Host
}

// initiatorFromBody reports whether Copilot should count the request as the
// user typing (user) or the agent continuing a tool loop (agent). A title
// call forces agent instead of reading the body.
func initiatorFromBody(body []byte) string {
	if len(body) == 0 {
		return "user"
	}
	var probe struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Input []struct {
			Role string `json:"role"`
			Type string `json:"type"`
		} `json:"input"`
	}
	if json.Unmarshal(body, &probe) != nil {
		return "user"
	}
	if n := len(probe.Input); n > 0 {
		last := probe.Input[n-1]
		if last.Type == "function_call" || last.Type == "function_call_output" || last.Role == "assistant" {
			return "agent"
		}
		return "user"
	}
	if n := len(probe.Messages); n > 0 {
		last := probe.Messages[n-1]
		if last.Role != "user" {
			return "agent"
		}
		if onlyToolResults(last.Content) {
			return "agent"
		}
	}
	return "user"
}

func onlyToolResults(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return false
	}
	var parts []struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &parts) != nil || len(parts) == 0 {
		return false
	}
	for _, p := range parts {
		if p.Type != "tool_result" {
			return false
		}
	}
	return true
}

func visionFromBody(body []byte) bool {
	return bytes.Contains(body, []byte(`"image_url"`)) ||
		bytes.Contains(body, []byte(`"input_image"`)) ||
		bytes.Contains(body, []byte(`"media_type":"image/`))
}
