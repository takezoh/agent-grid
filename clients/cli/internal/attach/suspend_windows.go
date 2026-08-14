//go:build windows

package attach

import "os"

// Windows has no job control, so no signal asks this process to suspend. Every
// catchable signal there ends the attach, which is the same outcome the Unix
// path reaches for everything except SIGTSTP.
func isSuspend(os.Signal) bool { return false }

func suspendProcess() {}
