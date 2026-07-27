package proto

import (
	"encoding/json"
	"time"
)

// Command is the closed sum type of every IPC request the daemon
// accepts. Each impl carries the typed args + a Name() string that
// matches the wire "cmd" field.
type Command interface {
	isCommand()
	CommandName() string
}

// Command name constants — used by both Encode and Decode so a typo
// breaks both ends symmetrically.
const (
	CmdNameSubscribe          = "subscribe"
	CmdNameUnsubscribe        = "unsubscribe"
	CmdNameEvent              = "event"
	CmdNameSubsystem          = "subsystem-event"
	CmdNameFrameList          = "frame-messaging.list"
	CmdNameFrameRead          = "frame-messaging.read"
	CmdNameFrameSend          = "frame-messaging.send"
	CmdNameFrameReply         = "frame-messaging.reply"
	CmdNameFrameListByThread  = "frame-messaging.list_by_thread"
	CmdNameFrameReadByThread  = "frame-messaging.read_by_thread"
	CmdNameFrameSendByThread  = "frame-messaging.send_by_thread"
	CmdNameFrameReplyByThread = "frame-messaging.reply_by_thread"

	// surface.* — frame I/O operations
	CmdNameSurfaceReadText = "surface.read_text"
	CmdNameSurfaceSendText = "surface.send_text"
	CmdNameSurfaceSendKey  = "surface.send_key"

	// driver.* — driver registry queries
	CmdNameDriverList = "driver.list"

	// hook-event — container endpoint only. Carries a driver hook notification
	// with a bearer token that resolves to the spawning frame. Not accepted on
	// the host endpoint.
	CmdNameHookEvent = "hook-event"
)

type CmdSubscribe struct {
	Filters []string `json:"filters,omitempty"`
}

func (CmdSubscribe) isCommand()          {}
func (CmdSubscribe) CommandName() string { return CmdNameSubscribe }

type CmdUnsubscribe struct{}

func (CmdUnsubscribe) isCommand()          {}
func (CmdUnsubscribe) CommandName() string { return CmdNameUnsubscribe }

// CmdEvent is the generic event envelope sent by the `server event` CLI.
type CmdEvent struct {
	Event     string          `json:"event"`
	Timestamp time.Time       `json:"timestamp"`
	SenderID  string          `json:"sender_id"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func (CmdEvent) isCommand()          {}
func (CmdEvent) CommandName() string { return CmdNameEvent }

type CmdSubsystemEvent struct {
	Token     string          `json:"token,omitempty"`
	FrameID   string          `json:"frame_id,omitempty"`
	Source    string          `json:"source"`
	Kind      string          `json:"kind"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func (CmdSubsystemEvent) isCommand()          {}
func (CmdSubsystemEvent) CommandName() string { return CmdNameSubsystem }

// CmdSurfaceReadText reads the trailing Lines of a session's head frame
// surface content. SessionID identifies the target session; Lines=0 uses the
// server default (30).
type CmdSurfaceReadText struct {
	SessionID string `json:"session_id"`
	Lines     int    `json:"lines,omitempty"`
}

func (CmdSurfaceReadText) isCommand()          {}
func (CmdSurfaceReadText) CommandName() string { return CmdNameSurfaceReadText }

// CmdSurfaceSendText sends Text followed by Enter to a session's head frame
// surface.
type CmdSurfaceSendText struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

func (CmdSurfaceSendText) isCommand()          {}
func (CmdSurfaceSendText) CommandName() string { return CmdNameSurfaceSendText }

// CmdSurfaceSendKey sends a named key (e.g. "Escape", "C-c") to a session's
// head frame surface without appending Enter.
type CmdSurfaceSendKey struct {
	SessionID string `json:"session_id"`
	Key       string `json:"key"`
}

func (CmdSurfaceSendKey) isCommand()          {}
func (CmdSurfaceSendKey) CommandName() string { return CmdNameSurfaceSendKey }

// CmdDriverList lists all registered driver names and display names.
type CmdDriverList struct{}

func (CmdDriverList) isCommand()          {}
func (CmdDriverList) CommandName() string { return CmdNameDriverList }

// CmdHookEvent is the container-only command that delivers a driver hook
// notification. Token authenticates the sender and resolves to the FrameID
// of the spawning frame.
type CmdHookEvent struct {
	Token     string          `json:"token"`
	Hook      string          `json:"hook"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func (CmdHookEvent) isCommand()          {}
func (CmdHookEvent) CommandName() string { return CmdNameHookEvent }

// CmdFrameList lists same-session claude/codex frames visible to the source
// frame identified by Token. Only the container endpoint accepts this command.
type CmdFrameList struct {
	Token string `json:"token"`
}

func (CmdFrameList) isCommand()          {}
func (CmdFrameList) CommandName() string { return CmdNameFrameList }

// CmdFrameRead reads same-session frame messages visible to the source frame.
// Optional PeerFrameID narrows the result set to a specific conversation peer.
type CmdFrameRead struct {
	Token       string `json:"token"`
	PeerFrameID string `json:"peer_frame_id,omitempty"`
}

func (CmdFrameRead) isCommand()          {}
func (CmdFrameRead) CommandName() string { return CmdNameFrameRead }

// CmdFrameSend stores an inbox message for a same-session target frame.
type CmdFrameSend struct {
	Token         string `json:"token"`
	TargetFrameID string `json:"target_frame_id"`
	Topic         string `json:"topic,omitempty"`
	Body          string `json:"body"`
	Priority      string `json:"priority,omitempty"`
}

func (CmdFrameSend) isCommand()          {}
func (CmdFrameSend) CommandName() string { return CmdNameFrameSend }

// CmdFrameReply appends a reply to an existing message.
type CmdFrameReply struct {
	Token       string `json:"token"`
	MessageID   string `json:"message_id"`
	Body        string `json:"body,omitempty"`
	FinalAnswer string `json:"final_answer,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	Confidence  string `json:"confidence,omitempty"`
}

func (CmdFrameReply) isCommand()          {}
func (CmdFrameReply) CommandName() string { return CmdNameFrameReply }

// CmdFrameListByThread lists same-session claude/codex frames visible to the
// frame bound to ThreadID. Accepted on the host IPC socket for managed helper
// processes such as the Codex app-server shim.
type CmdFrameListByThread struct {
	SessionID string `json:"session_id"`
	ThreadID  string `json:"thread_id"`
}

func (CmdFrameListByThread) isCommand()          {}
func (CmdFrameListByThread) CommandName() string { return CmdNameFrameListByThread }

// CmdFrameReadByThread reads frame messages visible to the frame bound to
// ThreadID. Optional PeerFrameID narrows the result set.
type CmdFrameReadByThread struct {
	SessionID   string `json:"session_id"`
	ThreadID    string `json:"thread_id"`
	PeerFrameID string `json:"peer_frame_id,omitempty"`
}

func (CmdFrameReadByThread) isCommand()          {}
func (CmdFrameReadByThread) CommandName() string { return CmdNameFrameReadByThread }

// CmdFrameSendByThread stores an inbox message for the frame bound to ThreadID.
type CmdFrameSendByThread struct {
	SessionID     string `json:"session_id"`
	ThreadID      string `json:"thread_id"`
	TargetFrameID string `json:"target_frame_id"`
	Topic         string `json:"topic,omitempty"`
	Body          string `json:"body"`
	Priority      string `json:"priority,omitempty"`
}

func (CmdFrameSendByThread) isCommand()          {}
func (CmdFrameSendByThread) CommandName() string { return CmdNameFrameSendByThread }

// CmdFrameReplyByThread appends a reply using the frame bound to ThreadID.
type CmdFrameReplyByThread struct {
	SessionID   string `json:"session_id"`
	ThreadID    string `json:"thread_id"`
	MessageID   string `json:"message_id"`
	Body        string `json:"body,omitempty"`
	FinalAnswer string `json:"final_answer,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	Confidence  string `json:"confidence,omitempty"`
}

func (CmdFrameReplyByThread) isCommand()          {}
func (CmdFrameReplyByThread) CommandName() string { return CmdNameFrameReplyByThread }
