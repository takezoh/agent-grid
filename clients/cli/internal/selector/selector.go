// Package selector turns a declared selector and an optional server qualifier
// into one outcome.
//
// It is a pure function over an already-observed fleet. That is not an
// implementation detail: FR-020 requires every failure path to leave every
// session untouched, and the way to guarantee that is for resolution to have no
// capability to touch one. Nothing here opens a socket, sends a request, or
// changes a terminal.
package selector

import (
	"strings"

	"github.com/takezoh/agent-grid/clients/cli/internal/fleet"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Kind is a declared selector kind. There are two, and a selector resolves only
// under the kind it declared — never by falling back
// (adr-20260814-cli-selector-kinds-id-and-prefix).
//
// There is no name kind. Measurement shows the public session payload carries
// no stable, unique, operator-assigned name: the card title is server-owned and
// changes as the work changes, and the driver name is shared by every session
// of that driver. A selector keyed on either would resolve differently between
// two runs with no user action in between.
type Kind int

const (
	// KindID matches the full session identifier.
	KindID Kind = iota
	// KindPrefix matches any session whose identifier starts with the value.
	KindPrefix
)

// Query is one resolution request.
type Query struct {
	Kind Kind
	// Value is the selector text, interpreted only under Kind.
	Value string
	// Server is the optional qualifier, supplied independently of Value. It
	// accepts both the operator's display name and the derived stable server
	// identifier, because those are the two ways the listing names a server and
	// automation should be able to feed either one back.
	Server string
}

// OutcomeKind is what resolution concluded. Each maps to exactly one failure
// class, which is what lets automation branch on the exit code.
type OutcomeKind int

const (
	// Resolved is exactly one usable match.
	Resolved OutcomeKind = iota
	// NotFound is zero matches, with every scoped server readable — so the
	// absence is a fact rather than a gap in what could be seen.
	NotFound
	// Ambiguous is more than one usable match. Nothing is selected.
	Ambiguous
	// Unreachable is zero matches with at least one scoped server that could
	// not be read. The session may well exist; we could not look.
	Unreachable
	// QualifierUnknown is a server qualifier matching no configured entry.
	QualifierUnknown
	// QualifierUnusable is a server qualifier whose every match is
	// configuration-invalid. It outranks Ambiguous: offering unchoosable
	// candidates as an ambiguity would ask the operator to pick between servers
	// none of which can be connected to.
	QualifierUnusable
	// QualifierAmbiguous is a server qualifier matching more than one usable
	// entry.
	QualifierAmbiguous
)

// Candidate is one distinguishable match. Every field is here so a candidate
// list can be read without going back to the listing, and so each line names a
// value that uniquely specifies the next invocation's target.
type Candidate struct {
	Ref gateway.SessionRef
	Cwd string
}

// Outcome is the resolution result. It is a typed value rather than an
// identifier plus an error so that every caller has to handle each kind
// explicitly, and so a failure carries the evidence the user needs.
type Outcome struct {
	Kind OutcomeKind
	// Ref is set only when Kind is Resolved.
	Ref gateway.SessionRef
	// Member is the owning fleet member, set only when Kind is Resolved.
	Member fleet.Member
	// Candidates is set when Kind is Ambiguous.
	Candidates []Candidate
	// Entries is set when Kind is QualifierUnusable or QualifierAmbiguous: the
	// configured entries the qualifier matched, each with the rule it violates
	// when it has one.
	Entries []fleet.Member
	// Value echoes what was asked for, so a diagnostic can name it.
	Value string
	// Server echoes the qualifier.
	Server string
}

// Resolve is the whole of selector resolution.
func Resolve(snapshot fleet.Snapshot, query Query) Outcome {
	outcome := Outcome{Value: query.Value, Server: query.Server}

	scope, qualifierOutcome := scopeFor(snapshot, query.Server)
	if qualifierOutcome != nil {
		qualifierOutcome.Value = query.Value
		qualifierOutcome.Server = query.Server
		return *qualifierOutcome
	}

	var matches []struct {
		member  fleet.Member
		session gateway.SessionInfo
	}
	for _, member := range scope {
		if member.Observation.Kind != fleet.KindConnected {
			continue
		}
		for _, session := range member.Sessions {
			if matchesKind(query, session.ID) {
				matches = append(matches, struct {
					member  fleet.Member
					session gateway.SessionInfo
				}{member, session})
			}
		}
	}

	switch len(matches) {
	case 1:
		outcome.Kind = Resolved
		outcome.Member = matches[0].member
		outcome.Ref = gateway.NewSessionRef(matches[0].member.Server, matches[0].session)
		return outcome
	case 0:
		// A server that could not be read might hold the session. Reporting
		// not-found here would tell the operator the session does not exist,
		// which is a claim this observation cannot support.
		if scopeHasUnreachable(scope) {
			outcome.Kind = Unreachable
			return outcome
		}
		outcome.Kind = NotFound
		return outcome
	default:
		outcome.Kind = Ambiguous
		for _, match := range matches {
			outcome.Candidates = append(outcome.Candidates, Candidate{
				Ref: gateway.NewSessionRef(match.member.Server, match.session),
				Cwd: match.session.Project,
			})
		}
		return outcome
	}
}

// matchesKind applies exactly the declared kind.
//
// The counterexample this rejects is a resolver that "helpfully" tries the
// other kind when the declared one finds nothing, or finds one. A string can be
// one session's full identifier and simultaneously a prefix of another's; under
// fallback, which session that string names depends on what else exists at the
// time.
func matchesKind(query Query, sessionID string) bool {
	switch query.Kind {
	case KindID:
		return sessionID == query.Value
	case KindPrefix:
		return query.Value != "" && strings.HasPrefix(sessionID, query.Value)
	default:
		return false
	}
}

// scopeFor narrows the fleet to the qualified server, or returns the qualifier
// outcome that stops resolution.
//
// The qualifier is matched over every configured entry, not only the usable
// ones. An entry that is configuration-invalid still answers to its name, and
// has to: an operator who typed a name that matches only broken entries needs
// to be told which rule each one breaks, not that no such server is configured.
func scopeFor(snapshot fleet.Snapshot, qualifier string) ([]fleet.Member, *Outcome) {
	if qualifier == "" {
		return snapshot.Members, nil
	}
	var matched []fleet.Member
	for _, member := range snapshot.Members {
		if member.DisplayName == qualifier || member.ServerID == qualifier {
			matched = append(matched, member)
		}
	}
	if len(matched) == 0 {
		return nil, &Outcome{Kind: QualifierUnknown}
	}
	var usable []fleet.Member
	for _, member := range matched {
		if member.Server.Admission() == clientconfig.AdmissionUsable {
			usable = append(usable, member)
		}
	}
	switch len(usable) {
	case 0:
		// Unusable outranks ambiguous. Every match is a server that can never
		// be connected to, so presenting them as candidates would be offering a
		// choice with no valid options.
		return nil, &Outcome{Kind: QualifierUnusable, Entries: matched}
	case 1:
		return usable, nil
	default:
		return nil, &Outcome{Kind: QualifierAmbiguous, Entries: usable}
	}
}

func scopeHasUnreachable(scope []fleet.Member) bool {
	for _, member := range scope {
		switch member.Observation.Kind {
		case fleet.KindUnavailable, fleet.KindIncompatibleProtocol:
			return true
		}
	}
	return false
}
