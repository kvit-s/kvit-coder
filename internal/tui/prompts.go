package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// textSpec is a one-line question.
type textSpec struct {
	Title string
	Hint  string
	// Initial is the answer the field starts with, which Enter accepts as is.
	Initial string
	// Secret shows dots in place of what is typed, for API keys.
	Secret bool
	// Validate rejects an answer with a message shown under the field, which
	// stays open for another try.
	Validate func(string) error
}

// textModel is the bubbletea program behind a textSpec.
type textModel struct {
	spec      textSpec
	input     textinput.Model
	problem   string
	done      bool
	cancelled bool
}

func newTextModel(spec textSpec) *textModel {
	in := textinput.New()
	in.Prompt = "> "
	in.PromptStyle = pickCurrent
	in.SetValue(spec.Initial)
	in.CursorEnd()
	if spec.Secret {
		in.EchoMode = textinput.EchoPassword
		in.EchoCharacter = '•'
	}
	in.Focus()
	return &textModel{spec: spec, input: in}
}

func (m *textModel) Init() tea.Cmd { return textinput.Blink }

func (m *textModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled, m.done = true, true
			return m, tea.Quit
		case tea.KeyEnter:
			value := strings.TrimSpace(m.input.Value())
			if m.spec.Validate != nil {
				if err := m.spec.Validate(value); err != nil {
					m.problem = err.Error()
					return m, nil
				}
			}
			m.done = true
			return m, tea.Quit
		}
		m.problem = ""
	}
	if w, ok := msg.(tea.WindowSizeMsg); ok {
		m.input.Width = w.Width - 4
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *textModel) View() string {
	if m.done {
		return ""
	}
	var b strings.Builder
	b.WriteString(pickTitle.Render(m.spec.Title) + "\n")
	hint := m.spec.Hint
	if hint == "" {
		hint = "Enter to accept · Esc to go back"
	}
	b.WriteString(pickHint.Render(hint) + "\n")
	b.WriteString(m.input.View())
	if m.problem != "" {
		b.WriteString("\n" + pickError.Render(m.problem))
	}
	return b.String()
}

// waitModel shows a spinner and a line while work runs, and cancels the
// work's context on Esc or Ctrl+C.
type waitModel struct {
	message string
	spin    spinner.Model
	cancel  context.CancelFunc
	err     error
	done    bool
}

type workDone struct{ err error }

func newWaitModel(message string, cancel context.CancelFunc) *waitModel {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = pickCurrent
	return &waitModel{message: message, spin: s, cancel: cancel}
}

func (m *waitModel) Init() tea.Cmd { return m.spin.Tick }

func (m *waitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case workDone:
		m.err, m.done = msg.err, true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc || msg.Type == tea.KeyCtrlC {
			m.cancel()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.spin, cmd = m.spin.Update(msg)
	return m, cmd
}

func (m *waitModel) View() string {
	if m.done {
		return ""
	}
	return m.spin.View() + " " + pickText.Render(m.message) + pickHint.Render("  (Esc to stop)")
}

// asker puts questions to the person. The terminal one below draws them; the
// tests give the setup code a scripted one.
type asker interface {
	pick(spec pickSpec) (pickResult, bool)
	text(spec textSpec) (string, bool)
	// wait runs work, showing message until it returns, and returns its
	// error. The work's context is cancelled if the person stops it.
	wait(message string, work func(context.Context) error) error
	// say prints a line into the scrollback.
	say(format string, args ...any)
}

// termAsker draws on the terminal kcu runs in.
type termAsker struct{}

func (termAsker) pick(spec pickSpec) (pickResult, bool) {
	if len(spec.Items) == 0 {
		return pickResult{}, false
	}
	m := newPicker(spec)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m[error] %v\033[0m\n", err)
		return pickResult{}, false
	}
	if m.cancelled {
		return pickResult{}, false
	}
	return m.result, true
}

func (termAsker) text(spec textSpec) (string, bool) {
	m := newTextModel(spec)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m[error] %v\033[0m\n", err)
		return "", false
	}
	if m.cancelled {
		return "", false
	}
	return strings.TrimSpace(m.input.Value()), true
}

func (termAsker) wait(message string, work func(context.Context) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newWaitModel(message, cancel)
	p := tea.NewProgram(m)
	go func() { p.Send(workDone{err: work(ctx)}) }()
	if _, err := p.Run(); err != nil {
		cancel()
		return err
	}
	if m.err != nil && ctx.Err() != nil {
		return fmt.Errorf("stopped")
	}
	return m.err
}

func (termAsker) say(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}
