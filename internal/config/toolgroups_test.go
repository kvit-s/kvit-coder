package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes a config file into a temp directory and loads it, so the
// tests exercise the same path a real start does, including the validation that
// happens inside Load.
func writeConfig(t *testing.T, body string) (*Config, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

const browserServers = `
mcp:
  enabled: true
  servers:
    - name: playwright
      enabled: true
      command: npx
`

func TestToolGroupLoads(t *testing.T) {
	cfg, err := writeConfig(t, browserServers+`
tool_groups:
  - name: Web.browsing
    enabled: true
    category: web
    description: Drive a real browser.
    instructions: Work from a snapshot.
    mcp_servers: [playwright]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	groups := cfg.EnabledToolGroups()
	if len(groups) != 1 || groups[0].Name != "Web.browsing" {
		t.Fatalf("got %+v", groups)
	}
	if got := groups[0].GetCategory(); got != "web" {
		t.Errorf("category %q", got)
	}
	if !cfg.DeferredMCPServers()["playwright"] {
		t.Error("playwright should be deferred to its group, not dialed at startup")
	}
}

func TestToolGroupCategoryDefaultsToMCP(t *testing.T) {
	cfg, err := writeConfig(t, browserServers+`
tool_groups:
  - name: G
    enabled: true
    description: d
    mcp_servers: [playwright]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.ToolGroups[0].GetCategory(); got != "mcp" {
		t.Errorf("got %q, want mcp", got)
	}
}

func TestToolGroupDisabledClaimsNothing(t *testing.T) {
	// A disabled group must leave its server on the normal startup path, or
	// switching a group off would silently remove its tools altogether.
	cfg, err := writeConfig(t, browserServers+`
tool_groups:
  - name: Web.browsing
    enabled: false
    description: d
    mcp_servers: [playwright]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.EnabledToolGroups()) != 0 {
		t.Error("disabled group is enabled")
	}
	if len(cfg.DeferredMCPServers()) != 0 {
		t.Error("disabled group still deferred its server")
	}
}

func TestToolGroupClaimsBuiltinTools(t *testing.T) {
	cfg, err := writeConfig(t, `
tool_groups:
  - name: Background
    enabled: true
    category: shell
    description: d
    tools: [Shell.start, Observe.wait]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	claimed := cfg.DeferredBuiltinTools()
	if claimed["Shell.start"] != "Background" || claimed["Observe.wait"] != "Background" {
		t.Errorf("got %v", claimed)
	}
}

func TestToolGroupInstructionsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "browsing.md"), []byte("Work from a snapshot.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(browserServers+`
tool_groups:
  - name: Web.browsing
    enabled: true
    description: d
    instructions_file: browsing.md
    mcp_servers: [playwright]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.ToolGroups[0].Instructions; got != "Work from a snapshot." {
		t.Errorf("got %q", got)
	}
}

func TestToolGroupRejections(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"unknown server",
			browserServers + "\ntool_groups:\n  - name: G\n    enabled: true\n    description: d\n    mcp_servers: [nope]\n",
			"not in mcp.servers",
		},
		{
			"two groups, one server",
			browserServers + `
tool_groups:
  - name: A
    enabled: true
    description: d
    mcp_servers: [playwright]
  - name: B
    enabled: true
    description: d
    mcp_servers: [playwright]
`,
			`already belongs to group "A"`,
		},
		{
			"duplicate group name",
			browserServers + `
tool_groups:
  - name: A
    enabled: true
    description: d
    mcp_servers: [playwright]
  - name: A
    enabled: false
    description: d
    tools: [Read]
`,
			"duplicate group name",
		},
		{
			"no name",
			"tool_groups:\n  - enabled: true\n    description: d\n    tools: [Read]\n",
			"name is required",
		},
		{
			"no description",
			"tool_groups:\n  - name: G\n    enabled: true\n    tools: [Read]\n",
			"description is required",
		},
		{
			"no members",
			"tool_groups:\n  - name: G\n    enabled: true\n    description: d\n",
			"needs mcp_servers or tools",
		},
		{
			"unknown category",
			"tool_groups:\n  - name: G\n    enabled: true\n    category: nonsense\n    description: d\n    tools: [Read]\n",
			"unknown category",
		},
		{
			"group inside itself",
			"tool_groups:\n  - name: G\n    enabled: true\n    description: d\n    tools: [G]\n",
			"cannot contain itself",
		},
		{
			"missing instructions file",
			"tool_groups:\n  - name: G\n    enabled: true\n    description: d\n    instructions_file: gone.md\n    tools: [Read]\n",
			"instructions_file",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := writeConfig(t, c.body)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %q, want it to mention %q", err, c.want)
			}
		})
	}
}
