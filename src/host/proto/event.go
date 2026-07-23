package proto

// ServerEvent is the closed sum type of broadcasts the daemon pushes
// to subscribed clients. Each impl carries the typed payload + a
// Name() string that matches the wire "name" field.
type ServerEvent interface {
	isEvent()
	EventName() string
}

const (
	EvtNameSessionsChanged   = "sessions-changed"
	EvtNameProjectSelected   = "project-selected"
	EvtNameLogLine           = "log-line"
	EvtNameSessionFileLine   = "session-file-line"
	EvtNameAgentNotification = "agent-notification"
	EvtNameActivityEvents    = "activity-events"
)

// EvtSessionsChanged carries the current session table. Sent on
// every state change that affects what clients should render.
type EvtSessionsChanged struct {
	Sessions []SessionInfo `json:"sessions"`
	Features []string      `json:"features,omitempty"`
}

func (EvtSessionsChanged) isEvent()          {}
func (EvtSessionsChanged) EventName() string { return EvtNameSessionsChanged }

// EvtProjectSelected fires when the user picks a project from the
// session list (preview-project IPC).
type EvtProjectSelected struct {
	Project string `json:"project"`
}

func (EvtProjectSelected) isEvent()          {}
func (EvtProjectSelected) EventName() string { return EvtNameProjectSelected }

// EvtLogLine pushes one new line of the global daemon log to event
// subscribers.
type EvtLogLine struct {
	Path string `json:"path"`
	Line string `json:"line"`
}

func (EvtLogLine) isEvent()          {}
func (EvtLogLine) EventName() string { return EvtNameLogLine }

// EvtSessionFileLine pushes one new line from a session's log/transcript
// file to event subscribers.
type EvtSessionFileLine struct {
	SessionID string `json:"session_id"`
	Kind      string `json:"kind"`
	Line      string `json:"line"`
}

func (EvtSessionFileLine) isEvent()          {}
func (EvtSessionFileLine) EventName() string { return EvtNameSessionFileLine }

// EvtAgentNotification is emitted when an OSC 9/99/777 notification
// escape is captured from an agent frame.
type EvtAgentNotification struct {
	SessionID string `json:"session_id"`
	Cmd       int    `json:"cmd"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
}

func (EvtAgentNotification) isEvent()          {}
func (EvtAgentNotification) EventName() string { return EvtNameAgentNotification }

// ActivityDrillDownWire is one classified tool call within a turn row.
type ActivityDrillDownWire struct {
	ToolUseID     string `json:"tool_use_id,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	FileEventKind string `json:"file_event_kind,omitempty"`
	TS            string `json:"ts,omitempty"`
}

// ActivityTurnSubRowWire aggregates nested sub-agent activity.
type ActivityTurnSubRowWire struct {
	TurnID string                  `json:"turn_id"`
	Path   string                  `json:"workspace_relative_path"`
	Count  int                     `json:"count"`
	Events []ActivityDrillDownWire `json:"events,omitempty"`
}

// ActivityEventWire is one turn_row or mid_turn_touch payload element.
type ActivityEventWire struct {
	Type          string                   `json:"type"`
	Sequence      uint64                   `json:"sequence"`
	SessionID     string                   `json:"session_id"`
	TurnID        string                   `json:"turn_id,omitempty"`
	Path          string                   `json:"workspace_relative_path,omitempty"`
	FileEventKind string                   `json:"file_event_kind,omitempty"`
	ToolUseID     string                   `json:"tool_use_id,omitempty"`
	ToolName      string                   `json:"tool_name,omitempty"`
	Actor         string                   `json:"actor,omitempty"`
	Count         int                      `json:"count,omitempty"`
	TurnFailure   bool                     `json:"turn_failure,omitempty"`
	TS            string                   `json:"ts,omitempty"`
	Events        []ActivityDrillDownWire  `json:"events,omitempty"`
	SubRows       []ActivityTurnSubRowWire `json:"sub_rows,omitempty"`
}

// EvtActivityEvents batches workspace activity events for one session.
type EvtActivityEvents struct {
	SessionID string              `json:"session_id"`
	Events    []ActivityEventWire `json:"events"`
}

func (EvtActivityEvents) isEvent()          {}
func (EvtActivityEvents) EventName() string { return EvtNameActivityEvents }
