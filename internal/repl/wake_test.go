package repl

import (
	"testing"
)

// TestIsWakeTurn: only an empty prompt is an inbox-only turn. Anything with
// text — including whitespace-looking structure — is an ordinary turn.
func TestIsWakeTurn(t *testing.T) {
	for _, prompt := range []string{"", "   ", "\n\t \n"} {
		if !isWakeTurn(prompt) {
			t.Errorf("isWakeTurn(%q) = false, want true", prompt)
		}
	}
	for _, prompt := range []string{"fix the build", " :help", "0"} {
		if isWakeTurn(prompt) {
			t.Errorf("isWakeTurn(%q) = true, want false", prompt)
		}
	}
}
