package permissions

import "strings"

// FromConfig turns tools.shell.allowed_commands and disallowed_commands into
// rules. An entry is a pattern: one word means any invocation of that program,
// and more words mean the command must start with them. That is close enough to
// the old prefix-on-the-whole-string behaviour for existing configs, and it now
// applies to every command in a line rather than only the first.
func FromConfig(allowed, disallowed []string) []Rule {
	var rules []Rule
	for _, entry := range disallowed {
		if pattern := normalizeEntry(entry); pattern != "" {
			rules = append(rules, Rule{Pattern: pattern, Effect: EffectDeny, Source: "config",
				Reason: "it is in tools.shell.disallowed_commands"})
		}
	}
	for _, entry := range allowed {
		if pattern := normalizeEntry(entry); pattern != "" {
			rules = append(rules, Rule{Pattern: pattern, Effect: EffectAllow, Source: "config",
				Reason: "it is in tools.shell.allowed_commands"})
		}
	}
	return rules
}

func normalizeEntry(entry string) string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return ""
	}
	// The entry already covers any arguments when its last word is "*". A "*"
	// inside the last word does not: "mkfs.*" names a family of programs and
	// still needs the arguments added, or it would match only an invocation
	// with none.
	fields := strings.Fields(entry)
	if fields[len(fields)-1] == "*" {
		return entry
	}
	return entry + " *"
}
