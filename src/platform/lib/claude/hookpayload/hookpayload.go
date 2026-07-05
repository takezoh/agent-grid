package hookpayload

import (
	"encoding/json"
	"strings"
)

// HookPayload is Claude's hook stdin payload schema shared by the production
// driver, fakeclaude, and the Claude hook E2E tests.
type HookPayload struct {
	SessionID      string `json:"session_id,omitempty"`
	HookEventName  string `json:"hook_event_name,omitempty"`
	Prompt         string `json:"prompt,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`

	NotificationType string         `json:"notification_type,omitempty"`
	ToolName         string         `json:"tool_name,omitempty"`
	ToolInput        map[string]any `json:"tool_input,omitempty"`
	Source           string         `json:"source,omitempty"`
	Model            string         `json:"model,omitempty"`
	Effort           *Effort        `json:"effort,omitempty"`

	ToolUseID      string `json:"tool_use_id,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	Error          string `json:"error,omitempty"`
	IsInterrupt    bool   `json:"is_interrupt,omitempty"`
}

// Effort is Claude's effort payload shape. Real payloads are observed as an
// object with a level field, but the decoder also accepts a plain string so
// transient schema differences do not silently drop metadata.
type Effort struct {
	Level string `json:"level,omitempty"`
}

func (e *Effort) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*e = Effort{}
		return nil
	}
	var level string
	if err := json.Unmarshal(data, &level); err == nil {
		e.Level = strings.TrimSpace(level)
		return nil
	}
	var payload struct {
		Level string `json:"level"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	e.Level = strings.TrimSpace(payload.Level)
	return nil
}

func (e *Effort) Value() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Level)
}
