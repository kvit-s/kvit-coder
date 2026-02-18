package benchmark

import (
	"bytes"
	"testing"
)

func TestANSIStripWriter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple color code",
			input:    "\x1b[32mgreen text\x1b[0m",
			expected: "green text",
		},
		{
			name:     "multiple color codes",
			input:    "\x1b[37;2m✨ \x1b[0m\x1b[37;2m✨ .\x1b[0m\x1b[37;2m  (2.9k/128k) Search\x1b[0;22m",
			expected: "✨ ✨ .  (2.9k/128k) Search",
		},
		{
			name:     "progress bar with colors",
			input:    "[S2 run 1/1] \x1b[32m✓ PASS\x1b[0m | 4s | turns: 4 | tokens: 14070",
			expected: "[S2 run 1/1] ✓ PASS | 4s | turns: 4 | tokens: 14070",
		},
		{
			name:     "no ANSI codes",
			input:    "plain text without colors",
			expected: "plain text without colors",
		},
		{
			name:     "complex ANSI sequence",
			input:    "\x1b[1m[%d/%d %.1f%%]\x1b[0m",
			expected: "[%d/%d %.1f%%]",
		},
		{
			name:     "red and green colors",
			input:    "Result: \x1b[31mFAIL\x1b[0m or \x1b[32mPASS\x1b[0m",
			expected: "Result: FAIL or PASS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewANSIStripWriter(&buf)

			n, err := writer.Write([]byte(tt.input))
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			if n != len(tt.input) {
				t.Errorf("Write returned %d, expected %d", n, len(tt.input))
			}

			got := buf.String()
			if got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}
