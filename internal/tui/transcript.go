package tui

import (
	"fmt"

	"github.com/kvit-s/kvit-coder/internal/report"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// reportTranscript returns the scrollback copy of the report the composer just
// answered: the whole report with every block's details shown, plus what the
// user picked from it when they picked anything.
//
// It paints with the same colors as the live card (see ui.Transcript), so the
// history reads like the report that was just answered rather than plain
// text.
//
// The composer runs in bubbletea, whose last frame is cleared on submit (the
// quitting view is empty), so without this the previous turn's card vanishes
// as soon as the next turn starts and there is nothing to scroll back to. The
// transcript is printed after the user's action, before the next turn runs, so
// the scrollback reads: turn output, expanded report, user action, next turn.
func reportTranscript(card *report.Report, ans *ui.CardAnswer) string {
	return ui.Transcript(card, ans)
}

// printReportTranscript prints the scrollback copy, or nothing when there is
// no report to keep.
func printReportTranscript(card *report.Report, ans *ui.CardAnswer) {
	if s := reportTranscript(card, ans); s != "" {
		fmt.Print(s)
	}
}

// keepTranscript reports whether the scrollback copy is worth leaving: the
// user typed something, staged images, or picked an answer, so the expanded
// report plus what they did with it belongs in the history. An empty submit
// with no answer restates the completed report the composer was already
// showing, so it stays out and the live card is the only copy.
func keepTranscript(input string, images []string, ans *ui.CardAnswer) bool {
	return input != "" || len(images) > 0 || ans != nil
}
