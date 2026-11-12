package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// TestHelperMCPServer is not a real test: when GO_WANT_MCP_SERVER=1 it runs a
// minimal stdio MCP server (one "echo" tool) and exits. Tests spawn the test
// binary with -test.run=TestHelperMCPServer to get a real subprocess speaking
// the protocol over stdin/stdout.
func TestHelperMCPServer(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_SERVER") != "1" {
		return
	}
	runEchoServer()
	os.Exit(0)
}

func runEchoServer() {
	out := bufio.NewWriter(os.Stdout)
	var outMu sync.Mutex
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	send := func(msg map[string]any) {
		b, _ := json.Marshal(msg)
		outMu.Lock()
		defer outMu.Unlock()
		_, _ = out.Write(b)
		_ = out.WriteByte('\n')
		_ = out.Flush()
	}
	respond := func(id *int64, result any) {
		send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	text := func(s string) map[string]any {
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": s}}, "isError": false}
	}

	// What the "wait" and "report" tools share: the wait calls still running,
	// the id of every wait call, and every cancellation received.
	var mu sync.Mutex
	running := map[int64]chan struct{}{}
	var waitIDs []int64
	var cancelled []cancelledParams

	for scanner.Scan() {
		var msg rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		if msg.ID == nil { // notification
			if msg.Method == "notifications/cancelled" {
				var p cancelledParams
				_ = json.Unmarshal(msg.Params, &p)
				mu.Lock()
				cancelled = append(cancelled, p)
				if stop, ok := running[p.RequestID]; ok {
					close(stop)
					delete(running, p.RequestID)
				}
				mu.Unlock()
			}
			continue
		}
		switch msg.Method {
		case "initialize":
			result := map[string]any{
				"protocolVersion": ProtocolVersion,
				"serverInfo":      map[string]any{"name": "echo-server", "version": "1.0"},
				"capabilities":    map[string]any{"tools": map[string]any{}},
			}
			// A test that wants server instructions sets them here.
			if text := os.Getenv("ECHO_INSTRUCTIONS"); text != "" {
				result["instructions"] = text
			}
			respond(msg.ID, result)
		case "tools/list":
			respond(msg.ID, map[string]any{
				"tools": []any{
					map[string]any{
						"name":        "echo",
						"description": "Echoes its message back",
						"inputSchema": map[string]any{
							"type":       "object",
							"properties": map[string]any{"message": map[string]any{"type": "string"}},
							"required":   []any{"message"},
						},
					},
				},
			})
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
				Meta      struct {
					ProgressToken json.RawMessage `json:"progressToken"`
				} `json:"_meta"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			switch p.Name {
			case "wait":
				// Answers after ms milliseconds, sending progress every
				// every_ms meanwhile when the call has a progress token, as
				// Kvit Works' ask_user does. A cancelled call is not answered.
				ms, _ := p.Arguments["ms"].(float64)
				everyMS, _ := p.Arguments["every_ms"].(float64)
				stop := make(chan struct{})
				mu.Lock()
				running[*msg.ID] = stop
				waitIDs = append(waitIDs, *msg.ID)
				mu.Unlock()
				go func(id *int64, token json.RawMessage) {
					done := time.After(time.Duration(ms) * time.Millisecond)
					var tick <-chan time.Time
					if everyMS > 0 && len(token) > 0 {
						ticker := time.NewTicker(time.Duration(everyMS) * time.Millisecond)
						defer ticker.Stop()
						tick = ticker.C
					}
					for n := 1; ; n++ {
						select {
						case <-stop:
							return
						case <-tick:
							send(map[string]any{"jsonrpc": "2.0", "method": "notifications/progress",
								"params": map[string]any{"progressToken": token, "progress": n}})
						case <-done:
							mu.Lock()
							delete(running, *id)
							mu.Unlock()
							respond(id, text("waited"))
							return
						}
					}
				}(msg.ID, p.Meta.ProgressToken)
			case "report":
				mu.Lock()
				b, _ := json.Marshal(map[string]any{"waits": waitIDs, "cancelled": cancelled})
				mu.Unlock()
				respond(msg.ID, text(string(b)))
			default:
				respond(msg.ID, text(fmt.Sprintf("echo: %v", p.Arguments["message"])))
			}
		}
	}
}

// echoServerConfig returns a stdio server config that launches this test binary
// as the echo MCP server.
func echoServerConfig(name string) config.MCPServerConfig {
	return config.MCPServerConfig{
		Name:      name,
		Enabled:   true,
		Transport: "stdio",
		Command:   os.Args[0],
		Args:      []string{"-test.run=TestHelperMCPServer"},
		Env:       []string{"GO_WANT_MCP_SERVER=1"},
	}
}

// TestExternalStdioServer connects to a real external MCP stdio server binary
// and lists its tools. It is skipped unless CAG_CLIENT_BIN (path to the binary)
// is set, so it never runs in normal CI. Optional CAG_CLIENT_ENV supplies one
// "KEY=VALUE" env entry (e.g. CAG_SERVER=...). This verifies kvit-coder's own
// client drives a third-party server through initialize + tools/list.
func TestExternalStdioServer(t *testing.T) {
	bin := os.Getenv("CAG_CLIENT_BIN")
	if bin == "" {
		t.Skip("set CAG_CLIENT_BIN to run the external MCP server integration test")
	}
	var env []string
	if e := os.Getenv("CAG_CLIENT_ENV"); e != "" {
		env = []string{e}
	}
	client, err := newStdioClient(bin, nil, env, "", nil)
	if err != nil {
		t.Fatalf("newStdioClient: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	toolsList, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(toolsList) == 0 {
		t.Fatal("expected at least one tool from external server")
	}
	for _, td := range toolsList {
		t.Logf("discovered tool: %s — %s", td.Name, td.Description)
	}
}

func TestStdioClient_Roundtrip(t *testing.T) {
	sc := echoServerConfig("echo")
	client, err := newStdioClient(sc.Command, sc.Args, sc.Env, "", nil)
	if err != nil {
		t.Fatalf("newStdioClient: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	toolsList, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(toolsList) != 1 || toolsList[0].Name != "echo" {
		t.Fatalf("expected one tool 'echo', got %#v", toolsList)
	}

	res, err := client.CallTool(ctx, "echo", map[string]any{"message": "hi"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError || len(res.Content) != 1 || res.Content[0].Text != "echo: hi" {
		t.Fatalf("unexpected call result: %#v", res)
	}
}

func TestManager_StdioEndToEnd(t *testing.T) {
	cfg := config.MCPConfig{
		Enabled: true,
		Confirm: ConfirmTrust,
		Servers: []config.MCPServerConfig{echoServerConfig("echo")},
	}
	m := NewManager(cfg, "", nil, nil)
	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer m.Close()

	adapters := m.Tools()
	if len(adapters) != 1 {
		t.Fatalf("expected 1 adapter, got %d", len(adapters))
	}
	tool := adapters[0]
	if tool.Name() != "mcp.echo.echo" {
		t.Fatalf("expected mcp.echo.echo, got %s", tool.Name())
	}

	// Check (trust policy => allowed) then Call round-trips through the subprocess.
	if err := tool.Check(context.Background(), json.RawMessage(`{"message":"yo"}`)); err != nil {
		t.Fatalf("Check: %v", err)
	}
	out, err := tool.Call(context.Background(), json.RawMessage(`{"message":"yo"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	m2 := out.(map[string]any)
	if m2["content"] != "echo: yo" {
		t.Fatalf("unexpected content: %#v", m2)
	}
}

func TestExpandArgsSubstitutesTheEnvironment(t *testing.T) {
	t.Setenv("KVIT_TEST_RUN_ID", "sess-abc")
	got := expandArgs([]string{
		"--headless",
		"--user-data-dir=/tmp/browser/${KVIT_TEST_RUN_ID}",
		"--plain",
	})
	want := []string{"--headless", "--user-data-dir=/tmp/browser/sess-abc", "--plain"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExpandArgsLeavesAnUnsetVariableEmpty(t *testing.T) {
	// A path that collapses to a shared directory is a worse outcome than a
	// per-session one and a better outcome than a crash, so this is recorded
	// rather than guarded against.
	t.Setenv("KVIT_TEST_ABSENT", "")
	got := expandArgs([]string{"--dir=/tmp/b/${KVIT_TEST_ABSENT}"})
	if got[0] != "--dir=/tmp/b/" {
		t.Errorf("got %q", got[0])
	}
}
