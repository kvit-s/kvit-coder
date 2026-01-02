package safety

// Rule is the interface for all safety rules
type Rule interface {
	// Name returns the rule identifier (for logging/audit)
	Name() string

	// Applies returns true if this rule should check the given command
	Applies(parsed *ParsedCommand) bool

	// Check evaluates the command against this rule
	Check(parsed *ParsedCommand, ctx *Context) *RuleResult
}

// Registry holds all registered rules
type Registry struct {
	rules []Rule
}

// NewRegistry creates a new empty rule registry
func NewRegistry() *Registry {
	return &Registry{
		rules: make([]Rule, 0),
	}
}

// Register adds a rule to the registry
func (r *Registry) Register(rule Rule) {
	r.rules = append(r.rules, rule)
}

// CheckAll runs all applicable rules, returning on first blocking result
// Rules are checked in registration order; first Block/Warn/Prompt wins
func (r *Registry) CheckAll(parsed *ParsedCommand, ctx *Context) *RuleResult {
	var warnResults []*RuleResult

	for _, rule := range r.rules {
		if !rule.Applies(parsed) {
			continue
		}

		result := rule.Check(parsed, ctx)
		if result == nil {
			continue
		}

		switch result.Action {
		case Block:
			// Immediate return on block
			return result
		case Prompt:
			// Return prompt immediately (caller will handle)
			return result
		case Warn:
			// Collect warnings, continue checking
			warnResults = append(warnResults, result)
		}
	}

	// If we have warnings, return the first one
	if len(warnResults) > 0 {
		return warnResults[0]
	}

	return AllowResult()
}

// Rules returns all registered rules (for testing/debugging)
func (r *Registry) Rules() []Rule {
	return r.rules
}
