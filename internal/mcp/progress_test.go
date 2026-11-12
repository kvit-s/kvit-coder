package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests cover the two things a long tools/call needs (progress.go):
// progress from the server keeps the call waiting past call_timeout, and a call
// the client gives up on is cancelled with the server.

// startHelperServer launches the test binary as the stdio helper server (see
// runEchoServer) and completes the handshake.
func startHelperServer(t *testing.T) *stdioClient {
	t.Helper()
	sc := echoServerConfig("helper")
	client, err := newStdioClient(sc.Command, sc.Args, sc.Env, "", nil)
	if err != nil {
		t.Fatalf("newStdioClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return client
}

// helperReport is what the helper server's report tool answers.
type helperReport struct {
	Waits     []int64           `json:"waits"`
	Cancelled []cancelledParams `json:"cancelled"`
}

func askHelperReport(t *testing.T, client Client) helperReport {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := client.CallTool(ctx, "report", nil)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	var rep helperReport
	if err := json.Unmarshal([]byte(res.Content[0].Text), &rep); err != nil {
		t.Fatalf("decode report %q: %v", res.Content[0].Text, err)
	}
	return rep
}

func TestCallWithProgressWaitsPastCallTimeout(t *testing.T) {
	client := startHelperServer(t)
	limit := 300 * time.Millisecond
	tool := newMCPTool("helper", ToolDescriptor{Name: "wait"}, 0, client, nil, ConfirmTrust, limit, false, nil)

	// The server answers after 1.2 s, four times the limit, reporting progress
	// every 50 ms meanwhile.
	start := time.Now()
	out, err := tool.Call(context.Background(), json.RawMessage(`{"ms": 1200, "every_ms": 50}`))
	if err != nil {
		t.Fatalf("a call whose server reports progress was given up on after %s: %v",
			time.Since(start).Round(time.Millisecond), err)
	}
	if got := out.(map[string]any)["content"]; got != "waited" {
		t.Fatalf("content = %v, want waited", got)
	}
	if elapsed := time.Since(start); elapsed < 4*limit {
		t.Fatalf("call returned after %s; the server should have taken 1.2 s", elapsed)
	}
	if rep := askHelperReport(t, client); len(rep.Cancelled) != 0 {
		t.Errorf("an answered call was cancelled: %+v", rep.Cancelled)
	}
}

func TestSilentCallTimesOutAndIsCancelled(t *testing.T) {
	client := startHelperServer(t)
	limit := 300 * time.Millisecond
	tool := newMCPTool("helper", ToolDescriptor{Name: "wait"}, 0, client, nil, ConfirmTrust, limit, false, nil)

	start := time.Now()
	_, err := tool.Call(context.Background(), json.RawMessage(`{"ms": 10000, "every_ms": 0}`))
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("a silent call succeeded; it should have timed out")
	}
	if elapsed < limit || elapsed > 3*time.Second {
		t.Fatalf("silent call ended after %s, want about %s", elapsed, limit)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error should count as a deadline: %v", err)
	}
	if !strings.Contains(err.Error(), "no reply or progress from the server for 300ms") {
		t.Errorf("error should say the server was silent: %v", err)
	}

	rep := askHelperReport(t, client)
	if len(rep.Waits) != 1 {
		t.Fatalf("server saw %d wait calls, want 1", len(rep.Waits))
	}
	if len(rep.Cancelled) != 1 {
		t.Fatalf("server got %d cancellations, want 1: %+v", len(rep.Cancelled), rep.Cancelled)
	}
	if got := rep.Cancelled[0]; got.RequestID != rep.Waits[0] || got.Reason == "" {
		t.Errorf("cancellation = %+v, want request %d with a reason", got, rep.Waits[0])
	}
}

func TestStoppedCallIsCancelled(t *testing.T) {
	client := startHelperServer(t)
	tool := newMCPTool("helper", ToolDescriptor{Name: "wait"}, 0, client, nil, ConfirmTrust, time.Minute, false, nil)

	// The turn is stopped 100 ms into a call that would otherwise wait a minute.
	ctx, stop := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, stop)
	_, err := tool.Call(ctx, json.RawMessage(`{"ms": 60000, "every_ms": 20}`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want a cancellation", err)
	}

	rep := askHelperReport(t, client)
	if len(rep.Waits) != 1 || len(rep.Cancelled) != 1 || rep.Cancelled[0].RequestID != rep.Waits[0] {
		t.Fatalf("want one cancellation naming the wait call; got %+v", rep)
	}
}

// wireLog is an rpcEndpoint transport that records what the client writes.
type wireLog struct {
	mu   sync.Mutex
	msgs []rpcMessage
}

func (w *wireLog) write(b []byte) error {
	var m rpcMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.msgs = append(w.msgs, m)
	return nil
}

func (w *wireLog) cancellations(t *testing.T) []cancelledParams {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []cancelledParams
	for _, m := range w.msgs {
		if m.Method != "notifications/cancelled" {
			continue
		}
		if m.ID != nil {
			t.Errorf("notifications/cancelled was sent with an id: %d", *m.ID)
		}
		var p cancelledParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			t.Fatalf("decode cancellation: %v", err)
		}
		out = append(out, p)
	}
	return out
}

func TestEndpointCancelsOnlyWhatItAbandons(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		ctx        func() (context.Context, context.CancelFunc)
		answer     bool
		wantCancel bool
		wantReason string
	}{
		{
			name:   "deadline",
			method: "tools/call",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 30*time.Millisecond)
			},
			wantCancel: true,
			wantReason: "the client's deadline for the request passed",
		},
		{
			name:   "silence",
			method: "tools/call",
			ctx: func() (context.Context, context.CancelFunc) {
				return withSilenceTimeout(context.Background(), 30*time.Millisecond)
			},
			wantCancel: true,
			wantReason: "no reply or progress from the server for 30ms",
		},
		{
			name:   "stopped",
			method: "tools/call",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				time.AfterFunc(30*time.Millisecond, cancel)
				return ctx, cancel
			},
			wantCancel: true,
			wantReason: "the client stopped waiting for the request",
		},
		{
			// The specification forbids cancelling initialize.
			name:   "initialize",
			method: "initialize",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 30*time.Millisecond)
			},
		},
		{
			name:   "answered",
			method: "tools/call",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 5*time.Second)
			},
			answer: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &wireLog{}
			var e *rpcEndpoint
			e = newRPCEndpoint(func(b []byte) error {
				if err := log.write(b); err != nil {
					return err
				}
				var m rpcMessage
				_ = json.Unmarshal(b, &m)
				if tc.answer && m.ID != nil {
					go e.dispatch([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{}}`, *m.ID)))
				}
				return nil
			}, nil)

			ctx, cancel := tc.ctx()
			defer cancel()
			_, err := e.Call(ctx, tc.method, map[string]any{})
			if tc.answer != (err == nil) {
				t.Fatalf("Call err = %v", err)
			}

			got := log.cancellations(t)
			if !tc.wantCancel {
				if len(got) != 0 {
					t.Fatalf("sent %d cancellations, want none: %+v", len(got), got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("sent %d cancellations, want 1: %+v", len(got), got)
			}
			if reqID := *log.msgs[0].ID; got[0].RequestID != reqID {
				t.Errorf("cancellation names request %d, want %d", got[0].RequestID, reqID)
			}
			if got[0].Reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", got[0].Reason, tc.wantReason)
			}
		})
	}
}

// httpTestServer is a streamable-HTTP MCP server with the same two tools as the
// stdio helper: "wait" answers on an event stream after ms milliseconds, with
// progress every every_ms when the call has a token; a call with every_ms 0
// stays silent until the client goes away.
type httpTestServer struct {
	mu        sync.Mutex
	waitIDs   []int64
	tokens    []string
	cancelled []cancelledParams
}

func (s *httpTestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var msg rpcMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if msg.ID == nil {
		if msg.Method == "notifications/cancelled" {
			var p cancelledParams
			_ = json.Unmarshal(msg.Params, &p)
			s.mu.Lock()
			s.cancelled = append(s.cancelled, p)
			s.mu.Unlock()
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	reply := func(result any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": result})
	}
	switch msg.Method {
	case "initialize":
		reply(map[string]any{"protocolVersion": ProtocolVersion, "capabilities": map[string]any{},
			"serverInfo": map[string]any{"name": "http-helper", "version": "1"}})
	case "tools/call":
		var p struct {
			Arguments struct {
				MS      int `json:"ms"`
				EveryMS int `json:"every_ms"`
			} `json:"arguments"`
			Meta struct {
				ProgressToken string `json:"progressToken"`
			} `json:"_meta"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		s.mu.Lock()
		s.waitIDs = append(s.waitIDs, *msg.ID)
		s.tokens = append(s.tokens, p.Meta.ProgressToken)
		s.mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		flusher.Flush()
		event := func(v any) {
			b, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		done := time.After(time.Duration(p.Arguments.MS) * time.Millisecond)
		var tick <-chan time.Time
		if p.Arguments.EveryMS > 0 && p.Meta.ProgressToken != "" {
			ticker := time.NewTicker(time.Duration(p.Arguments.EveryMS) * time.Millisecond)
			defer ticker.Stop()
			tick = ticker.C
		}
		for n := 1; ; n++ {
			select {
			case <-r.Context().Done():
				return
			case <-tick:
				event(map[string]any{"jsonrpc": "2.0", "method": "notifications/progress",
					"params": map[string]any{"progressToken": p.Meta.ProgressToken, "progress": n}})
			case <-done:
				event(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": map[string]any{
					"content": []any{map[string]any{"type": "text", "text": "waited"}}}})
				return
			}
		}
	default:
		reply(map[string]any{})
	}
}

func startHTTPHelper(t *testing.T) (*httpTestServer, *httpClient) {
	t.Helper()
	srv := &httpTestServer{}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	client, err := newHTTPClient(ts.URL, nil, nil)
	if err != nil {
		t.Fatalf("newHTTPClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return srv, client
}

func TestHTTPCallWithProgressWaitsPastCallTimeout(t *testing.T) {
	srv, client := startHTTPHelper(t)
	limit := 300 * time.Millisecond
	tool := newMCPTool("http", ToolDescriptor{Name: "wait"}, 0, client, nil, ConfirmTrust, limit, false, nil)

	out, err := tool.Call(context.Background(), json.RawMessage(`{"ms": 1200, "every_ms": 50}`))
	if err != nil {
		t.Fatalf("a call whose server reports progress was given up on: %v", err)
	}
	if got := out.(map[string]any)["content"]; got != "waited" {
		t.Fatalf("content = %v, want waited", got)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.tokens) != 1 || srv.tokens[0] == "" {
		t.Errorf("tools/call should carry a progress token; got %q", srv.tokens)
	}
	if len(srv.cancelled) != 0 {
		t.Errorf("an answered call was cancelled: %+v", srv.cancelled)
	}
}

func TestHTTPSilentCallTimesOutAndIsCancelled(t *testing.T) {
	srv, client := startHTTPHelper(t)
	limit := 300 * time.Millisecond
	tool := newMCPTool("http", ToolDescriptor{Name: "wait"}, 0, client, nil, ConfirmTrust, limit, false, nil)

	_, err := tool.Call(context.Background(), json.RawMessage(`{"ms": 10000, "every_ms": 0}`))
	if err == nil || !strings.Contains(err.Error(), "no reply or progress from the server for 300ms") {
		t.Fatalf("err = %v, want the silence error", err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.waitIDs) != 1 || len(srv.cancelled) != 1 || srv.cancelled[0].RequestID != srv.waitIDs[0] {
		t.Fatalf("want one cancellation naming request %v; got %+v", srv.waitIDs, srv.cancelled)
	}
}

// TestProgressForAnotherTokenIsIgnored checks that progress only extends the
// call it names.
func TestProgressForAnotherTokenIsIgnored(t *testing.T) {
	var l progressListeners
	hits := 0
	token := l.add(func() { hits++ })
	for _, params := range []string{
		`{"progressToken":"call-999","progress":1}`,
		`{"progressToken":1,"progress":1}`,
		`{"progress":1}`,
		`not json`,
	} {
		l.notify(json.RawMessage(params))
	}
	if hits != 0 {
		t.Fatalf("progress for another call ran this call's callback %d times", hits)
	}
	l.notify(json.RawMessage(fmt.Sprintf(`{"progressToken":%q,"progress":1}`, token)))
	l.remove(token)
	l.notify(json.RawMessage(fmt.Sprintf(`{"progressToken":%q,"progress":2}`, token)))
	if hits != 1 {
		t.Fatalf("callback ran %d times, want 1 (once before remove, not after)", hits)
	}
}
