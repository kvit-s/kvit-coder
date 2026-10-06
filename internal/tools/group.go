package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// A tool group is a set of tools the model cannot see until it asks for them.
// One registered tool stands in for the whole set: called with no arguments it
// returns the group's instructions and a compact reference to every tool in it,
// and called with a tool name and arguments it runs that tool.
//
// Why the reference arrives as a tool result, rather than by adding the tools
// to what the request advertises: the advertised tools sit ahead of the whole
// conversation in the server-side prompt cache, so changing them mid-session
// invalidates every message after them, which on a long conversation costs far
// more than the tools would have. A tool result is appended at the end, where
// the cache absorbs it, and it is written to the session history like any other
// result, so it replays on later turns without being sent again.
//
// The second saving is that nothing behind the group has to be running. An MCP
// server's tools can be described from a cached tools/list without dialing it,
// so a turn that opens the group and decides against it never spawns anything.
//
// GroupTool deliberately holds its members rather than registering them. They
// are absent from Registry.Specs() and from the system prompt, and the model
// reaches them only through this tool.

// GroupMemberSource resolves a group's members, and any notes that come with
// them. It is a function so a group can take its members from somewhere
// internal/tools must not import: cmd/kvit-coder builds one over the MCP
// manager, which is also what keeps the members' dial lazy. Nil means the
// group has only the members handed to it directly.
type GroupMemberSource func(ctx context.Context) ([]Tool, []GroupNote, error)

// GroupNote is guidance that arrives with some of a group's members rather
// than from the configuration: the instructions an MCP server sent for using
// its tools. The group shows each one, under Source, after its own
// instructions.
type GroupNote struct {
	Source string // what sent it, as the model should see it: "MCP server h3"
	Text   string
}

// GroupTool is the single registered tool that stands in for a group.
type GroupTool struct {
	name         string
	description  string
	instructions string
	category     string
	order        int

	source GroupMemberSource

	mu      sync.Mutex
	local   []Tool      // members handed over directly (built-in tools moved into the group)
	members []Tool      // resolved members, local first; nil until resolve runs
	notes   []GroupNote // from the source, alongside members
	err     error
}

// NewGroupTool builds the tool that stands in for one group. Members from
// source are resolved the first time something needs them, once per process.
func NewGroupTool(name, description, instructions, category string, order int, source GroupMemberSource) *GroupTool {
	return &GroupTool{
		name:         name,
		description:  strings.TrimSpace(description),
		instructions: strings.TrimSpace(instructions),
		category:     category,
		order:        order,
		source:       source,
	}
}

// AddLocalMembers hands the group tools that are already built -- built-in
// tools the configuration moved into this group. SetupRegistry calls it instead
// of registering them.
func (t *GroupTool) AddLocalMembers(members ...Tool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.local = append(t.local, members...)
	t.members, t.err = nil, nil
}

func (t *GroupTool) Name() string { return t.name }

func (t *GroupTool) Description() string {
	return t.description + " Call it with no arguments to get the list of what it can do, " +
		"then call it again with one of those tool names to run it."
}

func (t *GroupTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tool": map[string]any{
				"type": "string",
				"description": "The tool to run, from the list this one returns. " +
					"Leave it out to get that list.",
			},
			"args": map[string]any{
				"type":        "object",
				"description": "The arguments for that tool.",
			},
		},
	}
}

func (t *GroupTool) PromptCategory() string     { return t.category }
func (t *GroupTool) PromptOrder() int           { return t.order }
func (t *GroupTool) PromptTemplateName() string { return "" }

func (t *GroupTool) PromptSection() string {
	return fmt.Sprintf(`### %s

%s

The tools it holds are not described anywhere else, including in this prompt.
Call it with no arguments to get them:

%s({})

That comes back with instructions and one entry per tool, each with its
parameters. Then run one of them by name:

%s({"tool": "<name>", "args": {...}})`,
		t.name, t.description, t.name, t.name)
}

// ShortPromptSection says the one thing the schema cannot: that the tools exist
// but are undescribed until asked for.
func (t *GroupTool) ShortPromptSection() string {
	return fmt.Sprintf(`### %s

%s

Its tools are undescribed until you ask: %s({}) returns instructions and every
tool with its parameters, then %s({"tool": "<name>", "args": {...}}) runs one.
Asking costs one call and nothing is started until you run something.`,
		t.name, t.description, t.name, t.name)
}

// SelfTimeout opts out of the loop's blanket per-tool timeout: a group call is
// as slow as the member it dispatches to, and members set their own deadlines.
func (t *GroupTool) SelfTimeout() bool { return true }

type groupArgs struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

// Check validates the shape of the call only. Whether the named tool exists is
// settled in Call, because answering it means resolving the members and that
// can mean reaching a server -- work that belongs to the call itself.
func (t *GroupTool) Check(ctx context.Context, args json.RawMessage) error {
	if len(strings.TrimSpace(string(args))) == 0 {
		return nil
	}
	var parsed groupArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return SemanticErrorf("invalid arguments for %s: %v", t.name, err)
	}
	return nil
}

// Call either returns the group's reference (no tool named) or runs one member.
func (t *GroupTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var parsed groupArgs
	if len(strings.TrimSpace(string(args))) > 0 {
		if err := json.Unmarshal(args, &parsed); err != nil {
			return nil, SemanticErrorf("invalid arguments for %s: %v", t.name, err)
		}
	}

	members, notes, err := t.resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s is unavailable: %w", t.name, err)
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("%s has no tools available", t.name)
	}

	if strings.TrimSpace(parsed.Tool) == "" {
		return map[string]any{
			"success": true,
			"content": t.reference(members, notes),
		}, nil
	}

	member := findGroupMember(members, parsed.Tool)
	if member == nil {
		return nil, SemanticErrorf("%s has no tool %q. It has: %s",
			t.name, parsed.Tool, strings.Join(groupMemberLabels(members), ", "))
	}

	memberArgs := parsed.Args
	if len(memberArgs) == 0 {
		memberArgs = json.RawMessage("{}")
	}
	if normalized, err := NormalizeToolCallArguments(member, memberArgs); err == nil {
		memberArgs = normalized
	}

	if err := member.Check(ctx, memberArgs); err != nil {
		return nil, err
	}
	// The member's result is returned as it is, so anything the runner reads
	// off a result -- image attachments in particular -- still reaches it.
	return member.Call(ctx, memberArgs)
}

// resolve builds the member list once per process. A turn is one process, so
// the reveal and the calls that follow it in the same turn resolve once.
func (t *GroupTool) resolve(ctx context.Context) ([]Tool, []GroupNote, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.members != nil || t.err != nil {
		return t.members, t.notes, t.err
	}

	members := make([]Tool, 0, len(t.local))
	members = append(members, t.local...)

	var notes []GroupNote
	if t.source != nil {
		sourced, sourcedNotes, err := t.source(ctx)
		if err != nil {
			t.err = err
			return nil, nil, err
		}
		members = append(members, sourced...)
		notes = sourcedNotes
	}

	t.members, t.notes = members, notes
	return t.members, t.notes, nil
}

// reference renders what the model gets when it opens the group: how to run a
// member, the group's own instructions, what the members' servers asked to be
// told, then every member with its parameters.
func (t *GroupTool) reference(members []Tool, notes []GroupNote) string {
	labels := groupMemberLabels(members)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d tools in %s. Run one with:\n\n", len(members), t.name)
	fmt.Fprintf(&sb, "  %s({\"tool\": %q, \"args\": {...}})\n", t.name, labels[0])

	if t.instructions != "" {
		sb.WriteString("\n")
		sb.WriteString(t.instructions)
		sb.WriteString("\n")
	}
	for _, note := range notes {
		if strings.TrimSpace(note.Text) == "" {
			continue
		}
		fmt.Fprintf(&sb, "\nInstructions from %s:\n%s\n", note.Source, strings.TrimSpace(note.Text))
	}

	sb.WriteString("\n")
	for i, m := range members {
		sb.WriteString(renderGroupMember(labels[i], m))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// mcpNamePrefix is the namespace MCP tools are registered under
// (mcp.NamespacedName builds mcp.<server>.<tool>). It is spelled out here
// rather than imported because internal/tools must not depend on internal/mcp.
const mcpNamePrefix = "mcp."

// groupMemberLabels gives each member the shortest name that is unambiguous
// within the group. An MCP tool is registered as mcp.<server>.<tool>, and
// "browser_click" is what the model should have to type; the full name is kept
// when two servers in one group offer the same tool name.
//
// Only MCP names are shortened. A built-in tool moved into a group keeps the
// name it has everywhere else -- Shell.start stays Shell.start, because that is
// what the prompt and the session history call it.
func groupMemberLabels(members []Tool) []string {
	count := make(map[string]int, len(members))
	for _, m := range members {
		count[shortMemberName(m.Name())]++
	}
	labels := make([]string, len(members))
	for i, m := range members {
		short := shortMemberName(m.Name())
		if count[short] == 1 {
			labels[i] = short
			continue
		}
		labels[i] = m.Name()
	}
	return labels
}

func shortMemberName(name string) string {
	if !strings.HasPrefix(name, mcpNamePrefix) {
		return name
	}
	if i := strings.LastIndex(name, "."); i >= 0 && i < len(name)-1 {
		return name[i+1:]
	}
	return name
}

// findGroupMember accepts either the short label or the member's full
// registered name, so a model that reads mcp.playwright.browser_click off an
// earlier result and passes that is not punished for it.
func findGroupMember(members []Tool, want string) Tool {
	want = strings.TrimSpace(want)
	labels := groupMemberLabels(members)
	for i, m := range members {
		if m.Name() == want || labels[i] == want {
			return m
		}
	}
	// Last resort: match on the short name even where it was ambiguous, which
	// is better than refusing when only one of the candidates is plausible.
	var found Tool
	for _, m := range members {
		if shortMemberName(m.Name()) == want {
			if found != nil {
				return nil
			}
			found = m
		}
	}
	if found != nil {
		return found
	}
	// A model that prefixes every tool with a namespace it invented
	// ("default.browser_click") means the member name. Try each suffix after
	// a dot, longest first, against the same exact/label/short rules above.
	for i := 0; i < len(want); i++ {
		if want[i] != '.' {
			continue
		}
		suffix := strings.TrimSpace(want[i+1:])
		if suffix == "" {
			continue
		}
		if member := findGroupMember(members, suffix); member != nil {
			return member
		}
	}
	return nil
}

// renderGroupMember writes one member as a compact signature. This is not JSON
// Schema on purpose: the model reads it, nothing parses it, and the schema for
// a browser server's two dozen tools is about twice the size in JSON.
func renderGroupMember(label string, m Tool) string {
	var sb strings.Builder
	sb.WriteString(label)
	if desc := oneLine(m.Description(), 0); desc != "" {
		sb.WriteString(" — ")
		sb.WriteString(desc)
	}
	sb.WriteString("\n")
	sb.WriteString(renderGroupParams(m.JSONSchema()))
	return sb.String()
}

// renderGroupParams writes one line per top-level parameter: name, whether it
// is required, its type, and what it is for.
func renderGroupParams(schema map[string]any) string {
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return ""
	}

	required := make(map[string]bool)
	switch req := schema["required"].(type) {
	case []any:
		for _, r := range req {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	case []string:
		for _, r := range req {
			required[r] = true
		}
	}

	// Map iteration order is random, and this text goes into the conversation,
	// so it is sorted for the same reason Registry.Specs() sorts.
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, n := range names {
		p, _ := props[n].(map[string]any)
		optional := ""
		if !required[n] {
			optional = "?"
		}
		fmt.Fprintf(&sb, "  %s%s: %s", n, optional, schemaTypeName(p))
		if desc, _ := p["description"].(string); desc != "" {
			if d := oneLine(desc, 200); d != "" {
				sb.WriteString(" — ")
				sb.WriteString(d)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// schemaTypeName names a parameter's type the way a signature would, preferring
// the enum's values over the word "string" when there is one: knowing a field
// takes "error" or "warning" is worth more than knowing it takes text.
func schemaTypeName(p map[string]any) string {
	if p == nil {
		return "any"
	}
	if enum, ok := p["enum"].([]any); ok && len(enum) > 0 {
		vals := make([]string, 0, len(enum))
		for _, e := range enum {
			vals = append(vals, fmt.Sprintf("%v", e))
		}
		return strings.Join(vals, "|")
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		if alts, ok := p[key].([]any); ok && len(alts) > 0 {
			seen := make(map[string]bool, len(alts))
			names := make([]string, 0, len(alts))
			for _, a := range alts {
				am, _ := a.(map[string]any)
				n := schemaTypeName(am)
				if n == "" || n == "any" || seen[n] {
					continue
				}
				seen[n] = true
				names = append(names, n)
			}
			if len(names) > 0 {
				return strings.Join(names, "|")
			}
		}
	}

	typeName := ""
	switch tv := p["type"].(type) {
	case string:
		typeName = tv
	case []any:
		names := make([]string, 0, len(tv))
		for _, v := range tv {
			if s, ok := v.(string); ok && s != "null" {
				names = append(names, s)
			}
		}
		typeName = strings.Join(names, "|")
	}
	if typeName == "" {
		return "any"
	}
	if typeName == "array" {
		items, _ := p["items"].(map[string]any)
		return schemaTypeName(items) + "[]"
	}
	return typeName
}

// oneLine collapses whitespace so a multi-paragraph description takes one line,
// and caps the result when a limit is given. Zero means no cap.
func oneLine(s string, limit int) string {
	out := strings.Join(strings.Fields(s), " ")
	if limit > 0 && len(out) > limit {
		out = strings.TrimSpace(out[:limit]) + "…"
	}
	return out
}
