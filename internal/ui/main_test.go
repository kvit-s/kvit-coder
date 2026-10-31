package ui

import (
	"os"
	"testing"
)

// TestMain turns off the Windows paste handling, which is on by default on
// Windows, so the suite behaves the same on every platform. The tests that
// drive it turn it on themselves (withBurstKeys).
func TestMain(m *testing.M) {
	pasteBurstKeys = false
	os.Exit(m.Run())
}
