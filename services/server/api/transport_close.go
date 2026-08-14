package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-grid/services/host/proto"
)

// transport_close.go owns the single answer this gateway gives to "why did the
// connection end". Before it, the gateway had one close code, four ad-hoc
// reason strings no schema modelled, and most departures unwound into an
// abrupt close carrying nothing at all — so a draining server, a rejected
// ticket and a pulled cable were mutually indistinguishable to every client.
//
// The vocabulary is published at protocol/transport-close.schema.json and is
// closed: TestTransportCloseReasonVocabularyIsRegisteredAndExhaustive fails
// when this file and the schema disagree, or when a close site bypasses the
// helpers below (adr-20260814-transport-close-reason-taxonomy).

// CloseReason names why the server ended a client connection. The same
// vocabulary spans both sides of the upgrade: after it, in the close frame's
// reason slot; before it — where no close frame exists — in the JSON
// rejection body's code field.
type CloseReason string

const (
	// CloseGoingAway is a normal server-side end of this connection: the
	// process is draining, or the request context was cancelled.
	CloseGoingAway CloseReason = "going_away"
	// CloseBackendUnavailable is the session daemon behind the gateway being
	// gone or not yet able to serve. Retrying can succeed.
	CloseBackendUnavailable CloseReason = "backend_unavailable"
	// CloseUnauthenticated is a missing, unknown, already-used or expired
	// credential. Retrying the same credential never succeeds.
	CloseUnauthenticated CloseReason = "unauthenticated"
	// CloseIncompatibleProtocol is a wire version that is not byte-equal to
	// the compiled one. Retrying never succeeds without a matching build.
	CloseIncompatibleProtocol CloseReason = "incompatible_protocol"
	// CloseCapacity is a refusal to admit more work right now. Retrying later
	// can succeed.
	CloseCapacity CloseReason = "capacity"
	// CloseInvalidRequest is the client asking for something the server cannot
	// act on — a malformed opening frame, or a target that is not attachable.
	CloseInvalidRequest CloseReason = "invalid_request"
	// CloseInternalError is a server-side failure the client cannot act on.
	CloseInternalError CloseReason = "internal_error"
)

// PublishedCloseReasons is the closed vocabulary in schema order. A departure
// that carries nothing is deliberately absent: by construction nothing
// arrives, so it cannot be a server value, and naming it here would invite a
// server to send it. That case is the client's to classify, as an
// unattributed loss rather than as one of these.
var PublishedCloseReasons = []CloseReason{
	CloseGoingAway,
	CloseBackendUnavailable,
	CloseUnauthenticated,
	CloseIncompatibleProtocol,
	CloseCapacity,
	CloseInvalidRequest,
	CloseInternalError,
}

// closeWriteGrace bounds the two frames a departure writes. The context that
// carried the connection is usually already cancelled by the time we depart —
// that is precisely the graceful-shutdown case — so the departure needs a
// context of its own or it would write nothing exactly when it matters.
const closeWriteGrace = time.Second

// errClientProtocol marks a failure the client caused by sending something the
// opening handshake does not accept. It separates "you sent nonsense"
// (invalid_request) from "your socket died mid-read" (an unattributed loss),
// which the raw read error alone cannot.
var errClientProtocol = errors.New("server/api: client protocol violation")

// frameWriter writes one text frame. The lifecycle socket is written from more
// than one goroutine and serialises through a mutex; the per-session socket is
// not. Departures happen on both, so they take the writer rather than the
// connection.
type frameWriter func(ctx context.Context, frame []byte) error

func plainFrameWriter(c *websocket.Conn) frameWriter {
	return func(ctx context.Context, frame []byte) error {
		return c.Write(ctx, websocket.MessageText, frame)
	}
}

func lockedFrameWriter(c *websocket.Conn, mu *sync.Mutex) frameWriter {
	return func(ctx context.Context, frame []byte) error {
		return writeLifecycleFrameLocked(ctx, c, mu, frame)
	}
}

// closeWith ends a client socket with exactly one published reason, in the
// two-step shape adr-20260624-0011 established: a control frame naming the
// reason while the socket is still writable, then the close frame carrying the
// same reason.
//
// legacy is the pre-taxonomy `data` payload. It is passed through verbatim
// where one existed, so the shipped SPA — which matches `data` string literals
// and cannot read `reason` yet — keeps behaving as it does today. Adopting
// `reason` is each client's own change.
//
// Both writes are best-effort. A socket that is already gone silently produces
// the unattributed case, which is the defined outcome for a departure that
// carries nothing rather than an omission.
func closeWith(c *websocket.Conn, write frameWriter, reason CloseReason, legacy string) {
	ctx, cancel := context.WithTimeout(context.Background(), closeWriteGrace)
	defer cancel()
	if write != nil {
		_ = write(ctx, departureControlFrame(reason, legacy))
	}
	// The close code stays StatusGoingAway for every reason. The taxonomy
	// lives in the reason slot so one vocabulary also covers the pre-upgrade
	// rejections, where no close code exists at all.
	_ = c.Close(websocket.StatusGoingAway, string(reason))
}

// departureReason maps a daemon-side failure onto the published vocabulary.
// Every branch is a statement about what the client should do next, which is
// the only thing the value is for.
func departureReason(err error) CloseReason {
	switch {
	case err == nil:
		return CloseGoingAway
	case errors.Is(err, errDaemonGone), errors.Is(err, ErrDaemonUnavailable):
		return CloseBackendUnavailable
	case errors.Is(err, errClientProtocol):
		return CloseInvalidRequest
	}
	var eb *proto.ErrorBody
	if errors.As(err, &eb) {
		switch eb.Code {
		case proto.ErrResourceExhausted:
			return CloseCapacity
		case proto.ErrNotFound, proto.ErrInvalidArgument, proto.ErrUnsupported, proto.ErrSessionStopped:
			return CloseInvalidRequest
		case proto.ErrFrameNotReady:
			// The surface is not ready yet — the backend cannot serve this
			// connection now, but retrying can succeed.
			return CloseBackendUnavailable
		case proto.ErrAlreadyExists, proto.ErrResolvedByOther, proto.ErrInternal, proto.ErrUnknown:
			// A server-side failure, or a race the client did not cause and
			// cannot resolve by changing its request.
			return CloseInternalError
		}
		return CloseInternalError
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return CloseGoingAway
	}
	return CloseInternalError
}

// legacyDepartureData preserves the pre-taxonomy `data` payload of a subscribe
// refusal, which carried the proto error code. Nothing in a shipped client
// reads it; it is kept so this change is additive on the wire rather than a
// silent removal.
func legacyDepartureData(err error) string {
	var eb *proto.ErrorBody
	if errors.As(err, &eb) {
		return string(eb.Code)
	}
	return ""
}

// legacyDaemonGoneData is the `data` literal the shipped SPA matches to raise
// its disconnected banner (clients/ui/src/socket/connection.ts). It rides
// alongside the typed reason until that client adopts `reason`.
const legacyDaemonGoneData = "daemon-disconnected"

// transportRejection is the pre-upgrade counterpart of a close frame. A
// rejection that happens before websocket.Accept cannot carry a close reason,
// so it carries the same vocabulary in its body — which is what makes a
// rejected credential distinguishable from a dropped connection, where both
// are otherwise just a failed handshake.
type transportRejection struct {
	Code    CloseReason `json:"code"`
	Message string      `json:"message"`
}

func writeTransportRejection(w http.ResponseWriter, status int, reason CloseReason, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(transportRejection{Code: reason, Message: message})
}
