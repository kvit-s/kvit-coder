package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

// captureConn records the initialize params and answers with a minimal result.
type captureConn struct {
	gotMethod string
	gotParams json.RawMessage
	notified  string
}

func (c *captureConn) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.gotMethod = method
	b, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	c.gotParams = b
	return json.Marshal(map[string]any{
		"protocolVersion": ProtocolVersion,
		"serverInfo":      map[string]any{"name": "test-server", "version": "1.0"},
		"capabilities":    map[string]any{},
	})
}

func (c *captureConn) Notify(method string, params any) error {
	c.notified = method
	return nil
}

func TestInitializeReportsClientVersion(t *testing.T) {
	old := ClientVersion
	defer func() { ClientVersion = old }()
	ClientVersion = "v9.9.9-test"

	conn := &captureConn{}
	p := &proto{conn: conn, logger: nopLogger{}}
	if err := p.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if conn.gotMethod != "initialize" {
		t.Fatalf("got method %q, want initialize", conn.gotMethod)
	}
	var params initializeParams
	if err := json.Unmarshal(conn.gotParams, &params); err != nil {
		t.Fatalf("decode initialize params: %v", err)
	}
	if params.ClientInfo.Name != "kvit-coder" {
		t.Errorf("client name = %q, want kvit-coder", params.ClientInfo.Name)
	}
	if params.ClientInfo.Version != "v9.9.9-test" {
		t.Errorf("client version = %q, want %q", params.ClientInfo.Version, "v9.9.9-test")
	}
	if conn.notified != "notifications/initialized" {
		t.Errorf("notified = %q, want notifications/initialized", conn.notified)
	}
}
