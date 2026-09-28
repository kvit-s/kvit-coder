package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/update"
)

// updateAutoEnabled reports whether the background release check may run:
// the config gate and the environment opt-out both apply.
func (u *UI) updateAutoEnabled() bool {
	if update.DisabledByEnv() {
		return false
	}
	if u.cfg == nil {
		return true
	}
	return u.cfg.Updates.AutoCheckEnabled()
}

// updateInterval is the minimum time between automatic checks.
func (u *UI) updateInterval() time.Duration {
	if u.cfg == nil {
		return update.DefaultInterval
	}
	return u.cfg.Updates.Interval()
}

// updateConfigLine is the :config row describing the check.
func (u *UI) updateConfigLine() string {
	if !u.updateAutoEnabled() {
		return "Updates: auto-check off"
	}
	interval := u.updateInterval()
	if interval >= 24*time.Hour && interval%(24*time.Hour) == 0 {
		days := interval / (24 * time.Hour)
		if days == 1 {
			return "Updates: auto-check daily"
		}
		return fmt.Sprintf("Updates: auto-check every %d days", days)
	}
	return fmt.Sprintf("Updates: auto-check every %s", interval.Round(time.Hour))
}

// cachedUpdateNotice replays the last known release when it is newer than
// this build, so a fresh start says so without waiting on the network.
func (u *UI) cachedUpdateNotice() string {
	if !u.updateAutoEnabled() || update.IsDevVersion(u.version) {
		return ""
	}
	st := update.LoadState()
	if st == nil || st.LatestTag == "" {
		return ""
	}
	if !update.NewerThan(u.bannerVersion(), st.LatestTag) {
		return ""
	}
	return updateNoticeLine(u.bannerVersion(), st.LatestTag)
}

func updateNoticeLine(current, latest string) string {
	return fmt.Sprintf("Update available: %s → %s — :update to install (%s)",
		current, latest, update.ReleaseURL(latest))
}

// maybeStartUpdateCheck fires the once-per-interval background query. It
// never blocks startup and never prints: the result surfaces at the top of
// the loop through pollUpdateCheck.
func (u *UI) maybeStartUpdateCheck() {
	if !u.updateAutoEnabled() || update.IsDevVersion(u.version) {
		return
	}
	if !update.ShouldAutoCheck(time.Now(), u.version, update.LoadState(), true, u.updateInterval()) {
		return
	}
	ch := make(chan update.Result, 1)
	u.updateCh = ch
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		res, err := update.Check(ctx, u.version)
		if err != nil {
			// Still note the attempt, so an offline machine tries once per
			// interval rather than on every start.
			update.NoteCheck("")
			return
		}
		update.NoteCheck(res.Latest)
		ch <- res
	}()
}

// pollUpdateCheck prints the background result, once, when it names a newer
// release. It runs at the top of the loop, never inside the composer.
func (u *UI) pollUpdateCheck() {
	if u.updateCh == nil || u.updateNoted {
		return
	}
	select {
	case res := <-u.updateCh:
		u.updateCh = nil
		if res.Available {
			fmt.Println(updateNoticeLine(res.Current, res.Latest))
			fmt.Println()
			u.updateNoted = true
		}
	default:
	}
}

// finishStagedUpdates reports binaries a previous Windows :update staged but
// could not move, and cleans nothing else. The .old backups from a
// successful swap are removed by the next start in main.
func (u *UI) finishStagedUpdates() {
	uiDest, coderDest, err := update.Destinations("", u.agentPath)
	if err != nil {
		return
	}
	seen := map[string]bool{}
	var dirs []string
	for _, d := range []string{dirOf(uiDest), dirOf(coderDest)} {
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	if staged := update.PendingStaged(dirs...); len(staged) > 0 {
		fmt.Printf("A staged update is waiting (%s). Quit and move it into place, or run :update again.\n\n",
			strings.Join(staged, ", "))
	}
}

func dirOf(dest string) string {
	if i := strings.LastIndexAny(dest, `/\`); i >= 0 {
		return dest[:i]
	}
	return "."
}

// showVersion prints this build, the agent it spawns, and the newest known
// release.
func (u *UI) showVersion() {
	fmt.Printf("kvit-coder-ui %s", u.bannerVersion())
	if u.commitHash != "" && u.commitHash != "dev" {
		fmt.Printf(" (commit %s", u.commitHash)
		if u.commitDate != "" && u.commitDate != "unknown" {
			fmt.Printf(" of %s", u.commitDate)
		}
		fmt.Printf(")")
	}
	fmt.Println()
	if agent, agentVer := u.agentVersion(); agent != "" {
		if agentVer != "" {
			fmt.Printf("agent: %s (%s)\n", agent, agentVer)
		} else {
			fmt.Printf("agent: %s\n", agent)
		}
	}
	switch st := update.LoadState(); {
	case st != nil && st.LatestTag != "" && update.NewerThan(u.bannerVersion(), st.LatestTag):
		fmt.Printf("latest release: %s — update available, :update to install\n", st.LatestTag)
	case st != nil && st.LatestTag != "":
		fmt.Printf("latest release: %s (up to date)\n", st.LatestTag)
	default:
		fmt.Println("latest release: unknown (offline or not checked yet — :update --check)")
	}
	fmt.Println()
}

// agentVersion runs the agent's --version once, briefly, so :version shows
// whether the pair matches after an update.
func (u *UI) agentVersion() (string, string) {
	if u.agentPath == "" {
		return "", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, u.agentPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return u.agentPath, ""
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	return u.agentPath, line
}

// handleUpdate runs :update [--check] [<tag>] [--apply].
func (u *UI) handleUpdate(args []string) {
	var checkOnly, apply bool
	var tag string
	for _, a := range args {
		switch {
		case a == "--check" || a == "check":
			checkOnly = true
		case a == "--apply" || a == "--yes" || a == "-y":
			apply = true
		case a == "--help" || a == "-h":
			fmt.Println("Usage: :update [--check] [<tag>] [--apply]")
			fmt.Println("  no args    check, confirm, and install the latest release")
			fmt.Println("  --check    only report the latest release")
			fmt.Println("  <tag>      install a specific release (e.g. :update v0.5.0)")
			fmt.Println("  --apply    install without asking")
			fmt.Println()
			return
		case strings.HasPrefix(a, "-"):
			fmt.Printf("Unknown flag: %s (see :update --help)\n\n", a)
			return
		default:
			if tag == "" {
				tag = update.NormalizeTag(a)
			}
		}
	}

	if tag == "" {
		res, err := u.forceCheck()
		if err != nil {
			fmt.Printf("Could not check for updates: %v\n\n", err)
			return
		}
		if checkOnly {
			u.printCheckResult(res)
			return
		}
		if !res.Available {
			fmt.Printf("Already up to date: %s is the latest release.\n\n", res.Latest)
			return
		}
		tag = res.Latest
	} else if checkOnly {
		fmt.Printf("Latest known release is not needed: installing %s.\n", tag)
	}

	current := u.bannerVersion()
	if tag == current {
		fmt.Printf("Already on %s.\n\n", current)
		return
	}

	uiDest, coderDest, err := update.Destinations("", u.agentPath)
	if err != nil {
		fmt.Printf("Cannot update: %v\n\n", err)
		return
	}
	fmt.Printf("Will install %s for %s/%s:\n", tag, runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  %s\n  %s\n", coderDest, uiDest)
	fmt.Printf("Release notes: %s\n", update.ReleaseURL(tag))
	if !update.IsDevVersion(current) && update.Compare(current, tag) > 0 {
		fmt.Printf("Note: %s is older than the running %s (a downgrade).\n", tag, current)
	}
	if !apply && !confirmStdin(fmt.Sprintf("Install %s?", tag)) {
		fmt.Println("Cancelled; nothing was changed.")
		fmt.Println()
		return
	}
	u.installUpdate(tag)
}

// forceCheck queries GitHub now, ignoring the interval cache (which it
// refreshes on success).
func (u *UI) forceCheck() (update.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := update.Check(ctx, u.version)
	if err != nil {
		return update.Result{}, err
	}
	update.NoteCheck(res.Latest)
	return res, nil
}

func (u *UI) printCheckResult(res update.Result) {
	if res.Available {
		fmt.Println(updateNoticeLine(res.Current, res.Latest))
	} else {
		fmt.Printf("Up to date: %s is the latest release.\n", res.Latest)
	}
	fmt.Println()
}

// installUpdate downloads, verifies and installs a release. The checksum
// failure aborts before anything is touched; the running UI keeps serving
// from memory either way and picks the new build up on restart.
func (u *UI) installUpdate(tag string) {
	tmp, err := os.MkdirTemp("", "kvit-update-*")
	if err != nil {
		fmt.Printf("Cannot update: %v\n\n", err)
		return
	}
	defer os.RemoveAll(tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fmt.Printf("Downloading %s for %s/%s...\n", tag, runtime.GOOS, runtime.GOARCH)
	archive, err := update.FetchRelease(ctx, tag, tmp)
	if err != nil {
		fmt.Printf("Update failed: %v\n\n", err)
		return
	}
	extractDir, err := os.MkdirTemp(tmp, "extract-*")
	if err != nil {
		fmt.Printf("Update failed: %v\n\n", err)
		return
	}
	coderSrc, uiSrc, err := update.ExtractBinaries(archive, extractDir)
	if err != nil {
		fmt.Printf("Update failed: %v\n\n", err)
		return
	}
	uiDest, coderDest, err := update.Destinations("", u.agentPath)
	if err != nil {
		fmt.Printf("Update failed: %v\n\n", err)
		return
	}
	if err := update.Install(coderSrc, uiSrc, coderDest, uiDest); err != nil {
		if pending, ok := err.(*update.Pending); ok {
			u.printPending(pending)
			return
		}
		fmt.Printf("Update failed: %v\n\n", err)
		return
	}
	update.NoteCheck(tag)
	fmt.Printf("Updated to %s.\n", tag)
	if runtime.GOOS == "windows" {
		fmt.Println("Previous binaries are kept as .old and removed on the next start.")
	}
	fmt.Println("Quit (:quit) and start the UI again to run it; the next turn already uses the new agent.")
	fmt.Println()
}

// printPending explains the Windows fallback: the files are verified and
// staged, but the running binaries would not release their lock.
func (u *UI) printPending(pending *update.Pending) {
	fmt.Println("The new files are downloaded and verified, but Windows would not")
	fmt.Println("release the running binaries. Quit the UI, then run:")
	for _, pr := range pending.Pairs {
		fmt.Printf("  move /Y %q %q\n", pr[0], pr[1])
	}
	fmt.Println("then start the UI again.")
	fmt.Println()
}

// confirmStdin asks a [y/N] question on stdout. Anything but y/yes,
// including EOF, declines.
func confirmStdin(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(line) == 0 {
		fmt.Println()
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
