// Package gateway is this module's only speaker of the public wire contract:
// the REST routes, the WebSocket admission handshake, and the frame codec.
// Nothing outside it builds a request, a URL, or a frame.
package gateway

// ProtocolVersion is the exact client/server wire contract version this build
// speaks. protocol/protocol-version.json is the repository source of truth and
// TestProtocolVersionManifestMatchesEveryShippedProjection keeps this
// projection byte-identical to it and to every other shipped client's.
//
// It appears exactly once in this module. A mismatch is fail-closed: the CLI
// performs no operational work against a server that answers with a different
// version, rather than trying a reduced feature set.
const ProtocolVersion = "protocol-3"

// ProtocolVersionHeader carries ProtocolVersion on every REST request. The
// WebSocket upgrade cannot set headers from a browser, so the same value also
// travels as the protocolVersion query parameter there; both spellings are
// checked by the same server-side gate.
const ProtocolVersionHeader = "X-Agent-Grid-Protocol-Version"
