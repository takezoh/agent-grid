package main

import (
	"fmt"
	"os"
)

// run dispatches CLI subcommands and the Windows SCM service entry.
func run(args []string) error {
	if len(args) == 0 {
		// No subcommand: on Windows enter SCM service mode; elsewhere show usage.
		return runDefault()
	}
	switch args[0] {
	case "register":
		return runRegister(args[1:], os.Stdout, newDefaultSource(), newServiceGate())
	case "apply-check":
		return runApplyCheck(args[1:], os.Stdout)
	case "print-runtime":
		return runPrintRuntime(args[1:], os.Stdout)
	case "run", "service":
		return runService(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
