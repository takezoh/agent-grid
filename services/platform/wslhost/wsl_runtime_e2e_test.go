//go:build e2e

package wslhost

import "testing"

// TestE2E_FakeVsRealWSLBoundDistribution is the opt-in T3 registration for
// BuildWSLExecArgv and the real Windows executable marker "wsl.exe".
func TestE2E_FakeVsRealWSLBoundDistribution(t *testing.T) {
	TestFakeVsRealWSLBoundDistribution(t)
}

// TestE2E_FakeVsRealHealthzSemantics is the opt-in T3 registration for the
// HTTPHealthProber/FakeHealthProber comparison.
func TestE2E_FakeVsRealHealthzSemantics(t *testing.T) {
	TestFakeVsRealHealthzSemantics(t)
}
