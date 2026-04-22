package main

import (
	"fmt"
	"os"
)

const usage = `masto - Mastodon CLI

Usage:
  masto login <server>
  masto post <text> [--visibility public|unlisted|private|direct] [--cw <text>] [--reply-to <id>]
  masto timeline [--kind home|public] [--limit N]
  masto reply <status-id> <text>
  masto delete <status-id>
  masto whoami
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "login":
		err = cmdLogin(args)
	case "post":
		err = cmdPost(args)
	case "timeline":
		err = cmdTimeline(args)
	case "reply":
		err = cmdReply(args)
	case "delete":
		err = cmdDelete(args)
	case "whoami":
		err = cmdWhoami(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
