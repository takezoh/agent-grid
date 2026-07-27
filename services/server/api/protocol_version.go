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
	ProtocolVersion = "protocol-2"

	ProtocolVersionHeader = "X-Agent-Grid-Protocol-Version"
)

type protocolVersionError struct {
	Code            string `json:"code"`
	ExpectedVersion string `json:"expectedVersion"`
	ActualVersion   string `json:"actualVersion"`
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
		Code:            "incompatible_protocol",
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

func noAuthWSProtocolVersion(r *http.Request) (string, bool) {
	queryVersions := r.URL.Query()["protocolVersion"]
	if len(queryVersions) > 0 {
		actual := strings.Join(queryVersions, ",")
		return actual, len(queryVersions) == 1 && actual == ProtocolVersion
	}
	headerVersions := r.Header.Values(ProtocolVersionHeader)
	actual := strings.Join(headerVersions, ",")
	return actual, len(headerVersions) == 1 && actual == ProtocolVersion
}
