package fakeclaude

import (
	"encoding/json"

	claudehookpayload "github.com/takezoh/agent-grid/platform/lib/claude/hookpayload"
)

// HookPayload is Claude's hook stdin payload schema shared with the
// production Claude driver and E2E tests.
type HookPayload = claudehookpayload.HookPayload

// Marshal renders p to the exact JSON shape Claude writes to a hook's stdin.
// Panics on encoding failure — the input is a plain-old struct, so the only
// route to failure is a caller bug in ToolInput.
func Marshal(p HookPayload) []byte {
	b, err := json.Marshal(p)
	if err != nil {
		panic("fakeclaude: marshal HookPayload: " + err.Error())
	}
	return b
}
