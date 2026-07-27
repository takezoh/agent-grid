package runtime

import "testing"

func TestTerminalBarrierRequiresForwardedFinalSequenceOrExplicitGap(t *testing.T) {
	var barrier TerminalBarrier
	barrier.MarkForwarded(2)
	if barrier.Ready(3) {
		t.Fatal("terminal passed an unforwarded output")
	}
	gap, missing := barrier.DeliveryGap(3)
	if !gap || missing != 1 {
		t.Fatalf("delivery gap = %v, missing=%d", gap, missing)
	}
	barrier.MarkForwarded(3)
	if !barrier.Ready(3) {
		t.Fatal("terminal did not pass after final sequence")
	}
	if gap, _ := barrier.DeliveryGap(3); gap {
		t.Fatal("delivery gap reported after barrier closed")
	}
}
