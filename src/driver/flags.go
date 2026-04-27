package driver

import "strings"

// hasFlagToken returns true when command contains the exact flag as a token
// (or with a = suffix like "--flag=value"). Assumes alias expansion has
// already been applied by the caller.
func hasFlagToken(command, flag string) bool {
	for _, p := range strings.Fields(command) {
		if p == flag || strings.HasPrefix(p, flag+"=") {
			return true
		}
	}
	return false
}
