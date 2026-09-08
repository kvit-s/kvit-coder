package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A tool group is a set of tools the model cannot see until it asks for them.
// One registered tool stands in for the whole set: called with no arguments it
// returns the group's instructions and a compact reference to every tool in it,
// and called with a tool name and arguments it runs that tool.
//
// The reason to have one is that a large tool set is expensive to advertise on
// every request even when it is never used. A browser server advertises two
// dozen tools whose schemas are about 4,000 tokens, doubles the number of
// options the model reads before every decision, and -- because the agent is
// one process per turn -- is spawned and killed on every turn whether or not
// the turn touches a web page. Behind a group it costs one paragraph, and
// nothing is spawned until the model calls something.
//
// The group's tools arrive as a tool result rather than by rewriting the tools
// the request advertises. That is deliberate: the advertised tools sit ahead of
// the whole conversation in the server-side prompt cache, so changing them
// invalidates every message after them, while a tool result is appended at the
// end where the cache absorbs it.

// ToolGroupConfig configures one tool group.
type ToolGroupConfig struct {
	// Name is the tool the model sees, e.g. "Web.browsing". Must be unique
	// across groups and must not collide with a built-in tool.
	Name string `yaml:"name"`

	// Enabled opts the group in. Zero value false = skipped (opt-in).
	Enabled bool `yaml:"enabled"`

	// Description is what the model reads when deciding whether to open the
	// group, and the only thing a group costs while it is unopened. Write it
	// as when to reach for the group rather than as a list of what is in it.
	Description string `yaml:"description"`

	// Category places the group's documentation in the system prompt:
	// "filesystem", "shell", "web", "plan", "checkpoint" or "mcp".
	// Defaults to "mcp".
	Category string `yaml:"category"`

	// Instructions is prose returned with the reference, for the things the
	// member schemas cannot say. InstructionsFile is read into it at load
	// time when set; a relative path resolves against the config file's
	// directory, so a missing file fails at startup rather than at the call.
	Instructions     string `yaml:"instructions"`
	InstructionsFile string `yaml:"instructions_file"`

	// MCPServers names entries of mcp.servers whose tools become this group's
	// members. Those servers are not dialed at startup and their tools are not
	// advertised to the model directly.
	MCPServers []string `yaml:"mcp_servers"`

	// Tools names built-in tools to move into this group. They are registered
	// as members instead of as tools of their own, so they stop appearing in
	// every request.
	Tools []string `yaml:"tools"`
}

// GetCategory returns the system-prompt category, defaulting to "mcp".
func (g *ToolGroupConfig) GetCategory() string {
	if g.Category == "" {
		return "mcp"
	}
	return g.Category
}

// toolGroupCategories are the categories the registry knows how to render.
var toolGroupCategories = map[string]bool{
	"filesystem": true,
	"shell":      true,
	"web":        true,
	"plan":       true,
	"checkpoint": true,
	"mcp":        true,
}

// EnabledToolGroups returns the groups that are switched on, in configuration
// order. Order is stable so the tools the model is offered stay byte-identical
// between requests and the server-side prompt cache keeps hitting.
func (c *Config) EnabledToolGroups() []ToolGroupConfig {
	var out []ToolGroupConfig
	for _, g := range c.ToolGroups {
		if g.Enabled {
			out = append(out, g)
		}
	}
	return out
}

// DeferredMCPServers returns the names of MCP servers that belong to an enabled
// group. Those are the servers Connect must not dial at startup: the group
// dials them the first time the model calls one of their tools.
func (c *Config) DeferredMCPServers() map[string]bool {
	out := make(map[string]bool)
	for _, g := range c.EnabledToolGroups() {
		for _, s := range g.MCPServers {
			out[s] = true
		}
	}
	return out
}

// DeferredBuiltinTools returns the built-in tool names claimed by an enabled
// group, mapped to the group that claimed them. SetupRegistry uses it to hand
// those tools to their group instead of registering them.
func (c *Config) DeferredBuiltinTools() map[string]string {
	out := make(map[string]string)
	for _, g := range c.EnabledToolGroups() {
		for _, t := range g.Tools {
			out[t] = g.Name
		}
	}
	return out
}

// validateToolGroups checks the tool_groups: section and resolves each group's
// instructions_file into Instructions. Everything here fails at startup with
// the group's name, rather than at the moment the model first opens a group.
func (c *Config) validateToolGroups(configPath string) error {
	configDir := filepath.Dir(configPath)

	seenName := make(map[string]bool, len(c.ToolGroups))
	claimedServer := make(map[string]string)
	claimedTool := make(map[string]string)

	serverExists := make(map[string]bool, len(c.MCP.Servers))
	for _, s := range c.MCP.Servers {
		serverExists[s.Name] = true
	}

	for i := range c.ToolGroups {
		g := &c.ToolGroups[i]
		where := fmt.Sprintf("tool_groups[%d]", i)
		if g.Name != "" {
			where = fmt.Sprintf("tool_groups[%d] (%s)", i, g.Name)
		}

		if strings.TrimSpace(g.Name) == "" {
			return fmt.Errorf("%s: %s: name is required", configPath, where)
		}
		if seenName[g.Name] {
			return fmt.Errorf("%s: %s: duplicate group name", configPath, where)
		}
		seenName[g.Name] = true

		if !g.Enabled {
			continue
		}

		if strings.TrimSpace(g.Description) == "" {
			return fmt.Errorf("%s: %s: description is required; it is the only thing "+
				"the model sees before opening the group", configPath, where)
		}
		if !toolGroupCategories[g.GetCategory()] {
			return fmt.Errorf("%s: %s: unknown category %q", configPath, where, g.Category)
		}
		if len(g.MCPServers) == 0 && len(g.Tools) == 0 {
			return fmt.Errorf("%s: %s: needs mcp_servers or tools, or it has no members", configPath, where)
		}

		for _, s := range g.MCPServers {
			if !serverExists[s] {
				return fmt.Errorf("%s: %s: mcp_servers names %q, which is not in mcp.servers", configPath, where, s)
			}
			if owner, taken := claimedServer[s]; taken {
				return fmt.Errorf("%s: %s: mcp server %q already belongs to group %q", configPath, where, s, owner)
			}
			claimedServer[s] = g.Name
		}

		for _, t := range g.Tools {
			if owner, taken := claimedTool[t]; taken {
				return fmt.Errorf("%s: %s: tool %q already belongs to group %q", configPath, where, t, owner)
			}
			claimedTool[t] = g.Name
			if t == g.Name {
				return fmt.Errorf("%s: %s: a group cannot contain itself", configPath, where)
			}
		}

		if g.InstructionsFile != "" {
			path := g.InstructionsFile
			if !filepath.IsAbs(path) {
				path = filepath.Join(configDir, path)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("%s: %s: instructions_file: %w", configPath, where, err)
			}
			g.Instructions = strings.TrimRight(string(data), "\n")
		}
	}

	return nil
}
