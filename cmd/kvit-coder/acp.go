package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kvit-s/kvit-coder/internal/acp"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/mcp"
	"github.com/kvit-s/kvit-coder/internal/repl"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// `kvit-coder acp` is the agent speaking the Agent Client Protocol (version 1)
// on its standard input and output, for an editor or for Kvit Works to drive.
// It runs the same turn as `-p`: prepareTurn sets it up, repl.RunTurn runs the
// loop and records it in the session, and this file reports what the loop does
// as the protocol's updates and answers the client's requests.
//
// A session is a kvit-coder session, and its id is the session's name. Each
// prompt is set up afresh, as a `-p` turn is, so a client that starts one
// process per turn and one that sends every prompt to one process get the
// same behaviour. Prompts run one at a time in a process, because the
// questions a tool puts and the run id sent to the endpoint are the process's.
//
// Nothing but protocol messages goes to standard output. What a person would
// read at a terminal goes to standard error, which clients keep as the agent's
// log.

// acpProtocolVersion is the protocol version this side speaks.
const acpProtocolVersion = 1

type acpOptions struct {
	ConfigPath string
	LogFile    string
	// Log is where the writer's output goes: standard error.
	Log io.Writer
}

func runACP(args []string) int {
	fs := flag.NewFlagSet("acp", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config file (default: the usual places, as for -p)")
	logFile := fs.String("log", "~/.kvit-coder/logs/kvit-coder.log", "log file path (empty to disable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	in, out, err := claimStdio()
	if err != nil {
		fmt.Fprintf(os.Stderr, "kvit-coder acp: %v\n", err)
		return 1
	}
	agent := newACPAgent(acpOptions{ConfigPath: *configPath, LogFile: *logFile, Log: os.Stderr})
	if err := agent.serve(context.Background(), in, out); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "kvit-coder acp: %v\n", err)
		return 1
	}
	return 0
}

type acpAgent struct {
	opts acpOptions
	conn *acp.Conn

	// runIDPinned is set when the caller fixed the endpoint's run id in the
	// environment; otherwise each prompt sets it from its session.
	runIDPinned bool

	// forms is set when the client declared that it shows questions as
	// forms (clientCapabilities.elicitation.form).
	forms bool

	mu       sync.Mutex
	sessions map[string]*acpSession
	// turnMu lets one prompt run at a time in this process.
	turnMu sync.Mutex
}

func newACPAgent(opts acpOptions) *acpAgent {
	if opts.Log == nil {
		opts.Log = os.Stderr
	}
	return &acpAgent{
		opts:        opts,
		runIDPinned: os.Getenv(session.RunIDVar) != "",
		sessions:    map[string]*acpSession{},
	}
}

func (a *acpAgent) serve(ctx context.Context, in io.Reader, out io.Writer) error {
	a.conn = acp.NewConn(in, out, a)
	return a.conn.Serve(ctx)
}

// acpSession is what the client has said about one session in this process.
type acpSession struct {
	id          string
	cwd         string
	mcpServers  []config.MCPServerConfig
	extraPrompt string
	policy      *kvitPolicy
	// model is the catalogue entry chosen, empty for the config's default;
	// effort is the effort chosen, nil for the entry's default.
	model  string
	effort *string

	mu sync.Mutex
	// cancel ends the running prompt; nil while none runs.
	cancel    context.CancelFunc
	cancelled bool
	// allowed holds the questions answered "for this session".
	allowed map[string]bool
	title   string
}

// ---- requests ------------------------------------------------------------

func (a *acpAgent) HandleRequest(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		return a.initialize(params)
	case "authenticate":
		return struct{}{}, nil
	case "session/new":
		return a.newSession(params)
	case "session/resume":
		return a.reopenSession(ctx, params, false)
	case "session/load":
		return a.reopenSession(ctx, params, true)
	case "session/list":
		return a.listSessions(params)
	case "session/close":
		return a.closeSession(params)
	case "session/set_config_option":
		return a.setConfigOption(params)
	case "session/prompt":
		return a.prompt(ctx, params)
	}
	return nil, acp.Errorf(acp.CodeMethodNotFound, "method not found: %s", method)
}

func (a *acpAgent) HandleNotification(method string, params json.RawMessage) {
	if method != "session/cancel" {
		return
	}
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	if s := a.session(p.SessionID); s != nil {
		s.cancelPrompt()
	}
}

func (s *acpSession) cancelPrompt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancelled = true
		s.cancel()
	}
}

func decodeParams(params json.RawMessage, into any) error {
	if len(params) == 0 {
		return acp.Errorf(acp.CodeInvalidParams, "missing params")
	}
	if err := json.Unmarshal(params, into); err != nil {
		return acp.Errorf(acp.CodeInvalidParams, "invalid params: %v", err)
	}
	return nil
}

func (a *acpAgent) initialize(params json.RawMessage) (any, error) {
	var p struct {
		ProtocolVersion    int `json:"protocolVersion"`
		ClientCapabilities struct {
			Elicitation struct {
				Form json.RawMessage `json:"form"`
			} `json:"elicitation"`
		} `json:"clientCapabilities"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	form := p.ClientCapabilities.Elicitation.Form
	a.mu.Lock()
	a.forms = len(form) > 0 && string(form) != "null"
	a.mu.Unlock()
	return map[string]any{
		"protocolVersion": acpProtocolVersion,
		"agentCapabilities": map[string]any{
			"loadSession": true,
			"promptCapabilities": map[string]any{
				"image":           true,
				"audio":           false,
				"embeddedContext": true,
			},
			"mcpCapabilities": map[string]any{
				"http": true,
				"sse":  false,
			},
			"sessionCapabilities": map[string]any{
				"resume": map[string]any{},
				"list":   map[string]any{},
				"close":  map[string]any{},
			},
		},
		"authMethods": []any{},
		"agentInfo": map[string]any{
			"name":    "kvit-coder",
			"title":   "kvit-coder",
			"version": version,
		},
	}, nil
}

// sessionParams are the fields session/new, session/resume and session/load
// share.
type sessionParams struct {
	SessionID  string            `json:"sessionId"`
	Cwd        string            `json:"cwd"`
	MCPServers []acpMCPServer    `json:"mcpServers"`
	Meta       *sessionParamMeta `json:"_meta"`
}

type sessionParamMeta struct {
	// SystemPrompt is a string, or an object whose `append` is the text.
	SystemPrompt json.RawMessage `json:"systemPrompt"`
	Kvit         *struct {
		Policy *kvitPolicy `json:"policy"`
	} `json:"kvit"`
}

func (m *sessionParamMeta) systemPrompt() string {
	if m == nil || len(m.SystemPrompt) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(m.SystemPrompt, &text) == nil {
		return text
	}
	var obj struct {
		Append string `json:"append"`
	}
	if json.Unmarshal(m.SystemPrompt, &obj) == nil {
		return obj.Append
	}
	return ""
}

func (m *sessionParamMeta) policy() *kvitPolicy {
	if m == nil || m.Kvit == nil {
		return nil
	}
	return m.Kvit.Policy
}

type acpEnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// acpMCPServer is a Model Context Protocol server the client wants the
// session to have: over standard input and output when it names a command,
// over HTTP when its type is "http".
type acpMCPServer struct {
	Type    string           `json:"type"`
	Name    string           `json:"name"`
	Command string           `json:"command"`
	Args    []string         `json:"args"`
	Env     []acpEnvVariable `json:"env"`
	URL     string           `json:"url"`
	Headers []acpEnvVariable `json:"headers"`
}

func (p sessionParams) servers() ([]config.MCPServerConfig, error) {
	var out []config.MCPServerConfig
	for _, s := range p.MCPServers {
		server := config.MCPServerConfig{Name: s.Name, Enabled: true}
		switch {
		case s.Type == "http":
			server.Transport = "http"
			server.URL = s.URL
			for _, h := range s.Headers {
				server.Headers = append(server.Headers, h.Name+"="+h.Value)
			}
		case s.Type == "" || s.Type == "stdio":
			server.Transport = "stdio"
			server.Command = s.Command
			server.Args = s.Args
			for _, e := range s.Env {
				server.Env = append(server.Env, e.Name+"="+e.Value)
			}
		default:
			return nil, acp.Errorf(acp.CodeInvalidParams, "MCP server %q: transport %q is not supported", s.Name, s.Type)
		}
		out = append(out, server)
	}
	return out, nil
}

func (a *acpAgent) session(id string) *acpSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessions[id]
}

// adopt records what the client said about a session, keeping what a
// session already open in this process had chosen.
func (a *acpAgent) adopt(id string, p sessionParams) (*acpSession, error) {
	if !filepath.IsAbs(p.Cwd) {
		return nil, acp.Errorf(acp.CodeInvalidParams, "cwd must be an absolute path, not %q", p.Cwd)
	}
	servers, err := p.servers()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.sessions[id]
	if s == nil {
		s = &acpSession{id: id, allowed: map[string]bool{}}
		a.sessions[id] = s
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cwd = filepath.Clean(p.Cwd)
	s.mcpServers = servers
	s.extraPrompt = p.Meta.systemPrompt()
	s.policy = p.Meta.policy()
	return s, nil
}

func (a *acpAgent) newSession(params json.RawMessage) (any, error) {
	var p sessionParams
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(p.Cwd) {
		return nil, acp.Errorf(acp.CodeInvalidParams, "cwd must be an absolute path, not %q", p.Cwd)
	}
	if _, err := p.servers(); err != nil {
		return nil, err
	}
	mgr, err := session.NewManager()
	if err != nil {
		return nil, err
	}
	name := mgr.GenerateSessionName()
	sess, err := mgr.Open(name)
	if err != nil {
		return nil, err
	}
	sess.Meta().Workspace = filepath.Clean(p.Cwd)
	if err := sess.SaveMeta(); err != nil {
		return nil, err
	}
	s, err := a.adopt(name, p)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"sessionId":     s.id,
		"configOptions": a.configOptions(s),
	}, nil
}

func (a *acpAgent) reopenSession(ctx context.Context, params json.RawMessage, replay bool) (any, error) {
	var p sessionParams
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	mgr, err := session.NewManager()
	if err != nil {
		return nil, err
	}
	if p.SessionID == "" || strings.ContainsAny(p.SessionID, `/\`) || !mgr.SessionExists(p.SessionID) {
		return nil, acp.Errorf(acp.CodeInvalidParams, "no session %q", p.SessionID)
	}
	s, err := a.adopt(p.SessionID, p)
	if err != nil {
		return nil, err
	}
	if replay {
		sess, err := mgr.Open(p.SessionID)
		if err != nil {
			return nil, err
		}
		messages, err := sess.Load()
		if err != nil {
			return nil, err
		}
		r := &turnReporter{a: a, s: s, workspace: s.cwd, calls: map[string]*reportedCall{}}
		r.replay(messages)
		r.sendTitle(sess.Meta().Title)
	}
	return map[string]any{"configOptions": a.configOptions(s)}, nil
}

func (a *acpAgent) listSessions(params json.RawMessage) (any, error) {
	var p struct {
		Cwd string `json:"cwd"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, acp.Errorf(acp.CodeInvalidParams, "invalid params: %v", err)
		}
	}
	mgr, err := session.NewManager()
	if err != nil {
		return nil, err
	}
	infos, err := mgr.ListSessions()
	if err != nil {
		return nil, err
	}
	sessions := []map[string]any{}
	for _, info := range infos {
		meta := mgr.ReadMeta(info.Name)
		if meta == nil || meta.Workspace == "" {
			continue
		}
		if p.Cwd != "" && filepath.Clean(meta.Workspace) != filepath.Clean(p.Cwd) {
			continue
		}
		entry := map[string]any{
			"sessionId": info.Name,
			"cwd":       meta.Workspace,
			"updatedAt": info.ModTime.UTC().Format(time.RFC3339),
		}
		if info.Title != "" {
			entry["title"] = info.Title
		}
		sessions = append(sessions, entry)
	}
	return map[string]any{"sessions": sessions}, nil
}

func (a *acpAgent) closeSession(params json.RawMessage) (any, error) {
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	a.mu.Lock()
	s := a.sessions[p.SessionID]
	delete(a.sessions, p.SessionID)
	a.mu.Unlock()
	if s != nil {
		s.cancelPrompt()
	}
	return struct{}{}, nil
}

// ---- configuration options -------------------------------------------------

func (a *acpAgent) loadConfig() (*config.Config, error) {
	path, _, err := config.ResolveNamed(a.opts.ConfigPath, config.DefaultConfigName)
	if err != nil {
		return nil, err
	}
	return config.Load(path)
}

type acpConfigValue struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type acpConfigOption struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description,omitempty"`
	Category     string           `json:"category,omitempty"`
	Type         string           `json:"type"`
	CurrentValue string           `json:"currentValue"`
	Options      []acpConfigValue `json:"options"`
}

func modelID(entry config.ModelEntry, index int) string {
	if entry.ID != "" {
		return entry.ID
	}
	return fmt.Sprint(index + 1)
}

// chosenEntry is the catalogue entry the session runs on.
func (s *acpSession) chosenEntry(cfg *config.Config) (config.ModelEntry, int) {
	list := cfg.ModelList()
	if s.model != "" {
		for i, e := range list {
			if modelID(e, i) == s.model {
				return e, i
			}
		}
	}
	idx := cfg.DefaultModelIndex()
	if idx < 0 || idx >= len(list) {
		idx = 0
	}
	return list[idx], idx
}

// configOptions are the session's two settings: the model, from the config's
// catalogue, and the reasoning effort, for a model that takes one.
func (a *acpAgent) configOptions(s *acpSession) []acpConfigOption {
	cfg, err := a.loadConfig()
	if err != nil {
		return []acpConfigOption{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := cfg.ModelList()
	entry, idx := s.chosenEntry(cfg)
	model := acpConfigOption{
		ID:           "model",
		Name:         "Model",
		Category:     "model",
		Type:         "select",
		CurrentValue: modelID(entry, idx),
	}
	for i, e := range list {
		name := e.Name
		if name == "" {
			name = e.Model
		}
		model.Options = append(model.Options, acpConfigValue{Value: modelID(e, i), Name: name})
	}
	options := []acpConfigOption{model}
	if menu := cfg.EffortOptions(entry); len(menu) > 0 {
		current := cfg.DefaultEffort(entry)
		if s.effort != nil {
			current = *s.effort
		}
		effort := acpConfigOption{
			ID:           "effort",
			Name:         "Reasoning effort",
			Category:     "thought_level",
			Type:         "select",
			CurrentValue: current,
		}
		for _, o := range menu {
			effort.Options = append(effort.Options, acpConfigValue{Value: o.Value, Name: o.Display()})
		}
		options = append(options, effort)
	}
	return options
}

func (a *acpAgent) setConfigOption(params json.RawMessage) (any, error) {
	var p struct {
		SessionID string `json:"sessionId"`
		ConfigID  string `json:"configId"`
		Value     string `json:"value"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	s := a.session(p.SessionID)
	if s == nil {
		return nil, acp.Errorf(acp.CodeInvalidParams, "no session %q in this process", p.SessionID)
	}
	cfg, err := a.loadConfig()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	switch p.ConfigID {
	case "model":
		found := false
		for i, e := range cfg.ModelList() {
			if modelID(e, i) == p.Value {
				found = true
			}
		}
		if !found {
			s.mu.Unlock()
			return nil, acp.Errorf(acp.CodeInvalidParams, "no model %q in the config", p.Value)
		}
		// A new model starts on its own default effort.
		s.model, s.effort = p.Value, nil
	case "effort":
		entry, _ := s.chosenEntry(cfg)
		value, err := cfg.ResolveEffort(entry, p.Value)
		if err != nil {
			s.mu.Unlock()
			return nil, acp.Errorf(acp.CodeInvalidParams, "%v", err)
		}
		s.effort = &value
	default:
		s.mu.Unlock()
		return nil, acp.Errorf(acp.CodeInvalidParams, "no setting %q", p.ConfigID)
	}
	s.mu.Unlock()
	return map[string]any{"configOptions": a.configOptions(s)}, nil
}

// ---- the rules Kvit Works sends -------------------------------------------

// kvitPolicy is what a turn may do, as Kvit Works works it out
// (src/agent/turnrules.h there) and sends under _meta.kvit.policy: the roots
// that may be read, the roots that may be written, paths denied inside those,
// and whether shell commands may run. Paths are absolute; "/" is the whole
// filesystem.
type kvitPolicy struct {
	Readable []string       `json:"readable"`
	Writable []string       `json:"writable"`
	Denied   []policyDenial `json:"denied"`
	Shell    bool           `json:"shell"`
}

// policyDenial is one denied path and which accesses it denies. A path denied
// for reading is denied for everything; one denied only for writing may
// still be read.
type policyDenial struct {
	Path  string `json:"path"`
	Read  bool   `json:"read"`
	Write bool   `json:"write"`
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// apply lays the rules over the person's config for one turn. They replace
// the person's own path lists, so a turn that only reads cannot write through
// a folder the person's config opens; the session's own directory is added
// after this, as for every turn, since it is the agent's scratch space.
func (p *kvitPolicy) apply(cfg *config.Config, cwd string) {
	ws := &cfg.Workspace
	ws.Root = cwd
	ws.PathSafetyMode = "block"
	ws.AllowOutsideWorkspace = false
	ws.AllowedReadPaths = append([]string(nil), p.Readable...)
	ws.AllowedPaths = append([]string(nil), p.Writable...)
	ws.DeniedPaths = append([]string(nil), ws.DeniedPaths...)
	for _, d := range p.Denied {
		switch {
		case d.Read:
			ws.DeniedPaths = append(ws.DeniedPaths, d.Path)
		case d.Write:
			ws.DeniedWritePaths = append(ws.DeniedWritePaths, d.Path)
		}
	}
	ws.ReadOnlyRoot = true
	for _, root := range p.Writable {
		if within(root, cwd) {
			ws.ReadOnlyRoot = false
		}
	}
	if len(p.Writable) == 0 {
		cfg.Tools.Edit.Enabled = false
	}
	if !p.Shell {
		cfg.Tools.Shell.Enabled = false
		cfg.Tools.Procs.Enabled = false
	}
}

// ---- a prompt ----------------------------------------------------------------

type acpContentBlock struct {
	Type     string               `json:"type"`
	Text     string               `json:"text"`
	Data     string               `json:"data"`
	MimeType string               `json:"mimeType"`
	URI      string               `json:"uri"`
	Name     string               `json:"name"`
	Resource *acpEmbeddedResource `json:"resource"`
}

type acpEmbeddedResource struct {
	URI      string `json:"uri"`
	Text     string `json:"text"`
	MimeType string `json:"mimeType"`
}

type acpImage struct {
	data     []byte
	mimeType string
}

// promptText joins the prompt's blocks into the text the model is given, and
// returns its pictures apart.
func promptText(blocks []acpContentBlock) (string, []acpImage, error) {
	var parts []string
	var images []acpImage
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "image":
			data, err := base64.StdEncoding.DecodeString(b.Data)
			if err != nil {
				return "", nil, acp.Errorf(acp.CodeInvalidParams, "an image block is not base64: %v", err)
			}
			images = append(images, acpImage{data: data, mimeType: b.MimeType})
		case "resource_link":
			parts = append(parts, fmt.Sprintf("[file: %s]", filePath(b.URI)))
		case "resource":
			if b.Resource != nil && b.Resource.Text != "" {
				parts = append(parts, fmt.Sprintf("<file path=%q>\n%s\n</file>", filePath(b.Resource.URI), b.Resource.Text))
			}
		default:
			return "", nil, acp.Errorf(acp.CodeInvalidParams, "content of type %q is not supported", b.Type)
		}
	}
	return strings.Join(parts, "\n"), images, nil
}

func filePath(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}

func imageExtension(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func (a *acpAgent) prompt(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		SessionID string            `json:"sessionId"`
		Prompt    []acpContentBlock `json:"prompt"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	s := a.session(p.SessionID)
	if s == nil {
		return nil, acp.Errorf(acp.CodeInvalidParams, "no session %q in this process: send session/new, session/resume or session/load first", p.SessionID)
	}
	text, images, err := promptText(p.Prompt)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return nil, acp.Errorf(acp.CodeInvalidRequest, "a prompt is already running in session %q", s.id)
	}
	s.cancel, s.cancelled = cancel, false
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
	}()

	a.turnMu.Lock()
	defer a.turnMu.Unlock()
	if ctx.Err() != nil {
		return map[string]any{"stopReason": "cancelled"}, nil
	}
	return a.runPrompt(ctx, s, text, images)
}

func (a *acpAgent) runPrompt(ctx context.Context, s *acpSession, text string, images []acpImage) (any, error) {
	s.mu.Lock()
	model, cwd, extra, policy := s.model, s.cwd, s.extraPrompt, s.policy
	servers := append([]config.MCPServerConfig(nil), s.mcpServers...)
	effort := &stringFlag{}
	if s.effort != nil {
		effort = &stringFlag{set: true, val: *s.effort}
	}
	s.mu.Unlock()

	if !a.runIDPinned {
		os.Setenv(session.RunIDVar, session.RunIDFor(s.id))
	}

	writer := ui.NewWriter(0)
	writer.SetHeadless(true)
	writer.SetStdout(a.opts.Log)
	writer.SetStderr(a.opts.Log)
	writer.SetStdoutIsTerminal(false)
	writer.SetStderrIsTerminal(false)

	turn, err := prepareTurn(turnOptions{
		Parent:      ctx,
		Writer:      writer,
		LogFile:     a.opts.LogFile,
		ConfigPath:  a.opts.ConfigPath,
		ConfigName:  config.DefaultConfigName,
		ModelRef:    model,
		Effort:      effort,
		SessionName: s.id,
		Workspace:   cwd,
		ExtraPrompt: extra,
		Configure: func(cfg *config.Config) {
			if policy != nil {
				policy.apply(cfg, cwd)
			}
			for _, server := range servers {
				if !hasMCPServer(cfg, server.Name) {
					cfg.MCP.Servers = append(cfg.MCP.Servers, server)
					cfg.MCP.Enabled = true
				}
			}
		},
	})
	if err != nil {
		return nil, err
	}
	defer turn.Close()

	r := &turnReporter{
		a:         a,
		s:         s,
		ctx:       ctx,
		workspace: turn.cfg.Workspace.Root,
		registry:  turn.registry,
		toolCtx:   turn.toolCtx,
		plans:     turn.planManager,
		title:     func() string { return turn.sess.Meta().Title },
		calls:     map[string]*reportedCall{},
	}
	turn.toolCtx.SetPermissionAsker(r.askShell)
	a.mu.Lock()
	forms := a.forms
	a.mu.Unlock()
	if forms {
		turn.toolCtx.SetFormAsker(r.askForm)
	}
	config.SetLinePrompter(r.askLine)
	mcp.SetLinePrompter(r.askLine)
	defer config.SetLinePrompter(nil)
	defer mcp.SetLinePrompter(nil)
	turn.runner.SetToolStart(r.toolStart)

	var imagePaths []string
	for i, img := range images {
		path := filepath.Join(turn.sess.TmpDir(), fmt.Sprintf("prompt-image-%d-%d%s", time.Now().UnixNano(), i, imageExtension(img.mimeType)))
		if err := os.WriteFile(path, img.data, 0o600); err != nil {
			return nil, fmt.Errorf("cannot keep a picture of the prompt: %v", err)
		}
		imagePaths = append(imagePaths, path)
	}

	instructions, err := repl.LoadProjectInstructions(turn.cfg, cwd)
	if err != nil {
		return nil, err
	}

	result, err := repl.RunTurn(turn.runCtx, turn.runner, writer, turn.cfg, turn.systemPrompt, text, false, turn.sess, instructions, imagePaths, r.observe)
	r.sendTitle(turn.sess.Meta().Title)
	if err != nil {
		return nil, err
	}
	r.sendUsage(turn.cfg.LLM.Context, result.Stats.MaxContextUsed, turn.sess.Meta().Cost)

	s.mu.Lock()
	cancelled := s.cancelled
	s.mu.Unlock()
	stop := "end_turn"
	switch {
	case cancelled || result.Cancelled:
		stop = "cancelled"
	case result.Failure != nil:
		return nil, result.Failure
	case result.BudgetExhausted:
		stop = "max_turn_requests"
	}
	st := result.Stats
	return map[string]any{
		"stopReason": stop,
		"usage": map[string]any{
			"inputTokens":      st.TotalPromptTokens,
			"outputTokens":     st.TotalCompletionTokens,
			"cachedReadTokens": st.TotalCacheReadTokens,
			"totalTokens":      st.TotalPromptTokens + st.TotalCompletionTokens,
		},
	}, nil
}

func hasMCPServer(cfg *config.Config, name string) bool {
	for _, s := range cfg.MCP.Servers {
		if s.Name == name {
			return true
		}
	}
	return false
}

// ---- asking the client --------------------------------------------------------

type permissionOutcome int

const (
	outcomeRefused permissionOutcome = iota
	outcomeOnce
	outcomeSession
	outcomeCancelled
)

var permissionOptions = []map[string]string{
	{"optionId": "allow_once", "name": "Allow once", "kind": "allow_once"},
	{"optionId": "allow_always", "name": "Allow for this session", "kind": "allow_always"},
	{"optionId": "reject_once", "name": "Refuse", "kind": "reject_once"},
}

// requestPermission asks the client about the tool call running now. The two
// answers that would write a grant for every later session are not offered,
// so a client answering on its own cannot widen what the next session may
// do.
func (r *turnReporter) requestPermission(title string) permissionOutcome {
	call := map[string]any{"toolCallId": r.current, "title": title}
	if c := r.calls[r.current]; c != nil {
		call["kind"] = c.kind
		if len(c.locations) > 0 {
			call["locations"] = c.locations
		}
	}
	if r.current == "" {
		call["toolCallId"] = "permission-" + newID()
	}
	var answer struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	err := r.a.conn.Call(r.ctx, "session/request_permission", map[string]any{
		"sessionId": r.s.id,
		"toolCall":  call,
		"options":   permissionOptions,
	}, &answer)
	if err != nil {
		if r.ctx.Err() != nil {
			return outcomeCancelled
		}
		return outcomeRefused
	}
	switch {
	case answer.Outcome.Outcome == "cancelled":
		return outcomeCancelled
	case answer.Outcome.OptionID == "allow_once":
		return outcomeOnce
	case answer.Outcome.OptionID == "allow_always":
		return outcomeSession
	}
	return outcomeRefused
}

// askShell answers a shell command's permission question.
func (r *turnReporter) askShell(req tools.PermissionRequest) tools.PermissionAnswer {
	switch r.requestPermission(req.Title) {
	case outcomeOnce:
		return tools.PermissionAllowOnce
	case outcomeSession:
		return tools.PermissionAllowSession
	}
	return tools.PermissionRefused
}

// askLine answers the yes-or-no questions that are put as a line of text: a
// path outside the workspace, and a Model Context Protocol tool not yet
// allowed. An answer for the session is kept for the same question.
func (r *turnReporter) askLine(prompt string) (string, bool) {
	key := strings.TrimSpace(prompt)
	r.s.mu.Lock()
	allowed := r.s.allowed[key]
	r.s.mu.Unlock()
	if allowed {
		return "y", true
	}
	switch r.requestPermission(questionTitle(prompt)) {
	case outcomeOnce:
		return "y", true
	case outcomeSession:
		r.s.mu.Lock()
		r.s.allowed[key] = true
		r.s.mu.Unlock()
		return "y", true
	case outcomeCancelled:
		return "", false
	}
	return "n", true
}

// askForm puts the Question tool's questions to the client as one form
// (elicitation/create), a field per question: a choice for a question with
// options, several for one that takes several, and text otherwise. Beside
// each choice is a field `<name>_other` for an answer in the person's own
// words, which is always possible at a terminal. It waits for the answer with
// no time limit; the prompt being cancelled ends the wait.
func (r *turnReporter) askForm(ctx context.Context, questions []tools.FormQuestion) (tools.FormReply, error) {
	properties := map[string]any{}
	names := make([]string, len(questions))
	for i, q := range questions {
		name := fmt.Sprintf("q%02d", i+1)
		names[i] = name
		title := q.Header
		if title == "" {
			title = fmt.Sprintf("Question %d", i+1)
		}
		field := map[string]any{"title": title, "description": q.Question}
		if len(q.Options) == 0 {
			field["type"] = "string"
		} else {
			entries := make([]map[string]any, 0, len(q.Options))
			for _, o := range q.Options {
				entry := map[string]any{"const": o.Label, "title": o.Label}
				if o.Description != "" {
					entry["description"] = o.Description
				}
				entries = append(entries, entry)
			}
			if q.Multi {
				field["type"] = "array"
				field["items"] = map[string]any{"anyOf": entries}
			} else {
				field["type"] = "string"
				field["oneOf"] = entries
			}
			properties[name+"_other"] = map[string]any{
				"type":        "string",
				"title":       "Other",
				"description": "An answer in your own words.",
			}
		}
		properties[name] = field
	}
	message := "A question from the agent."
	if len(questions) == 1 {
		message = questions[0].Question
	} else if len(questions) > 1 {
		message = fmt.Sprintf("%d questions from the agent.", len(questions))
	}
	params := map[string]any{
		"sessionId":       r.s.id,
		"mode":            "form",
		"message":         message,
		"requestedSchema": map[string]any{"type": "object", "properties": properties},
	}
	if r.current != "" {
		params["toolCallId"] = r.current
	}
	var answer struct {
		Action  string                     `json:"action"`
		Content map[string]json.RawMessage `json:"content"`
	}
	if err := r.a.conn.Call(ctx, "elicitation/create", params, &answer); err != nil {
		return tools.FormReply{}, err
	}
	reply := tools.FormReply{Action: answer.Action}
	if answer.Action != "accept" {
		return reply, nil
	}
	reply.Chosen = make([][]string, len(questions))
	reply.Typed = make([]string, len(questions))
	for i, name := range names {
		if raw, ok := answer.Content[name]; ok {
			var one string
			var several []string
			switch {
			case json.Unmarshal(raw, &one) == nil:
				if len(questions[i].Options) == 0 {
					reply.Typed[i] = one
				} else if one != "" {
					reply.Chosen[i] = []string{one}
				}
			case json.Unmarshal(raw, &several) == nil:
				reply.Chosen[i] = several
			}
		}
		if raw, ok := answer.Content[name+"_other"]; ok {
			var typed string
			if json.Unmarshal(raw, &typed) == nil && typed != "" {
				reply.Typed[i] = typed
			}
		}
	}
	return reply, nil
}

// questionTitle makes one line of a question written for a terminal.
func questionTitle(prompt string) string {
	var words []string
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "⚠️"))
		line = strings.TrimSpace(strings.TrimSuffix(line, "[y/N]:"))
		if line != "" {
			words = append(words, line)
		}
	}
	return strings.Join(words, " ")
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprint(time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
