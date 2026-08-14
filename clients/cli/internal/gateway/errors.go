package gateway

import (
	"errors"
	"fmt"
	"strings"
)

// errors.go turns everything that can go wrong on the wire into a small set of
// values the caller can branch on, and it is the single place where a transport
// failure becomes a human-readable reason. Redacting here rather than at each
// call site is what makes FR-027 checkable: there is one function to audit.

var (
	// ErrIncompatibleProtocol is a server whose wire version is not byte-equal
	// to this build's. It is terminal for that server: no further operational
	// request or socket frame is issued to it in this process.
	ErrIncompatibleProtocol = errors.New("incompatible protocol")
	// ErrUnauthenticated is a rejected credential or ticket.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrCapacity is a refusal to admit more work right now.
	ErrCapacity = errors.New("capacity")
	// ErrUnavailable is a server that could not be reached or answered.
	ErrUnavailable = errors.New("unavailable")
	// ErrInvalidRequest is a request the server will not act on.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrNotFound is a well-formed request for something that is not there.
	ErrNotFound = errors.New("not found")
	// ErrInternal is a server-side failure the caller cannot act on.
	ErrInternal = errors.New("internal error")
)

// TransportError carries the reason the plane published alongside the kind of
// failure it is. Reason is what the CLI presents and what it classifies on; it
// is never a sentence this client wrote. A departure that published nothing
// carries ReasonUnattributed, which is a statement that the cause is unknown
// rather than a guess at which cause it was.
type TransportError struct {
	Reason CloseReason
	Kind   error
	Detail string
}

func (transport *TransportError) Error() string {
	if transport.Detail == "" {
		return string(transport.Reason)
	}
	return string(transport.Reason) + ": " + transport.Detail
}

func (transport *TransportError) Unwrap() error { return transport.Kind }

// FromCloseReason maps a published reason onto the failure kind it implies.
// The mapping is total over PublishedCloseReasons and is the only place the two
// vocabularies meet, so adding a reason without deciding its kind is a compile
// error rather than a silent fall-through to "internal".
func FromCloseReason(reason CloseReason, detail string) *TransportError {
	var kind error
	switch reason {
	case ReasonUnauthenticated, ReasonIncompatibleProtocol:
		// Both are refusals that retrying unchanged never resolves.
		kind = ErrUnauthenticated
		if reason == ReasonIncompatibleProtocol {
			kind = ErrIncompatibleProtocol
		}
	case ReasonCapacity:
		kind = ErrCapacity
	case ReasonGoingAway, ReasonBackendUnavailable:
		kind = ErrUnavailable
	case ReasonInvalidRequest:
		kind = ErrInvalidRequest
	case ReasonInternalError:
		kind = ErrInternal
	case ReasonUnattributed:
		// The connection ended and said nothing. That is a transport loss of
		// unknown cause, which is exactly ErrUnavailable — retrying can work.
		kind = ErrUnavailable
	default:
		// A reason outside the published vocabulary. Treating it as one of the
		// known kinds would be a guess; treating it as unavailable at least
		// keeps the retry advice honest, and the reason string is carried
		// through verbatim so the operator sees what arrived.
		kind = ErrUnavailable
	}
	return &TransportError{Reason: reason, Kind: kind, Detail: detail}
}

// The daemon's error-code vocabulary, as it arrives in a `{"k":"e"}` response
// frame. It is a second closed vocabulary, distinct from the close reasons: a
// refusal on a socket that stays open cannot be a close reason, because nothing
// is closing.
const (
	CodeResourceExhausted = "resource_exhausted"
	CodeNotFound          = "not_found"
	CodeInvalidArgument   = "invalid_argument"
	CodeSessionStopped    = "session_stopped"
	CodeUnsupported       = "unsupported"
	CodeAlreadyExists     = "already_exists"
	CodeFrameNotReady     = "frame-not-ready"
	CodeInternal          = "internal"
	CodeUnknown           = "unknown"
)

// FromResponseCode maps a refusal on a live socket onto the same failure kinds
// a departure produces, so callers have one thing to classify on.
//
// The published reason it carries is the close reason the same condition would
// have produced had it ended the connection. That is what "derive the class
// from the reason the plane published" means when the plane refuses without
// disconnecting: the classification still comes from the server's word, not
// from a sentence this client wrote.
func FromResponseCode(code, message string) *TransportError {
	switch code {
	case CodeResourceExhausted:
		return &TransportError{Reason: ReasonCapacity, Kind: ErrCapacity, Detail: message}
	case CodeNotFound:
		return &TransportError{Reason: ReasonInvalidRequest, Kind: ErrNotFound, Detail: message}
	case CodeInvalidArgument, CodeUnsupported, CodeSessionStopped, CodeAlreadyExists:
		return &TransportError{Reason: ReasonInvalidRequest, Kind: ErrInvalidRequest, Detail: message}
	case CodeFrameNotReady:
		// The session exists but its surface is not ready. Retrying can work,
		// which makes it a backend availability problem rather than a bad
		// request.
		return &TransportError{
			Reason: ReasonBackendUnavailable, Kind: ErrUnavailable, Detail: message,
		}
	default:
		return &TransportError{Reason: ReasonInternalError, Kind: ErrInternal, Detail: message}
	}
}

// redact removes anything that could carry a credential from text that is about
// to be presented, logged, or returned as a reason.
//
// The realistic leak is not someone printing the token deliberately — it is
// net/url and net/http embedding the request URL in their error strings, and a
// URL that carries a freshly minted single-use ticket in its query. So the rule
// is: no URL survives into a diagnostic, and neither do the secret values
// themselves.
func redact(text string, secrets ...string) string {
	cleaned := text
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		cleaned = strings.ReplaceAll(cleaned, secret, "[redacted]")
	}
	return stripURLs(cleaned)
}

// stripURLs replaces every http(s)/ws(s) URL in text with a scheme-only
// placeholder. A URL is removed wholesale rather than having its query trimmed,
// because the parts that look safe — host, port, path — are not what this rule
// is protecting, and a partial rule invites a leak through a spelling nobody
// anticipated.
func stripURLs(text string) string {
	var builder strings.Builder
	remaining := text
	for {
		index, scheme := firstURLScheme(remaining)
		if index < 0 {
			builder.WriteString(remaining)
			return builder.String()
		}
		builder.WriteString(remaining[:index])
		builder.WriteString(scheme + "[redacted]")
		rest := remaining[index+len(scheme):]
		end := strings.IndexFunc(rest, func(r rune) bool {
			return r == ' ' || r == '"' || r == '\'' || r == '\n' || r == '\t'
		})
		if end < 0 {
			return builder.String()
		}
		remaining = rest[end:]
	}
}

func firstURLScheme(text string) (int, string) {
	best := -1
	bestScheme := ""
	for _, scheme := range []string{"https://", "http://", "wss://", "ws://"} {
		if index := strings.Index(text, scheme); index >= 0 && (best < 0 || index < best) {
			best, bestScheme = index, scheme
		}
	}
	return best, bestScheme
}

// wrapTransport turns a low-level failure into a redacted TransportError.
func wrapTransport(reason CloseReason, err error, secrets ...string) *TransportError {
	detail := ""
	if err != nil {
		detail = redact(err.Error(), secrets...)
	}
	return FromCloseReason(reason, detail)
}

// protocolMismatch is the terminal admission failure. The detail is a closed
// sentence built from two version tokens, never from server-supplied text.
func protocolMismatch(actual string) *TransportError {
	return &TransportError{
		Reason: ReasonIncompatibleProtocol,
		Kind:   ErrIncompatibleProtocol,
		Detail: fmt.Sprintf("client %s, server %s", ProtocolVersion, sanitizeVersionToken(actual)),
	}
}

// sanitizeVersionToken keeps an untrusted version string from becoming an
// arbitrary sentence in the CLI's output. The same closed token shape the web
// host uses.
func sanitizeVersionToken(actual string) string {
	if actual == "" {
		return "<missing>"
	}
	if !strings.HasPrefix(actual, "protocol-") {
		return "<invalid>"
	}
	for _, character := range actual[len("protocol-"):] {
		if character < '0' || character > '9' {
			return "<invalid>"
		}
	}
	return actual
}
