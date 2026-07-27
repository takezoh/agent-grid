// Package main is the Windows SCM entry for the WSL host-server supervisor.
// Non-Windows builds only expose a clear diagnostic; registration snapshot
// ownership lives in platform/wslhost.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "agent-grid-wsl-supervisor: %v\n", err)
		os.Exit(1)
	}
}
