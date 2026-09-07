package permissions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Scope of a grant: how long it lasts and how far it reaches.
type GrantScope string

const (
	// GrantOnce lets this one command through now and is not written down.
	GrantOnce GrantScope = "once"
	// GrantSession lasts for this conversation.
	GrantSession GrantScope = "session"
	// GrantProject lasts for this workspace, across conversations.
	GrantProject GrantScope = "project"
	// GrantGlobal lasts everywhere on this machine.
	GrantGlobal GrantScope = "global"
)

// Store is a file of grants. Grant files live outside the workspace on purpose:
// the agent can write anywhere inside the workspace, and a permission file it
// could edit would not be a permission file.
type Store struct {
	path   string
	source string
	grants grants
}

type grants struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

// OpenStore reads a grant file, or returns an empty store when there is none.
func OpenStore(path, source string) (*Store, error) {
	s := &Store{path: path, source: source}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &s.grants); err != nil {
		return nil, fmt.Errorf("%s is not readable: %w", path, err)
	}
	return s, nil
}

// Path is the file this store is kept in.
func (s *Store) Path() string { return s.path }

// Rules renders the store's grants as policy rules.
func (s *Store) Rules() []Rule {
	if s == nil {
		return nil
	}
	out := make([]Rule, 0, len(s.grants.Allow)+len(s.grants.Deny))
	for _, pattern := range s.grants.Deny {
		out = append(out, Rule{Pattern: pattern, Effect: EffectDeny, Source: s.source,
			Reason: "it was refused for this " + s.source})
	}
	for _, pattern := range s.grants.Allow {
		out = append(out, Rule{Pattern: pattern, Effect: EffectAllow, Source: s.source,
			Reason: "it was allowed for this " + s.source})
	}
	return out
}

// Allow records a pattern as permitted and writes the file.
func (s *Store) Allow(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("cannot grant an empty pattern")
	}
	if slices.Contains(s.grants.Allow, pattern) {
		return nil
	}
	s.grants.Allow = append(s.grants.Allow, pattern)
	sort.Strings(s.grants.Allow)
	return s.save()
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(s.path), err)
	}
	data, err := json.MarshalIndent(s.grants, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", s.path, err)
	}
	return os.WriteFile(s.path, append(data, '\n'), 0644)
}

// ProjectStorePath is where grants for one workspace are kept. It is under the
// user's home rather than in the workspace, because a file inside the workspace
// is one the agent can edit.
func ProjectStorePath(home, workspaceRoot string) string {
	abs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		abs = workspaceRoot
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(home, ".kvit-coder", "permissions", hex.EncodeToString(sum[:6])+".json")
}

// GlobalStorePath is where grants that apply everywhere are kept.
func GlobalStorePath(home string) string {
	return filepath.Join(home, ".kvit-coder", "permissions", "global.json")
}
