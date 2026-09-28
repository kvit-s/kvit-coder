package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// editor headers the Copilot token endpoint checks. Without Editor-Version
// it answers 404, which is indistinguishable from "this account has no
// Copilot" unless the request carries the editor identity the endpoint was
// written for. Chat requests do not send these; they identify as kvit-coder.
const (
	editorVersion = "vscode/1.104.1"
	editorPlugin  = "copilot-chat/0.32.0"
	editorIntegr  = "vscode-chat"

	userAPIVersion = "2025-04-01"
	chatAPIVersion = "2026-08-01"
	userAgent      = "kvit-coder"
)

// account is one exchanged Copilot session: the bearer token chat calls
// send, when it stops being valid, and the API origin for this subscription.
type account struct {
	token     string
	expires   time.Time
	apiBase   string
	sku       string
	exchanged bool
}

type cacheKey struct {
	token string
	host  string
}

// gate caches session tokens for one process. A turn and the session-title
// call share the default gate, so the exchange happens once.
type gate struct {
	mu         sync.Mutex
	cache      map[cacheKey]account
	httpClient *http.Client
	// githubAPI, when set, replaces the host-derived GitHub API origin.
	// Tests point it at an httptest server.
	githubAPI string
	now       func() time.Time
	find      *Finder
}

func (g *gate) clock() time.Time {
	if g != nil && g.now != nil {
		return g.now()
	}
	return time.Now()
}

func (g *gate) client() *http.Client {
	if g != nil && g.httpClient != nil {
		return g.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (g *gate) finder() Finder {
	if g != nil && g.find != nil {
		return *g.find
	}
	return Finder{}
}

func (g *gate) apiRoot(host string) string {
	if g != nil && g.githubAPI != "" {
		return strings.TrimRight(g.githubAPI, "/")
	}
	return githubAPI(host)
}

var defaultGate gate

type copilotUser struct {
	ChatEnabled         *bool `json:"chat_enabled"`
	CanSignupForLimited *bool `json:"can_signup_for_limited"`
	Endpoints           struct {
		API string `json:"api"`
	} `json:"endpoints"`
	Message string `json:"message"`
}

type copilotToken struct {
	Token     string  `json:"token"`
	ExpiresAt float64 `json:"expires_at"`
	RefreshIn float64 `json:"refresh_in"`
	SKU       string  `json:"sku"`
	Endpoints struct {
		API string `json:"api"`
	} `json:"endpoints"`
	Message string `json:"message"`
}

// current returns a session that is still inside its lifetime, exchanging
// again when the cached one is missing or within two minutes of expiry.
func (g *gate) current(ctx context.Context, githubToken, host string) (account, error) {
	if acct, ok := g.cached(githubToken, host); ok {
		return acct, nil
	}
	acct, err := g.exchange(ctx, githubToken, host)
	if err != nil {
		return account{}, err
	}
	g.store(githubToken, host, acct)
	return acct, nil
}

func (g *gate) cached(githubToken, host string) (account, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	acct, ok := g.cache[cacheKey{githubToken, host}]
	if !ok {
		return account{}, false
	}
	if !acct.expires.IsZero() && !g.clock().Add(2*time.Minute).Before(acct.expires) {
		return account{}, false
	}
	return acct, true
}

func (g *gate) store(githubToken, host string, acct account) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cache == nil {
		g.cache = map[cacheKey]account{}
	}
	g.cache[cacheKey{githubToken, host}] = acct
}

func (g *gate) invalidate(githubToken, host string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.cache, cacheKey{githubToken, host})
}

// exchange turns a GitHub token into the session token and API host for
// this subscription.
//
// /copilot_internal/user answers whether the account may chat and which API
// origin it belongs to. /copilot_internal/v2/token then trades the GitHub
// token for a short-lived session token. Business and enterprise accounts
// put their own origin in that response; calling the personal origin with
// the GitHub token is what produces "model not supported" after a model
// list that looked fine. When the token endpoint is absent, the GitHub
// token is sent to the origin /copilot_internal/user named, which is the
// path that works for subscriptions the token endpoint does not serve.
func (g *gate) exchange(ctx context.Context, githubToken, host string) (account, error) {
	root := g.apiRoot(host)
	header := internalHeader(githubToken)

	status, body, err := g.get(ctx, root+"/copilot_internal/user", header)
	if err != nil {
		return account{}, fmt.Errorf("GitHub Copilot: %w", err)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return account{}, fmt.Errorf("GitHub Copilot refused the GitHub token (%d). %s", status, tokenRefused)
	}

	var user copilotUser
	userOK := status == http.StatusOK && json.Unmarshal(body, &user) == nil
	if userOK {
		if msg := entitlementError(user); msg != "" {
			return account{}, fmt.Errorf("%s", msg)
		}
	}

	tStatus, tBody, tErr := g.get(ctx, root+"/copilot_internal/v2/token", header)
	var session copilotToken
	sessionOK := tErr == nil && tStatus == http.StatusOK && json.Unmarshal(tBody, &session) == nil && session.Token != ""
	// A 401 or 403 here is not by itself a bad GitHub token. This account's
	// /copilot_internal/user call succeeds and its model list accepts the
	// GitHub token, while the token-exchange URL answers 403. Only the user
	// record's 401/403 means the token was refused. A failed exchange falls
	// through to that record's API host.

	apiBase := ""
	if sessionOK {
		apiBase = strings.TrimRight(session.Endpoints.API, "/")
	}
	if apiBase == "" && userOK {
		apiBase = strings.TrimRight(user.Endpoints.API, "/")
	}
	if apiBase == "" {
		apiBase = defaultAPI(host)
	}

	if sessionOK {
		return account{
			token:     session.Token,
			expires:   expiry(g.clock(), session.ExpiresAt, session.RefreshIn),
			apiBase:   apiBase,
			sku:       session.SKU,
			exchanged: true,
		}, nil
	}

	// The token endpoint is how a business or enterprise seat is routed.
	// Falling back to the GitHub token is only useful when /copilot_internal/user
	// already named an origin, which is what a personal subscription does
	// when the token endpoint is unavailable.
	if userOK && (user.Endpoints.API != "" || user.ChatEnabled == nil || *user.ChatEnabled) {
		return account{token: githubToken, apiBase: apiBase, exchanged: false}, nil
	}

	detail := trimBody(tBody)
	if tErr != nil {
		detail = tErr.Error()
	} else if detail == "" {
		detail = trimBody(body)
	}
	return account{}, fmt.Errorf("GitHub Copilot token exchange failed (%d). %s", tStatus, detail)
}

const tokenRefused = "Sign in again with `copilot login`, or set COPILOT_GITHUB_TOKEN to a gho_, ghu_, or github_pat_ token. " +
	"A fine-grained token needs the Copilot Requests account permission."

func entitlementError(user copilotUser) string {
	if user.ChatEnabled == nil || *user.ChatEnabled {
		return ""
	}
	if user.CanSignupForLimited != nil && *user.CanSignupForLimited {
		return "This GitHub account is not signed up for GitHub Copilot. Sign up at https://github.com/features/copilot/plans and try again."
	}
	return "This GitHub account does not have GitHub Copilot. It needs an active subscription or a seat assigned by an organization."
}

func internalHeader(githubToken string) http.Header {
	h := make(http.Header)
	// The internal endpoints take the GitHub token with the "token" scheme,
	// which is what Copilot's own clients send. Chat calls use Bearer and
	// the exchanged session token, not this value.
	h.Set("Authorization", "token "+githubToken)
	h.Set("Accept", "application/json")
	h.Set("User-Agent", userAgent)
	h.Set("Editor-Version", editorVersion)
	h.Set("Editor-Plugin-Version", editorPlugin)
	h.Set("Copilot-Integration-Id", editorIntegr)
	h.Set("X-GitHub-Api-Version", userAPIVersion)
	return h
}

func expiry(now time.Time, expiresAt, refreshIn float64) time.Time {
	if expiresAt > 1e12 {
		expiresAt = expiresAt / 1000
	}
	if expiresAt > 0 {
		return time.Unix(int64(expiresAt), 0)
	}
	if refreshIn > 0 {
		return now.Add(time.Duration(refreshIn) * time.Second)
	}
	return time.Time{}
}

func (g *gate) get(ctx context.Context, rawURL string, header http.Header) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header = header
	resp, err := g.client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func trimBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}
