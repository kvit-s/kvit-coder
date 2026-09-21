// Package tui provides the interactive terminal UI for kvit-coder-ui.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/report"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// Options contains configuration for the UI
type Options struct {
	// Version is the build stamped into the binary by scripts/build.sh, shown
	// in the banner so a session says which build produced it.
	Version     string
	AgentPath   string
	ConfigPath  string
	SessionName string
	SessionMgr  *session.Manager
	Config      *config.Config
	// Yolo passes --yolo to every turn: read and write anywhere, no prompts.
	Yolo bool
	// Structured passes --structured to every turn, so the turn ends with a
	// report this front end draws as a card. It is on by default here and off
	// in a bare "kvit-coder -p", which keeps a scripted run's output the shape
	// it was written against.
	Structured bool
	// InitialModelSet/InitialModel select the startup row, from -m/--model:
	// a :mN, index, id, name or wire id, with optional :effort ("m3:xhigh").
	// Unset means the configured default.
	InitialModelSet bool
	InitialModel    int
	// InitialEffortSet/InitialEffort override the startup row's effort, from
	// -e/--effort or the :effort suffix in InitialModel. Unset means the
	// entry default. Zero values select the default with its default
	// effort, so Options{Config: cfg} behaves as before.
	InitialEffortSet bool
	InitialEffort    string
}

// UI manages the interactive terminal interface
type UI struct {
	version        string
	agentPath      string
	configPath     string
	currentSession string
	sessionMgr     *session.Manager
	cfg            *config.Config
	yolo           bool
	structured     bool
	history        []string
	historyFile    string
	// pendingImages are image files staged by :image and :paste for the next
	// turn. They are consumed (and cleared) when a prompt is sent.
	pendingImages []string
	// models is the snapshot of the models: catalog this UI selects from
	// (one synthesized entry for a legacy single-model config). The
	// selection lives here, in the long-lived process, and travels to the
	// short-lived agent per turn as argv in runAgent.
	models []config.ModelEntry
	// currentModel is the index into models; currentEffort overrides that
	// entry's default ("" = entry default, which is "" for a model with no
	// menu — the krok "cleared on non-reasoning" rule).
	currentModel  int
	currentEffort string
	// title writes the terminal's window title, which is also the tab label
	// in a tabbed terminal: a running or waiting icon, then sessionLabel.
	// The agent process writes it too while a prompt of its own waits for an
	// answer, which is a state only that process can see.
	title *ui.TerminalTitle
	// sessionTitle caches the display title read from the session's
	// meta.json, and titledSession says which session it was read for so a
	// switch drops it. A session has no title until its first turn ends,
	// which is why this is re-read rather than taken once at startup.
	sessionTitle  string
	titledSession string
}

// New creates a new UI instance
func New(opts Options) *UI {
	homeDir, _ := os.UserHomeDir()
	historyFile := filepath.Join(homeDir, ".kvit-coder-history")

	// Load history
	history, _ := ui.LoadHistory(historyFile)

	u := &UI{
		version:        opts.Version,
		agentPath:      opts.AgentPath,
		configPath:     opts.ConfigPath,
		currentSession: opts.SessionName,
		sessionMgr:     opts.SessionMgr,
		cfg:            opts.Config,
		yolo:           opts.Yolo,
		structured:     opts.Structured,
		history:        history,
		historyFile:    historyFile,
	}
	// Snapshot the catalog and start on the default row (the llm.model row,
	// else the first), or on the -m/--model row when one was given. A
	// config edit mid-session needs a UI restart, same as today.
	u.models = opts.Config.ModelList()
	u.currentModel = opts.Config.DefaultModelIndex()
	if u.currentModel < 0 || u.currentModel >= len(u.models) {
		u.currentModel = 0
	}
	if opts.InitialModelSet && opts.InitialModel >= 0 && opts.InitialModel < len(u.models) {
		u.currentModel = opts.InitialModel
		u.currentEffort = ""
	}
	if opts.InitialEffortSet {
		// Validated by the caller (kvit-coder-ui fails startup on a bad
		// -e); a bad value here just keeps the entry default, so direct
		// New callers cannot wedge the banner.
		if v, err := opts.Config.ResolveEffort(u.currentEntry(), opts.InitialEffort); err == nil {
			u.currentEffort = v
		}
	}
	// The window title says what the front end is doing. It is only ever
	// written to a terminal, so a piped run and the tests get an inert
	// setter without asking for one.
	u.title = ui.NewTerminalTitle(os.Stdout)
	if !opts.Config.UI.TerminalTitle.On() {
		u.title.Disable()
	}
	u.title.SetIcons(opts.Config.UI.TerminalTitle.Icons(
		ui.DefaultTitleRunningIcon, ui.DefaultTitleWaitingIcon, ui.DefaultTitleAskingIcon))

	u.pinRunID()
	return u
}

// sessionLabel is what the window title says after the state icon: the
// session's display title once a turn has produced one, the session's
// directory name until then, and nothing at all when there is no session.
func (u *UI) sessionLabel() string {
	if u.currentSession == "" {
		return ""
	}
	if u.titledSession != u.currentSession {
		u.titledSession, u.sessionTitle = u.currentSession, ""
	}
	if u.sessionTitle == "" && u.sessionMgr != nil {
		u.sessionTitle = u.sessionMgr.SessionTitle(u.currentSession)
	}
	if u.sessionTitle != "" {
		return u.sessionTitle
	}
	return u.currentSession
}

// pinRunID puts a run ID derived from the current session into the environment,
// which every agent this UI spawns inherits. Deriving it from the session name
// rather than at random means a turn in this session always reaches the backend
// that already holds its prompt cache.
func (u *UI) pinRunID() {
	if u.currentSession == "" {
		return
	}
	os.Setenv(session.RunIDVar, session.RunIDFor(u.currentSession))
}

// Run starts the interactive UI loop
func (u *UI) Run() error {
	// Ensure terminal is properly reset on exit
	restoreTerminal := func() {
		if os.Stdin.Fd() == 0 {
			cmd := exec.Command("sh", "-c", "stty sane </dev/tty >/dev/tty 2>&1")
			_ = cmd.Run()
		}
	}
	defer func() {
		fmt.Println()
		u.title.Clear()
		restoreTerminal()
	}()

	// Show startup info
	fmt.Printf("\033[38;5;136mkvit-coder-ui %s\033[0m\n", u.bannerVersion())
	fmt.Printf("\033[38;5;136mModel: %s @ %s\033[0m\n", u.activeDisplay(), u.activeBaseURL())
	if u.currentSession != "" {
		if u.sessionMgr.SessionExists(u.currentSession) {
			fmt.Printf("\033[38;5;136mSession: %s (continuing)\033[0m\n", u.currentSession)
		} else {
			fmt.Printf("\033[38;5;136mSession: %s (new)\033[0m\n", u.currentSession)
		}
	}
	fmt.Println("\033[38;5;136mPress Ctrl+C to exit, ':help' for commands\033[0m")
	fmt.Println()

	for {
		// Nothing is running at the top of the loop: whatever happens
		// below either leaves it that way or sets the title itself.
		// Writing the same title twice writes nothing, so this costs
		// nothing on the common path.
		u.title.Waiting(u.sessionLabel())

		// The report the last turn ended with, when it is still the last
		// message. It is drawn as a card above the composer, and while it is
		// waiting on an answer it holds back the inbox turn below.
		card := u.currentReport()

		// Turn boundary, agent off: materialize due proc events into the
		// inbox, then fire an inbox-only turn when anything waits — the
		// startup case and the post-turn case alike. The composer below
		// covers arrivals while it is open. An unanswered decision is
		// something in flight, so the turn waits behind it rather than
		// starting out from under the card; the composer shows the count.
		if n := u.checkWake(); n > 0 && !card.HasInteractive() {
			// The turn below makes the report no longer last, and there is
			// no composer to answer it first, so leave its expanded copy in
			// the scrollback before it goes away.
			printReportTranscript(card, nil)
			fmt.Printf("\033[38;5;136m[inbox: %d pending — processing]\033[0m\n", n)
			u.runAgent("", nil)
			continue
		}

		input, composerImages, woke, shouldExit, err := u.readInput(card)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31m[error] Input error: %v\033[0m\n", err)
			break
		}
		if shouldExit {
			break
		}

		// A wake submit carries no text: the inbox is the prompt.
		if woke {
			u.runAgent("", composerImages)
			continue
		}

		if input == "" && len(composerImages) == 0 {
			continue
		}

		// Add to history and save
		u.history = append(u.history, input)
		_ = ui.SaveHistory(u.historyFile, u.history) // Silently ignore history save errors

		// Handle UI commands
		if strings.HasPrefix(input, ":") {
			shouldExit := u.handleCommand(input)
			if shouldExit {
				break
			}
			continue
		}

		// The composer was seeded with the staged images (see readInput), so
		// its list already holds them in label order: replace, don't append,
		// or every staged image would attach twice. takeImages then adds any
		// @path references and clears the staging for the next turn.
		u.pendingImages = composerImages
		u.runAgent(input, u.takeImages(input))
	}

	return nil
}

// readInput reads user input using the BubbleTea-based input model. It also
// returns the staged images in label order: the ones staged before the
// composer opened (:image, :paste), which seed it, plus any pasted inside
// with Alt+V. The caller hands them to takeImages, which adds @path
// references and clears the staging. woke reports that the composer
// submitted itself on inbox activity: the input is empty and the inbox is
// the prompt.
func (u *UI) readInput(card *report.Report) (string, []string, bool, bool, error) {
	promptText := u.buildPromptText()

	// Create and run input model
	inputModel := ui.NewInputModel(promptText, u.history)
	// While the composer is open, proc events materialize into the inbox
	// and a pending inbox with still-empty text fires an inbox-only turn.
	inputModel.SetWakePoll(u.wakePoll)
	// The card the composer draws above itself. Nil leaves the composer
	// exactly as it was before reports existed.
	inputModel.SetReport(card)
	// Seed the composer list so [imageN] numbering covers the already-staged
	// images too, not just the ones pasted below.
	inputModel.SetStagedImages(u.pendingImages)
	// @path completes live against the workspace root (empty config falls
	// back to the working directory inside the completer).
	if u.cfg != nil {
		inputModel.SetCompletionBaseDir(u.cfg.Workspace.Root)
	}
	// Alt+V would type √ on macOS, so the paste key is offered everywhere
	// except darwin — same exclusion krok uses for its Alt+V escape hatch.
	if runtime.GOOS != "darwin" {
		inputModel.SetImagePasteHandler(StageClipboardImage)
	}
	p := tea.NewProgram(inputModel)
	result, err := p.Run()

	if err != nil {
		return "", nil, false, false, err
	}

	// Get the result
	finalModel := result.(ui.InputModel)
	if finalModel.Cancelled() || !finalModel.Submitted() {
		// Exiting leaves the card behind too: its bubbletea frame is cleared
		// on quit, so print the expanded copy for the scrollback.
		if card != nil {
			printReportTranscript(card, finalModel.CardAnswerPicked())
			fmt.Println("[cancelled]")
			fmt.Println()
		}
		return "", nil, false, true, nil // User cancelled (Ctrl+C)
	}

	// A wake submit carries no text to echo: the inbox is the prompt.
	if finalModel.WakeFired() {
		if card != nil {
			printReportTranscript(card, finalModel.CardAnswerPicked())
			fmt.Println("[wake: processing inbox]")
			fmt.Println()
		}
		return "", finalModel.PastedImages(), true, false, nil
	}

	input := strings.TrimSpace(finalModel.Value())
	images := finalModel.PastedImages()
	ans := finalModel.CardAnswerPicked()
	if card != nil && (input != "" || len(images) > 0 || ans != nil) {
		// The composer frame that showed this report is cleared on submit,
		// so leave the expanded report plus what the user did with it in the
		// scrollback before the next turn runs. The prompt echo below follows
		// it, so the scrollback reads: report, selection, prompt, next turn.
		printReportTranscript(card, ans)
		if strings.TrimSpace(finalModel.Value()) == "" {
			// No text to echo (a resolve pick with no follow-up, or an
			// image-only turn): the pick line above already says what
			// happened, and runAgent lists images when there are any.
			fmt.Println()
			return input, images, false, false, nil
		}
	}

	// Display the submitted input with gray background
	colorStart := "\033[97;100m"
	colorEnd := "\033[0m"

	inputLines := strings.Split(finalModel.Value(), "\n")
	for _, line := range inputLines {
		fmt.Printf("%s%s%s\n", colorStart, line, colorEnd)
	}
	fmt.Println()

	return input, images, false, false, nil
}

// buildPromptText builds the prompt text with session info
func (u *UI) buildPromptText() string {
	var sessionInfo string
	if u.currentSession != "" {
		sessionInfo = fmt.Sprintf(" %s", u.currentSession)
	}
	return ui.MakePrompt(fmt.Sprintf("[ui%s]> ", sessionInfo))
}

// stageImage validates one :image reference and stages it for the next turn.
// The staged position is its future [imageN] label, so the confirmation
// echoes it in the same [imageN: path] form the composer list uses.
func (u *UI) stageImage(ref string) {
	clean := ExtractImageRefs(ref, nil)
	if len(clean) == 0 {
		fmt.Printf("Not an image file (or not found): %s\n", ref)
		return
	}
	u.pendingImages = append(u.pendingImages, clean...)
	fmt.Printf("Staged for next turn: [image%d: %s]\n", len(u.pendingImages), clean[0])
}

// takeImages consumes staged images and any @path references in this prompt,
// returning the files to attach to the turn in label order: staged images
// first ([image1..N], as the composer list showed them), @path references
// after. Attachments and labels share this order end to end.
func (u *UI) takeImages(input string) []string {
	detected := ExtractImageRefs(input, nil)
	out := make([]string, 0, len(u.pendingImages)+len(detected))
	out = append(out, u.pendingImages...)
	seen := map[string]bool{}
	for _, p := range out {
		seen[p] = true
	}
	for _, p := range detected {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	u.pendingImages = nil
	return out
}

// handleCommand handles UI meta-commands
func (u *UI) handleCommand(input string) bool {
	cmd := strings.TrimPrefix(input, ":")
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}

	// :mN / :eN select the model and its effort; :e <value> sets the effort
	// by canonical value. This precedes the exact-match switch: no existing
	// command is m/e-plus-digits, and matching digits-only keeps a future
	// :memory from colliding.
	if kind, n, ok := config.ParseIndexedCommand(parts[0]); ok {
		switch kind {
		case 'm':
			u.switchModel(n)
		case 'e':
			u.setEffortIndex(n)
		}
		return false
	}
	if parts[0] == "e" {
		if len(parts) < 2 {
			fmt.Println("Usage: :e <effort>  (or :eN for the Nth level in :help)")
			fmt.Println()
			return false
		}
		u.setEffortValue(strings.Join(parts[1:], " "))
		return false
	}
	if parts[0] == "m" {
		fmt.Println("Usage: :mN  (e.g. :m1; the list is in :help)")
		fmt.Println()
		return false
	}

	switch parts[0] {
	case "quit", "q", "exit":
		return true

	case "help", "h":
		u.showHelp()

	case "new":
		// Start a new session
		u.currentSession = u.sessionMgr.GenerateSessionName()
		u.pinRunID()
		fmt.Printf("Started new session: %s\n\n", u.currentSession)

	case "resume", "last":
		recent, err := u.sessionMgr.MostRecent()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding the most recent session: %v\n\n", err)
			return false
		}
		if recent == "" {
			fmt.Println("No previous session to resume.")
			fmt.Println()
			return false
		}
		u.currentSession = recent
		u.pinRunID()
		fmt.Printf("Resumed session: %s\n\n", recent)

	case "switch":
		if len(parts) < 2 {
			fmt.Println("Usage: :switch <session-name>")
			fmt.Println()
			return false
		}
		sessionName := parts[1]
		if !u.sessionMgr.SessionExists(sessionName) {
			fmt.Printf("Session %q not found. Use :sessions to list available sessions.\n\n", sessionName)
			return false
		}
		u.currentSession = sessionName
		u.pinRunID()
		fmt.Printf("Switched to session: %s\n\n", sessionName)

	case "sessions":
		sessions, err := u.sessionMgr.ListSessions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing sessions: %v\n\n", err)
			return false
		}
		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
		} else {
			fmt.Printf("%-30s  %-20s  %-8s  %s\n", "NAME", "MODIFIED", "MESSAGES", "TITLE")
			fmt.Println("────────────────────────────────────────────────────────────────────────────────")
			for _, s := range sessions {
				marker := ""
				if s.Name == u.currentSession {
					marker = " *"
				}
				fmt.Printf("%-30s  %-20s  %-8d  %s%s\n", s.Name, s.ModTime.Format("2006-01-02 15:04"), s.MessageCount, s.Title, marker)
			}
		}
		fmt.Println()

	case "report":
		// The card comes back on its own: any command ends the composer and
		// the next one reads the report again. This says so when there is
		// nothing to come back to, which is the only case that needs words.
		if u.currentReport() == nil {
			fmt.Println("No report to show. The last message is not a report.")
			fmt.Println()
		}
		return false

	case "history":
		if u.currentSession == "" {
			fmt.Println("No active session. Use :new or :switch to select a session.")
			fmt.Println()
			return false
		}
		content, err := u.sessionMgr.ShowSession(u.currentSession)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error showing session: %v\n\n", err)
			return false
		}
		fmt.Print(content)
		fmt.Println()

	case "clear":
		// Clear terminal
		fmt.Print("\033[2J\033[H")

	case "config":
		fmt.Printf("Config: %s\n", u.configPath)
		fmt.Printf("Model: %s\n", u.activeDisplay())
		fmt.Printf("Base URL: %s\n", u.activeBaseURL())
		fmt.Printf("Agent: %s\n", u.agentPath)
		if u.currentSession != "" {
			fmt.Printf("Session: %s\n", u.currentSession)
		}
		fmt.Println("Models:")
		for i, e := range u.models {
			marker := ""
			if i == u.currentModel {
				marker = " *"
			}
			if e.Summarizer {
				marker += " [summarizer]"
			}
			fmt.Printf("  :m%d %s (%s)%s\n", i+1, e.Name, config.EntryDisplay(e, u.effortFor(i)), marker)
		}
		fmt.Println()

	case "image":
		// :image <path>... stages image files for the next turn.
		if len(parts) < 2 {
			if len(u.pendingImages) == 0 {
				fmt.Println("Usage: :image <path>...  (or @path in any prompt, or :paste)")
			} else {
				fmt.Printf("Staged for next turn (%d):\n", len(u.pendingImages))
				for i, p := range u.pendingImages {
					fmt.Printf("  [image%d: %s]\n", i+1, p)
				}
			}
			fmt.Println()
			return false
		}
		for _, ref := range splitImageFields(strings.Join(parts[1:], " ")) {
			u.stageImage(ref)
		}
		fmt.Println()

	case "paste":
		// :paste reads an image from the system clipboard for the next turn.
		path, err := StageClipboardImage()
		if err != nil {
			fmt.Printf("No image pasted: %v\n\n", err)
			return false
		}
		u.pendingImages = append(u.pendingImages, path)
		fmt.Printf("Staged for next turn: [image%d: %s]\n\n", len(u.pendingImages), path)

	default:
		fmt.Printf("Unknown command: %s. Type :help for available commands.\n\n", parts[0])
	}

	return false
}

// currentEntry is the selected catalog row.
func (u *UI) currentEntry() config.ModelEntry {
	if u.currentModel < 0 || u.currentModel >= len(u.models) {
		u.currentModel = 0
	}
	return u.models[u.currentModel]
}

// effectiveEffort is what the current row runs with: the :eN override when
// set, else the entry default ("" for a model with no menu).
func (u *UI) effectiveEffort() string {
	if u.currentEffort != "" {
		return u.currentEffort
	}
	return u.cfg.DefaultEffort(u.currentEntry())
}

// effortFor is what catalog row i would run with: the override when it is
// the current row, else the entry default.
func (u *UI) effortFor(i int) string {
	if i == u.currentModel {
		return u.effectiveEffort()
	}
	return u.cfg.DefaultEffort(u.models[i])
}

// activeDisplay is the banner/:config model string for the selection.
func (u *UI) activeDisplay() string {
	return config.EntryDisplay(u.currentEntry(), u.effectiveEffort())
}

// activeBaseURL is the endpoint the selection sends to.
func (u *UI) activeBaseURL() string {
	if base := u.currentEntry().BaseURL; base != "" {
		return base
	}
	return u.cfg.LLM.BaseURL
}

// switchModel handles :mN: select models[N-1] and reset the effort to the
// new entry's default (cleared when it has no menu). KVIT_RUN_ID is left
// alone: the prompt cache is keyed per model server-side, and a new ID would
// also drop the shared prefix.
func (u *UI) switchModel(n int) {

	if n < 1 || n > len(u.models) {
		fmt.Printf("unknown model :m%d; use one of :m1-:m%d\n\n", n, len(u.models))
		return
	}

	u.currentModel = n - 1

	u.currentEffort = ""

	e := u.currentEntry()

	shown := u.effectiveEffort()
	if shown == "" {
		shown = "no effort"
	}

	if u.catalogUsesProfiles() {
		fmt.Printf("Switched to %s (%s:%s) [%s] @ %s\n\n", e.Name, e.Model, shown, u.profileDisplay(e), u.activeBaseURL())
		return
	}

	fmt.Printf("Switched to %s (%s:%s) @ %s\n\n", e.Name, e.Model, shown, u.activeBaseURL())
}

// catalogUsesProfiles reports whether any catalog row sets its own profile.
// The :h list and the :mN confirmation name the effective profile only then,
// so configs from before per-model profiles read exactly as before.
func (u *UI) catalogUsesProfiles() bool {

	for _, e := range u.models {
		if e.Profile != "" {
			return true
		}
	}

	return false
}

// profileDisplay is the effective profile for a row for display: "weak" or
// "strong" (the default-strong normalizes the empty value).
func (u *UI) profileDisplay(e config.ModelEntry) string {

	if prof := u.cfg.ProfileFor(e); prof != "" {
		return prof
	}

	return "strong"
}

// setEffortIndex handles :eN: the Nth row of the current model's menu, in
// config order.
func (u *UI) setEffortIndex(n int) {
	e := u.currentEntry()
	menu := u.cfg.EffortOptions(e)
	if len(menu) == 0 {
		fmt.Printf("current model %q does not support reasoning effort\n\n", e.Name)
		return
	}
	if n < 1 || n > len(menu) {
		names := make([]string, len(menu))
		for i, o := range menu {
			names[i] = o.Value
		}
		fmt.Printf("unknown effort level ':e%d'; use one of: :e1-:e%d (%s)\n\n",
			n, len(menu), strings.Join(names, ", "))
		return
	}
	u.currentEffort = menu[n-1].Value
	fmt.Printf("Effort set to %s for %s\n\n", menu[n-1].Value, e.Name)
}

// setEffortValue handles :e <value>: the same menu by canonical value, for
// scripts and menus longer than :e9 is comfortable for.
func (u *UI) setEffortValue(value string) {
	e := u.currentEntry()
	v, err := u.cfg.ResolveEffort(e, strings.TrimSpace(value))
	if err != nil {
		fmt.Printf("%v\n\n", err)
		return
	}
	u.currentEffort = v
	if v == "" {
		fmt.Printf("Effort cleared for %s\n\n", e.Name)
		return
	}
	fmt.Printf("Effort set to %s for %s\n\n", v, e.Name)
}

// showModels prints the :mN catalog and the current entry's :eN menu, the
// list :h promises. Numbering is 1-based config-file order, with * on the
// active row like :sessions.
func (u *UI) showModels() {

	fmt.Println("Models (:mN to switch):")

	showProfile := u.catalogUsesProfiles()

	for i, e := range u.models {

		marker := " "
		if i == u.currentModel {
			marker = "*"
		}

		eff := u.effortFor(i)
		if eff != "" {
			eff = " :" + eff
		}

		prof := ""
		if showProfile {
			prof = " [" + u.profileDisplay(e) + "]"
		}

		base := e.BaseURL
		if base == "" {
			base = u.cfg.LLM.BaseURL
		}

		fmt.Printf("  :m%d %s %s (%s)%s%s @ %s\n", i+1, marker, e.Name, e.Model, eff, prof, base)
	}
	e := u.currentEntry()
	menu := u.cfg.EffortOptions(e)
	if len(menu) == 0 {
		fmt.Printf("No effort levels for %s (non-reasoning model)\n", e.Name)
		return
	}
	fmt.Printf("Effort for %s (:eN to set):\n", e.Name)
	active := u.effectiveEffort()
	for i, o := range menu {
		marker := " "
		if o.Value == active {
			marker = "*"
		}
		def := ""
		if o.Default {
			def = " (default)"
		}
		fmt.Printf("  :e%d %s %s%s\n", i+1, marker, o.Display(), def)
	}
}

// showHelp displays the help message
func (u *UI) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  :help, :h        Show this help message")
	fmt.Println("  :quit, :q        Exit the UI")
	fmt.Println("  :new             Start a new session")
	fmt.Println("  :switch <name>   Switch to an existing session")
	fmt.Println("  :resume          Continue the most recent session")
	fmt.Println("  :sessions        List all sessions")
	fmt.Println("  :history         Show current session history")
	fmt.Println("  :report          Show the last turn's report again")
	fmt.Println("  :clear           Clear the terminal")
	fmt.Println("  :config          Show configuration")
	fmt.Println("  :image <path>..  Stage image files for the next turn")
	fmt.Println("  :paste           Stage the clipboard image for the next turn")
	fmt.Println("  :mN              Switch model (e.g. :m1; the list follows)")
	fmt.Println("  :eN, :e <level>  Set reasoning effort for the current model")
	fmt.Println()
	u.showModels()
	fmt.Println()
	fmt.Println("Enter any other text to send as a prompt to the agent.")
	fmt.Println("Attach images: Alt+V pastes the clipboard image as an [imageN]")
	fmt.Println("label at the cursor, @path/to/shot.png (or drag-drop it) attaches")
	fmt.Println("a file. The list under the input maps each label to its file,")
	fmt.Println("so you can write \"compare [image1] with [image2]\".")
	fmt.Println("Ctrl+V cannot carry images: the terminal intercepts it before")
	fmt.Println("the app ever sees the keypress.")
	fmt.Println()
}

// runAgent spawns kvit-coder with the given prompt. An empty prompt is an
// inbox-only turn: the inbox files are the prompt, so --wake replaces -p
// (an empty -p would not even select exec mode) and the agent skips the
// user message.
// agentArgs is the command line one turn is spawned with. It is separate from
// runAgent so what the front end asks of the agent can be read without starting
// a process.
func (u *UI) agentArgs(prompt string, images []string) []string {

	var args []string
	if strings.TrimSpace(prompt) == "" {
		args = []string{"--wake"}
	} else {
		args = []string{"-p", prompt}
	}

	// Images travel as paths, not pixels: the agent normalizes them into the
	// session on arrival. argv stays small and the prompt cache undisturbed.
	// Numbered to match the [imageN] labels the composer list showed.
	for _, img := range images {
		args = append(args, "-image", img)
	}

	// Pass config file
	if u.configPath != "" {
		args = append(args, "-config", u.configPath)
	}

	// Pass session if set
	if u.currentSession != "" {
		args = append(args, "-s", u.currentSession)
	}

	// The selection travels per turn as explicit wire fields, so a skewed
	// UI/agent config pair still sends the right endpoint. Empty effort is
	// omitted: with --model the agent falls back to the entry default (which
	// is empty for a model with no menu), without it the config is unchanged.
	entry := u.currentEntry()
	args = append(args, "--model", entry.Model)
	if base := u.activeBaseURL(); base != "" {
		args = append(args, "--base-url", base)
	}
	backend := entry.APIBackend
	if backend == "" {
		backend = u.cfg.LLM.APIBackend
	}
	if backend != "" {
		args = append(args, "--api-backend", backend)
	}
	if eff := u.effectiveEffort(); eff != "" {
		args = append(args, "--effort", eff)
	}

	// The UI took the flag, but the agent it spawns is what enforces paths.
	if u.yolo {
		args = append(args, "-yolo")
	}

	// Reports are this front end's default because it has somewhere to put
	// them; the agent on its own defaults to prose.
	if u.structured {
		args = append(args, "-structured")
	}
	return args
}

func (u *UI) runAgent(prompt string, images []string) {
	// The turn is what the window title is for: a window that is not on
	// screen still says whether the agent is working. The session's title
	// is re-read on the way out because the first turn is what creates it.
	u.title.Running(u.sessionLabel())
	defer func() { u.title.Waiting(u.sessionLabel()) }()

	args := u.agentArgs(prompt, images)
	for i, img := range images {
		fmt.Printf("\033[38;5;136m[image%d: %s]\033[0m\n", i+1, img)
	}

	// Ctrl-C at the terminal signals every process in the foreground group,
	// which is this UI as well as the agent it started. While the agent runs
	// the interrupt is the agent's to handle — it stops the turn, saves it and
	// exits — so the UI takes the signal and throws it away, staying at its
	// prompt. Without this a single ctrl-c ended the whole session, which is
	// not what "cancel this turn" should mean.
	//
	// Catching rather than ignoring matters: an ignored signal stays ignored
	// across exec, so signal.Ignore here would leave the agent unable to see
	// ctrl-c at all. A caught one is reset to the default on exec, which is
	// what lets the agent install its own handler.
	interrupts := make(chan os.Signal, 4)
	signal.Notify(interrupts, syscall.SIGINT)
	defer signal.Stop(interrupts)
	go func() {
		for range interrupts {
			// The agent has it.
		}
	}()

	// Create command
	cmd := exec.Command(u.agentPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// The UI already showed its banner and echoed the prompt, so tell the
	// child to skip its copies rather than printing both.
	cmd.Env = append(os.Environ(), session.FromUIVar+"=1")
	// The agent reads stdin for steering: a line typed while a turn is running
	// reaches the model at its next iteration. The UI is not reading stdin
	// while it waits for the agent, so there is no contest for it.
	cmd.Stdin = os.Stdin

	// Run and wait for completion
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 130 is the shell's convention for "killed by SIGINT"; the agent
			// exits with it after a second ctrl-c.
			if code := exitErr.ExitCode(); code == 2 || code == 130 {
				fmt.Println("[cancelled]")
			}
		} else {
			fmt.Fprintf(os.Stderr, "\033[31m[error] Agent failed: %v\033[0m\n", err)
		}
	}

	fmt.Println()
}

// currentReport is the report the last turn ended with, or nil when the
// conversation's last message is not one.
//
// There is nothing to track between turns: a cancelled turn, an exhausted
// iteration budget and an answered report each append messages of their own, so
// each takes the card away by being there. A read error means no card rather
// than a failed prompt — the report is a convenience, and the conversation is
// still in the history either way.
func (u *UI) currentReport() *report.Report {
	if u.sessionMgr == nil || u.currentSession == "" {
		return nil
	}
	rep, current, err := u.sessionMgr.LastReport(u.currentSession)
	if err != nil || !current {
		return nil
	}
	return rep
}

// bannerVersion is what the banner shows for the build: whatever ldflags
// stamped in, or "dev" when the binary was built with a plain "go build" and
// nothing was stamped.
func (u *UI) bannerVersion() string {
	if u.version == "" {
		return "dev"
	}
	return u.version
}
