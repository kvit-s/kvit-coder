package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// activityCap is how many entries each list on the activity line names before
// it says how many more there were.
const activityCap = 8

// commandChars is how much of a shell command the activity line keeps.
const commandChars = 80

// ActivityLine says in one line which files a turn changed and read and which
// commands it ran, taken from its tool calls' arguments, or "" when it did
// none of those. A turn summary keeps it beside the final answer, which often
// does not say where the work happened.
func ActivityLine(msgs []llm.Message) string {
	var changed, read, ran orderedSet
	for _, m := range msgs {
		if m.Role != llm.RoleAssistant {
			continue
		}
		for _, tc := range m.ToolCalls {
			collectActivity(tc.Function.Name, json.RawMessage(tc.Function.Arguments), &changed, &read, &ran, 0)
		}
	}
	var parts []string
	if s := changed.String(""); s != "" {
		parts = append(parts, "changed "+s)
	}
	if s := read.String(""); s != "" {
		parts = append(parts, "read "+s)
	}
	if s := ran.String("`"); s != "" {
		parts = append(parts, "ran "+s)
	}
	if len(parts) == 0 {
		return ""
	}
	return "[In this turn: " + strings.Join(parts, "; ") + "]"
}

func collectActivity(name string, raw json.RawMessage, changed, read, ran *orderedSet, depth int) {
	var args map[string]json.RawMessage
	if json.Unmarshal(raw, &args) != nil {
		return
	}
	str := func(key string) string {
		var v string
		if b, ok := args[key]; ok {
			_ = json.Unmarshal(b, &v)
		}
		return strings.TrimSpace(v)
	}
	// Dotted tool names reach the wire with underscores (Shell.start becomes
	// Shell_start), and the history keeps what the model sent.
	switch strings.ReplaceAll(name, "_", ".") {
	case "Edit", "Write", "DeleteLines":
		if p := str("path"); p != "" {
			changed.Add(p)
		}
		for _, p := range patchPaths(str("patch")) {
			changed.Add(p)
		}
	case "Read", "ReadImage":
		if p := str("path"); p != "" {
			read.Add(p)
		}
	case "Shell", "Shell.start":
		if c := firstLine(str("command")); c != "" {
			if len(c) > commandChars {
				c = c[:commandChars] + "…"
			}
			ran.Add(c)
		}
	case "Batch":
		if depth > 0 {
			return
		}
		var calls []struct {
			Tool string          `json:"tool"`
			Args json.RawMessage `json:"args"`
		}
		if b, ok := args["calls"]; ok && json.Unmarshal(b, &calls) == nil {
			for _, c := range calls {
				collectActivity(c.Tool, c.Args, changed, read, ran, depth+1)
			}
		}
	}
}

// patchPaths are the files a patch-mode edit names.
func patchPaths(patch string) []string {
	var out []string
	for _, line := range strings.Split(patch, "\n") {
		for _, prefix := range []string{"*** Update File:", "*** Add File:", "*** Delete File:"} {
			if strings.HasPrefix(line, prefix) {
				if p := strings.TrimSpace(strings.TrimPrefix(line, prefix)); p != "" {
					out = append(out, p)
				}
			}
		}
	}
	return out
}

// orderedSet keeps the first occurrence of each entry, in order.
type orderedSet struct {
	items []string
	seen  map[string]bool
}

func (s *orderedSet) Add(v string) {
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	if s.seen[v] {
		return
	}
	s.seen[v] = true
	s.items = append(s.items, v)
}

func (s *orderedSet) String(quote string) string {
	if len(s.items) == 0 {
		return ""
	}
	shown := s.items
	if len(shown) > activityCap {
		shown = shown[:activityCap]
	}
	quoted := make([]string, len(shown))
	for i, v := range shown {
		quoted[i] = quote + v + quote
	}
	out := strings.Join(quoted, ", ")
	if extra := len(s.items) - len(shown); extra > 0 {
		out += fmt.Sprintf(" and %d more", extra)
	}
	return out
}

// firstLine is s up to its first newline.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
