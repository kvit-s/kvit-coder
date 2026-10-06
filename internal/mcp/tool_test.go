package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/tools"
)

// fakeClient is an in-memory Client for adapter tests.
type fakeClient struct {
	listResult []ToolDescriptor
	callResult *CallResult
	callErr    error

	lastName string
	lastArgs map[string]any
}

func (f *fakeClient) Initialize(ctx context.Context) error { return nil }
func (f *fakeClient) Instructions() string                 { return "" }
func (f *fakeClient) ListTools(ctx context.Context) ([]ToolDescriptor, error) {
	return f.listResult, nil
}
func (f *fakeClient) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	f.lastName = name
	f.lastArgs = args
	return f.callResult, f.callErr
}
func (f *fakeClient) Close() error { return nil }

func newTestTool(client Client, desc ToolDescriptor) *MCPTool {
	return newMCPTool("srv", desc, 0, client, nil /*no confirmer*/, ConfirmTrust, 5*time.Second, false, nil)
}

func TestMCPTool_NamingAndSchemaPassthrough(t *testing.T) {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"q": map[string]any{"type": "string"}},
	}
	tool := newTestTool(&fakeClient{}, ToolDescriptor{Name: "read_file", Description: "Read a file", InputSchema: schema})

	if tool.Name() != "mcp.srv.read_file" {
		t.Errorf("expected namespaced name mcp.srv.read_file, got %s", tool.Name())
	}
	if tool.Description() != "Read a file" {
		t.Errorf("unexpected description: %s", tool.Description())
	}
	// Schema passed through verbatim when sanitize is off.
	got := tool.JSONSchema()
	if got["type"] != "object" {
		t.Errorf("schema not passed through: %#v", got)
	}
	if tool.PromptCategory() != "mcp" {
		t.Errorf("expected category mcp, got %s", tool.PromptCategory())
	}
	if !tool.SelfTimeout() {
		t.Error("MCP tool should be SelfTimeout")
	}
}

func TestMCPTool_CallConcatenatesTextBlocks(t *testing.T) {
	fc := &fakeClient{callResult: &CallResult{
		Content: []ContentBlock{
			{Type: "text", Text: "hello"},
			{Type: "text", Text: "world"},
			{Type: "image", MimeType: "image/png", Data: strings.Repeat("A", 4000)},
		},
	}}
	tool := newTestTool(fc, ToolDescriptor{Name: "echo"})

	res, err := tool.Call(context.Background(), json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := res.(map[string]any)
	if m["success"] != true {
		t.Errorf("expected success true, got %#v", m)
	}
	if m["content"] != "hello\nworld" {
		t.Errorf("expected concatenated text, got %q", m["content"])
	}
	notes, _ := m["attachments"].([]string)
	if len(notes) != 1 || !strings.Contains(notes[0], "image/png") {
		t.Errorf("expected one image attachment placeholder, got %#v", notes)
	}
	// raw tool name (not namespaced) is used on the wire
	if fc.lastName != "echo" {
		t.Errorf("expected raw name echo on wire, got %s", fc.lastName)
	}
	if fc.lastArgs["x"].(float64) != 1 {
		t.Errorf("args not forwarded: %#v", fc.lastArgs)
	}
}

func TestMCPTool_IsErrorIsBacktrackable(t *testing.T) {
	fc := &fakeClient{callResult: &CallResult{
		IsError: true,
		Content: []ContentBlock{{Type: "text", Text: "bad input"}},
	}}
	tool := newTestTool(fc, ToolDescriptor{Name: "echo"})

	_, err := tool.Call(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for isError result")
	}
	if !tools.IsBacktrackable(err) {
		t.Errorf("isError should map to a backtrackable semantic error, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "bad input") {
		t.Errorf("error should carry tool message, got %v", err)
	}
}

func TestMCPTool_TransportFailureIsPlainError(t *testing.T) {
	fc := &fakeClient{callErr: errors.New("server crashed")}
	tool := newTestTool(fc, ToolDescriptor{Name: "echo"})

	_, err := tool.Call(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for transport failure")
	}
	if tools.IsBacktrackable(err) {
		t.Error("transport failure should NOT be backtrackable")
	}
}

func TestMCPTool_InvalidArgsRejected(t *testing.T) {
	tool := newTestTool(&fakeClient{}, ToolDescriptor{Name: "echo"})
	_, err := tool.Call(context.Background(), json.RawMessage(`{not json`))
	if err == nil || !tools.IsBacktrackable(err) {
		t.Errorf("invalid args should be a backtrackable error, got %v", err)
	}
}

func TestMCPTool_SanitizeApplied(t *testing.T) {
	schema := map[string]any{"type": "object", "oneOf": []any{map[string]any{"type": "string"}}}
	tool := newMCPTool("srv", ToolDescriptor{Name: "x", InputSchema: schema}, 0, &fakeClient{}, nil, ConfirmTrust, time.Second, true /*sanitize*/, nil)
	if _, ok := tool.JSONSchema()["oneOf"]; ok {
		t.Error("expected oneOf stripped when sanitize on")
	}
}

func TestMCPTool_EmptySchemaDefaultsToObject(t *testing.T) {
	tool := newTestTool(&fakeClient{}, ToolDescriptor{Name: "x"})
	if tool.JSONSchema()["type"] != "object" {
		t.Errorf("empty schema should default to object, got %#v", tool.JSONSchema())
	}
}

func TestConfirmer_BlockAndTrust(t *testing.T) {
	c := newConfirmer()
	if err := c.Confirm("mcp.s.t", "s", ConfirmTrust, ""); err != nil {
		t.Errorf("trust should allow, got %v", err)
	}
	if err := c.Confirm("mcp.s.t", "s", ConfirmBlock, ""); err == nil {
		t.Error("block should refuse")
	}
}
