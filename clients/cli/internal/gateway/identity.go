package gateway

import "github.com/takezoh/agent-grid/clients/goclient/clientconfig"

// identity.go is the only constructor of a server-qualified session identity.
//
// The reason is not tidiness. A CLI that talks to several servers needs to tell
// two identically-named sessions apart, and the obvious way to do that — glue
// the two identifiers into one string — produces a value that looks like a
// session identifier, is accepted nowhere, and eventually gets sent to a server
// that has never heard of it. So the compound exists only as a struct with
// unexported fields, built here, and the bare upstream identifier is the only
// thing that ever reaches the wire.

// SessionRef is a session together with the server that owns it.
//
// The fields are unexported so no other package can assemble one out of parts
// it happens to have; SessionID() is the only way to get the value that goes on
// the wire, and there is no method that returns the two glued together.
type SessionRef struct {
	serverID    string
	serverName  string
	sessionID   string
	displayName string
}

// NewSessionRef is the single constructor.
func NewSessionRef(server clientconfig.Server, session SessionInfo) SessionRef {
	return SessionRef{
		serverID:    server.ID,
		serverName:  server.DisplayName,
		sessionID:   session.ID,
		displayName: session.View.Card.Title,
	}
}

// ServerID is the derived stable server identifier: `srv_` plus the digest of
// the canonical gateway URL. Every conforming client derives the same value for
// the same server, which is what makes it usable as a qualifier in automation.
func (ref SessionRef) ServerID() string { return ref.serverID }

// ServerName is the operator-configured display name, as written.
func (ref SessionRef) ServerName() string { return ref.serverName }

// SessionID is the server's own identifier for the session. This is the only
// value that ever crosses the wire.
func (ref SessionRef) SessionID() string { return ref.sessionID }

// DisplayName is the server-provided card title, verbatim. There is no
// client-side fallback, derivation or truncation: an empty title is presented
// as empty, because inventing a name here would disagree with what every
// graphical client shows for the same session.
func (ref SessionRef) DisplayName() string { return ref.displayName }

// ServerLabel names the server for a human reader.
//
// The operator's display name comes first because that is what they wrote and
// what they will type back as a qualifier. The stable identifier is appended
// only when the caller says the name is ambiguous — which the naming contract
// makes impossible among usable entries, but which a fleet containing a
// configuration-invalid duplicate can still exhibit.
//
// This lives here rather than in the renderer for the same reason the
// constructor does: it is the only other place two identity values are joined
// into one string, and keeping both in one file is what lets the ownership gate
// forbid the pattern everywhere else without flagging legitimate presentation.
func (ref SessionRef) ServerLabel(ambiguous bool) string {
	if ambiguous {
		return ref.serverName + " (" + ref.serverID + ")"
	}
	return ref.serverName
}

// IsZero reports whether the reference names nothing.
func (ref SessionRef) IsZero() bool { return ref.serverID == "" && ref.sessionID == "" }
