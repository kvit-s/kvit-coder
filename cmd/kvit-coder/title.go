package main

import (
	"io"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// installAskTitle makes a prompt waiting for an answer show in the terminal's
// window title, which a tabbed terminal such as VS Code's uses as the tab
// label.
//
// The front end set the title to its "running" icon when it started this turn
// and then blocked waiting for this process to finish, so it cannot see that
// the turn has stopped to ask something — a Question, a path or MCP
// confirmation, the pause prompt. Every one of those waits in inbox.Ask, so
// one notifier covers them all: the title says "waiting on you" for as long as
// the wait lasts and goes back to "running" when the answer arrives. However
// the turn ends after that, the front end has the last word and puts its own
// idle title up.
//
// label is what the title says after the icon, and out is the terminal —
// nothing is written when it is not one.
func installAskTitle(box *inbox.Inbox, cfg *config.Config, out io.Writer, label string) {
	title := ui.NewTerminalTitle(out)
	if !cfg.UI.TerminalTitle.On() {
		title.Disable()
	}
	title.SetIcons(cfg.UI.TerminalTitle.Icons(
		ui.DefaultTitleRunningIcon, ui.DefaultTitleWaitingIcon, ui.DefaultTitleAskingIcon))

	box.SetAskNotifier(func(asking bool) {
		if asking {
			title.Asking(label)
			return
		}
		title.Running(label)
	})
}
