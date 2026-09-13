package tui

import (
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/report"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// reportTranscript returns the scrollback copy of the report the composer just
// answered: the whole report with every block's details shown, plus what the
// user picked from it when they picked anything.
//
// The composer runs in bubbletea, whose last frame is cleared on submit (the
// quitting view is empty), so without this the previous turn's card vanishes
// as soon as the next turn starts and there is nothing to scroll back to. The
// transcript is printed after the user's action, before the next turn runs, so
// the scrollback reads: turn output, expanded report, user action, next turn.
func reportTranscript(card *report.Report, ans *ui.CardAnswer) string {
	if card == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("── report (previous turn, expanded) ──\n")
	sb.WriteString(strings.TrimRight(card.PlainText(), "\n"))
	sb.WriteString("\n─────────────────────────────────────\n")
	if ans != nil {
		if d := ans.Describe(); d != "" {
			sb.WriteString(d + "\n")
		}
	}
	return sb.String()
}

// printReportTranscript prints the scrollback copy, or nothing when there is
// no report to keep.
func printReportTranscript(card *report.Report, ans *ui.CardAnswer) {
	if s := reportTranscript(card, ans); s != "" {
		fmt.Print(s)
	}
}
