package tui

import "testing"

// TestCompactArgs: :compact runs the agent with the arguments a turn would
// get, --compact in place of the prompt, and the focus as its own flag.
func TestCompactArgs(t *testing.T) {
	u := newTestUI(t, "compact", nil)
	args := u.compactArgs("the parser work")
	if args[0] != "--compact" || hasArg(args, "--wake") || hasArg(args, "-p") {
		t.Errorf("compact args = %v, want --compact in place of the prompt", args)
	}
	if !hasArg(args, "-s") || !hasArg(args, "compact") {
		t.Errorf("compact args = %v, want the session", args)
	}
	if n := len(args); args[n-2] != "--focus" || args[n-1] != "the parser work" {
		t.Errorf("compact args = %v, want the focus last", args)
	}
	if hasArg(u.compactArgs("  "), "--focus") {
		t.Errorf("an empty focus should not be passed")
	}
}
