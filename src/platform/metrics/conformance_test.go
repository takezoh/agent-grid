package metrics_test

import (
	"testing"

	"github.com/takezoh/agent-roost/platform/metrics"
)

// SPEC §17.5 — absolute thread totals are used; same absolute value from the same
// thread must not be double-counted (§13.5 (b)).
func TestSPEC_17_5_AbsoluteTokenNoDoubleCount(t *testing.T) {
	acc := metrics.NewAccumulator()

	first := acc.Observe(metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	if first.Total != 150 {
		t.Fatalf("after first report: Total want 150, got %d", first.Total)
	}

	// Reporting the same absolute value again must contribute zero delta.
	second := acc.Observe(metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	if second.Total != 150 {
		t.Errorf("after identical second report: Total want 150 (no double-count), got %d", second.Total)
	}
}
