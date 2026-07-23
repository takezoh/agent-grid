package runtimetest

import (
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/state"
)

func TestWaitForSnapshotTimeoutIncludesSnapshot(t *testing.T) {
	h := New(t, WithWaitTimeout(20*time.Millisecond))

	_, err := h.waitForSnapshot(func(_ state.State) bool { return false })
	if err == nil {
		t.Fatal("waitForSnapshot err = nil, want timeout")
	}
	msg := err.Error()
	if !strings.Contains(msg, "snapshot=") {
		t.Fatalf("timeout message = %q, want snapshot", msg)
	}
}
