package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// turnReporter tells the client what one prompt's loop does, as
// session/update notifications. The loop hands it each message right after
// the session has recorded it (repl.RunTurn's observer), and each tool call
// just before the call's checks run (Runner.SetToolStart):
//
//	assistant reasoning   agent_thought_chunk
//	assistant text        agent_message_chunk, with an id for the message
//	a tool call named     tool_call, status pending, with its kind, the file
//	                      it names and its arguments
//	the call starting     tool_call_update, status in_progress
//	its result            tool_call_update, completed or failed, with the
//	                      result and, for a call that changed a file, a diff
//	the plan changing     plan, every step with its status
//	a title for the session  session_info_update
//	the turn's end        usage_update: the context in use, its size, and
//	                      the session's cost so far
//
// session/load replays a session's history through the same function, so
// the client draws a reopened session the way it drew it live.
type turnReporter struct {
	a         *acpAgent
	s         *acpSession
	ctx       context.Context
	workspace string
	registry  *tools.Registry
	toolCtx   *tools.ToolContext
	plans     *tools.PlanManager
	// title returns the session's title, which the turn sets before the
	// loop's first call.
	title func() string

	calls    map[string]*reportedCall
	current  string // the tool call running now
	lastPlan string
}

// reportedCall is a tool call the client has been told about.
type reportedCall struct {
	name      string
	kind      string
	locations []map[string]any
	// path is the file an editing call changes, and before its content when
	// the call started; nil when the file did not exist.
	path     string
	before   *string
	snapshot bool
}

// A tool result is clipped to this many bytes before it is sent; the model
// had the whole of it, and the session keeps it.
const reportedResultLimit = 20000

// A file larger than this is not sent as a diff.
const diffFileLimit = 1 << 20

func (r *turnReporter) update(u map[string]any) {
	if r.a.conn == nil {
		return
	}
	_ = r.a.conn.Notify("session/update", map[string]any{"sessionId": r.s.id, "update": u})
}

func textContent(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

// observe is handed each batch of messages the loop records.
func (r *turnReporter) observe(batch []llm.Message) {
	if r.title != nil {
		r.sendTitle(r.title())
	}
	for _, m := range batch {
		r.message(m, false)
	}
	r.sendPlan()
}

// replay sends a session's history, for session/load.
func (r *turnReporter) replay(messages []llm.Message) {
	for _, m := range messages {
		r.message(m, true)
	}
}

func (r *turnReporter) message(m llm.Message, replay bool) {
	switch m.Role {
	case llm.RoleUser:
		// A live turn's prompt came from the client; a follower message
		// carrying a tool's pictures is not the person's.
		if replay && strings.TrimSpace(m.Content) != "" && !strings.HasPrefix(m.Content, "[Attachment from ") {
			r.update(map[string]any{"sessionUpdate": "user_message_chunk", "content": textContent(m.Content)})
		}
	case llm.RoleAssistant:
		if strings.TrimSpace(m.ReasoningContent) != "" {
			r.update(map[string]any{"sessionUpdate": "agent_thought_chunk", "content": textContent(m.ReasoningContent)})
		}
		if strings.TrimSpace(m.Content) != "" {
			r.update(map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       textContent(m.Content),
				"messageId":     newID(),
			})
		}
		for _, tc := range m.ToolCalls {
			r.announce(tc)
		}
	case llm.RoleTool:
		r.finish(m, replay)
	}
}

func (r *turnReporter) internalName(name string) string {
	if r.registry != nil {
		return r.registry.InternalName(name)
	}
	return name
}

// toolKind is the protocol's kind for a tool: Read and ReadImage read,
// Search searches, the calls that write a file edit, Shell and the
// background process tools execute, Web.fetch and Web.search fetch, and
// every other tool is other. Names are compared with dots and underscores
// alike, since a replayed history has the names the model used.
func toolKind(name string) string {
	n := strings.ReplaceAll(name, "_", ".")
	switch {
	case n == "Read" || n == "ReadImage":
		return "read"
	case n == "Search":
		return "search"
	case n == "Edit" || n == "Write" || n == "Edit.confirm" || n == "Write.confirm" ||
		n == "DeleteLines" || n == "RestoreFile":
		return "edit"
	case n == "Shell" || strings.HasPrefix(n, "Shell."):
		return "execute"
	case n == "Web.fetch" || n == "Web.search":
		return "fetch"
	}
	return "other"
}

func toolTitle(name string, args map[string]any) string {
	for _, key := range []string{"command", "path", "pattern", "query", "url"} {
		if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
			v = strings.Join(strings.Fields(v), " ")
			if len(v) > 100 {
				v = v[:97] + "..."
			}
			return name + ": " + v
		}
	}
	return name
}

// pathOf is the file a call names, absolute. A confirmation names none: its
// file is the one its pending edit or write holds.
func (r *turnReporter) pathOf(name string, args map[string]any) string {
	var path string
	switch strings.ReplaceAll(name, "_", ".") {
	case "Edit.confirm":
		path = r.toolCtx.GetPendingEditPath()
	case "Write.confirm":
		path = r.toolCtx.GetPendingWritePath()
	default:
		for _, key := range []string{"path", "file_path", "file"} {
			if v, ok := args[key].(string); ok && v != "" {
				path = v
				break
			}
		}
	}
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) && r.workspace != "" {
		path = filepath.Join(r.workspace, path)
	}
	return filepath.Clean(path)
}

func (r *turnReporter) announce(tc llm.ToolCall) *reportedCall {
	name := r.internalName(tc.Function.Name)
	var rawInput any = tc.Function.Arguments
	args := map[string]any{}
	if json.Unmarshal([]byte(tc.Function.Arguments), &args) == nil {
		rawInput = args
	}
	call := &reportedCall{name: name, kind: toolKind(name)}
	if path := r.pathOf(name, args); path != "" {
		call.path = path
		call.locations = []map[string]any{{"path": path}}
	}
	r.calls[tc.ID] = call
	u := map[string]any{
		"sessionUpdate": "tool_call",
		"toolCallId":    tc.ID,
		"title":         toolTitle(name, args),
		"kind":          call.kind,
		"status":        "pending",
		"rawInput":      rawInput,
	}
	if call.locations != nil {
		u["locations"] = call.locations
	}
	r.update(u)
	return call
}

// toolStart is told about a call just before its checks and its run.
func (r *turnReporter) toolStart(_ string, tc llm.ToolCall) {
	r.current = tc.ID
	call := r.calls[tc.ID]
	if call == nil {
		call = r.announce(tc)
	}
	if call.kind == "edit" && call.path != "" {
		call.before, call.snapshot = readForDiff(call.path)
	}
	r.update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": tc.ID, "status": "in_progress"})
}

// readForDiff reads a file for a diff. ok is false when it cannot be
// compared: too large, or unreadable for a reason other than not existing.
func readForDiff(path string) (content *string, ok bool) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, true
	}
	if err != nil || info.IsDir() || info.Size() > diffFileLimit {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	text := string(data)
	return &text, true
}

func failedResult(content string) bool {
	return strings.HasPrefix(content, "Error") || strings.HasPrefix(content, "[interrupted")
}

func (r *turnReporter) finish(m llm.Message, replay bool) {
	status := "completed"
	if failedResult(m.Content) {
		status = "failed"
	}
	text := m.Content
	if len(text) > reportedResultLimit {
		text = text[:reportedResultLimit] + "\n[… cut here; the model had the whole result]"
	}
	content := []any{map[string]any{"type": "content", "content": textContent(text)}}
	if call := r.calls[m.ToolCallID]; call != nil && call.snapshot && !replay {
		if after, ok := readForDiff(call.path); ok && !sameContent(call.before, after) {
			diff := map[string]any{"type": "diff", "path": call.path, "newText": ""}
			if call.before != nil {
				diff["oldText"] = *call.before
			} else {
				diff["oldText"] = nil
			}
			if after != nil {
				diff["newText"] = *after
			}
			content = append(content, diff)
		}
	}
	if r.current == m.ToolCallID {
		r.current = ""
	}
	r.update(map[string]any{
		"sessionUpdate": "tool_call_update",
		"toolCallId":    m.ToolCallID,
		"status":        status,
		"content":       content,
	})
}

func sameContent(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// sendPlan sends the plan when it has changed since it was last sent.
func (r *turnReporter) sendPlan() {
	if r.plans == nil {
		return
	}
	entries := []map[string]any{}
	if plan := r.plans.GetActivePlan(); plan != nil {
		for _, step := range plan.Steps {
			status := "pending"
			switch step.Status {
			case "active":
				status = "in_progress"
			case "complete":
				status = "completed"
			}
			entries = append(entries, map[string]any{"content": step.Description, "priority": "medium", "status": status})
		}
	}
	data, _ := json.Marshal(entries)
	if r.lastPlan == "" && len(entries) == 0 {
		r.lastPlan = string(data)
		return
	}
	if string(data) == r.lastPlan {
		return
	}
	r.lastPlan = string(data)
	r.update(map[string]any{"sessionUpdate": "plan", "entries": entries})
}

func (r *turnReporter) sendTitle(title string) {
	if title == "" {
		return
	}
	r.s.mu.Lock()
	sent := r.s.title == title
	r.s.title = title
	r.s.mu.Unlock()
	if !sent {
		r.update(map[string]any{"sessionUpdate": "session_info_update", "title": title})
	}
}

func (r *turnReporter) sendUsage(size, used int, cost float64) {
	u := map[string]any{"sessionUpdate": "usage_update", "used": used, "size": size}
	if cost > 0 {
		u["cost"] = map[string]any{"amount": cost, "currency": "USD"}
	}
	r.update(u)
}
