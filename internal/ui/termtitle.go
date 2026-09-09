package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Default icons for the three states TerminalTitle shows. The hourglass
// means a turn is running, the speech balloon means something is waiting for
// you to answer it — a Question, a path or MCP confirmation, the pause
// prompt — and the sleep symbol means nothing is running and the composer is
// open. All three are configurable under ui.terminal_title in config.yaml,
// and any of them can be set to the empty string for no icon.
const (
	DefaultTitleRunningIcon = "⏳"
	DefaultTitleAskingIcon  = "💬"
	DefaultTitleWaitingIcon = "💤"
)

// titleMaxRunes caps the session title inside the window title. A tab bar
// gives each tab a few characters, so a long title pushes every other tab
// off the screen rather than telling anybody more.
const titleMaxRunes = 48

// TerminalTitle sets the terminal's window title, which a tabbed terminal
// such as the one in VS Code also uses as the tab label. It shows an icon
// for what is happening — a turn is running, a prompt is waiting for you to
// answer it, or nothing is running — followed by the session's title, so a
// window that is not on screen still says whether the agent is still working
// or has stopped to ask something.
//
// Two processes write it. The front end owns it between turns and around a
// turn; the agent process borrows it while a prompt of its own is waiting,
// because the front end is blocked on the agent and cannot see the prompt.
// They never write at the same time: the front end is inside cmd.Run while
// the agent has it.
//
// A terminal takes its title from the escape sequence ESC ] 0 ; text BEL,
// which sets the window title and the icon name in one go and is understood
// by every terminal in common use. It is a control sequence rather than
// output: writing it while a turn is printing changes nothing on screen.
//
// Whether the terminal then shows that title is up to the terminal. VS Code
// in particular ignores it until "terminal.integrated.tabs.title" is set to
// "${sequence}", and tmux until "set -g set-titles on" is set.
//
// Nothing is written unless the output is a terminal, so a piped or
// redirected run stays byte-clean.
type TerminalTitle struct {
	mu      sync.Mutex
	out     io.Writer
	on      bool
	running string
	waiting string
	asking  string
	// last is the title written most recently. Repeating a title writes
	// nothing, which keeps the sequence out of the stream on the common
	// path where a state does not actually change.
	last string
}

// NewTerminalTitle returns a title setter writing to out, which the front
// end passes as os.Stdout. When out is a file it has to be a terminal, and
// not a dumb one, or the setter is inert — that is what keeps the escape
// sequence out of a redirected stream. Setting KVIT_CODER_NO_TITLE in the
// environment makes it inert too. Anything that is not a file (a buffer in
// a test) is written to as asked.
func NewTerminalTitle(out io.Writer) *TerminalTitle {
	return &TerminalTitle{
		out:     out,
		on:      terminalTitleSupported(out),
		running: DefaultTitleRunningIcon,
		waiting: DefaultTitleWaitingIcon,
		asking:  DefaultTitleAskingIcon,
	}
}

func terminalTitleSupported(out io.Writer) bool {
	if out == nil {
		return false
	}
	if os.Getenv("KVIT_CODER_NO_TITLE") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	f, ok := out.(*os.File)
	if !ok {
		return true
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// SetIcons replaces the three state icons. An empty string means that state
// gets no icon, leaving the session title on its own.
func (t *TerminalTitle) SetIcons(running, waiting, asking string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running, t.waiting, t.asking = running, waiting, asking
}

// Disable stops the title being touched at all, for ui.terminal_title.enabled
// set to false.
func (t *TerminalTitle) Disable() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.on = false
}

// Running shows that a turn is under way in the named session.
func (t *TerminalTitle) Running(session string) { t.set(t.running, session) }

// Waiting shows that the turn has finished and the composer is open.
func (t *TerminalTitle) Waiting(session string) { t.set(t.waiting, session) }

// Asking shows that the turn has stopped to put something to you and is
// waiting for the answer.
func (t *TerminalTitle) Asking(session string) { t.set(t.asking, session) }

// Clear empties the title on the way out. There is no way to ask a terminal
// what its title was before, so the empty title is what a program that has
// finished leaves behind: shells that set a title from their prompt put
// their own back at the next prompt.
func (t *TerminalTitle) Clear() { t.set("", "") }

func (t *TerminalTitle) set(icon, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.on {
		return
	}
	text := strings.TrimSpace(icon + " " + sanitizeTitle(name))
	if text == t.last {
		return
	}
	t.last = text
	fmt.Fprintf(t.out, "\033]0;%s\007", text)
}

// sanitizeTitle makes text fit to go inside the escape sequence. A session
// title is a model's summary of the first prompt, so it is not trusted
// input: a control character in it would end the sequence early and leave
// the rest of the text printed across the screen. Anything below a space,
// plus DEL, is dropped, runs of whitespace collapse to one space, and the
// result is cut to titleMaxRunes.
func sanitizeTitle(text string) string {
	var b strings.Builder
	space := false
	for _, r := range text {
		switch {
		case r < ' ' || r == 0x7f:
			space = b.Len() > 0
		case r == ' ' || r == '\t':
			space = b.Len() > 0
		default:
			if space {
				b.WriteByte(' ')
				space = false
			}
			b.WriteRune(r)
		}
	}
	out := b.String()
	runes := []rune(out)
	if len(runes) > titleMaxRunes {
		out = strings.TrimRight(string(runes[:titleMaxRunes-1]), " ") + "…"
	}
	return out
}
