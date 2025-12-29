// Package prompt provides system prompt generation for the agent.
package prompt

import (
	"strings"
	"text/template"
)

// templateFuncs returns the custom template functions available in prompt templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// String manipulation
		"join":    strings.Join,
		"split":   strings.Split,
		"trim":    strings.TrimSpace,
		"lower":   strings.ToLower,
		"upper":   strings.ToUpper,
		"replace": strings.ReplaceAll,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"contains":  strings.Contains,

		// Conditionals
		"ternary": func(t, f any, cond bool) any {
			if cond {
				return t
			}
			return f
		},
		"default": func(def, val any) any {
			// Return val if it's non-empty, otherwise def
			switch v := val.(type) {
			case string:
				if v != "" {
					return v
				}
			case []string:
				if len(v) > 0 {
					return v
				}
			case int:
				if v != 0 {
					return v
				}
			case bool:
				// Always return the actual value for bools
				return v
			case nil:
				return def
			default:
				return v
			}
			return def
		},

		// Formatting
		"indent": func(spaces int, s string) string {
			prefix := strings.Repeat(" ", spaces)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if line != "" {
					lines[i] = prefix + line
				}
			}
			return strings.Join(lines, "\n")
		},
		"backtick": func(s string) string {
			return "`" + s + "`"
		},
		"codeblock": func(lang, code string) string {
			return "```" + lang + "\n" + code + "\n```"
		},

		// Collections
		"first": func(slice []string) string {
			if len(slice) > 0 {
				return slice[0]
			}
			return ""
		},
		"last": func(slice []string) string {
			if len(slice) > 0 {
				return slice[len(slice)-1]
			}
			return ""
		},
		"len": func(v any) int {
			switch val := v.(type) {
			case string:
				return len(val)
			case []string:
				return len(val)
			case []any:
				return len(val)
			default:
				return 0
			}
		},

		// Numeric helpers
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},

		// Iteration helpers
		"seq": func(n int) []int {
			result := make([]int, n)
			for i := range result {
				result[i] = i + 1
			}
			return result
		},
	}
}
