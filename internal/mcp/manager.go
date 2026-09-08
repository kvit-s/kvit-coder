package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// Manager owns the set of MCP server clients for a session. It connects them at
// startup, exposes their tools as adapters for the registry, and closes them on
// shutdown. A missing or failing server is logged and skipped, never fatal: the
// agent must still run with its built-in tools.
type Manager struct {
	cfg           config.MCPConfig
	workspaceRoot string
	logger        Logger
	tempFileMgr   *tools.TempFileManager
	confirmer     *confirmer

	mu      sync.Mutex
	servers []*serverConn

	// What the last Connect cost, for the question in docs/redesign-mcp.md
	// section 9: is per-turn reconnection worth building a daemon to avoid?
	connectTook   time.Duration
	slowestServer string
}

// ConnectCost reports how long the last Connect took -- the wall clock of its
// slowest server, since they are dialed concurrently -- and which server that
// was. Zero when nothing was dialed.
func (m *Manager) ConnectCost() (time.Duration, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connectTook, m.slowestServer
}

// serverConn is one connected server and the tools it advertised.
type serverConn struct {
	name      string
	transport string
	client    Client
	confirm   string
	timeout   time.Duration
	descs     []ToolDescriptor
	filter    config.MCPToolFilter
}

// ServerStatus is a snapshot of one server for surfacing (e.g. a /mcp view).
type ServerStatus struct {
	Name      string
	Transport string
	Connected bool
	ToolCount int
	Err       string
}

// NewManager creates a manager. workspaceRoot is the default working directory
// for stdio servers that do not set their own cwd. tempFileMgr may be nil (large
// MCP results then truncate inline instead of spilling to a temp file).
func NewManager(cfg config.MCPConfig, workspaceRoot string, logger Logger, tempFileMgr *tools.TempFileManager) *Manager {
	if logger == nil {
		logger = nopLogger{}
	}
	return &Manager{
		cfg:           cfg,
		workspaceRoot: workspaceRoot,
		logger:        logger,
		tempFileMgr:   tempFileMgr,
		confirmer:     newConfirmer(),
	}
}

// SetApprovals decides where "ask_once" answers are remembered. Without it they
// live in process memory, which under one process per turn means they are
// forgotten between instructions. cmd/kvit-coder passes a store in the session
// directory.
func (m *Manager) SetApprovals(a Approvals) {
	m.confirmer.setApprovals(a)
}

// Connect dials every enabled server concurrently, each under its own startup
// timeout, running initialize → tools/list. Failures are logged and skipped; a
// summary error naming the failed servers is returned for the caller to warn on
// (it is informational, never fatal). A no-op when MCP is disabled.
func (m *Manager) Connect(ctx context.Context) error {
	if !m.cfg.Enabled {
		return nil
	}

	type result struct {
		conn *serverConn
		err  error
		name string
		took time.Duration
	}

	var wg sync.WaitGroup
	results := make(chan result, len(m.cfg.Servers))
	seen := make(map[string]bool)

	for _, sc := range m.cfg.Servers {
		if !sc.Enabled {
			continue
		}
		if sc.Name == "" {
			m.logger.Warn("mcp: skipping server with empty name")
			continue
		}
		if seen[sc.Name] {
			m.logger.Warn(fmt.Sprintf("mcp: skipping duplicate server name %q", sc.Name))
			continue
		}
		seen[sc.Name] = true

		sc := sc // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := time.Now()
			conn, err := m.connectOne(ctx, sc)
			results <- result{conn: conn, err: err, name: sc.Name, took: time.Since(started)}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var failed []string
	var conns []*serverConn
	// How long the slowest server took, and which it was. Under one process per
	// turn this is paid on every instruction, so it is the number the daemon in
	// docs/redesign-mcp.md would remove -- worth observing rather than
	// estimating before deciding to build one.
	var slowest time.Duration
	var slowestName string
	for r := range results {
		if r.took > slowest {
			slowest, slowestName = r.took, r.name
		}
		if r.err != nil {
			m.logger.Warn(fmt.Sprintf("mcp: server %q failed to connect after %s: %v", r.name, r.took.Round(time.Millisecond), r.err))
			failed = append(failed, r.name)
			continue
		}
		m.logger.Debug(fmt.Sprintf("mcp: server %q connected in %s, %d tools", r.conn.name, r.took.Round(time.Millisecond), len(r.conn.descs)))
		conns = append(conns, r.conn)
	}
	if slowestName != "" {
		m.connectTook, m.slowestServer = slowest, slowestName
		m.logger.Debug(fmt.Sprintf("mcp: connecting cost %s this turn, slowest was %q", slowest.Round(time.Millisecond), slowestName))
	}

	// Stable order by server name keeps tool ordering deterministic for prompt
	// caching regardless of connect race timing.
	sort.Slice(conns, func(i, j int) bool { return conns[i].name < conns[j].name })

	m.mu.Lock()
	m.servers = conns
	m.mu.Unlock()

	if len(failed) > 0 {
		return fmt.Errorf("failed to connect: %s", strings.Join(failed, ", "))
	}
	return nil
}

// connectOne dials and handshakes a single server under its startup deadline.
func (m *Manager) connectOne(ctx context.Context, sc config.MCPServerConfig) (*serverConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(m.cfg.GetStartupTimeout())*time.Second)
	defer cancel()

	var client Client
	var err error
	transport := sc.GetTransport()
	switch transport {
	case "stdio":
		client, err = newStdioClient(sc.Command, sc.Args, sc.Env, m.resolveCwd(sc.Cwd), m.logger)
	case "http":
		client, err = newHTTPClient(sc.URL, sc.Headers, m.logger)
	default:
		return nil, fmt.Errorf("unknown transport %q", transport)
	}
	if err != nil {
		return nil, err
	}

	if err := client.Initialize(dialCtx); err != nil {
		_ = client.Close()
		return nil, err
	}
	descs, err := client.ListTools(dialCtx)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("tools/list: %w", err)
	}

	return &serverConn{
		name:      sc.Name,
		transport: transport,
		client:    client,
		confirm:   sc.GetConfirm(&m.cfg),
		timeout:   time.Duration(sc.GetCallTimeout(&m.cfg)) * time.Second,
		descs:     descs,
		filter:    sc.Tools,
	}, nil
}

// Tools returns one adapter per discovered tool, after applying each server's
// allow/deny filter. Order is deterministic (server index × 100 + tool index)
// so the system prompt and request body stay byte-stable.
func (m *Manager) Tools() []tools.Tool {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []tools.Tool
	for si, sv := range m.servers {
		ti := 0
		for _, d := range sv.descs {
			if !allowTool(d.Name, sv.filter) {
				m.logger.Debug(fmt.Sprintf("mcp: filtering out tool %q from server %q", d.Name, sv.name))
				continue
			}
			order := si*100 + ti
			out = append(out, newMCPTool(sv.name, d, order, sv.client, m.confirmer, sv.confirm,
				sv.timeout, m.cfg.SanitizeSchemas, m.tempFileMgr))
			ti++
		}
	}
	return out
}

// Status returns a snapshot of connected servers for surfacing.
func (m *Manager) Status() []ServerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ServerStatus, 0, len(m.servers))
	for _, sv := range m.servers {
		count := 0
		for _, d := range sv.descs {
			if allowTool(d.Name, sv.filter) {
				count++
			}
		}
		out = append(out, ServerStatus{
			Name:      sv.name,
			Transport: sv.transport,
			Connected: true,
			ToolCount: count,
		})
	}
	return out
}

// Summary returns a one-line description of connected servers and tool counts,
// or "" when nothing is connected. Suitable for a startup log line.
func (m *Manager) Summary() string {
	st := m.Status()
	if len(st) == 0 {
		return ""
	}
	parts := make([]string, 0, len(st))
	total := 0
	for _, s := range st {
		parts = append(parts, fmt.Sprintf("%s (%d)", s.Name, s.ToolCount))
		total += s.ToolCount
	}
	return fmt.Sprintf("%d MCP tool(s) from %d server(s): %s", total, len(st), strings.Join(parts, ", "))
}

// Close shuts down every connected server. Idempotent.
func (m *Manager) Close() error {
	m.mu.Lock()
	servers := m.servers
	m.servers = nil
	m.mu.Unlock()

	for _, sv := range servers {
		if err := sv.client.Close(); err != nil {
			m.logger.Debug(fmt.Sprintf("mcp: error closing server %q: %v", sv.name, err))
		}
	}
	return nil
}

// resolveCwd determines the working directory for a stdio server. An empty
// configured cwd defaults to the workspace root; a relative cwd is resolved
// against the workspace root; an absolute cwd is used as-is. Returns "" only
// when neither is set, in which case the subprocess inherits kvit-coder's cwd.
func (m *Manager) resolveCwd(cwd string) string {
	if cwd == "" {
		return m.workspaceRoot
	}
	if filepath.IsAbs(cwd) {
		return cwd
	}
	if m.workspaceRoot != "" {
		return filepath.Join(m.workspaceRoot, cwd)
	}
	return cwd
}

// allowTool applies an allow/deny filter to a raw tool name. An empty Allow list
// allows everything; Deny is applied after Allow.
func allowTool(name string, filter config.MCPToolFilter) bool {
	if len(filter.Allow) > 0 {
		found := false
		for _, a := range filter.Allow {
			if a == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, d := range filter.Deny {
		if d == name {
			return false
		}
	}
	return true
}
