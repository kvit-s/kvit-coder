package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests drive `kvit-coder acp` as a client would: JSON-RPC lines on a
// pipe in, JSON-RPC lines on a pipe out, against a stand-in model endpoint
// that answers from a script. HOME is a temporary directory, so the sessions
// the agent makes are the test's own.

// scriptedEndpoint is a chat-completions endpoint answering each request with
// the next reply of its script, and keeping every request it was sent.
type scriptedEndpoint struct {
	t        *testing.T
	server   *httptest.Server
	mu       sync.Mutex
	replies  []map[string]any
	requests []map[string]any
	// hold, when set, is waited on before each answer.
	hold chan struct{}
}

func newScriptedEndpoint(t *testing.T) *scriptedEndpoint {
	e := &scriptedEndpoint{t: t}
	e.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(req.Body).Decode(&body)
		e.mu.Lock()
		e.requests = append(e.requests, body)
		hold := e.hold
		var reply map[string]any
		if len(e.replies) > 0 {
			reply, e.replies = e.replies[0], e.replies[1:]
		} else {
			reply = textReply("(the script has run out)")
		}
		e.mu.Unlock()
		if hold != nil {
			select {
			case <-hold:
			case <-req.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reply)
	}))
	t.Cleanup(e.server.Close)
	return e
}

func (e *scriptedEndpoint) script(replies ...map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.replies = append(e.replies, replies...)
}

func (e *scriptedEndpoint) lastRequest() map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.requests) == 0 {
		return nil
	}
	return e.requests[len(e.requests)-1]
}

func chatReply(message map[string]any, finish string) map[string]any {
	return map[string]any{
		"id":      "gen-1",
		"model":   "test-model",
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}},
		"usage":   map[string]any{"prompt_tokens": 100, "completion_tokens": 10, "total_tokens": 110},
	}
}

func textReply(text string) map[string]any {
	return chatReply(map[string]any{"role": "assistant", "content": text}, "stop")
}

func toolReply(id, name string, args map[string]any) map[string]any {
	data, _ := json.Marshal(args)
	return chatReply(map[string]any{
		"role":    "assistant",
		"content": "",
		"tool_calls": []any{map[string]any{
			"id":       id,
			"type":     "function",
			"function": map[string]any{"name": name, "arguments": string(data)},
		}},
	}, "tool_calls")
}

// acpClient is the client end of one connection.
type acpClient struct {
	t      *testing.T
	in     *io.PipeWriter
	lines  chan map[string]any
	nextID int
	// updates are the session/update notifications received, in order.
	updates []map[string]any
	// permission answers session/request_permission with an option id, or
	// "" for the cancelled outcome.
	permission func(params map[string]any) string
	asked      []map[string]any
	// form answers elicitation/create with the result to send.
	form  func(params map[string]any) map[string]any
	forms []map[string]any
	log   *bytes.Buffer
	done  chan error
}

func startACP(t *testing.T, configPath string) *acpClient {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	var log bytes.Buffer
	agent := newACPAgent(acpOptions{ConfigPath: configPath, Log: &lockedWriter{w: &log}})
	c := &acpClient{t: t, in: inW, lines: make(chan map[string]any, 1000), log: &log, done: make(chan error, 1)}
	go func() {
		err := agent.serve(context.Background(), inR, outW)
		outW.Close()
		c.done <- err
	}()
	go func() {
		scanner := bufio.NewScanner(outR)
		scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
		for scanner.Scan() {
			var msg map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				t.Errorf("the agent wrote a line that is not a message: %q", scanner.Text())
				continue
			}
			c.lines <- msg
		}
		close(c.lines)
	}()
	t.Cleanup(func() {
		inW.Close()
		select {
		case <-c.done:
		case <-time.After(10 * time.Second):
			t.Error("the agent did not stop when its input closed")
		}
	})
	return c
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func (c *acpClient) send(msg map[string]any) {
	msg["jsonrpc"] = "2.0"
	data, _ := json.Marshal(msg)
	if _, err := c.in.Write(append(data, '\n')); err != nil {
		c.t.Fatalf("cannot write to the agent: %v", err)
	}
}

func (c *acpClient) notify(method string, params any) {
	c.send(map[string]any{"method": method, "params": params})
}

// start sends a request and returns its id without waiting.
func (c *acpClient) start(method string, params any) float64 {
	c.nextID++
	c.send(map[string]any{"id": c.nextID, "method": method, "params": params})
	return float64(c.nextID)
}

// wait reads until the answer to request id arrives, answering the agent's
// own requests and keeping its notifications on the way.
func (c *acpClient) wait(id float64) (map[string]any, map[string]any) {
	c.t.Helper()
	timeout := time.After(60 * time.Second)
	for {
		select {
		case <-timeout:
			c.t.Fatalf("no answer to request %v; log:\n%s", id, c.log.String())
		case msg, ok := <-c.lines:
			if !ok {
				c.t.Fatalf("the agent closed its output before answering %v", id)
			}
			method, _ := msg["method"].(string)
			switch {
			case method == "session/update":
				params := msg["params"].(map[string]any)
				c.updates = append(c.updates, params["update"].(map[string]any))
			case method == "session/request_permission":
				params := msg["params"].(map[string]any)
				c.asked = append(c.asked, params)
				option := ""
				if c.permission != nil {
					option = c.permission(params)
				}
				outcome := map[string]any{"outcome": "cancelled"}
				if option != "" {
					outcome = map[string]any{"outcome": "selected", "optionId": option}
				}
				c.send(map[string]any{"id": msg["id"], "result": map[string]any{"outcome": outcome}})
			case method == "elicitation/create":
				params := msg["params"].(map[string]any)
				c.forms = append(c.forms, params)
				result := map[string]any{"action": "cancel"}
				if c.form != nil {
					result = c.form(params)
				}
				c.send(map[string]any{"id": msg["id"], "result": result})
			case method == "":
				if msg["id"] == id {
					result, _ := msg["result"].(map[string]any)
					rpcErr, _ := msg["error"].(map[string]any)
					return result, rpcErr
				}
			}
		}
	}
}

// waitForUpdate reads until an update satisfying match arrives, answering
// requests and keeping every update on the way.
func (c *acpClient) waitForUpdate(match func(map[string]any) bool) map[string]any {
	c.t.Helper()
	for _, u := range c.updates {
		if match(u) {
			return u
		}
	}
	// A request id nothing will answer, so wait returns only on timeout;
	// each update is checked as it is kept.
	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			c.t.Fatalf("the update never arrived; updates: %v", c.updates)
		case msg, ok := <-c.lines:
			if !ok {
				c.t.Fatalf("the agent closed its output")
			}
			if msg["method"] == "session/update" {
				u := msg["params"].(map[string]any)["update"].(map[string]any)
				c.updates = append(c.updates, u)
				if match(u) {
					return u
				}
			}
		}
	}
}

func (c *acpClient) call(method string, params any) map[string]any {
	c.t.Helper()
	result, rpcErr := c.wait(c.start(method, params))
	if rpcErr != nil {
		c.t.Fatalf("%s failed: %v\nlog:\n%s", method, rpcErr, c.log.String())
	}
	return result
}

func (c *acpClient) updatesOf(kind string) []map[string]any {
	var out []map[string]any
	for _, u := range c.updates {
		if u["sessionUpdate"] == kind {
			out = append(out, u)
		}
	}
	return out
}

func (c *acpClient) toolUpdate(id, status string) map[string]any {
	for _, u := range c.updates {
		if u["sessionUpdate"] == "tool_call_update" && u["toolCallId"] == id && u["status"] == status {
			return u
		}
	}
	return nil
}

// acpFixture is a home directory, a workspace and a config pointing at the
// stand-in endpoint.
type acpFixture struct {
	home, workspace, config string
	endpoint                *scriptedEndpoint
}

func newACPFixture(t *testing.T, extraConfig string) *acpFixture {
	t.Helper()
	root := t.TempDir()
	f := &acpFixture{
		home:      filepath.Join(root, "home"),
		workspace: filepath.Join(root, "project"),
		config:    filepath.Join(root, "config.yaml"),
		endpoint:  newScriptedEndpoint(t),
	}
	for _, dir := range []string{f.home, f.workspace} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", f.home)
	t.Setenv("KVIT_CODER_CONFIG", "")
	body := fmt.Sprintf(`llm:
  base_url: %s
  model: test-model
  api_key: none
  api_backend: chat_completions
workspace:
  root: "."
%s%s`, f.endpoint.server.URL, acpTestTools, extraConfig)
	if err := os.WriteFile(f.config, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

// acpTestTools switches on the tools the tests call, as the example config
// does.
const acpTestTools = `tools:
  read:
    enabled: true
  edit:
    enabled: true
  search:
    enabled: true
  shell:
    enabled: true
  question:
    enabled: true
`

func (f *acpFixture) write(t *testing.T, rel, content string) string {
	t.Helper()
	path := filepath.Join(f.workspace, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func textPrompt(sessionID, text string) map[string]any {
	return map[string]any{
		"sessionId": sessionID,
		"prompt":    []any{map[string]any{"type": "text", "text": text}},
	}
}

func requestedToolNames(req map[string]any) []string {
	var names []string
	list, _ := req["tools"].([]any)
	for _, entry := range list {
		fn, _ := entry.(map[string]any)["function"].(map[string]any)
		if name, ok := fn["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}

// TestACPTurnReportsItsToolCallsAndReply: a session is made, a prompt runs a
// tool and answers, and every step reaches the client as an update, with the
// prompt's response giving why it stopped and what it used.
func TestACPTurnReportsItsToolCallsAndReply(t *testing.T) {
	f := newACPFixture(t, "")
	notes := f.write(t, "notes.txt", "the launch code is 1234\n")
	c := startACP(t, f.config)

	init := c.call("initialize", map[string]any{"protocolVersion": 1, "clientCapabilities": map[string]any{}})
	if init["protocolVersion"] != float64(1) {
		t.Errorf("protocol version %v, want 1", init["protocolVersion"])
	}
	caps := init["agentCapabilities"].(map[string]any)
	if caps["loadSession"] != true {
		t.Error("loadSession is not declared")
	}
	sessionCaps := caps["sessionCapabilities"].(map[string]any)
	for _, name := range []string{"resume", "list", "close"} {
		if _, ok := sessionCaps[name]; !ok {
			t.Errorf("sessionCapabilities.%s is not declared", name)
		}
	}
	if caps["promptCapabilities"].(map[string]any)["image"] != true {
		t.Error("promptCapabilities.image is not declared")
	}

	created := c.call("session/new", map[string]any{"cwd": f.workspace, "mcpServers": []any{}})
	id, _ := created["sessionId"].(string)
	if id == "" {
		t.Fatalf("session/new gave no id: %v", created)
	}
	options, _ := created["configOptions"].([]any)
	if len(options) == 0 || options[0].(map[string]any)["category"] != "model" {
		t.Errorf("the first configuration option is not the model: %v", created["configOptions"])
	}

	f.endpoint.script(
		toolReply("call_read", "Read", map[string]any{"path": "notes.txt"}),
		textReply("The code is 1234."),
	)
	result := c.call("session/prompt", textPrompt(id, "What is the launch code?"))
	if result["stopReason"] != "end_turn" {
		t.Errorf("stopReason %v, want end_turn", result["stopReason"])
	}
	if usage, _ := result["usage"].(map[string]any); usage["inputTokens"] != float64(200) {
		t.Errorf("usage %v, want 200 input tokens over the two requests", result["usage"])
	}

	calls := c.updatesOf("tool_call")
	if len(calls) != 1 {
		t.Fatalf("got %d tool_call updates, want 1: %v", len(calls), c.updates)
	}
	call := calls[0]
	if call["toolCallId"] != "call_read" || call["kind"] != "read" || call["status"] != "pending" {
		t.Errorf("tool_call %v, want call_read of kind read, pending", call)
	}
	locations, _ := call["locations"].([]any)
	if len(locations) != 1 || locations[0].(map[string]any)["path"] != notes {
		t.Errorf("locations %v, want %s", call["locations"], notes)
	}
	if c.toolUpdate("call_read", "in_progress") == nil {
		t.Error("the call was never reported as running")
	}
	done := c.toolUpdate("call_read", "completed")
	if done == nil {
		t.Fatalf("the call was never reported as completed: %v", c.updates)
	}
	if !strings.Contains(fmt.Sprint(done["content"]), "1234") {
		t.Errorf("the completed call does not carry its result: %v", done["content"])
	}
	messages := c.updatesOf("agent_message_chunk")
	if len(messages) != 1 || messages[0]["content"].(map[string]any)["text"] != "The code is 1234." {
		t.Errorf("agent_message_chunk updates %v, want the reply", messages)
	}
	if messages[0]["messageId"] == "" || messages[0]["messageId"] == nil {
		t.Error("the reply has no messageId")
	}
	if titles := c.updatesOf("session_info_update"); len(titles) != 1 || titles[0]["title"] == "" {
		t.Errorf("session_info_update %v, want the session's title once", titles)
	}
	if usage := c.updatesOf("usage_update"); len(usage) != 1 || usage[0]["size"] == nil {
		t.Errorf("usage_update %v, want one with the context's size", usage)
	}

	listed := c.call("session/list", map[string]any{"cwd": f.workspace})
	sessions, _ := listed["sessions"].([]any)
	if len(sessions) != 1 || sessions[0].(map[string]any)["sessionId"] != id {
		t.Errorf("session/list %v, want the one session", listed)
	}
	other := c.call("session/list", map[string]any{"cwd": filepath.Join(f.workspace, "elsewhere")})
	if sessions, _ := other["sessions"].([]any); len(sessions) != 0 {
		t.Errorf("session/list for another directory %v, want none", other)
	}
}

// TestACPResumeAndLoadContinueTheSession: a second process reopens the
// session by id. session/resume sends nothing and the next prompt carries the
// conversation so far; session/load first replays it as updates.
func TestACPResumeAndLoadContinueTheSession(t *testing.T) {
	f := newACPFixture(t, "")
	first := startACP(t, f.config)
	first.call("initialize", map[string]any{"protocolVersion": 1})
	id := first.call("session/new", map[string]any{"cwd": f.workspace, "mcpServers": []any{}})["sessionId"].(string)
	f.endpoint.script(textReply("Noted: the colour is teal."))
	first.call("session/prompt", textPrompt(id, "Remember that the colour is teal."))

	second := startACP(t, f.config)
	second.call("initialize", map[string]any{"protocolVersion": 1})
	second.call("session/resume", map[string]any{"sessionId": id, "cwd": f.workspace, "mcpServers": []any{}})
	if len(second.updates) != 0 {
		t.Errorf("session/resume sent updates: %v", second.updates)
	}
	f.endpoint.script(textReply("Teal."))
	second.call("session/prompt", textPrompt(id, "Which colour?"))
	sent, _ := json.Marshal(f.endpoint.lastRequest()["messages"])
	if !strings.Contains(string(sent), "the colour is teal") || !strings.Contains(string(sent), "Noted: the colour is teal.") {
		t.Errorf("the resumed prompt did not carry the conversation so far: %s", sent)
	}

	third := startACP(t, f.config)
	third.call("initialize", map[string]any{"protocolVersion": 1})
	third.call("session/load", map[string]any{"sessionId": id, "cwd": f.workspace, "mcpServers": []any{}})
	var said, answered []string
	for _, u := range third.updates {
		content, _ := u["content"].(map[string]any)
		text, _ := content["text"].(string)
		switch u["sessionUpdate"] {
		case "user_message_chunk":
			said = append(said, text)
		case "agent_message_chunk":
			answered = append(answered, text)
		}
	}
	if len(said) != 2 || !strings.Contains(said[0], "the colour is teal") || !strings.Contains(said[1], "Which colour?") {
		t.Errorf("replayed prompts %q", said)
	}
	if len(answered) != 2 || answered[1] != "Teal." {
		t.Errorf("replayed replies %q", answered)
	}

	_, rpcErr := third.wait(third.start("session/resume", map[string]any{"sessionId": "no-such-session", "cwd": f.workspace}))
	if rpcErr == nil {
		t.Error("resuming a session that does not exist succeeded")
	}
}

// TestACPPolicyBoundsTheTurn: the rules Kvit Works sends under
// _meta.kvit.policy decide what the turn may touch. A conversation that
// proposes changes writes only into its staged folder, reads what the rules
// open, is refused a denied path, and is offered no shell.
func TestACPPolicyBoundsTheTurn(t *testing.T) {
	f := newACPFixture(t, "")
	f.write(t, "main.go", "package main\n")
	f.write(t, ".chat/transcript.md", "private\n")
	staged := filepath.Join(f.workspace, ".chat", "staged")
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatal(err)
	}
	c := startACP(t, f.config)
	c.call("initialize", map[string]any{"protocolVersion": 1})
	policy := map[string]any{
		"readable": []string{f.workspace},
		"writable": []string{staged},
		"denied": []any{
			map[string]any{"path": filepath.Join(f.workspace, ".chat", "transcript.md"), "read": true, "write": true},
			map[string]any{"path": filepath.Join(f.workspace, ".git"), "read": false, "write": true},
		},
		"shell": false,
	}
	id := c.call("session/new", map[string]any{
		"cwd":        f.workspace,
		"mcpServers": []any{},
		"_meta":      map[string]any{"kvit": map[string]any{"policy": policy}, "systemPrompt": map[string]any{"append": "You propose changes."}},
	})["sessionId"].(string)

	f.endpoint.script(
		toolReply("call_project", "Write", map[string]any{"path": "main.go", "text": "package changed\n"}),
		toolReply("call_staged", "Write", map[string]any{"path": filepath.Join(staged, "main.go"), "text": "package proposed\n"}),
		toolReply("call_private", "Read", map[string]any{"path": ".chat/transcript.md"}),
		textReply("Proposed."),
	)
	c.call("session/prompt", textPrompt(id, "Change main.go."))

	if data, _ := os.ReadFile(filepath.Join(f.workspace, "main.go")); string(data) != "package main\n" {
		t.Errorf("the project's own file was written: %q", data)
	}
	if refused := c.toolUpdate("call_project", "failed"); refused == nil || !strings.Contains(fmt.Sprint(refused["content"]), "read-only") {
		t.Errorf("writing into the project was not refused as read-only: %v", refused)
	}
	if data, _ := os.ReadFile(filepath.Join(staged, "main.go")); string(data) != "package proposed\n" {
		t.Errorf("the staged file holds %q", data)
	}
	done := c.toolUpdate("call_staged", "completed")
	if done == nil {
		t.Fatalf("writing into staged/ was not completed: %v", c.updates)
	}
	if !strings.Contains(fmt.Sprint(done["content"]), "type:diff") || !strings.Contains(fmt.Sprint(done["content"]), "package proposed") {
		t.Errorf("the staged write carries no diff: %v", done["content"])
	}
	if c.toolUpdate("call_private", "failed") == nil {
		t.Errorf("reading a denied path was not reported as failed: %v", c.updates)
	}

	request := f.endpoint.lastRequest()
	for _, name := range requestedToolNames(request) {
		if strings.HasPrefix(name, "Shell") {
			t.Errorf("a turn with no shell was offered %s", name)
		}
	}
	system, _ := json.Marshal(request["messages"].([]any)[0])
	if !strings.Contains(string(system), "You propose changes.") {
		t.Error("_meta.systemPrompt did not reach the system prompt")
	}
}

// TestACPAsksBeforeACommandThatNeedsPermission: a command on the list that
// needs permission is put to the client as session/request_permission with
// three options, and a refusal reaches the model as the call's failure.
func TestACPAsksBeforeACommandThatNeedsPermission(t *testing.T) {
	f := newACPFixture(t, "")
	c := startACP(t, f.config)
	c.call("initialize", map[string]any{"protocolVersion": 1})
	id := c.call("session/new", map[string]any{"cwd": f.workspace, "mcpServers": []any{}})["sessionId"].(string)
	c.permission = func(map[string]any) string { return "reject_once" }

	f.endpoint.script(
		toolReply("call_curl", "Shell", map[string]any{"command": "curl http://127.0.0.1:9/"}),
		textReply("I could not fetch it."),
	)
	c.call("session/prompt", textPrompt(id, "Fetch the page."))

	if len(c.asked) != 1 {
		t.Fatalf("asked %d times, want once", len(c.asked))
	}
	asked := c.asked[0]
	if asked["sessionId"] != id || asked["toolCall"].(map[string]any)["toolCallId"] != "call_curl" {
		t.Errorf("the request names %v, want call_curl in %s", asked["toolCall"], id)
	}
	var kinds []string
	for _, o := range asked["options"].([]any) {
		kinds = append(kinds, o.(map[string]any)["kind"].(string))
	}
	if strings.Join(kinds, ",") != "allow_once,allow_always,reject_once" {
		t.Errorf("options %v, want allow_once, allow_always and reject_once", kinds)
	}
	refused := c.toolUpdate("call_curl", "failed")
	if refused == nil || !strings.Contains(fmt.Sprint(refused["content"]), "refused") {
		t.Errorf("the refusal did not reach the call's result: %v", refused)
	}

	// Allowed for the session, the same command is not asked about again.
	c.permission = func(map[string]any) string { return "allow_always" }
	f.endpoint.script(
		toolReply("call_again", "Shell", map[string]any{"command": "curl --version"}),
		toolReply("call_third", "Shell", map[string]any{"command": "curl --help"}),
		textReply("Done."),
	)
	c.call("session/prompt", textPrompt(id, "Try again."))
	if len(c.asked) != 2 {
		t.Errorf("asked %d times in all, want twice: an answer for the session covers the second command", len(c.asked))
	}
}

// TestACPCancelEndsThePrompt: session/cancel while the model is thinking
// ends the prompt with stopReason cancelled rather than an error.
func TestACPCancelEndsThePrompt(t *testing.T) {
	f := newACPFixture(t, "")
	c := startACP(t, f.config)
	c.call("initialize", map[string]any{"protocolVersion": 1})
	id := c.call("session/new", map[string]any{"cwd": f.workspace, "mcpServers": []any{}})["sessionId"].(string)

	hold := make(chan struct{})
	defer close(hold)
	f.endpoint.mu.Lock()
	f.endpoint.hold = hold
	f.endpoint.mu.Unlock()

	prompt := c.start("session/prompt", textPrompt(id, "Think for a long time."))
	deadline := time.Now().Add(10 * time.Second)
	for f.endpoint.lastRequest() == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	c.notify("session/cancel", map[string]any{"sessionId": id})
	result, rpcErr := c.wait(prompt)
	if rpcErr != nil {
		t.Fatalf("a cancelled prompt answered with an error: %v", rpcErr)
	}
	if result["stopReason"] != "cancelled" {
		t.Errorf("stopReason %v, want cancelled", result["stopReason"])
	}
}

// TestACPModelAndEffortAreConfigurationOptions: the config's catalogue is the
// model option, the chosen model's effort menu is the thought_level option,
// and setting them changes what the next request asks for.
func TestACPModelAndEffortAreConfigurationOptions(t *testing.T) {
	f := newACPFixture(t, "")
	catalogue := fmt.Sprintf(`llm:
  base_url: %[1]s
  model: fast-model
  api_key: none
  api_backend: chat_completions
models:
  - id: fast
    name: Fast
    model: fast-model
    base_url: %[1]s
  - id: deep
    name: Deep
    model: deep-model
    base_url: %[1]s
    efforts:
      - value: low
      - value: high
        default: true
workspace:
  root: "."
`+acpTestTools, f.endpoint.server.URL)
	if err := os.WriteFile(f.config, []byte(catalogue), 0o644); err != nil {
		t.Fatal(err)
	}
	c := startACP(t, f.config)
	c.call("initialize", map[string]any{"protocolVersion": 1})
	created := c.call("session/new", map[string]any{"cwd": f.workspace, "mcpServers": []any{}})
	id := created["sessionId"].(string)
	options := created["configOptions"].([]any)
	if len(options) != 1 || options[0].(map[string]any)["currentValue"] != "fast" {
		t.Fatalf("options %v, want the model alone, on fast", options)
	}

	changed := c.call("session/set_config_option", map[string]any{"sessionId": id, "configId": "model", "value": "deep"})
	options = changed["configOptions"].([]any)
	if len(options) != 2 || options[1].(map[string]any)["category"] != "thought_level" || options[1].(map[string]any)["currentValue"] != "high" {
		t.Fatalf("after choosing deep: %v, want an effort option on high", options)
	}
	c.call("session/set_config_option", map[string]any{"sessionId": id, "configId": "effort", "value": "low"})
	if _, rpcErr := c.wait(c.start("session/set_config_option", map[string]any{"sessionId": id, "configId": "model", "value": "none-such"})); rpcErr == nil {
		t.Error("choosing a model the config does not have succeeded")
	}

	f.endpoint.script(textReply("Hello."))
	c.call("session/prompt", textPrompt(id, "Hello?"))
	request := f.endpoint.lastRequest()
	if request["model"] != "deep-model" {
		t.Errorf("the request asked for %v, want deep-model", request["model"])
	}
	if kwargs, _ := request["chat_template_kwargs"].(map[string]any); kwargs["reasoning_effort"] != "low" {
		t.Errorf("the request asked for effort %v, want low", request["chat_template_kwargs"])
	}
}

// TestACPQuestionsAreAskedAsAForm: with a client that shows forms, the
// Question tool's questions become one elicitation/create, a field per
// question, and the answer comes back to the model as the tool's result.
func TestACPQuestionsAreAskedAsAForm(t *testing.T) {
	f := newACPFixture(t, "")
	c := startACP(t, f.config)
	c.call("initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{"elicitation": map[string]any{"form": map[string]any{}}},
	})
	id := c.call("session/new", map[string]any{"cwd": f.workspace, "mcpServers": []any{}})["sessionId"].(string)
	c.form = func(params map[string]any) map[string]any {
		return map[string]any{"action": "accept", "content": map[string]any{
			"q01":       "Reuse the wrapper",
			"q02":       "call it Retry",
			"q03":       []any{"Linux", "macOS"},
			"q03_other": "",
		}}
	}
	f.endpoint.script(
		toolReply("call_ask", "Question", map[string]any{"questions": []any{
			map[string]any{"question": "Reuse the wrapper or write a new one?", "header": "Retry",
				"options": []any{map[string]any{"label": "Reuse the wrapper", "description": "already tested"},
					map[string]any{"label": "Write a new one"}}},
			map[string]any{"question": "What should it be called?"},
			map[string]any{"question": "Which systems?", "multi": true,
				"options": []any{map[string]any{"label": "Linux"}, map[string]any{"label": "macOS"}, map[string]any{"label": "Windows"}}},
		}}),
		textReply("Reusing it as Retry."),
	)
	c.call("session/prompt", textPrompt(id, "Add retries."))

	if len(c.forms) != 1 {
		t.Fatalf("got %d forms, want one for the call", len(c.forms))
	}
	form := c.forms[0]
	if form["mode"] != "form" || form["toolCallId"] != "call_ask" || form["sessionId"] != id {
		t.Errorf("form %v, want mode form for call_ask in %s", form, id)
	}
	props := form["requestedSchema"].(map[string]any)["properties"].(map[string]any)
	first := props["q01"].(map[string]any)
	if first["type"] != "string" || first["title"] != "Retry" || len(first["oneOf"].([]any)) != 2 {
		t.Errorf("the first field is %v, want a choice of two titled Retry", first)
	}
	if _, ok := props["q01_other"]; !ok {
		t.Error("the choice has no field for an answer in the person's own words")
	}
	if props["q02"].(map[string]any)["type"] != "string" || props["q02"].(map[string]any)["oneOf"] != nil {
		t.Errorf("the free question is %v, want a plain string", props["q02"])
	}
	if props["q03"].(map[string]any)["type"] != "array" {
		t.Errorf("the question that takes several is %v, want an array", props["q03"])
	}

	var result struct {
		Answers []struct {
			Question string   `json:"question"`
			Answers  []string `json:"answers"`
			FreeText string   `json:"free_text"`
		} `json:"answers"`
	}
	for _, m := range f.endpoint.lastRequest()["messages"].([]any) {
		msg := m.(map[string]any)
		if msg["role"] == "tool" && msg["tool_call_id"] == "call_ask" {
			if err := json.Unmarshal([]byte(msg["content"].(string)), &result); err != nil {
				t.Fatalf("the tool's result is not JSON: %v: %s", err, msg["content"])
			}
		}
	}
	if len(result.Answers) != 3 ||
		strings.Join(result.Answers[0].Answers, ",") != "Reuse the wrapper" ||
		result.Answers[1].FreeText != "call it Retry" ||
		strings.Join(result.Answers[2].Answers, ",") != "Linux,macOS" {
		t.Errorf("the tool's result is %+v", result)
	}

	// Declined, the model is told the person chose not to answer.
	c.form = func(map[string]any) map[string]any { return map[string]any{"action": "decline"} }
	f.endpoint.script(
		toolReply("call_again", "Question", map[string]any{"questions": []any{map[string]any{"question": "Proceed?"}}}),
		textReply("Proceeding."),
	)
	c.call("session/prompt", textPrompt(id, "Go on."))
	sent, _ := json.Marshal(f.endpoint.lastRequest()["messages"])
	if !strings.Contains(string(sent), "chose not to answer") {
		t.Errorf("a declined form did not reach the model as such: %s", sent)
	}
}

// TestACPBackgroundProcessesAreAnnouncedAndWakeTheModel: a process Shell.start
// runs is announced with the JetBrains extension. One started without
// `persistent` is reported stopped when the prompt ends; a persistent one
// keeps running, and its end wakes a pass of the loop outside any prompt,
// whose reply arrives before the process is reported completed.
func TestACPBackgroundProcessesAreAnnouncedAndWakeTheModel(t *testing.T) {
	watchInterval = 100 * time.Millisecond
	defer func() { watchInterval = time.Second }()
	f := newACPFixture(t, "  procs:\n    enabled: true\n")
	c := startACP(t, f.config)
	c.call("initialize", map[string]any{"protocolVersion": 1})
	id := c.call("session/new", map[string]any{"cwd": f.workspace, "mcpServers": []any{}})["sessionId"].(string)

	f.endpoint.script(
		toolReply("call_short", "Shell_start", map[string]any{"command": "sleep 30", "name": "idle"}),
		toolReply("call_long", "Shell_start", map[string]any{"command": "sleep 1; echo built", "name": "build", "persistent": true}),
		textReply("The build is running."),
		textReply("The build finished and printed built."),
	)
	result := c.call("session/prompt", textPrompt(id, "Start the build."))
	if result["stopReason"] != "end_turn" {
		t.Fatalf("stopReason %v", result["stopReason"])
	}
	spawned := c.updatesOf("async_task_spawned")
	if len(spawned) != 2 {
		t.Fatalf("got %d announcements, want 2: %v", len(spawned), c.updates)
	}
	if spawned[1]["toolCallId"] != "call_long" || spawned[1]["type"] != "shell" || spawned[1]["canStop"] != true || spawned[1]["name"] != "build" {
		t.Errorf("the persistent process was announced as %v", spawned[1])
	}
	shortID := spawned[0]["asyncTaskId"]
	longID := spawned[1]["asyncTaskId"]
	stopped := false
	for _, u := range c.updatesOf("async_task_state_update") {
		if u["asyncTaskId"] == shortID && u["state"] == "stopped" {
			stopped = true
		}
		if u["asyncTaskId"] == longID {
			t.Errorf("the persistent process was reported ended with the prompt: %v", u)
		}
	}
	if !stopped {
		t.Errorf("the process started without persistent was not reported stopped: %v", c.updates)
	}

	before := len(c.updatesOf("agent_message_chunk"))
	c.waitForUpdate(func(u map[string]any) bool {
		return u["sessionUpdate"] == "async_task_state_update" && u["asyncTaskId"] == longID
	})
	chunks := c.updatesOf("agent_message_chunk")
	if len(chunks) != before+1 || chunks[len(chunks)-1]["content"].(map[string]any)["text"] != "The build finished and printed built." {
		var kinds []string
		for _, u := range c.updates {
			kinds = append(kinds, fmt.Sprintf("%v/%v/%v", u["sessionUpdate"], u["asyncTaskId"], u["state"]))
			if content, ok := u["content"].(map[string]any); ok {
				kinds = append(kinds, fmt.Sprint(content["text"]))
			}
		}
		t.Errorf("the model's answer to the process ending did not come before the end was reported: %v", kinds)
	}
	last := c.updatesOf("async_task_state_update")
	if final := last[len(last)-1]; final["state"] != "completed" {
		t.Errorf("the persistent process ended as %v, want completed", final)
	}
	sent, _ := json.Marshal(f.endpoint.lastRequest()["messages"])
	if !strings.Contains(string(sent), "built") {
		t.Errorf("the pass was not told what the process printed: %s", sent)
	}
}

// TestACPStopsABackgroundProcessOnRequest: `_session/async_task/stop` ends
// an announced process, which is then reported stopped.
func TestACPStopsABackgroundProcessOnRequest(t *testing.T) {
	f := newACPFixture(t, "  procs:\n    enabled: true\n")
	c := startACP(t, f.config)
	c.call("initialize", map[string]any{"protocolVersion": 1})
	id := c.call("session/new", map[string]any{"cwd": f.workspace, "mcpServers": []any{}})["sessionId"].(string)
	f.endpoint.script(
		toolReply("call_serve", "Shell_start", map[string]any{"command": "sleep 60", "name": "server", "persistent": true}),
		textReply("The server is up."),
	)
	c.call("session/prompt", textPrompt(id, "Start the server."))
	spawned := c.updatesOf("async_task_spawned")
	if len(spawned) != 1 {
		t.Fatalf("got %d announcements: %v", len(spawned), c.updates)
	}
	task := spawned[0]["asyncTaskId"]
	c.call("_session/async_task/stop", map[string]any{"sessionId": id, "asyncTaskId": task})
	u := c.waitForUpdate(func(u map[string]any) bool {
		return u["sessionUpdate"] == "async_task_state_update" && u["asyncTaskId"] == task
	})
	if u["state"] != "stopped" {
		t.Errorf("the process ended as %v, want stopped", u["state"])
	}
}
