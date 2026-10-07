package modelsetup

import (
	"os"
	"os/exec"
	"runtime"
)

// CanOpenBrowser reports whether a browser opened here would appear in front
// of the person: not over SSH, and on Linux only with a display to draw on.
func CanOpenBrowser() bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return false
	}
	switch runtime.GOOS {
	case "darwin", "windows":
		return true
	case "linux", "freebsd", "openbsd", "netbsd":
		return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	}
	return false
}

// OpenBrowser opens url in the default browser and does not wait for it.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() //nolint:errcheck // nobody is waiting for the browser
	return nil
}
