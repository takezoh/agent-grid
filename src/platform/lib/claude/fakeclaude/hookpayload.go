package fakeclaude

import "encoding/json"

// HookPayload is the JSON body Claude passes on a hook command's stdin. Fields
// mirror client/driver/claude_event.go:hookPayload so a real payload parsed
// there and a fake payload marshalled here roundtrip through the same struct.
//
// The driver's private hookPayload cannot be shared here: it lives in
// client/state which platform/* is forbidden to import. Duplication is
// intentional and pinned by TestE2E_HookPayloadSchema / TestFakeVsRealPayload.
type HookPayload struct {
	SessionID      string `json:"session_id,omitempty"`
	HookEventName  string `json:"hook_event_name,omitempty"`
	Prompt         string `json:"prompt,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`

	NotificationType string         `json:"notification_type,omitempty"`
	ToolName         string         `json:"tool_name,omitempty"`
	ToolInput        map[string]any `json:"tool_input,omitempty"`
	Source           string         `json:"source,omitempty"`

	ToolUseID      string `json:"tool_use_id,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	Error          string `json:"error,omitempty"`
	IsInterrupt    bool   `json:"is_interrupt,omitempty"`
}

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
