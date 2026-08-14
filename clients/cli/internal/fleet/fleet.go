// Package fleet turns the configured server list into one observation per
// entry, plus the sessions of the entries that answered. It is what `ls` and
// selector resolution both branch on.
package fleet

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// The admission-and-reachability vocabulary. It is the one the web host and
// Workspace already publish, reused verbatim so the same server reads the same
// way in every client, plus one addition.
//
// KindConfigurationInvalid is that addition, and it belongs in this set rather
// than beside it for the same reason KindDisabled already does: neither is a
// reachability observation, both are reasons an entry was never contacted, and
// a client that has to look in two places to answer "what is this entry doing"
// will eventually look in only one.
//
// None of these words is a session lifecycle state. That separation is the
// whole point: "the server is unreachable" and "the session stopped" are
// different facts about different things, and collapsing them into one value
// space is what makes an operator restart a session because a network dropped.
const (
	// KindDisabled is a valid entry the operator turned off. Never contacted.
	KindDisabled = "disabled"
	// KindConfigurationInvalid is an entry that broke a contract rule, or that
	// collides with one that did. Never contacted, and therefore never able to
	// acquire a reachability observation at all.
	KindConfigurationInvalid = "configuration_invalid"
	// KindConnected is a server that answered with a session list.
	KindConnected = "connected"
	// KindUnavailable is a server that could not be reached or did not answer.
	KindUnavailable = "unavailable"
	// KindIncompatibleProtocol is a server speaking a different wire version.
	KindIncompatibleProtocol = "incompatible_protocol"
)

// The closed reason set, unchanged from the shared vocabulary.
const (
	ReasonConnectionTimeout = "connection_timeout"
	ReasonConnectionFailed  = "connection_failed"
	ReasonHTTPError         = "http_error"
	ReasonWebSocketFailed   = "websocket_failed"
	ReasonEmptyTicket       = "empty_ticket"
	ReasonProtocolMismatch  = "protocol_mismatch"
)

// probeTimeout bounds one server's whole observation: admission, ticket, dial
// and the wait for the first session list. It is the same three seconds the web
// host allows each phase, applied once because the CLI probes once and exits
// rather than following a server.
const probeTimeout = 5 * time.Second

// Observation is what was learned about one entry.
//
// Rule is populated only for KindConfigurationInvalid and names the contract
// rule the entry broke. Reason is populated only for a reachability failure and
// is drawn from the closed reason set. They are separate fields because they
// answer different questions, and one field carrying either would be a value
// space that means two things.
type Observation struct {
	Kind   string
	Reason string
	Detail string
	Rule   string
}

// Member is one configured entry with what was observed about it.
//
// Index and DisplayName are carried even when the entry is unusable, because
// FR-031 requires an invalid entry to be named by its position in the entry
// list and by its display name as written — which is exactly the information a
// loader that renamed or dropped the entry would have destroyed.
type Member struct {
	Index       int
	ServerID    string
	DisplayName string
	Server      clientconfig.Server
	Observation Observation
	Sessions    []gateway.SessionInfo
}

// Snapshot is one observation of the whole configured fleet.
type Snapshot struct {
	Members []Member
}

// Prober observes one usable server. It is a seam so that tests can drive every
// observation kind without a listener, and so that the never-probe rule for an
// unusable entry is checkable by giving the prober a body that fails the test.
type Prober func(ctx context.Context, server clientconfig.Server) (Observation, []gateway.SessionInfo)

// Observe produces one Member per configured entry, in configuration order.
//
// Usable entries are probed concurrently and independently: one server's
// failure cannot suppress another's result, because nothing in this function
// aggregates across them. Unusable entries are not probed at all — the loop
// that dispatches work skips them, so there is no path on which a
// configuration-invalid entry could acquire a reachability observation.
func Observe(ctx context.Context, configured clientconfig.Snapshot, probe Prober) Snapshot {
	members := make([]Member, len(configured.Servers))
	var waiting sync.WaitGroup

	for index, server := range configured.Servers {
		members[index] = Member{
			Index:       server.Index,
			ServerID:    server.ID,
			DisplayName: server.DisplayName,
			Server:      server,
			Sessions:    []gateway.SessionInfo{},
		}
		switch server.Admission() {
		case clientconfig.AdmissionConfigurationInvalid:
			members[index].Observation = Observation{
				Kind: KindConfigurationInvalid,
				Rule: violationRule(server),
				// Detail is the loader's own explanation, not a sentence this
				// package invents about a file it did not read.
				Detail: violationDetail(server),
			}
			continue
		case clientconfig.AdmissionDisabled:
			members[index].Observation = Observation{Kind: KindDisabled}
			continue
		}
		waiting.Add(1)
		go func(slot int, target clientconfig.Server) {
			defer waiting.Done()
			probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			observation, sessions := probe(probeCtx, target)
			members[slot].Observation = observation
			if sessions != nil {
				members[slot].Sessions = sessions
			}
		}(index, server)
	}
	waiting.Wait()
	return Snapshot{Members: members}
}

func violationRule(server clientconfig.Server) string {
	if server.Violation == nil {
		return ""
	}
	return server.Violation.Rule
}

func violationDetail(server clientconfig.Server) string {
	if server.Violation == nil {
		return ""
	}
	return server.Violation.Detail
}

// Usable returns the members that answered and can be acted on.
func (snapshot Snapshot) Usable() []Member {
	usable := make([]Member, 0, len(snapshot.Members))
	for _, member := range snapshot.Members {
		if member.Observation.Kind == KindConnected {
			usable = append(usable, member)
		}
	}
	return usable
}

// HasUnreachable reports whether any entry that was probed failed to answer.
// Selector resolution needs this to tell "no such session" from "we could not
// look everywhere".
func (snapshot Snapshot) HasUnreachable() bool {
	for _, member := range snapshot.Members {
		switch member.Observation.Kind {
		case KindUnavailable, KindIncompatibleProtocol:
			return true
		}
	}
	return false
}

// Invalid returns the configuration-invalid members, which every presentation
// must show rather than omit.
func (snapshot Snapshot) Invalid() []Member {
	invalid := make([]Member, 0)
	for _, member := range snapshot.Members {
		if member.Observation.Kind == KindConfigurationInvalid {
			invalid = append(invalid, member)
		}
	}
	return invalid
}

// NewProber builds the production prober over the real transport.
func NewProber(doer gateway.Doer, dialer gateway.Dialer) Prober {
	return func(ctx context.Context, server clientconfig.Server) (Observation, []gateway.SessionInfo) {
		client, err := gateway.New(server, doer, dialer)
		if err != nil {
			return observationFor(err), nil
		}
		return ProbeWith(ctx, client)
	}
}

// ProbeWith performs one server's whole observation against an already-built
// client. It is separate from NewProber so a test can drive it with a fake
// transport without reimplementing the sequence.
func ProbeWith(ctx context.Context, client *gateway.Client) (Observation, []gateway.SessionInfo) {
	if err := client.Admit(ctx); err != nil {
		return observationFor(err), nil
	}
	socket, _, err := client.Dial(ctx)
	if err != nil {
		return observationFor(err), nil
	}
	defer func() { _ = socket.Close() }()

	// The session list arrives on the lifecycle socket, not over REST: the REST
	// listing carries only identity and command, while status and card title —
	// the two columns the contract requires — are on the hello frame.
	for {
		raw, readErr := socket.Read(ctx)
		if readErr != nil {
			return observationFor(readErr), nil
		}
		frame, decodeErr := gateway.DecodeFrame(raw)
		if decodeErr != nil {
			continue
		}
		switch typed := frame.(type) {
		case gateway.HelloFrame:
			return Observation{Kind: KindConnected}, typed.Sessions
		case gateway.ViewUpdateFrame:
			return Observation{Kind: KindConnected}, typed.Sessions
		case gateway.ControlFrame:
			return Observation{
				Kind:   KindUnavailable,
				Reason: ReasonWebSocketFailed,
				Detail: string(typed.Reason),
			}, nil
		}
	}
}

// observationFor maps a transport failure onto the closed observation
// vocabulary. Every branch names a reason from the closed reason set; none of
// them invents a sentence.
func observationFor(err error) Observation {
	var transport *gateway.TransportError
	if !errors.As(err, &transport) {
		if errors.Is(err, context.DeadlineExceeded) {
			return Observation{Kind: KindUnavailable, Reason: ReasonConnectionTimeout}
		}
		return Observation{Kind: KindUnavailable, Reason: ReasonConnectionFailed}
	}
	switch {
	case errors.Is(transport, gateway.ErrIncompatibleProtocol):
		return Observation{
			Kind: KindIncompatibleProtocol, Reason: ReasonProtocolMismatch, Detail: transport.Detail,
		}
	case errors.Is(transport, gateway.ErrUnauthenticated):
		return Observation{Kind: KindUnavailable, Reason: ReasonHTTPError, Detail: transport.Detail}
	case errors.Is(transport, gateway.ErrCapacity):
		return Observation{Kind: KindUnavailable, Reason: ReasonHTTPError, Detail: transport.Detail}
	case errors.Is(transport, gateway.ErrInvalidRequest):
		return Observation{Kind: KindUnavailable, Reason: ReasonConnectionFailed, Detail: transport.Detail}
	}
	if transport.Reason == gateway.ReasonBackendUnavailable &&
		containsAny(transport.Detail, "timeout", "deadline") {
		return Observation{Kind: KindUnavailable, Reason: ReasonConnectionTimeout}
	}
	if containsAny(transport.Detail, "empty websocket ticket") {
		return Observation{Kind: KindUnavailable, Reason: ReasonEmptyTicket}
	}
	return Observation{Kind: KindUnavailable, Reason: ReasonConnectionFailed, Detail: transport.Detail}
}

// containsAny matches case-insensitively so the reason classification does not
// depend on how a particular Go version or proxy capitalised its error text.
func containsAny(text string, needles ...string) bool {
	lowered := strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(lowered, needle) {
			return true
		}
	}
	return false
}
