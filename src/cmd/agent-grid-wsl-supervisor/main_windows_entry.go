//go:build windows

package main

// runDefault enters the Windows service host when launched with no subcommand.
func runDefault() error {
	return runService(nil)
}
