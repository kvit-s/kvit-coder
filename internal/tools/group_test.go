package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// fakeMember is a stand-in for a tool held by a group. It records what it was
// asked to do so a test can tell Check from Call.
type fakeMember struct {
	name     string
	desc     string
	schema   map[string]any
	checkErr error
	callErr  error

	checked  int
	called   int
	lastArgs string
}

func (f *fakeMember) Name() string        { return f.name }
func (f *fakeMember) Description() string { return f.desc }
func (f *fakeMember) JSONSchema() map[string]any {
	if f.schema == nil {
		return map[string]any{"type": "object"}
	}
	return f.schema
}
func (f *fakeMember) Check(context.Context, json.RawMessage) error {
	f.checked++
	return f.checkErr
}
func (f *fakeMember) Call(_ context.Context, args json.RawMessage) (any, error) {
	f.called++
	f.lastArgs = string(args)
	if f.callErr != nil {
		return nil, f.callErr
	}
	return map[string]any{"ok": true}, nil
}
func (f *fakeMember) PromptSection() string      { return "" }
func (f *fakeMember) PromptCategory() string     { return "mcp" }
func (f *fakeMember) PromptOrder() int           { return 0 }
func (f *fakeMember) PromptTemplateName() string { return "" }

func staticSource(members ...Tool) GroupMemberSource {
	return func(context.Context) ([]Tool, []GroupNote, error) { return members, nil, nil }
}

func callGroup(t *testing.T, g *GroupTool, args string) (any, error) {
	t.Helper()
	raw := json.RawMessage(args)
	if err := g.Check(context.Background(), raw); err != nil {
		return nil, err
	}
	return g.Call(context.Background(), raw)
}

func revealText(t *testing.T, g *GroupTool) string {
	t.Helper()
	res, err := callGroup(t, g, "{}")
	if err != nil {
		t.Fatalf("reveal: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("reveal returned %T, want map", res)
	}
	content, _ := m["content"].(string)
	if content == "" {
		t.Fatal("reveal returned no content")
	}
	return content
}

func TestGroupRevealListsMembersAndInstructions(t *testing.T) {
	g := NewGroupTool("Web.browsing", "Drive a real browser.", "Work from a snapshot.", "web", 900,
		staticSource(
			&fakeMember{
				name: "mcp.playwright.browser_click",
				desc: "Perform click on a web page",
				schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"ref":         map[string]any{"type": "string", "description": "Element reference"},
						"doubleClick": map[string]any{"type": "boolean", "description": "Double click"},
					},
					"required": []any{"ref"},
				},
			},
			&fakeMember{name: "mcp.playwright.browser_close", desc: "Close the page"},
		))

	got := revealText(t, g)

	for _, want := range []string{
		"2 tools in Web.browsing",
		`Web.browsing({"tool": "browser_click"`,
		"Work from a snapshot.",
		"browser_click — Perform click on a web page",
		"  ref: string — Element reference",
		"  doubleClick?: boolean — Double click",
		"browser_close — Close the page",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("reveal missing %q\n---\n%s", want, got)
		}
	}

	// Required and optional must be distinguishable, or the model guesses.
	if strings.Contains(got, "ref?:") {
		t.Errorf("required parameter marked optional\n---\n%s", got)
	}
}

func TestGroupRevealIsDeterministic(t *testing.T) {
	// Parameters come out of a Go map, whose iteration order is random. The
	// reveal goes into the conversation, so it has to be stable.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"zulu": map[string]any{"type": "string"}, "alpha": map[string]any{"type": "string"},
			"mike": map[string]any{"type": "string"}, "bravo": map[string]any{"type": "string"},
		},
	}
	first := ""
	for i := 0; i < 20; i++ {
		g := NewGroupTool("G", "d", "", "mcp", 900,
			staticSource(&fakeMember{name: "a", desc: "x", schema: schema}))
		got := revealText(t, g)
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("reveal differs between builds:\n%s\n---\n%s", first, got)
		}
	}
	alpha, mike := strings.Index(first, "alpha"), strings.Index(first, "mike")
	if alpha < 0 || mike < 0 || alpha > mike {
		t.Errorf("parameters not sorted:\n%s", first)
	}
}

func TestGroupDispatchesToMember(t *testing.T) {
	member := &fakeMember{name: "mcp.playwright.browser_navigate", desc: "Navigate"}
	g := NewGroupTool("Web.browsing", "d", "", "web", 900, staticSource(member))

	res, err := callGroup(t, g, `{"tool":"browser_navigate","args":{"url":"https://example.com"}}`)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if member.checked != 1 || member.called != 1 {
		t.Fatalf("member checked %d times, called %d; want 1 and 1", member.checked, member.called)
	}
	if !strings.Contains(member.lastArgs, "example.com") {
		t.Errorf("member got args %q", member.lastArgs)
	}
	// The member's own result is returned untouched, so anything the runner
	// reads off a result still reaches it.
	if m, ok := res.(map[string]any); !ok || m["ok"] != true {
		t.Errorf("member result was rewritten: %#v", res)
	}
}

func TestGroupAcceptsFullMemberName(t *testing.T) {
	member := &fakeMember{name: "mcp.playwright.browser_navigate", desc: "Navigate"}
	g := NewGroupTool("Web.browsing", "d", "", "web", 900, staticSource(member))

	if _, err := callGroup(t, g, `{"tool":"mcp.playwright.browser_navigate","args":{}}`); err != nil {
		t.Fatalf("full name rejected: %v", err)
	}
	if member.called != 1 {
		t.Fatalf("member called %d times, want 1", member.called)
	}
}

func TestGroupCheckFailureStopsTheCall(t *testing.T) {
	member := &fakeMember{name: "mcp.s.t", desc: "d", checkErr: errors.New("refused by policy")}
	g := NewGroupTool("G", "d", "", "mcp", 900, staticSource(member))

	_, err := callGroup(t, g, `{"tool":"t","args":{}}`)
	if err == nil || !strings.Contains(err.Error(), "refused by policy") {
		t.Fatalf("got %v, want the member's Check error", err)
	}
	if member.called != 0 {
		t.Error("member ran despite Check failing")
	}
}

func TestGroupUnknownToolListsWhatItHas(t *testing.T) {
	g := NewGroupTool("Web.browsing", "d", "", "web", 900, staticSource(
		&fakeMember{name: "mcp.p.browser_click", desc: "d"},
		&fakeMember{name: "mcp.p.browser_type", desc: "d"},
	))

	_, err := callGroup(t, g, `{"tool":"browser_scroll","args":{}}`)
	if err == nil {
		t.Fatal("unknown tool accepted")
	}
	// The error doubles as a second reveal, so a model that guessed a name is
	// not left having to ask again.
	for _, want := range []string{"browser_scroll", "browser_click", "browser_type"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %s", want, err)
		}
	}
	var toolErr *ToolError
	if !errors.As(err, &toolErr) || toolErr.Type != ToolErrorSemantic {
		t.Errorf("want a semantic error, got %T", err)
	}
}

func TestGroupDisambiguatesCollidingShortNames(t *testing.T) {
	a := &fakeMember{name: "mcp.one.search", desc: "d"}
	b := &fakeMember{name: "mcp.two.search", desc: "d"}
	g := NewGroupTool("G", "d", "", "mcp", 900, staticSource(a, b, &fakeMember{name: "mcp.one.fetch", desc: "d"}))

	got := revealText(t, g)
	if !strings.Contains(got, "mcp.one.search") || !strings.Contains(got, "mcp.two.search") {
		t.Errorf("colliding members not listed by full name:\n%s", got)
	}
	if !strings.Contains(got, "fetch — ") {
		t.Errorf("unambiguous member not listed by short name:\n%s", got)
	}

	if _, err := callGroup(t, g, `{"tool":"mcp.two.search","args":{}}`); err != nil {
		t.Fatalf("full name rejected: %v", err)
	}
	if b.called != 1 || a.called != 0 {
		t.Errorf("dispatched to the wrong member: one=%d two=%d", a.called, b.called)
	}
}

func TestGroupResolvesMembersOnce(t *testing.T) {
	calls := 0
	g := NewGroupTool("G", "d", "", "mcp", 900, func(context.Context) ([]Tool, []GroupNote, error) {
		calls++
		return []Tool{&fakeMember{name: "mcp.s.t", desc: "d"}}, nil, nil
	})

	revealText(t, g)
	if _, err := callGroup(t, g, `{"tool":"t","args":{}}`); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if calls != 1 {
		t.Errorf("member source ran %d times in one turn, want 1", calls)
	}
}

func TestGroupSourceFailureIsReported(t *testing.T) {
	g := NewGroupTool("Web.browsing", "d", "", "web", 900,
		func(context.Context) ([]Tool, []GroupNote, error) { return nil, nil, errors.New("npx not found") })

	_, err := callGroup(t, g, "{}")
	if err == nil || !strings.Contains(err.Error(), "npx not found") ||
		!strings.Contains(err.Error(), "Web.browsing") {
		t.Fatalf("got %v, want an error naming the group and the cause", err)
	}
}

func TestGroupCombinesLocalAndSourcedMembers(t *testing.T) {
	local := &fakeMember{name: "Shell.start", desc: "Run detached"}
	g := NewGroupTool("Background", "d", "", "shell", 900,
		staticSource(&fakeMember{name: "mcp.s.remote", desc: "Remote"}))
	g.AddLocalMembers(local)

	got := revealText(t, g)
	if !strings.Contains(got, "Shell.start — Run detached") {
		t.Errorf("local member missing:\n%s", got)
	}
	if !strings.Contains(got, "remote — Remote") {
		t.Errorf("sourced member missing:\n%s", got)
	}
	if !strings.Contains(got, "2 tools in Background") {
		t.Errorf("member count wrong:\n%s", got)
	}
}

func TestGroupRejectsMalformedArguments(t *testing.T) {
	g := NewGroupTool("G", "d", "", "mcp", 900, staticSource(&fakeMember{name: "t", desc: "d"}))
	if err := g.Check(context.Background(), json.RawMessage(`{"tool":`)); err == nil {
		t.Fatal("malformed arguments accepted")
	}
}

func TestSchemaTypeName(t *testing.T) {
	cases := []struct {
		name   string
		schema map[string]any
		want   string
	}{
		{"plain", map[string]any{"type": "string"}, "string"},
		{"missing", map[string]any{}, "any"},
		{"nil", nil, "any"},
		{"enum beats type", map[string]any{"type": "string", "enum": []any{"error", "warning"}}, "error|warning"},
		{"array of strings", map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "string[]"},
		{"array of anything", map[string]any{"type": "array"}, "any[]"},
		{"nullable", map[string]any{"type": []any{"string", "null"}}, "string"},
		{"anyOf", map[string]any{"anyOf": []any{
			map[string]any{"type": "string"}, map[string]any{"type": "number"},
		}}, "string|number"},
		{"anyOf dedupes", map[string]any{"oneOf": []any{
			map[string]any{"type": "string"}, map[string]any{"type": "string"},
		}}, "string"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := schemaTypeName(c.schema); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestOneLineCollapsesAndCaps(t *testing.T) {
	if got := oneLine("first line\n\nsecond   line\t", 0); got != "first line second line" {
		t.Errorf("got %q", got)
	}
	got := oneLine(strings.Repeat("x", 300), 200)
	if len([]rune(got)) != 201 || !strings.HasSuffix(got, "…") {
		t.Errorf("cap not applied: %d runes, %q", len([]rune(got)), got[len(got)-5:])
	}
}

func TestSetupRegistryRegistersGroupsAndClaimsTools(t *testing.T) {
	cfg := newTestConfig()
	cfg.Tools.Read.Enabled = true
	cfg.ToolGroups = []config.ToolGroupConfig{{
		Name:        "Web.browsing",
		Enabled:     true,
		Category:    "web",
		Description: "Drive a real browser.",
		Tools:       []string{"Read"},
	}}

	group := NewGroupTool("Web.browsing", "Drive a real browser.", "", "web", 900, nil)
	reg := SetupRegistry(SetupConfig{Cfg: cfg, ToolGroups: []*GroupTool{group}})

	if !reg.IsEnabled("Web.browsing") {
		t.Fatal("group tool was not registered")
	}
	// A claimed tool stops being advertised: that is the whole point.
	if reg.IsEnabled("Read") {
		t.Error("Read is still registered despite belonging to a group")
	}
	for _, spec := range reg.Specs() {
		if spec.Function.Name == "Read" {
			t.Error("Read is still in the tool specs sent to the model")
		}
	}

	got := revealText(t, group)
	if !strings.Contains(got, "Read") {
		t.Errorf("claimed tool is not reachable through its group:\n%s", got)
	}
}

func TestSetupRegistryWithoutGroupsIsUnchanged(t *testing.T) {
	cfg := newTestConfig()
	cfg.Tools.Read.Enabled = true

	reg := SetupRegistry(SetupConfig{Cfg: cfg})
	if !reg.IsEnabled("Read") {
		t.Error("Read should be registered when no group claims it")
	}
	if len(reg.ListTools()) == 0 {
		t.Error("no tools registered at all")
	}
}

// What a member's server sent comes after the group's own instructions and
// before the tools, under the name of what sent it. A blank note adds nothing.
func TestGroupReferenceShowsNotesFromItsSource(t *testing.T) {
	g := NewGroupTool("Heroes3", "Play the game.", "Group instructions.", "mcp", 900,
		func(context.Context) ([]Tool, []GroupNote, error) {
			return []Tool{&fakeMember{name: "mcp.h3.move", desc: "Moves a hero."}},
				[]GroupNote{{Source: "MCP server h3", Text: "Read the map first.\nThen move."}, {Source: "MCP server blank", Text: "  "}}, nil
		})
	text := revealText(t, g)
	own := strings.Index(text, "Group instructions.")
	note := strings.Index(text, "Instructions from MCP server h3:\nRead the map first.\nThen move.")
	tool := strings.Index(text, "move")
	if own < 0 || note < 0 || own > note {
		t.Fatalf("want the group's instructions, then the server's:\n%s", text)
	}
	if tool < 0 || strings.LastIndex(text, "Moves a hero.") < note {
		t.Errorf("the tools should follow the notes:\n%s", text)
	}
	if strings.Contains(text, "MCP server blank") {
		t.Errorf("a blank note was shown:\n%s", text)
	}
}
