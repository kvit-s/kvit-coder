package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// Deferred servers belong to a tool group (see internal/tools/group.go): the
// model does not see their tools until it opens the group, so there is no
// reason to dial them at startup. Two things follow.
//
// The first is that the group has to be able to describe a server's tools
// without the server running, or opening the group would cost the spawn it was
// meant to avoid. The tools/list answer is therefore cached on disk, keyed on
// the server's resolved command line, and the description comes from there.
//
// The second is that the adapters handed to the group must dial on their first
// real call. That is what lazyClient does, and doing it at the Client boundary
// means MCPTool needs no changes: it still holds a Client, still applies the
// per-server confirm policy under the tool's own name, and still enforces the
// per-server call timeout.

// toolListCacheTTL bounds how long a cached tools/list is used before the
// server is asked again. A server pinned to a version never changes its tools;
// one launched as @latest changes them whenever it is updated, and a week is
// short enough that a description cannot drift far while being long enough
// that the cache is almost always the one that answers.
const toolListCacheTTL = 7 * 24 * time.Hour

// toolListCacheFormat is written into every cache file. A file with another
// value is from a version that recorded less -- before format 2 the server's
// instructions were not kept -- and is treated as missing, so the server is
// asked once more rather than its instructions going unseen for a week.
const toolListCacheFormat = 2

// Defer marks servers as belonging to a tool group. Connect skips them, and
// ToolsForServers builds their adapters on demand. Call it before Connect.
func (m *Manager) Defer(names map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deferred = names
}

// isDeferred reports whether a server is held back for a tool group.
func (m *Manager) isDeferred(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deferred[name]
}

// ToolsForServers returns adapters for the named servers' tools, in the order
// the names are given, with the instructions those servers sent (see
// instructions.go). Nothing is dialed here when the tool list is cached: the
// adapters describe themselves from the cache and dial on their first call.
//
// A server that has never been reached and is not in the cache is dialed now,
// because there is no other way to learn what it offers.
func (m *Manager) ToolsForServers(ctx context.Context, names []string) ([]tools.Tool, []tools.GroupNote, error) {
	var out []tools.Tool
	var notes []tools.GroupNote
	var failed []string

	if !m.cfg.Enabled {
		return nil, nil, fmt.Errorf("MCP is switched off (mcp.enabled: false)")
	}

	for si, name := range names {
		sc, ok := m.serverConfig(name)
		if !ok {
			failed = append(failed, fmt.Sprintf("%s (not configured)", name))
			continue
		}
		if !sc.Enabled {
			failed = append(failed, fmt.Sprintf("%s (switched off in mcp.servers)", name))
			continue
		}

		descs, instructions, err := m.descriptorsFor(ctx, sc)
		if err != nil {
			m.logger.Warn(fmt.Sprintf("mcp: server %q: %v", name, err))
			failed = append(failed, fmt.Sprintf("%s (%v)", name, err))
			continue
		}

		client := m.lazyClientFor(sc)
		ti := 0
		for _, d := range descs {
			if !allowTool(d.Name, sc.Tools) {
				continue
			}
			out = append(out, newMCPTool(sc.Name, d, si*100+ti, client, m.confirmer,
				sc.GetConfirm(&m.cfg), time.Duration(sc.GetCallTimeout(&m.cfg))*time.Second,
				m.cfg.SanitizeSchemas, m.tempFileMgr))
			ti++
		}
		// Instructions for a server whose tools are all filtered out would
		// describe tools the model cannot run.
		if instructions != "" && ti > 0 {
			notes = append(notes, tools.GroupNote{Source: "MCP server " + sc.Name, Text: instructions})
		}
	}

	if len(out) == 0 && len(failed) > 0 {
		return nil, nil, fmt.Errorf("no tools available: %s", strings.Join(failed, "; "))
	}
	return out, notes, nil
}

// serverConfig finds a configured server by name.
func (m *Manager) serverConfig(name string) (config.MCPServerConfig, bool) {
	for _, sc := range m.cfg.Servers {
		if sc.Name == name {
			return sc, true
		}
	}
	return config.MCPServerConfig{}, false
}

// ServerUsable reports whether a server could be reached at all: MCP is on, the
// server is configured, and it is not switched off. cmd/kvit-coder asks before
// building a tool group, so a group left in the configuration with its server
// disabled is not registered as a tool that can only fail.
func (m *Manager) ServerUsable(name string) bool {
	if !m.cfg.Enabled {
		return false
	}
	sc, ok := m.serverConfig(name)
	return ok && sc.Enabled
}

// lazyClientFor returns the shared lazy client for a server, creating it on
// first use. Sharing matters: two adapters for the same server must dial the
// same process, not one each.
func (m *Manager) lazyClientFor(sc config.MCPServerConfig) *lazyClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lazy == nil {
		m.lazy = make(map[string]*lazyClient)
	}
	if c, ok := m.lazy[sc.Name]; ok {
		return c
	}
	c := &lazyClient{
		name: sc.Name,
		dial: func(ctx context.Context) (Client, error) {
			started := time.Now()
			conn, err := m.connectOne(ctx, sc)
			if err != nil {
				return nil, err
			}
			m.logger.Debug(fmt.Sprintf("mcp: server %q dialed on demand in %s, %d tools",
				sc.Name, time.Since(started).Round(time.Millisecond), len(conn.descs)))
			// A real answer supersedes whatever was cached.
			m.writeToolListCache(sc, conn.descs, conn.instructions)
			return conn.client, nil
		},
	}
	m.lazy[sc.Name] = c
	return c
}

// descriptorsFor returns what a server offers and the instructions it sent,
// from the on-disk cache when it is there and recent, and from the server
// itself otherwise.
func (m *Manager) descriptorsFor(ctx context.Context, sc config.MCPServerConfig) ([]ToolDescriptor, string, error) {
	if cached, ok := m.readToolListCache(sc); ok {
		return cached.Tools, cached.Instructions, nil
	}

	client := m.lazyClientFor(sc)
	if _, err := client.get(ctx); err != nil {
		return nil, "", err
	}
	if cached, ok := m.readToolListCache(sc); ok {
		return cached.Tools, cached.Instructions, nil
	}
	// The dial worked but the cache could not be written (read-only home, say).
	// Ask the live connection instead of failing.
	descs, err := client.ListTools(ctx)
	return descs, client.Instructions(), err
}

// toolListCache is what is written per server.
type toolListCache struct {
	Format       int              `json:"format"`
	Server       string           `json:"server"`
	Key          string           `json:"key"`
	Written      time.Time        `json:"written"`
	Tools        []ToolDescriptor `json:"tools"`
	Instructions string           `json:"instructions,omitempty"`
}

// toolListCachePath names the cache file for a server. The key is the resolved
// command line rather than the server's name, so editing a server's arguments
// invalidates its cache without anything having to notice.
//
// Configured headers are deliberately not part of the key: they can hold
// credentials, and they do not decide what a server offers.
func toolListCachePath(sc config.MCPServerConfig) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	parts := []string{
		sc.Name,
		sc.GetTransport(),
		sc.Command,
		strings.Join(sc.Args, "\x00"),
		sc.Cwd,
		sc.URL,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x01")))
	key := hex.EncodeToString(sum[:12])
	return filepath.Join(home, ".kvit-coder", "mcp", "tools-"+key+".json"), key, nil
}

// readToolListCache returns a cached tools/list, with the server's
// instructions, when one is present, in the current format and within the
// time-to-live. Any problem reading it is a cache miss, never an error: the
// server itself is always the fallback.
func (m *Manager) readToolListCache(sc config.MCPServerConfig) (toolListCache, bool) {
	path, key, err := toolListCachePath(sc)
	if err != nil {
		return toolListCache{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return toolListCache{}, false
	}
	var cached toolListCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return toolListCache{}, false
	}
	if cached.Key != key || len(cached.Tools) == 0 {
		return toolListCache{}, false
	}
	if cached.Format != toolListCacheFormat {
		m.logger.Debug(fmt.Sprintf("mcp: cached tool list for %q is from an older version, will re-dial", sc.Name))
		return toolListCache{}, false
	}
	if time.Since(cached.Written) > toolListCacheTTL {
		m.logger.Debug(fmt.Sprintf("mcp: cached tool list for %q is stale, will re-dial", sc.Name))
		return toolListCache{}, false
	}
	return cached, true
}

// writeToolListCache records what a server answered. Failures are logged and
// ignored: the cache is an optimisation, and a run with an unwritable home
// directory should dial every turn rather than stop working.
func (m *Manager) writeToolListCache(sc config.MCPServerConfig, descs []ToolDescriptor, instructions string) {
	if len(descs) == 0 {
		return
	}
	path, key, err := toolListCachePath(sc)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		m.logger.Debug(fmt.Sprintf("mcp: cannot create tool list cache directory: %v", err))
		return
	}
	data, err := json.MarshalIndent(toolListCache{
		Format:       toolListCacheFormat,
		Server:       sc.Name,
		Key:          key,
		Written:      time.Now(),
		Tools:        descs,
		Instructions: instructions,
	}, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		m.logger.Debug(fmt.Sprintf("mcp: cannot write tool list cache: %v", err))
	}
}

// lazyClient is a Client that dials on first use. Everything it implements
// forwards to the real connection once there is one; the dial happens under
// whatever deadline the caller brought, which for a tool call is the server's
// own call timeout.
type lazyClient struct {
	name string
	dial func(context.Context) (Client, error)

	mu   sync.Mutex
	real Client
	err  error
}

// get dials once. A failed dial is remembered, so a server that is not going to
// start does not cost a spawn attempt on every call of the turn.
func (l *lazyClient) get(ctx context.Context) (Client, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.real != nil || l.err != nil {
		return l.real, l.err
	}
	client, err := l.dial(ctx)
	if err != nil {
		l.err = fmt.Errorf("could not start MCP server %q: %w", l.name, err)
		return nil, l.err
	}
	l.real = client
	return l.real, nil
}

// Initialize is a no-op: the dial performed by get runs the handshake.
func (l *lazyClient) Initialize(ctx context.Context) error {
	_, err := l.get(ctx)
	return err
}

// Instructions are the connected server's, or "" before it has been dialed.
// It never dials: the cached copy is what describes an undialed server.
func (l *lazyClient) Instructions() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.real == nil {
		return ""
	}
	return l.real.Instructions()
}

func (l *lazyClient) ListTools(ctx context.Context) ([]ToolDescriptor, error) {
	client, err := l.get(ctx)
	if err != nil {
		return nil, err
	}
	return client.ListTools(ctx)
}

func (l *lazyClient) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	client, err := l.get(ctx)
	if err != nil {
		return nil, err
	}
	return client.CallTool(ctx, name, args)
}

// Close shuts down the connection if one was ever made.
func (l *lazyClient) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.real == nil {
		return nil
	}
	client := l.real
	l.real = nil
	return client.Close()
}
