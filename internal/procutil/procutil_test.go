package procutil

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestAliveSelf(t *testing.T) {
	if !Alive(os.Getpid()) {
		t.Error("Alive(self) = false, want true")
	}
	if Alive(-1) {
		t.Error("Alive(-1) = true, want false")
	}
	// A pid that almost certainly does not exist.
	if Alive(1 << 30) {
		t.Error("Alive(huge pid) = true, want false")
	}
}

func TestKillTree(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	shell, err := ResolveShell()
	if err != nil {
		t.Skipf("no shell: %v", err)
	}
	cmd := exec.Command(shell, "-c", "sleep 60")
	Detach(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Reap the child so a kill does not leave a zombie that Alive still sees
	// (Kill(pid, 0) succeeds on zombies). The registry does the same.
	go func() { _ = cmd.Wait() }()
	pid := cmd.Process.Pid
	if !Alive(pid) {
		t.Fatalf("Alive(%d) = false right after start", pid)
	}
	if err := KillTree(pid); err != nil {
		t.Fatalf("KillTree: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for Alive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if Alive(pid) {
		t.Errorf("pid %d still alive after KillTree", pid)
	}
}

func TestMediateMCPCommandPassthrough(t *testing.T) {
	cmd, args := MediateMCPCommand("npx", []string{"-y", "server"})
	if cmd != "npx" || len(args) != 2 {
		t.Errorf("MediateMCPCommand(npx) = (%q, %q)", cmd, args)
	}
}
