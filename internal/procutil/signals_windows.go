//go:build windows

package procutil

import "os"

// InterruptSignals is what cancels a turn. Windows has no SIGTERM/SIGKILL
// for arbitrary children (os.Interrupt and os.Kill exist; SIGTERM does not),
// so only os.Interrupt is subscribed. Graceful-then-force shutdown sequences
// elsewhere collapse to an immediate kill on Windows. The double-ctrl-c exit
// code convention (130) is kept so the UI keeps recognizing [cancelled].
func InterruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
