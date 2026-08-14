// Command agent-grid is the terminal client for Agent Grid sessions.
//
// The binary's canonical published name is still an open packaging question
// (handoff-cli-binary-canonical-name): the experience contract writes `ag`
// throughout, and whether that short name is safe to claim on a user's PATH is
// a boundary question this change records rather than answers. The import path
// stays `cmd/agent-grid`, which is unambiguous whatever the installed name
// turns out to be.
package main

import (
	"os"

	"github.com/takezoh/agent-grid/clients/cli/internal/app"
	"github.com/takezoh/agent-grid/clients/cli/internal/tty"
)

func main() {
	streams := app.Streams{
		In:            os.Stdin,
		Out:           os.Stdout,
		Err:           os.Stderr,
		OutIsTerminal: tty.IsTerminal(os.Stdout),
	}
	os.Exit(app.ExitCode(app.Run(streams, os.Args[1:])))
}
