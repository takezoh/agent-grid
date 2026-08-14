package gateway

import "slices"

// closereason.go is this module's projection of
// protocol/transport-close.schema.json. TestCloseReasonVocabularyMatchesTheSchema
// pins it to that file, so the CLI cannot invent a reason the plane never sends
// or quietly stop handling one it does.

// CloseReason is why the server ended a connection. The same vocabulary spans
// both sides of the WebSocket upgrade: after it, in the departure control
// frame's reason slot and the close frame; before it, in the JSON rejection
// body's code field.
type CloseReason string

// The closed vocabulary, in the order protocol/transport-close.schema.json
// declares it.
const (
	ReasonGoingAway            CloseReason = "going_away"
	ReasonBackendUnavailable   CloseReason = "backend_unavailable"
	ReasonUnauthenticated      CloseReason = "unauthenticated"
	ReasonIncompatibleProtocol CloseReason = "incompatible_protocol"
	ReasonCapacity             CloseReason = "capacity"
	ReasonInvalidRequest       CloseReason = "invalid_request"
	ReasonInternalError        CloseReason = "internal_error"
)

// PublishedCloseReasons is the closed vocabulary in schema order.
//
// There is deliberately no member for "the departure carried nothing". By
// construction nothing arrives in that case, so it is not a server value; it is
// an unattributed loss the client classifies locally and must never dress up as
// one of these. ReasonUnattributed below is that local classification, and it is
// kept out of this list on purpose.
var PublishedCloseReasons = []CloseReason{
	ReasonGoingAway,
	ReasonBackendUnavailable,
	ReasonUnauthenticated,
	ReasonIncompatibleProtocol,
	ReasonCapacity,
	ReasonInvalidRequest,
	ReasonInternalError,
}

// ReasonUnattributed is the client-side value for a connection that ended
// without saying why. It is not a published reason and never travels on the
// wire; it exists so that "we do not know" is a value the CLI can hold and
// present, rather than a gap it fills with a guess.
const ReasonUnattributed CloseReason = "unattributed"

// IsPublished reports whether a reason came from the closed server vocabulary.
func (reason CloseReason) IsPublished() bool {
	return slices.Contains(PublishedCloseReasons, reason)
}
