package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	// ProtocolVersion is the exact client/server wire contract version.
	// protocol/protocol-version.json is the repository SSOT; parity tests keep
	// this projection and every shipped client projection byte-identical.
	ProtocolVersion = "protocol-3"

	ProtocolVersionHeader = "X-Agent-Grid-Protocol-Version"
)

// protocolVersionError is a pre-upgrade rejection: Code is drawn from the same
// published transport-close vocabulary a post-upgrade departure carries, so
// one vocabulary spans both sides of the WebSocket upgrade.
type protocolVersionError struct {
	Code            CloseReason `json:"code"`
	ExpectedVersion string      `json:"expectedVersion"`
	ActualVersion   string      `json:"actualVersion"`
}

func ProtocolVersionGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values(ProtocolVersionHeader)
		if len(values) != 1 || values[0] != ProtocolVersion {
			actual := "<missing>"
			if len(values) > 0 {
				actual = strings.Join(values, ",")
			}
			writeProtocolVersionError(w, actual)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeProtocolVersionError(w http.ResponseWriter, actual string) {
	if actual == "" {
		actual = "<missing>"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUpgradeRequired)
	_ = json.NewEncoder(w).Encode(protocolVersionError{
		Code:            CloseIncompatibleProtocol,
		ExpectedVersion: ProtocolVersion,
		ActualVersion:   actual,
	})
}

func protocolAwareAPIHandler(
	d *DaemonClient,
	tickets *ticketStore,
	dependencies muxDependencies,
) http.Handler {
	operational := ProtocolVersionGate(apiHandler(d, tickets, dependencies))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Version discovery is the only ungated protocol endpoint. Operational
		// APIs require exact equality; capabilities never authorize fallback.
		if r.Method == http.MethodGet && r.URL.Path == "/api/capabilities" {
			handleCapabilities().ServeHTTP(w, r)
			return
		}
		operational.ServeHTTP(w, r)
	})
}

// wsProtocolVersion reads the wire version a WebSocket upgrade declares, from
// the query string (a browser cannot set headers) or from the header.
//
// supplied separates "declared nothing" from "declared something wrong". The
// ticket-authenticated path admits the former, because the ticket was minted
// over the version-gated REST API and a mismatched build cannot hold one; the
// no-auth path has no such gate and requires a declaration.
func wsProtocolVersion(r *http.Request) (actual string, supplied, ok bool) {
	queryVersions := r.URL.Query()["protocolVersion"]
	if len(queryVersions) > 0 {
		actual = strings.Join(queryVersions, ",")
		return actual, true, len(queryVersions) == 1 && actual == ProtocolVersion
	}
	headerVersions := r.Header.Values(ProtocolVersionHeader)
	actual = strings.Join(headerVersions, ",")
	return actual, len(headerVersions) > 0, len(headerVersions) == 1 && actual == ProtocolVersion
}
