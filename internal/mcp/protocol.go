package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// rpcConn is the transport-level request/response surface the MCP protocol
// methods build on. The stdio transport satisfies it with a persistent reader
// goroutine (rpcEndpoint); the HTTP transport satisfies it with one POST per
// call.
type rpcConn interface {
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	Notify(method string, params any) error
}

// proto implements the MCP method calls (initialize, tools/list, tools/call) on
// top of any rpcConn. Both transports embed it.
type proto struct {
	conn         rpcConn
	logger       Logger
	negotiated   string // protocol version the server agreed to
	instructions string // what the server said about using it; see instructions.go

	// progress maps each tools/call in flight to the callback that restarts
	// its silence timer; see progress.go.
	progress progressListeners
}

// initializeParams / initializeResult model the handshake.
type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      clientInfo     `json:"clientInfo"`
}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
	Capabilities map[string]any `json:"capabilities"`
	// Instructions is optional guidance on using the server, for the model.
	Instructions string `json:"instructions,omitempty"`
}

// Initialize runs initialize → notifications/initialized. A negotiated version
// different from ProtocolVersion is logged, not failed: MCP servers are
// generally backward compatible and downgrade to a version they support.
func (p *proto) Initialize(ctx context.Context) error {
	raw, err := p.conn.Call(ctx, "initialize", initializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    map[string]any{}, // tools-only client: no capabilities to advertise
		ClientInfo:      clientInfo{Name: clientName, Version: ClientVersion},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	var res initializeResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return fmt.Errorf("initialize: decode result: %w", err)
	}
	p.negotiated = res.ProtocolVersion
	p.instructions = strings.TrimSpace(res.Instructions)
	if res.ProtocolVersion != "" && res.ProtocolVersion != ProtocolVersion {
		p.logger.Debug(fmt.Sprintf("mcp: server %q negotiated protocol %s (client offered %s)",
			res.ServerInfo.Name, res.ProtocolVersion, ProtocolVersion))
	}

	// Per spec the client signals readiness before issuing further requests.
	if err := p.conn.Notify("notifications/initialized", map[string]any{}); err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}
	return nil
}

// Instructions returns what the server sent in its initialize result for the
// model, or "" when it sent nothing or the handshake has not run.
func (p *proto) Instructions() string { return p.instructions }

type listToolsParams struct {
	Cursor string `json:"cursor,omitempty"`
}

type listToolsResult struct {
	Tools      []ToolDescriptor `json:"tools"`
	NextCursor string           `json:"nextCursor"`
}

// ListTools fetches every advertised tool, following nextCursor pagination.
func (p *proto) ListTools(ctx context.Context) ([]ToolDescriptor, error) {
	var all []ToolDescriptor
	cursor := ""
	for {
		raw, err := p.conn.Call(ctx, "tools/list", listToolsParams{Cursor: cursor})
		if err != nil {
			return nil, fmt.Errorf("tools/list: %w", err)
		}
		var res listToolsResult
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, fmt.Errorf("tools/list: decode result: %w", err)
		}
		all = append(all, res.Tools...)
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return all, nil
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Meta      *requestMeta   `json:"_meta,omitempty"`
}

// CallTool invokes a tool and decodes its content blocks. A JSON-RPC protocol
// error (server crashed, unknown method) is returned as a Go error; a tool-level
// failure is conveyed via CallResult.IsError, left for the adapter to map.
//
// Every call carries a progress token. Progress the server sends with it goes
// to the callback withProgress put in ctx, when there is one.
func (p *proto) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	if args == nil {
		args = map[string]any{}
	}
	onProgress := progressFrom(ctx)
	if onProgress == nil {
		onProgress = func() {}
	}
	token := p.progress.add(onProgress)
	defer p.progress.remove(token)

	raw, err := p.conn.Call(ctx, "tools/call", callToolParams{
		Name:      name,
		Arguments: args,
		Meta:      &requestMeta{ProgressToken: token},
	})
	if err != nil {
		return nil, fmt.Errorf("tools/call %s: %w", name, err)
	}
	var res CallResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("tools/call %s: decode result: %w", name, err)
	}
	return &res, nil
}

// handleNotification acts on a notification from the server. The transports
// pass every one they receive here. Only progress is acted on; the rest are
// logged.
func (p *proto) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "notifications/progress":
		p.progress.notify(params)
	default:
		p.logger.Debug(fmt.Sprintf("mcp: ignoring server notification method=%s", method))
	}
}
