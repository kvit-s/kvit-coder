package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/session"
)

// runSteer implements "kvit-coder steer [-s session] <text>": it drops a
// message in a session's inbox, which the turn running there picks up at its
// next iteration. With no -s it looks for the one session with a live turn.
//
// The subcommand is intercepted before flag.Parse, because the main command
// uses the flag package, which has no notion of subcommands.
func runSteer(args []string) int {
	fs := flag.NewFlagSet("steer", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sessionName := fs.String("s", "", "session to steer (default: the one with a running turn)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: kvit-coder steer [-s session] <message>")
		fmt.Fprintln(os.Stderr, "       kvit-coder steer [-s session] -   # read the message from stdin")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	text := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if text == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "steer: could not read stdin: %v\n", err)
			return 1
		}
		text = strings.TrimSpace(string(data))
	}
	if text == "" {
		fs.Usage()
		return 2
	}

	mgr, err := session.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "steer: %v\n", err)
		return 1
	}

	name := *sessionName
	if name == "" {
		active, err := mgr.ActiveTurns()
		if err != nil {
			fmt.Fprintf(os.Stderr, "steer: %v\n", err)
			return 1
		}
		switch len(active) {
		case 0:
			fmt.Fprintln(os.Stderr, "steer: no session has a turn running; name one with -s")
			return 1
		case 1:
			name = active[0]
		default:
			fmt.Fprintf(os.Stderr, "steer: %d sessions have a turn running (%s); name one with -s\n",
				len(active), strings.Join(active, ", "))
			return 1
		}
	} else if !mgr.SessionExists(name) {
		fmt.Fprintf(os.Stderr, "steer: session %q not found\n", name)
		return 1
	}

	if _, err := inbox.Deliver(mgr.InboxDirFor(name), inbox.Message{
		Kind: inbox.KindUserLine,
		Text: text,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "steer: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "delivered to %s\n", name)
	return 0
}
