//go:build !windows

package main

import "errors"

// runDefault on non-Windows is a clear diagnostic (SCM is Windows-only).
func runDefault() error {
	return errors.New("usage: agent-grid-wsl-supervisor <register|apply-check|print-runtime|run> (service mode requires Windows)")
}

// runService is unavailable outside Windows.
func runService(args []string) error {
	_ = args
	return errors.New("agent-grid-wsl-supervisor: service mode requires Windows")
}
