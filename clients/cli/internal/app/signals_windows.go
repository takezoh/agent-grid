//go:build windows

package app

import "os"

// Windows has no job control and no SIGHUP. os.Interrupt is the one catchable
// signal a console delivers, and the attach restores the terminal before it
// ends on it.
func attachSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
