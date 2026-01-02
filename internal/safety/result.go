package safety

// Action represents the outcome of a rule check
type Action int

const (
	// Allow permits the command to execute
	Allow Action = iota
	// Block prevents the command from executing
	Block
	// Warn allows execution but logs a warning
	Warn
	// Prompt requires user confirmation before executing
	Prompt
)

func (a Action) String() string {
	switch a {
	case Allow:
		return "allow"
	case Block:
		return "block"
	case Warn:
		return "warn"
	case Prompt:
		return "prompt"
	default:
		return "unknown"
	}
}

// RuleResult encapsulates the outcome of applying safety rules
type RuleResult struct {
	Action  Action   // The action to take
	Rule    string   // Name of rule that triggered (for logging)
	Message string   // Human-readable explanation
	Paths   []string // Affected paths (for path-based prompting)
}

// AllowResult returns a result that permits execution
func AllowResult() *RuleResult {
	return &RuleResult{Action: Allow}
}

// BlockResult returns a result that prevents execution
func BlockResult(rule, message string) *RuleResult {
	return &RuleResult{
		Action:  Block,
		Rule:    rule,
		Message: message,
	}
}

// WarnResult returns a result that permits execution with a warning
func WarnResult(rule, message string) *RuleResult {
	return &RuleResult{
		Action:  Warn,
		Rule:    rule,
		Message: message,
	}
}

// PromptResult returns a result that requires user confirmation
func PromptResult(rule, message string, paths []string) *RuleResult {
	return &RuleResult{
		Action:  Prompt,
		Rule:    rule,
		Message: message,
		Paths:   paths,
	}
}

// IsBlocking returns true if the result prevents command execution
func (r *RuleResult) IsBlocking() bool {
	return r.Action == Block
}

// RequiresInteraction returns true if user interaction is needed
func (r *RuleResult) RequiresInteraction() bool {
	return r.Action == Prompt
}
