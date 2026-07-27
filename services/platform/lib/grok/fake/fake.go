// Package fake models the public Grok CLI lifecycle flag contract for tests.
package fake

import (
	"fmt"
)

// ValidateArgv accepts exactly one documented session lifecycle selection.
func ValidateArgv(argv []string) error {
	count := 0
	hasFork := false
	hasResume := false
	hasSessionID := false
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--session-id", "-s":
			count++
			hasSessionID = true
			i++
		case "--resume", "-r":
			count++
			hasResume = true
			i++
		case "--continue":
			count++
		case "--fork-session":
			hasFork = true
		}
	}
	if count != 1 && count != 2 {
		return fmt.Errorf("grok fake: want one lifecycle selector or a fork pair, got %d", count)
	}
	if count == 2 && (!hasFork || !hasResume || !hasSessionID) {
		return fmt.Errorf("grok fake: two selectors require resume, fork-session, and child session-id")
	}
	return nil
}
