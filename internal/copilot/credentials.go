// Package copilot calls models through a GitHub Copilot subscription: the
// same sign-in Copilot CLI uses, exchanged for a short-lived session token
// and the API host that subscription is supposed to call.
//
// A personal subscription is served at api.githubcopilot.com. Copilot
// Business and Copilot Enterprise are served at their own hosts, and the
// host is part of the token exchange, not something the caller guesses. A
// request that skips the exchange and calls the personal host with the
// GitHub token is the failure mode in which the model list loads and every
// completion then answers that the model is not supported.
package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Provider is the config value (llm.provider, or a models: entry's provider)
// that selects this package.
const Provider = "github-copilot"

// noCredentials explains every place a token is looked for, in the order
// they are tried. It is the whole error when none of them has one.
const noCredentials = "GitHub Copilot credentials were not found. " +
	"kvit-coder reads the same sign-in Copilot CLI uses, in this order: " +
	"COPILOT_GITHUB_TOKEN, GH_TOKEN, GITHUB_TOKEN, ~/.copilot/config.json, " +
	"the copilot-cli keychain entry, then `gh auth token`. " +
	"Sign in with `copilot login`, or set COPILOT_GITHUB_TOKEN to a gho_, ghu_, or github_pat_ token. " +
	"A fine-grained personal access token needs the Copilot Requests account permission. " +
	"Classic ghp_ tokens are not accepted."

// Finder reads a GitHub token from the environment, Copilot CLI's config
// file, the OS keychain, and the GitHub CLI. A zero Finder uses the real
// environment; tests replace the function fields.
type Finder struct {
	Getenv  func(string) string
	HomeDir string
	Run     CommandRunner
}

// CommandRunner is the slice of os/exec Finder needs. Tests substitute one
// that never touches the real keychain or a real `gh`.
type CommandRunner interface {
	LookPath(name string) (string, error)
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type osRunner struct{}

func (osRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (osRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (f Finder) getenv(k string) string {
	if f.Getenv != nil {
		return f.Getenv(k)
	}
	return os.Getenv(k)
}

func (f Finder) home() string {
	if f.HomeDir != "" {
		return f.HomeDir
	}
	h, _ := os.UserHomeDir()
	return h
}

func (f Finder) run() CommandRunner {
	if f.Run != nil {
		return f.Run
	}
	return osRunner{}
}

// Token returns a GitHub token Copilot will accept, and a short description
// of where it came from (for an error that has to say which one was refused).
// host is the GitHub Enterprise hostname, or empty for github.com; a stored
// token that names a different host is skipped.
func (f Finder) Token(ctx context.Context, host string) (token, source string, err error) {
	var notes []string
	for _, env := range []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		v := strings.TrimSpace(f.getenv(env))
		if v == "" {
			continue
		}
		if reason := rejectedToken(v); reason != "" {
			notes = append(notes, "$"+env+" is set but "+reason)
			continue
		}
		if !usableToken(v) {
			notes = append(notes, "$"+env+" is set but is not a gho_, ghu_, or github_pat_ token")
			continue
		}
		return v, "$" + env, nil
	}

	if home := f.home(); home != "" {
		path := filepath.Join(home, ".copilot", "config.json")
		if data, readErr := os.ReadFile(path); readErr == nil {
			if tok, ok := pickStored(tokensFromConfig(data), host); ok {
				return tok, path, nil
			}
		}
	}

	if tok, note, ok := f.keychainToken(ctx); ok {
		return tok, "the copilot-cli keychain entry", nil
	} else if note != "" {
		notes = append(notes, note)
	}

	if tok, ok := f.ghToken(ctx, host); ok {
		return tok, "`gh auth token`", nil
	}

	msg := noCredentials
	if len(notes) > 0 {
		msg += " " + strings.Join(notes, "; ") + "."
	}
	return "", "", fmt.Errorf("%s", msg)
}

// rejectedToken reports why a token must not be sent, or "" when it may.
func rejectedToken(token string) string {
	if strings.HasPrefix(strings.TrimSpace(token), "ghp_") {
		return "a classic personal access token (ghp_) is not accepted by GitHub Copilot"
	}
	return ""
}

func usableToken(token string) bool {
	token = strings.TrimSpace(token)
	return strings.HasPrefix(token, "gho_") ||
		strings.HasPrefix(token, "ghu_") ||
		strings.HasPrefix(token, "github_pat_")
}

type storedToken struct {
	token string
	host  string
}

func pickStored(tokens []storedToken, host string) (string, bool) {
	for _, t := range tokens {
		if hostOK(host, t.host) && usableToken(t.token) {
			return t.token, true
		}
	}
	return "", false
}

// hostOK reports whether a stored token's host (empty when the file does not
// say) can be used for the host the caller asked for.
func hostOK(want, got string) bool {
	got = strings.TrimSpace(got)
	if got == "" {
		return true
	}
	want = strings.TrimSpace(want)
	if want == "" || strings.EqualFold(want, "github.com") {
		return strings.EqualFold(got, "github.com")
	}
	return strings.EqualFold(got, want)
}

func tokensFromConfig(data []byte) []storedToken {
	if len(data) > 1<<20 {
		data = data[:1<<20]
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return nil
	}
	var out []storedToken
	if t, ok := tokenFromMap(root); ok {
		out = append(out, t)
	}
	if raw, ok := root["auth"]; ok {
		var auth map[string]json.RawMessage
		if json.Unmarshal(raw, &auth) == nil {
			if t, ok := tokenFromMap(auth); ok {
				out = append(out, t)
			}
		}
	}
	last := jsonString(root["last_logged_in_user"])
	if raw, ok := root["users"]; ok {
		var users map[string]json.RawMessage
		if json.Unmarshal(raw, &users) == nil {
			if last != "" {
				if u, ok := users[last]; ok {
					if t, ok := decodeUser(u); ok {
						out = append(out, t)
					}
				}
			}
			names := make([]string, 0, len(users))
			for name := range users {
				if name != last {
					names = append(names, name)
				}
			}
			sort.Strings(names)
			for _, name := range names {
				if t, ok := decodeUser(users[name]); ok {
					out = append(out, t)
				}
			}
		}
	}
	return out
}

func decodeUser(raw json.RawMessage) (storedToken, bool) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return storedToken{}, false
	}
	return tokenFromMap(m)
}

func tokenFromMap(m map[string]json.RawMessage) (storedToken, bool) {
	var token string
	for _, key := range []string{"github_token", "oauth_token", "access_token", "token"} {
		if v := jsonString(m[key]); usableToken(v) {
			token = v
			break
		}
	}
	if token == "" {
		return storedToken{}, false
	}
	host := ""
	for _, key := range []string{"host", "hostname", "github_host"} {
		if v := jsonString(m[key]); v != "" {
			host = v
			break
		}
	}
	return storedToken{token: token, host: host}, true
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func (f Finder) keychainToken(ctx context.Context) (string, string, bool) {
	name, args, ok := keychainLookup()
	if !ok {
		return "", "", false
	}
	if _, err := f.run().LookPath(name); err != nil {
		return "", "", false
	}
	out, err := f.command(ctx, name, args...)
	if tok, ok := firstToken(out); ok {
		return tok, "", true
	}
	note := ""
	if err != nil && ctx.Err() == nil {
		// A locked keychain can sit on a prompt. The command is bounded, and
		// the next source may still have a token, so this is a note rather
		// than a failure.
		if strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "killed") {
			note = "the copilot-cli keychain entry did not answer within 5s"
		}
	}
	if runtime.GOOS == "linux" {
		if out, err2 := f.command(ctx, "secret-tool", "search", "service", "copilot-cli"); err2 == nil {
			if tok, ok := firstToken(out); ok {
				return tok, "", true
			}
		}
	}
	return "", note, false
}

func keychainLookup() (name string, args []string, ok bool) {
	switch runtime.GOOS {
	case "linux":
		return "secret-tool", []string{"lookup", "service", "copilot-cli"}, true
	case "darwin":
		return "security", []string{"find-generic-password", "-s", "copilot-cli", "-w"}, true
	default:
		return "", nil, false
	}
}

func firstToken(out string) (string, bool) {
	out = strings.TrimSpace(out)
	if usableToken(out) {
		return out, true
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "secret = "); ok && usableToken(strings.TrimSpace(v)) {
			return strings.TrimSpace(v), true
		}
		if v, ok := strings.CutPrefix(line, "secret="); ok && usableToken(strings.TrimSpace(v)) {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

func (f Finder) ghToken(ctx context.Context, host string) (string, bool) {
	if _, err := f.run().LookPath("gh"); err != nil {
		return "", false
	}
	args := []string{"auth", "token"}
	if host != "" && !strings.EqualFold(host, "github.com") {
		args = append(args, "--hostname", host)
	}
	out, err := f.command(ctx, "gh", args...)
	if err != nil || !usableToken(out) {
		return "", false
	}
	return out, true
}

func (f Finder) command(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	b, err := f.run().Output(ctx, name, args...)
	return strings.TrimSpace(string(b)), err
}

// NormalizeHost accepts a hostname or an https URL and returns the hostname
// Copilot's enterprise routing wants (company.ghe.com). Empty stays empty.
func NormalizeHost(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.Trim(s, "/")
	if s == "" || strings.ContainsAny(s, "/ \t") {
		return "", fmt.Errorf("copilot_host must be a hostname such as company.ghe.com")
	}
	return s, nil
}

func publicHost(host string) bool {
	host = strings.TrimSpace(host)
	return host == "" || strings.EqualFold(host, "github.com")
}

func githubAPI(host string) string {
	if publicHost(host) {
		return "https://api.github.com"
	}
	return "https://api." + host
}

func defaultAPI(host string) string {
	if publicHost(host) {
		return "https://api.githubcopilot.com"
	}
	return "https://copilot-api." + host
}
