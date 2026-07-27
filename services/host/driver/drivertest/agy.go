package drivertest

import (
	"encoding/json"
	"time"

	"github.com/takezoh/agent-grid/services/host/state"
)

// FakeAgyLifecycle is the deterministic half of the agy fake/real contract.
// The fidelity test obtains the same semantic sequence from a real agy binary.
func FakeAgyLifecycle(at time.Time) []state.DEvHook {
	pre, _ := json.Marshal(map[string]any{
		"conversationId": "fake-conversation",
		"invocationNum":  0,
	})
	stop, _ := json.Marshal(map[string]any{
		"conversationId":    "fake-conversation",
		"executionNum":      0,
		"fullyIdle":         true,
		"terminationReason": "NO_TOOL_CALL",
	})
	return []state.DEvHook{
		{Event: "PreInvocation", Timestamp: at, Payload: pre},
		{Event: "Stop", Timestamp: at.Add(time.Second), Payload: stop},
	}
}
