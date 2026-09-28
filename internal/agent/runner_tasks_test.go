package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// tasksTranscriptSummary names the outcome of each Tasks tool in one line,
// and stays silent for anything else.
func TestTasksTranscriptSummary(t *testing.T) {
	cases := []struct {
		name  string
		tool  string
		res   any
		want  []string // every string must appear in the summary
		empty bool     // want "" instead
	}{
		{
			name: "start names branch parent and checkpoint",
			tool: "Tasks.Start",
			res: map[string]any{
				"status": "task started", "task": "explore",
				"branch": "1", "parent_branch": "0", "checkpoint_id": "checkpoint_197",
			},
			want: []string{"[tasks]", "branch 1", "parent 0", "checkpoint_197", "explore"},
		},
		{
			name: "finish with changes names summary and diff size",
			tool: "Tasks.Finish",
			res: map[string]any{
				"status": "Task completed.", "summary": "found it",
				"success": true, "has_changes": true, "diff": strings.Repeat("x", 2000),
			},
			want: []string{"[tasks]", "finished", "found it", "has_changes=true", "2.0k chars"},
		},
		{
			name: "finish failure is marked",
			tool: "Tasks.Finish",
			res: map[string]any{
				"summary": "nope", "success": false, "has_changes": false,
			},
			want: []string{"success=false", "has_changes=false"},
		},
		{
			name: "accept",
			tool: "Tasks.AcceptDiff",
			res:  map[string]any{"status": "accepted"},
			want: []string{"accepted"},
		},
		{
			name: "decline",
			tool: "Tasks.DeclineDiff",
			res:  map[string]any{"status": "declined"},
			want: []string{"discarded"},
		},
		{
			name: "revert names path",
			tool: "Tasks.RevertFile",
			res:  map[string]any{"status": "reverted", "path": "a.go", "content_size": 1200},
			want: []string{"reverted", "a.go"},
		},
		{
			name:  "non-tasks tool is silent",
			tool:  "Shell",
			res:   map[string]any{"exit_code": 0},
			empty: true,
		},
		{
			name:  "non-map result is silent",
			tool:  "Tasks.Start",
			res:   "task started",
			empty: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tasksTranscriptSummary(tc.tool, tc.res)
			if tc.empty {
				if got != "" {
					t.Fatalf("tasksTranscriptSummary(%q) = %q, want empty", tc.tool, got)
				}
				return
			}
			if got == "" {
				t.Fatalf("tasksTranscriptSummary(%q) is empty, want %v", tc.tool, tc.want)
			}
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("summary %q misses %q", got, w)
				}
			}
			if strings.Contains(got, "\n") {
				t.Errorf("summary is not one line: %q", got)
			}
		})
	}
}

// A Tasks tool success leaves a [tasks] line in the transcript; any other
// tool stays silent on success, as before.
func TestTasksToolResultShownInTranscript(t *testing.T) {
	mkCall := func(name string, args any) llm.ToolCall {
		raw, _ := json.Marshal(args)
		return llm.ToolCall{
			ID:   "call_1",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      name,
				Arguments: string(raw),
			},
		}
	}

	t.Run("tasks start", func(t *testing.T) {
		cfg := testConfig()
		tool := &scriptedTool{name: "Tasks.Start", call: func(context.Context, json.RawMessage) (any, error) {
			return map[string]any{
				"status": "task started", "task": "explore",
				"branch": "1", "parent_branch": "0", "checkpoint_id": "checkpoint_197",
			}, nil
		}}
		r, out := newTestRunner(t, cfg, nil, tool)
		_, _, _, _, _ = r.executeToolWithTimeout(context.Background(), tool, "Tasks.Start",
			mkCall("Tasks.Start", map[string]any{"task": "explore"}), &runState{})
		if got := out.String(); !strings.Contains(got, "[tasks]") || !strings.Contains(got, "branch 1") {
			t.Fatalf("transcript misses tasks start line:\n%s", got)
		}
	})

	t.Run("ordinary tool silent", func(t *testing.T) {
		cfg := testConfig()
		tool := &scriptedTool{name: "Read", call: func(context.Context, json.RawMessage) (any, error) {
			return map[string]any{"content": "hi"}, nil
		}}
		r, out := newTestRunner(t, cfg, nil, tool)
		_, _, _, _, _ = r.executeToolWithTimeout(context.Background(), tool, "Read",
			mkCall("Read", map[string]any{"path": "a.go"}), &runState{})
		if got := out.String(); strings.Contains(got, "[tasks]") {
			t.Fatalf("non-tasks tool leaked a tasks line:\n%s", got)
		}
	})
}
