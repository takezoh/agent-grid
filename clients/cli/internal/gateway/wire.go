package gateway

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

// wire.go is the only place in this module that turns bytes into frames and
// frames into bytes. Two properties depend on that being literally true rather
// than a convention, and both have gates: every outbound frame is built here,
// and every server-qualified identity is built in identity.go.
//
// Field names mirror the Phase 0 captures under testdata/ exactly. There is no
// convenience framing and no generated SDK: the shapes are small, and a
// hand-written codec against the captured bytes is what keeps a server-side
// rename a failing test rather than a silent zero value.

// SessionInfo is the session payload the server publishes. Its shape is pinned
// by protocol/fixtures/session-info-golden.json, which the server's own
// serializer reproduces byte for byte.
//
// Only the fields this client reads are declared. Unknown fields decode away,
// which is deliberate: a server that grows a field must not break a client that
// does not know about it yet.
type SessionInfo struct {
	ID            string        `json:"id"`
	Project       string        `json:"project"`
	Workspace     string        `json:"workspace,omitempty"`
	Command       string        `json:"command"`
	RootDriver    string        `json:"root_driver,omitempty"`
	CreatedAt     string        `json:"created_at"`
	Status        SessionStatus `json:"status"`
	View          View          `json:"view"`
	HeadFrameID   string        `json:"head_frame_id,omitempty"`
	WorkspaceRoot string        `json:"workspace_root,omitempty"`
}

// SessionStatus is the composite, driver-neutral status. The CLI never sees a
// driver's internal lifecycle observation.
type SessionStatus struct {
	Lifecycle  string        `json:"lifecycle"`
	HumanInput bool          `json:"human_input"`
	Failure    string        `json:"failure"`
	ChangedAt  string        `json:"changed_at,omitempty"`
	Signal     *StatusSignal `json:"signal,omitempty"`
}

// StatusSignal is a transient attention signal, independent of HumanInput.
type StatusSignal struct {
	Kind     string `json:"kind"`
	Epoch    string `json:"epoch"`
	Sequence uint64 `json:"sequence"`
}

// View is the driver-produced presentation payload.
type View struct {
	Card           Card   `json:"card"`
	DisplayName    string `json:"display_name,omitempty"`
	StatusLine     string `json:"status_line,omitempty"`
	LastUserPrompt string `json:"last_user_prompt,omitempty"`
	Model          string `json:"model,omitempty"`
	Effort         string `json:"effort,omitempty"`
}

// Card carries the session's title. Title is the value the CLI prints as the
// session's display name, verbatim — not View.DisplayName, which is the driver
// name and is shared by every session of that driver.
type Card struct {
	Title string `json:"title,omitempty"`
}

// Correlation is the browser-visible lifecycle correlation. ClientRevision is
// the lease revision: it must advance monotonically for the daemon to admit a
// newer desired publication over an older one.
type Correlation struct {
	ClientInstanceID     string `json:"clientInstanceID"`
	ConnectionGeneration uint64 `json:"connectionGeneration"`
	ClientRevision       uint64 `json:"clientRevision"`
}

// Lifecycle outcome statuses. accepted and waiting are deliberately
// non-terminal: treating either as an established subscription is the mistake
// ver-attach-never-reports-accepted-as-established rejects.
const (
	OutcomeAccepted   = "accepted"
	OutcomeWaiting    = "waiting"
	OutcomeApplied    = "applied"
	OutcomeRejected   = "rejected"
	OutcomeSuperseded = "superseded"
	OutcomeReleased   = "released"
	OutcomeDegraded   = "degraded"
	OutcomeNoOutput   = "no_output"
)

// IsTerminalOutcome reports whether an outcome settles the publication it
// answers. It is the whole of the accepted-is-not-established rule.
func IsTerminalOutcome(status string) bool {
	switch status {
	case OutcomeAccepted, OutcomeWaiting:
		return false
	default:
		return true
	}
}

// IsEstablishedOutcome reports whether an outcome means the surface
// subscription is live and output will follow.
func IsEstablishedOutcome(status string) bool {
	return status == OutcomeApplied
}

// Frame is one decoded server→client message.
type Frame interface{ frame() }

// HelloFrame is the first frame of a lifecycle socket.
type HelloFrame struct {
	Sessions         []SessionInfo
	ProtocolVersion  string
	ClientInstanceID string
}

// ViewUpdateFrame is a later session-list snapshot. An ended session arrives
// here, retained with a terminal lifecycle state, rather than by vanishing.
type ViewUpdateFrame struct {
	Sessions []SessionInfo
}

// HeartbeatFrame proves a quiet socket is live. It changes nothing.
type HeartbeatFrame struct{}

// OutputFrame is surface output for one session. Data is the decoded raw bytes,
// never a Go string: the wire payload stays base64 precisely so that non-UTF-8
// terminal output survives, and decoding it into a string would put every such
// byte back through the U+FFFD replacement this avoids.
type OutputFrame struct {
	SessionID string
	TimeSec   float64
	Data      []byte
}

// ControlFrame is a departure. Reason is the published cause, or
// ReasonUnattributed when the server said nothing.
type ControlFrame struct {
	Reason    CloseReason
	Data      string
	SessionID string
}

// LifecycleOutcomeFrame is the daemon's authoritative answer to a desired
// publication.
type LifecycleOutcomeFrame struct {
	Correlation Correlation
	Status      string
	Reason      string
}

// ResponseFrame answers a request envelope. OK distinguishes the two shapes the
// server uses: {"k":"r"} for success and {"k":"e"} for a refusal.
type ResponseFrame struct {
	RequestID string
	OK        bool
	Code      string
	Message   string
}

// UnknownFrame is a frame this build does not model. It is a value rather than
// an error so a server that grows a frame kind does not break this client.
type UnknownFrame struct {
	Kind string
}

func (HelloFrame) frame()            {}
func (ViewUpdateFrame) frame()       {}
func (HeartbeatFrame) frame()        {}
func (OutputFrame) frame()           {}
func (ControlFrame) frame()          {}
func (LifecycleOutcomeFrame) frame() {}
func (ResponseFrame) frame()         {}
func (UnknownFrame) frame()          {}

// ErrMalformedFrame is a frame that is not JSON at all, or an output array that
// does not have the shape the wire declares.
var ErrMalformedFrame = errors.New("malformed frame")

// DecodeFrame decodes one server→client text frame.
//
// The wire has two shapes on one socket: an asciicast-style array for surface
// output and a JSON object for everything else. The first non-space byte
// decides, which is exactly how the SPA's codec reads it.
func DecodeFrame(raw []byte) (Frame, error) {
	for _, character := range raw {
		switch character {
		case ' ', '\t', '\n', '\r':
			continue
		case '[':
			return decodeOutputFrame(raw)
		default:
			return decodeObjectFrame(raw)
		}
	}
	return nil, ErrMalformedFrame
}

// outputArray is [timeSec, "o", dataBase64, sessionId].
func decodeOutputFrame(raw []byte) (Frame, error) {
	var array []json.RawMessage
	if json.Unmarshal(raw, &array) != nil || len(array) < 4 {
		return nil, ErrMalformedFrame
	}
	var timeSec float64
	var kind, payload, sessionID string
	if json.Unmarshal(array[0], &timeSec) != nil ||
		json.Unmarshal(array[1], &kind) != nil ||
		json.Unmarshal(array[2], &payload) != nil ||
		json.Unmarshal(array[3], &sessionID) != nil {
		return nil, ErrMalformedFrame
	}
	if kind != "o" {
		return UnknownFrame{Kind: kind}, nil
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, ErrMalformedFrame
	}
	return OutputFrame{SessionID: sessionID, TimeSec: timeSec, Data: data}, nil
}

// objectFrame is the union of every JSON-object frame's fields. Decoding once
// into a union and then dispatching keeps the field names in one place.
type objectFrame struct {
	Kind             string        `json:"k"`
	Sessions         []SessionInfo `json:"sessions"`
	ProtocolVersion  string        `json:"protocolVersion"`
	ClientInstanceID string        `json:"clientInstanceId"`
	Reason           string        `json:"reason"`
	Data             string        `json:"data"`
	SessionID        string        `json:"sessionId"`
	RequestID        string        `json:"reqId"`
	Code             string        `json:"code"`
	Message          string        `json:"message"`
	Correlation      Correlation   `json:"correlation"`
	Status           string        `json:"status"`
}

func decodeObjectFrame(raw []byte) (Frame, error) {
	var object objectFrame
	if json.Unmarshal(raw, &object) != nil {
		return nil, ErrMalformedFrame
	}
	switch object.Kind {
	case "h":
		return HelloFrame{
			Sessions:         object.Sessions,
			ProtocolVersion:  object.ProtocolVersion,
			ClientInstanceID: object.ClientInstanceID,
		}, nil
	case "v":
		return ViewUpdateFrame{Sessions: object.Sessions}, nil
	case "p":
		return HeartbeatFrame{}, nil
	case "c":
		reason := CloseReason(object.Reason)
		if !reason.IsPublished() {
			// A control frame that names no published reason tells us the
			// connection is ending and nothing more. Inventing a cause here is
			// what the taxonomy exists to stop.
			reason = ReasonUnattributed
		}
		return ControlFrame{Reason: reason, Data: object.Data, SessionID: object.SessionID}, nil
	case "lo":
		return LifecycleOutcomeFrame{
			Correlation: object.Correlation, Status: object.Status, Reason: object.Reason,
		}, nil
	case "r":
		return ResponseFrame{RequestID: object.RequestID, OK: true}, nil
	case "e":
		return ResponseFrame{
			RequestID: object.RequestID, OK: false,
			Code: object.Code, Message: object.Message,
		}, nil
	default:
		return UnknownFrame{Kind: object.Kind}, nil
	}
}

// --- outbound ---------------------------------------------------------------
//
// Every outbound frame is built by one of the three functions below.
// TestOutboundFrameSerializationHasExactlyOneSite fails when a `k` field is
// written anywhere else in the module.

// desiredFrame is the complete desired publication: the lifecycle-v2 shape that
// replaced imperative subscribe/unsubscribe.
type desiredFrame struct {
	Kind        string      `json:"k"`
	RequestID   string      `json:"reqId,omitempty"`
	Correlation Correlation `json:"correlation"`
	SessionID   string      `json:"sessionId"`
	Cols        int         `json:"cols"`
	Rows        int         `json:"rows"`
	Desired     bool        `json:"desired"`
}

type inputFrame struct {
	Kind      string `json:"k"`
	SessionID string `json:"sessionId"`
	Data      string `json:"d"`
}

type resizeFrame struct {
	Kind      string `json:"k"`
	SessionID string `json:"sessionId"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

// EncodeLifecycleDesired builds the subscribe/unsubscribe publication.
//
// sessionID is always the server's bare identifier. A server-qualified identity
// exists only inside this client, to tell two servers' sessions apart; putting
// one on the wire would name something the server has never heard of.
func EncodeLifecycleDesired(
	correlation Correlation, sessionID string, cols, rows int, desired bool, requestID string,
) []byte {
	encoded, _ := json.Marshal(desiredFrame{
		Kind: "ld", RequestID: requestID, Correlation: correlation,
		SessionID: sessionID, Cols: cols, Rows: rows, Desired: desired,
	})
	return encoded
}

// EncodeInput builds one surface input frame.
//
// The payload travels as a JSON string, so a partial multi-byte sequence would
// be silently replaced by U+FFFD and reach the remote PTY as a different byte
// sequence than the one typed. Callers must hand only complete sequences here;
// SplitCompleteUTF8 is what produces them.
func EncodeInput(sessionID string, data []byte) []byte {
	encoded, _ := json.Marshal(inputFrame{Kind: "i", SessionID: sessionID, Data: string(data)})
	return encoded
}

// EncodeResize builds one geometry frame.
func EncodeResize(sessionID string, cols, rows int) []byte {
	encoded, _ := json.Marshal(resizeFrame{Kind: "r", SessionID: sessionID, Cols: cols, Rows: rows})
	return encoded
}

// SplitCompleteUTF8 divides a read buffer into the prefix that ends on a rune
// boundary and the trailing bytes of an incomplete sequence.
//
// Only a *valid prefix* of a multi-byte sequence is held back. A byte that can
// never begin a valid sequence is passed through instead of being held: holding
// it would stall the input stream forever waiting for a continuation that is
// never coming, which is a worse failure than the replacement character.
func SplitCompleteUTF8(buffer []byte) (complete, pending []byte) {
	if len(buffer) == 0 {
		return buffer, nil
	}
	// A UTF-8 sequence is at most 4 bytes, so at most the last 3 can be an
	// incomplete tail.
	start := max(len(buffer)-3, 0)
	for index := len(buffer) - 1; index >= start; index-- {
		if utf8.RuneStart(buffer[index]) {
			if utf8.Valid(buffer[index:]) {
				// The tail is already a complete rune (or runes); nothing is held.
				return buffer, nil
			}
			if isIncompleteUTF8Prefix(buffer[index:]) {
				return buffer[:index], buffer[index:]
			}
			// The tail starts a sequence that can never become valid. Pass it
			// through rather than waiting forever.
			return buffer, nil
		}
	}
	return buffer, nil
}

// isIncompleteUTF8Prefix reports whether tail is the beginning of a valid
// multi-byte sequence that has not finished arriving.
func isIncompleteUTF8Prefix(tail []byte) bool {
	if len(tail) == 0 || len(tail) > 3 {
		return false
	}
	lead := tail[0]
	var want int
	switch {
	case lead&0xE0 == 0xC0:
		want = 2
	case lead&0xF0 == 0xE0:
		want = 3
	case lead&0xF8 == 0xF0:
		want = 4
	default:
		return false
	}
	if len(tail) >= want {
		return false
	}
	for _, continuation := range tail[1:] {
		if continuation&0xC0 != 0x80 {
			return false
		}
	}
	return true
}
