package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// auth.go holds the two credentials this client uses and the rule that keeps
// them apart.
//
// The bearer credential authenticates REST and travels only in the
// Authorization header — never in a URL, never in a process argument. A
// WebSocket upgrade cannot carry a header from a browser, so the socket is
// authenticated by a separate short-lived single-use ticket minted over the
// authenticated REST API. That two-step is not a CLI invention: it is the flow
// every other client already uses, and reusing it is what keeps the server from
// needing a CLI-only authentication path.

// applyCredential attaches the bearer credential, if the entry declares one.
// An entry with mode "none" is valid only against a server started with
// -no-auth; sending an empty bearer would be a different request than sending
// none, so nothing is set.
func applyCredential(request *http.Request, auth clientconfig.Auth) {
	if auth.Mode != "bearer" || auth.Token == "" {
		return
	}
	request.Header.Set("Authorization", "Bearer "+auth.Token)
}

// ticketResponse is the POST /api/ws-ticket body.
type ticketResponse struct {
	Ticket           string `json:"ticket"`
	ClientInstanceID string `json:"client_instance_id"`
}

// mintTicket obtains a fresh single-use ticket. It is called immediately before
// every dial and its result is never stored: a ticket that outlives the dial it
// was minted for is a credential sitting in memory for no reason, and the
// server invalidates it on first use anyway.
func (client *Client) mintTicket(ctx context.Context) (ticketResponse, error) {
	var minted ticketResponse
	response, err := client.do(ctx, http.MethodPost, "/api/ws-ticket", nil)
	if err != nil {
		return minted, err
	}
	defer func() { _ = response.Body.Close() }()
	if failure := client.classifyStatus(response); failure != nil {
		return minted, failure
	}
	if err := json.NewDecoder(response.Body).Decode(&minted); err != nil {
		return minted, wrapTransport(ReasonBackendUnavailable, err, client.secrets()...)
	}
	if minted.Ticket == "" {
		// An empty ticket is a server that answered 200 with nothing usable.
		// Dialing with it would fail as "unauthenticated" and blame the
		// operator's credential for a server-side fault.
		return minted, &TransportError{
			Reason: ReasonBackendUnavailable,
			Kind:   ErrUnavailable,
			Detail: "the server minted an empty websocket ticket",
		}
	}
	return minted, nil
}

// socketEndpoint builds the WebSocket URL for the lifecycle-multiplexed attach
// transport.
//
// There is no session query parameter, and there must never be one: the
// per-session route shares a single empty subscriber identity across every
// client on it, so a second attaching client gets no snapshot and one
// detaching client drops both (adr-20260814-cli-attach-via-lifecycle-multiplexed-ws).
// TestAttachNeverUsesPerSessionRoute fails if that route reappears anywhere in
// this module.
//
// The ticket travels in the query because a WebSocket upgrade has nowhere else
// to put it. The bearer credential does not: it stays in the Authorization
// header of the REST call that minted the ticket.
func (client *Client) socketEndpoint(ticket string) (string, error) {
	target, err := url.Parse(client.server.URL)
	if err != nil {
		return "", &TransportError{
			Reason: ReasonInvalidRequest,
			Kind:   ErrInvalidRequest,
			Detail: "the configured server url is not a url",
		}
	}
	switch target.Scheme {
	case "http":
		target.Scheme = "ws"
	case "https":
		target.Scheme = "wss"
	default:
		return "", &TransportError{
			Reason: ReasonInvalidRequest,
			Kind:   ErrInvalidRequest,
			Detail: "the configured server url is neither http nor https",
		}
	}
	target.Path = strings.TrimRight(target.Path, "/") + "/ws"
	query := target.Query()
	query.Set("ticket", ticket)
	query.Set("protocolVersion", ProtocolVersion)
	target.RawQuery = query.Encode()
	return target.String(), nil
}
