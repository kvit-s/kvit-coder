// Package procs runs commands that outlive the turn's iterations.
//
// A turn-scoped command — the ordinary Shell tool — dies with its iteration.
// A process started here is detached into its own session, writes to a log
// file in the session directory, and records its exit status where a later
// iteration (or, if persistent, a later turn) can read it. Ephemeral
// processes die with the turn; persistent ones survive it, and are the only
// thing that does.
//
// Everything the registry knows lives in <session>/proc/, so a new process
// opening the same directory sees the same set:
//
//	<id>.json   what it is: command, working directory, pid, when it started
//	<id>.log    everything it has written
//	<id>.exit   the exit status, written by the wrapper when the command ends
package procs

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// State is where a process is in its life.
type State string

const (
	// StateRunning means the process is alive as far as the kernel is concerned.
	StateRunning State = "running"
	// StateExited means it finished on its own; ExitCode says how.
	StateExited State = "exited"
	// StateKilled means kvit-coder killed it.
	StateKilled State = "killed"
	// StateGone means the process is not alive and left no exit status. It was
	// killed by something outside kvit-coder, or the machine restarted.
	StateGone State = "gone"
)

// Info is everything the registry records about one process. It is the whole
// content of <id>.json, so a later turn reads the same thing this one wrote.
type Info struct {
	ID      string    `json:"id"`
	Name    string    `json:"name,omitempty"`
	Command string    `json:"command"`
	Cwd     string    `json:"cwd"`
	PID     int       `json:"pid"`
	Started time.Time `json:"started"`

	// Every, when non-zero, makes this a periodic probe: the command is run
	// again every Every seconds until it is killed.
	Every int `json:"every,omitempty"`
	// Report is when this process is worth interrupting the model about
	// while it runs: "exit" (the default, nothing until it ends), "changed"
	// (when it writes new output), or "always" (same as changed for now).
	// The ending itself is always reported once, whatever this says.
	Report string `json:"report,omitempty"`
	// Until, when set, is a regex the model wants to be told about: when
	// new output matches, an event fires even if Report is "exit". It is
	// set on Shell.start and adjustable later with Shell.tune.
	Until string `json:"until,omitempty"`
	// RemindEvery, when positive, is seconds between "still running"
	// reminders while the process runs. Each reminder carries new output
	// since the last one, or says there was none. Zero or less means off.
	RemindEvery int `json:"remind_every,omitempty"`
	// LastEventAt is when the model was last told about this running
	// process (match, output, or tick). It throttles RemindEvery and
	// restarts whenever the reminder policy is tuned. Exit reporting uses
	// ExitReported instead.
	LastEventAt time.Time `json:"last_event_at,omitempty"`
	// Persistent marks a process that survives the turn that started it.
	// The default (false) is ephemeral: it is stopped when the turn ends,
	// so an abandoned session leaves nothing behind. Persistent (true) is
	// the opt-in exception — a dev server the next turn still needs — and
	// is shown red instead of the usual blue. It still dies on interrupt
	// (when kill_on_exit holds), on Shell.kill, and on reboot (which
	// reports it as gone); for something that must survive those, write a
	// script and ask the user to run it. Old records without the field
	// decode as ephemeral.
	Persistent bool `json:"persistent,omitempty"`

	State    State     `json:"state"`
	ExitCode int       `json:"exit_code"`
	Ended    time.Time `json:"ended,omitempty"`

	// ReportedOffset is how far into the log the model has already been told
	// about, so the same output is not reported twice across turns.
	ReportedOffset int64 `json:"reported_offset,omitempty"`
	// ExitReported records that the model has been told this one ended.
	ExitReported bool `json:"exit_reported,omitempty"`
}

// Running reports whether this process is still going.
func (i Info) Running() bool { return i.State == StateRunning }

// Registry owns the processes recorded in one session's proc directory.
type Registry struct {
	dir string

	mu sync.Mutex
	// seq makes ids short and readable within a session rather than random.
	seq int
}

// New opens the registry over a session's proc directory.
func New(dir string) (*Registry, error) {
	if dir == "" {
		return nil, fmt.Errorf("procs: directory cannot be empty")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("procs: failed to create %s: %w", dir, err)
	}
	return &Registry{dir: dir}, nil
}

// Dir is the directory the registry keeps its records in.
func (r *Registry) Dir() string { return r.dir }

// maxChunk bounds one Output call. A process that has written a hundred
// megabytes must not put all of it in a message.
const maxChunk = 32 * 1024

// Start runs a command detached from this turn's iterations and returns the
// id to refer to it by. The command runs under "sh -c" in its own session,
// so it survives the iteration and takes its children with it when it is
// killed. It is ephemeral by default: stopped when the turn ends, so an
// abandoned session leaves nothing behind. Pass persistent to StartWithOptions
// for the opt-in exception that survives like the old default.
func (r *Registry) Start(command, cwd, name string) (string, error) {
	return r.start(command, cwd, name, 0, "exit", "", 0, false)
}

// StartWithOptions is Start with the reminder policy and lifetime set up
// front, so there is no round trip to another tool just to say when to be
// told: report picks output reminders ("exit" for nothing until it ends),
// until adds a pattern that reports on match, and remindEvery adds "still
// running" ticks every that many seconds. Zero remindEvery means off.
// persistent false (the default) stops the process when the turn ends;
// persistent true lets it survive into the next turn, shown red, still
// killed on interrupt, by Shell.kill, or by reboot (reported as gone).
func (r *Registry) StartWithOptions(command, cwd, name, report, until string, remindEvery int, persistent bool) (string, error) {
	if err := ValidateReport(report); err != nil {
		return "", err
	}
	if err := ValidateUntil(until); err != nil {
		return "", err
	}
	if err := ValidateRemindEvery(remindEvery); err != nil {
		return "", err
	}
	if report == "" {
		report = "exit"
	}
	return r.start(command, cwd, name, 0, report, until, remindEvery, persistent)
}

// ValidateReport says whether a report policy is known. Empty means the
// default ("exit") and is valid for callers that fill it in later.
func ValidateReport(report string) error {
	switch report {
	case "", "exit", "changed", "always":
		return nil
	}
	return fmt.Errorf("procs: unknown report %q, want exit, changed, or always", report)
}

// ValidateUntil says whether a pattern can be watched for. Empty disables
// pattern reminders.
func ValidateUntil(until string) error {
	if until == "" {
		return nil
	}
	if _, err := regexp.Compile(until); err != nil {
		return fmt.Errorf("procs: 'until' is not a valid regular expression: %w", err)
	}
	return nil
}

// ValidateRemindEvery says whether a tick interval is usable. Zero means
// off; negatives are rejected.
func ValidateRemindEvery(remindEvery int) error {
	if remindEvery < 0 {
		return fmt.Errorf("procs: 'remind_every' is %d, want 0 (off) or more seconds", remindEvery)
	}
	return nil
}

// Configure adjusts the reminder policy and lifetime of a process that is
// already running: report, until, the tick interval, and persistent. It is
// how the model changes its mind after a reminder without restarting the
// command. Passing the same values back is a no-op except that a changed
// tick interval restarts its clock from now.
func (r *Registry) Configure(id, report, until string, remindEvery int, persistent bool) (Info, error) {
	if err := ValidateReport(report); err != nil {
		return Info{}, err
	}
	if err := ValidateUntil(until); err != nil {
		return Info{}, err
	}
	if err := ValidateRemindEvery(remindEvery); err != nil {
		return Info{}, err
	}
	if report == "" {
		report = "exit"
	}
	info, err := r.load(id)
	if err != nil {
		return Info{}, err
	}
	if info.RemindEvery != remindEvery {
		info.LastEventAt = time.Now().UTC()
	}
	info.Report = report
	info.Until = until
	info.RemindEvery = remindEvery
	info.Persistent = persistent
	if err := r.save(info); err != nil {
		return Info{}, err
	}
	return info, nil
}

// StartPeriodic runs a command every `every` seconds until it is killed. It is
// the same kind of record as any other process, so one set of tools covers
// both a dev server and a probe that keeps checking something. A periodic
// probe is persistent by design: "run from now on" means across turns, and
// it is shown red until killed.
func (r *Registry) StartPeriodic(command, cwd, name string, every int, report string) (string, error) {
	if every < 1 {
		every = 1
	}
	if report == "" {
		report = "changed"
	}
	return r.start(command, cwd, name, every, report, "", 0, true)
}

func (r *Registry) start(command, cwd, name string, every int, report, until string, remindEvery int, persistent bool) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("procs: command cannot be empty")
	}
	if cwd == "" {
		cwd = "."
	}

	id, err := r.nextID()
	if err != nil {
		return "", err
	}

	logPath := r.logPath(id)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("procs: failed to create log file: %w", err)
	}
	defer logFile.Close()

	// The exit status is written by the command's own shell, because by the
	// time a later turn asks, the process that started this one is gone and
	// there is no one left to wait() for it. An EXIT trap is what catches it:
	// it runs whether the command falls off the end or calls exit itself,
	// which a line appended after the command would not.
	body := command
	if every > 0 {
		body = fmt.Sprintf("while :; do\n%s\nprintf '\\n--- %%s ---\\n' \"$(date +%%H:%%M:%%S)\"\nsleep %d\ndone", command, every)
	}
	script := "trap 'printf \"%s\" \"$?\" > \"$KVIT_EXIT_FILE\"' EXIT\n" + body + "\n"

	cmd := exec.Command("sh", "-c", script)
	cmd.Dir = cwd
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "KVIT_EXIT_FILE="+r.exitPath(id))
	// Setsid detaches the process into its own session, so it neither dies
	// with the turn nor receives the terminal's ctrl-c, and killing it by
	// negative pid takes everything it started with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("procs: failed to start command: %w", err)
	}

	info := Info{
		ID:          id,
		Name:        name,
		Command:     command,
		Cwd:         cwd,
		PID:         cmd.Process.Pid,
		Started:     time.Now().UTC(),
		Every:       every,
		Report:      report,
		Until:       until,
		RemindEvery: remindEvery,
		Persistent:  persistent,
		State:       StateRunning,
	}
	info.LastEventAt = info.Started
	if err := r.save(info); err != nil {
		_ = killGroup(info.PID)
		return "", err
	}

	// Reap it while this process is still here, so a command that finishes
	// quickly does not sit as a zombie for the rest of the turn. The exit file
	// the wrapper writes is what a later turn reads, so nothing depends on
	// this goroutine outliving the turn.
	go func() {
		_ = cmd.Wait()
	}()

	return id, nil
}

// Output returns what the process has written since the given byte offset,
// along with the new offset and whether anything was returned. Reading by
// offset is what lets a caller ask repeatedly and see only what is new.
func (r *Registry) Output(id string, cursor int64) (string, int64, bool, error) {
	f, err := os.Open(r.logPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return "", cursor, false, fmt.Errorf("procs: no process %q", id)
		}
		return "", cursor, false, fmt.Errorf("procs: failed to read output: %w", err)
	}
	defer f.Close()

	size, err := f.Seek(0, 2)
	if err != nil {
		return "", cursor, false, fmt.Errorf("procs: failed to read output: %w", err)
	}
	if cursor < 0 || cursor > size {
		// The log was truncated under us, or the caller passed nonsense.
		cursor = 0
	}
	if size == cursor {
		return "", cursor, false, nil
	}

	start := cursor
	// Keep the tail when there is more new output than fits in one message:
	// the end is what says what is happening now.
	if size-start > maxChunk {
		start = size - maxChunk
	}
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil {
		return "", cursor, false, fmt.Errorf("procs: failed to read output: %w", err)
	}

	text := string(buf)
	if start > cursor {
		text = fmt.Sprintf("[%d earlier bytes skipped]\n%s", start-cursor, text)
	}
	return text, size, true, nil
}

// Status returns what the registry knows about one process, after checking
// whether it is still alive.
func (r *Registry) Status(id string) (Info, error) {
	info, err := r.load(id)
	if err != nil {
		return Info{}, err
	}
	updated, changed := r.refresh(info)
	if changed {
		_ = r.save(updated)
	}
	return updated, nil
}

// List returns every process this session has started, oldest first.
func (r *Registry) List() []Info {
	ids := r.ids()
	out := make([]Info, 0, len(ids))
	for _, id := range ids {
		if info, err := r.Status(id); err == nil {
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.Before(out[j].Started) })
	return out
}

// Kill stops a process and everything it started. Killing one that has already
// finished is not an error: the caller wanted it stopped, and it is stopped.
// When refresh discovers the process already exited or went away, that
// transition is saved, so Ended is stamped when it was found rather than
// drifting to the next Status.
func (r *Registry) Kill(id string) error {
	info, err := r.load(id)
	if err != nil {
		return err
	}

	current, changed := r.refresh(info)
	if !current.Running() {
		if changed {
			return r.save(current)
		}
		return nil
	}
	if err := killGroup(current.PID); err != nil {
		return fmt.Errorf("procs: failed to kill %s (pid %d): %w", id, current.PID, err)
	}
	current.State = StateKilled
	current.Ended = time.Now().UTC()
	return r.save(current)
}

// KillAll stops every running process and returns the ids it stopped. The
// interrupt handler calls it, so ctrl-c leaves nothing behind.
func (r *Registry) KillAll() []string {
	var killed []string
	for _, info := range r.List() {
		if !info.Running() {
			continue
		}
		if err := r.Kill(info.ID); err == nil {
			killed = append(killed, info.ID)
		}
	}
	return killed
}

// KillEphemeral stops every running process that is not persistent and
// returns the ids it stopped. The turn calls it when it ends, so the default
// (ephemeral) leaves nothing behind in an abandoned session; persistent
// processes survive for the next turn.
func (r *Registry) KillEphemeral() []string {
	var killed []string
	for _, info := range r.List() {
		if !info.Running() || info.Persistent {
			continue
		}
		if err := r.Kill(info.ID); err == nil {
			killed = append(killed, info.ID)
		}
	}
	return killed
}

// RunningCounts splits the running processes into ephemeral (blue, die at
// turn end) and persistent (red, survive). Finished processes count for
// neither — their ending is reported through the inbox instead.
func (r *Registry) RunningCounts() (ephemeral, persistent int) {
	for _, info := range r.List() {
		if !info.Running() {
			continue
		}
		if info.Persistent {
			persistent++
		} else {
			ephemeral++
		}
	}
	return ephemeral, persistent
}

// Reconcile brings the records up to date with what is actually running and
// returns the processes whose state changed since anyone last looked. A turn
// calls it at the start, which is how a process that ended between turns still
// gets reported.
func (r *Registry) Reconcile() []Info {
	var changed []Info
	for _, id := range r.ids() {
		info, err := r.load(id)
		if err != nil {
			continue
		}
		updated, didChange := r.refresh(info)
		if didChange {
			_ = r.save(updated)
			changed = append(changed, updated)
		}
	}
	return changed
}

// Events returns what the model has not been told yet: processes that have
// ended, and running processes whose policy asks for a reminder — a pattern
// match, new output, or a tick of the clock. Calling it marks what it
// returns as reported, so the same thing is not raised twice.
func (r *Registry) Events() []string {
	var events []string
	for _, info := range r.List() {
		text, updated, ok := r.eventFor(info)
		if !ok {
			continue
		}
		events = append(events, text)
		_ = r.save(updated)
	}
	return events
}

func (r *Registry) eventFor(info Info) (string, Info, bool) {
	label := info.ID
	if info.Name != "" {
		label = fmt.Sprintf("%s (%s)", info.ID, info.Name)
	}

	if !info.Running() && !info.ExitReported {
		info.ExitReported = true
		tail, offset, _, _ := r.Output(info.ID, info.ReportedOffset)
		info.ReportedOffset = offset
		text := fmt.Sprintf("Background process %s %s", label, endedHow(info))
		if strings.TrimSpace(tail) != "" {
			text += ".\nIts last output:\n" + tail
		} else {
			text += "."
		}

		return text, info, true
	}

	// A finished process reports its ending once and then goes quiet: no
	// pattern match, no output notice, no tick. Without this a dead process
	// with a tick interval keeps saying "still running" forever — which is
	// what bg1/bg2 did after going gone.
	if !info.Running() {
		return "", info, false
	}

	// A running process only interrupts when it was asked to, in priority
	// order: a pattern match first, then new output, then the clock. The
	// ending above always fires, so "exit" — the default — stays silent
	// until then. At most one event per process per poll; anything else
	// waits for the next one.
	now := time.Now().UTC()

	// A pattern fires on new output alone, so a match is reported once and
	// never re-read. An invalid pattern (a hand-edited record, say) matches
	// nothing rather than failing the poll.
	if info.Until != "" {
		if re, err := regexp.Compile(info.Until); err == nil {
			if tail, offset, ok, outErr := r.Output(info.ID, info.ReportedOffset); outErr == nil && ok {
				if re.MatchString(tail) {
					info.ReportedOffset = offset
					info.LastEventAt = now
					return fmt.Sprintf("Background process %s matched %q:\n%s", label, info.Until, tail), info, true
				}
			}
		}
	}

	switch info.Report {
	case "always", "changed":
		tail, offset, ok, err := r.Output(info.ID, info.ReportedOffset)
		if err == nil && ok && strings.TrimSpace(tail) != "" {
			info.ReportedOffset = offset
			info.LastEventAt = now
			return fmt.Sprintf("Background process %s produced new output:\n%s", label, tail), info, true
		}
	}

	// A tick fires even with no new output, so a quiet command still says
	// it is alive. It carries whatever has accumulated since the last
	// reminder and advances past it, so the ending does not repeat it.
	if info.RemindEvery > 0 {
		last := info.LastEventAt
		if last.IsZero() {
			last = info.Started
		}
		if now.Sub(last) >= time.Duration(info.RemindEvery)*time.Second {
			tail, offset, ok, _ := r.Output(info.ID, info.ReportedOffset)
			info.LastEventAt = now
			if ok {
				info.ReportedOffset = offset
				if strings.TrimSpace(tail) != "" {
					return fmt.Sprintf("Background process %s still running (reminder every %ds):\n%s", label, info.RemindEvery, tail), info, true
				}
			}
			return fmt.Sprintf("Background process %s still running (reminder every %ds, no new output).", label, info.RemindEvery), info, true
		}
	}

	return "", info, false
}

func endedHow(info Info) string {
	switch info.State {
	case StateKilled:
		return "was killed"
	case StateGone:
		return "is no longer running (no exit status recorded)"
	default:
		return fmt.Sprintf("exited with status %d", info.ExitCode)
	}
}

// refresh works out the current state of a process from the exit file the
// wrapper writes and from whether the pid is still alive.
func (r *Registry) refresh(info Info) (Info, bool) {
	if !info.Running() {
		return info, false
	}

	if data, err := os.ReadFile(r.exitPath(info.ID)); err == nil {
		code, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if convErr != nil {
			code = -1
		}
		info.State = StateExited
		info.ExitCode = code
		if info.Ended.IsZero() {
			info.Ended = time.Now().UTC()
		}
		return info, true
	}

	if info.PID > 0 && syscall.Kill(info.PID, 0) == nil {
		return info, false
	}

	// Not alive and no exit status: something outside kvit-coder ended it.
	info.State = StateGone
	info.ExitCode = -1
	if info.Ended.IsZero() {
		info.Ended = time.Now().UTC()
	}
	return info, true
}

func killGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	// Negative pid means the whole process group, which Setsid made this
	// process the leader of, so nothing it started is left behind.
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return err
	}
	// Give it a moment to go quietly before insisting.
	for range 20 {
		if syscall.Kill(-pid, 0) != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

// --- the files ---------------------------------------------------------------

func (r *Registry) infoPath(id string) string { return filepath.Join(r.dir, id+".json") }
func (r *Registry) logPath(id string) string  { return filepath.Join(r.dir, id+".log") }
func (r *Registry) exitPath(id string) string { return filepath.Join(r.dir, id+".exit") }

// LogPath is where a process's output is accumulating, for a caller that wants
// to tell someone where to look.
func (r *Registry) LogPath(id string) string { return r.logPath(id) }

func (r *Registry) save(info Info) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("procs: failed to marshal record: %w", err)
	}
	return os.WriteFile(r.infoPath(info.ID), append(data, '\n'), 0644)
}

func (r *Registry) load(id string) (Info, error) {
	data, err := os.ReadFile(r.infoPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return Info{}, fmt.Errorf("procs: no process %q", id)
		}
		return Info{}, fmt.Errorf("procs: failed to read record: %w", err)
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, fmt.Errorf("procs: record for %q is unreadable: %w", id, err)
	}
	return info, nil
}

func (r *Registry) ids() []string {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(ids)
	return ids
}

// nextID names a process by its position in the session, which is short enough
// to type and stable across turns.
func (r *Registry) nextID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.seq == 0 {
		for _, id := range r.ids() {
			if n, err := strconv.Atoi(strings.TrimPrefix(id, "bg")); err == nil && n > r.seq {
				r.seq = n
			}
		}
	}
	for range 1000 {
		r.seq++
		id := fmt.Sprintf("bg%d", r.seq)
		if _, err := os.Stat(r.infoPath(id)); os.IsNotExist(err) {
			return id, nil
		}
	}
	return "", fmt.Errorf("procs: could not find a free process id")
}
