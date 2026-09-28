//go:build !windows

package procutil

import (
	"os"
	"syscall"
)

// InterruptSignals is what cancels a turn: SIGINT always, plus SIGTERM where
// the OS has one. The agent's double-ctrl-c path (first cancels the turn,
// second exits 130) is shared; only the set differs per OS.
func InterruptSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}
