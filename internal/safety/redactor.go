package safety

import (
	"regexp"
)

// Redactor removes sensitive data from strings
type Redactor struct {
	patterns []*regexp.Regexp
}

// Default patterns to redact
var defaultPatterns = []string{
	// OpenAI API keys
	`sk-[a-zA-Z0-9]{20,}`,

	// Anthropic API keys
	`sk-ant-[a-zA-Z0-9\-]{20,}`,

	// GitHub tokens
	`ghp_[a-zA-Z0-9]{36}`,  // Personal access token
	`gho_[a-zA-Z0-9]{36}`,  // OAuth token
	`ghr_[a-zA-Z0-9]{36}`,  // Refresh token
	`ghs_[a-zA-Z0-9]{36}`,  // Server-to-server token
	`github_pat_[a-zA-Z0-9_]{22,}`, // Fine-grained PAT

	// AWS
	`AKIA[A-Z0-9]{16}`, // Access key ID

	// Generic API key patterns
	`(?i)(api[_-]?key|apikey|secret[_-]?key|secret|password|passwd|token|bearer|auth[_-]?token)\s*[:=]\s*['"]?[a-zA-Z0-9_\-\.]+['"]?`,

	// Authorization headers
	`(?i)Authorization:\s*Bearer\s+[a-zA-Z0-9_\-\.]+`,

	// Private keys (just the header)
	`-----BEGIN [A-Z ]+ PRIVATE KEY-----`,

	// URLs with credentials
	`https?://[^:@\s]+:[^@\s]+@`,

	// Base64-like long strings that might be secrets (32+ chars of base64)
	`(?i)(key|secret|token|password)\s*[:=]\s*['"]?[a-zA-Z0-9+/]{32,}={0,2}['"]?`,
}

// NewRedactor creates a new secret redactor with default patterns
func NewRedactor() *Redactor {
	patterns := make([]*regexp.Regexp, 0, len(defaultPatterns))
	for _, p := range defaultPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			// Skip invalid patterns
			continue
		}
		patterns = append(patterns, re)
	}
	return &Redactor{patterns: patterns}
}

// Redact replaces sensitive data in the input string with [REDACTED]
func (r *Redactor) Redact(input string) string {
	result := input
	for _, pattern := range r.patterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// AddPattern adds a custom pattern to redact
func (r *Redactor) AddPattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	r.patterns = append(r.patterns, re)
	return nil
}
