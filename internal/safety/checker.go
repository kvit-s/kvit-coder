package safety

import (
	"fmt"
)

// Checker is the main orchestrator for safety checks
type Checker struct {
	config   *SafetyConfig
	registry *Registry
	auditor  *AuditLogger
}

// NewChecker creates a new safety checker with the given configuration
func NewChecker(cfg *SafetyConfig) *Checker {
	if cfg == nil {
		cfg = &SafetyConfig{}
	}

	reg := NewRegistry()

	// Register all rules
	reg.Register(&GitRule{})
	reg.Register(&RmRule{})
	reg.Register(&FindRule{})
	reg.Register(&XargsRule{})

	var auditor *AuditLogger
	if cfg.Audit.Enabled {
		auditor = NewAuditLogger(cfg.Audit)
	}

	return &Checker{
		config:   cfg,
		registry: reg,
		auditor:  auditor,
	}
}

// Check validates a command for safety
func (c *Checker) Check(cmd string, ctx *Context) (*RuleResult, error) {
	// Step 1: Parse the command
	parsed, err := ParseCommand(cmd)
	if err != nil {
		if c.config.StrictMode {
			result := BlockResult("strict_parse_error",
				fmt.Sprintf("strict mode: failed to parse command: %v", err))
			c.audit(cmd, result, ctx)
			return result, nil
		}
		// Non-strict: allow unparseable commands (legacy behavior will catch them)
		return AllowResult(), nil
	}

	// Step 2: Unwrap shell wrappers (bash -c, sh -c, etc.)
	unwrapped, err := UnwrapAll(parsed, 5) // max depth 5
	if err != nil {
		if c.config.StrictMode {
			result := BlockResult("strict_unwrap_error",
				fmt.Sprintf("strict mode: failed to unwrap command: %v", err))
			c.audit(cmd, result, ctx)
			return result, nil
		}
		// Non-strict: continue with the parsed command as-is
		unwrapped = []*ParsedCommand{parsed}
	}

	// Step 3: Check each unwrapped command against rules
	for _, innerCmd := range unwrapped {
		result := c.registry.CheckAll(innerCmd, ctx)
		if result.Action != Allow {
			c.audit(cmd, result, ctx)
			return result, nil
		}
	}

	return AllowResult(), nil
}

// audit logs blocked commands
func (c *Checker) audit(cmd string, result *RuleResult, ctx *Context) {
	if c.auditor != nil && result.Action == Block {
		_ = c.auditor.LogBlocked(cmd, result, ctx.WorkingDir)
	}
}

// Close cleans up resources (closes audit log)
func (c *Checker) Close() error {
	if c.auditor != nil {
		return c.auditor.Close()
	}
	return nil
}

// IsEnabled returns true if any safety features are enabled
func (c *Checker) IsEnabled() bool {
	if c.config == nil {
		return false
	}

	// Check if any feature is enabled
	return c.config.StrictMode ||
		c.config.ParanoidMode ||
		c.config.Audit.Enabled ||
		c.config.Git.BlockPush ||
		c.config.Git.BlockHardReset ||
		c.config.Git.BlockCheckoutDiscard ||
		c.config.Git.BlockStashDrop ||
		c.config.Git.BlockCleanForce ||
		c.config.Git.WarnBranchForceDelete ||
		c.config.Rm.AllowInTemp ||
		c.config.Rm.AllowInWorkspaceCwd ||
		c.config.Rm.BlockWorkspaceRoot
}

// Config returns the safety configuration
func (c *Checker) Config() *SafetyConfig {
	return c.config
}
