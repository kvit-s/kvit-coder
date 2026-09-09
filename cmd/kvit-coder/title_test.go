package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// TestInstallAskTitle: while a prompt waits for an answer the window title
// says so, and it goes back to "a turn is running" once the answer arrives.
func TestInstallAskTitle(t *testing.T) {
	box := inbox.New("")
	var buf strings.Builder
	installAskTitle(box, &config.Config{}, &buf, "add the retry loop")

	go func() {
		time.Sleep(30 * time.Millisecond)
		box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "y"})
	}()
	if _, outcome := box.Ask(context.Background(), nil, "", 0); outcome != inbox.AskAnswered {
		t.Fatalf("outcome is %v, want answered", outcome)
	}

	want := "\033]0;" + ui.DefaultTitleAskingIcon + " add the retry loop\007" +
		"\033]0;" + ui.DefaultTitleRunningIcon + " add the retry loop\007"
	if got := buf.String(); got != want {
		t.Fatalf("title stream = %q, want %q", got, want)
	}
}

// TestInstallAskTitleObeysTheConfig: ui.terminal_title turns it off and
// replaces the icon.
func TestInstallAskTitleObeysTheConfig(t *testing.T) {
	ask := func(cfg *config.Config) string {
		box := inbox.New("")
		var buf strings.Builder
		installAskTitle(box, cfg, &buf, "s")
		go func() {
			time.Sleep(20 * time.Millisecond)
			box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "y"})
		}()
		box.Ask(context.Background(), nil, "", 0)
		return buf.String()
	}

	off := &config.Config{}
	no := false
	off.UI.TerminalTitle.Enabled = &no
	if got := ask(off); got != "" {
		t.Errorf("with the title off, wrote %q", got)
	}

	custom := &config.Config{}
	icon := "??"
	custom.UI.TerminalTitle.Asking = &icon
	if got := ask(custom); !strings.Contains(got, "\033]0;?? s\007") {
		t.Errorf("the configured asking icon is missing from %q", got)
	}
}
