package api

import (
	"encoding/json"
	"testing"

	"github.com/takezoh/agent-grid/host/proto"
)

func TestCapabilityBundledSingleRoundTrip(t *testing.T) {
	// Bundled axis: hello carries protocolVersion + capabilities in the same
	// initial frame as sessions — no extra request/response pair (NFR-04).
	frame := encodeHelloFrame(proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{},
		Features: []string{},
	}, 1_700_000_001, "ci-bundled")
	if frame == nil {
		t.Fatal("nil hello frame")
	}
	var h helloFrame
	if err := json.Unmarshal(frame, &h); err != nil {
		t.Fatal(err)
	}
	if h.K != "h" {
		t.Fatalf("k = %q", h.K)
	}
	if h.ProtocolVersion != ProtocolVersion {
		t.Fatalf("protocolVersion = %q, want %q", h.ProtocolVersion, ProtocolVersion)
	}
	if len(h.Capabilities) == 0 {
		t.Fatal("expected non-empty capabilities on bundled hello")
	}
	if h.ClientInstanceID != "ci-bundled" {
		t.Fatalf("clientInstanceId = %q", h.ClientInstanceID)
	}
	// Assert capability set includes approval.respond without a second hop.
	found := false
	for _, c := range h.Capabilities {
		if c == "approval.respond" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("capabilities = %v, missing approval.respond", h.Capabilities)
	}
}

func TestCapabilityHelloIsAdditiveForOldClients(t *testing.T) {
	// Old clients that only read k/sessions/features/serverTime must still
	// decode the new payload (unknown fields ignored by JSON unmarshallers).
	frame := encodeHelloFrame(proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{{ID: "s1", Project: "p", Command: "c", CreatedAt: "t"}},
		Features: []string{"surface"},
	}, 42, "ci-x")
	var legacy struct {
		K          string `json:"k"`
		Sessions   []any  `json:"sessions"`
		Features   []any  `json:"features"`
		ServerTime int64  `json:"serverTime"`
	}
	if err := json.Unmarshal(frame, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.K != "h" || len(legacy.Sessions) != 1 || legacy.ServerTime != 42 {
		t.Fatalf("legacy decode failed: %+v", legacy)
	}
}
