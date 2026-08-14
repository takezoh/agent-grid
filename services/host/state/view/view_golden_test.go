package view_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/proto"
	stateview "github.com/takezoh/agent-grid/services/host/state/view"
)

// Contract: contract-wire-shape-conformance
// (adr-20260814-sessioninfo-golden-vector-conformance).
//
// The terminal CLI decodes the session payload this package serializes. Nothing
// on the server side used to fail when a field the CLI reads was renamed or
// moved — the SPA would break at runtime and the CLI would silently decode a
// zero value. The golden vector closes that: it is one checked-in payload that
// the server serializer reproduces byte for byte here, and that the CLI decodes
// in clients/cli. A rename fails on the server side, where the rename happens.
//
// The vector lives under protocol/ rather than inside either module because
// neither owns it: clients/cli is a separate Go module and cannot import
// services, and services must not reach into a client module for a test input.

var updateSessionInfoGolden = flag.Bool(
	"update-session-info-golden", false, "rewrite the checked-in SessionInfo golden vector")

// SessionInfoGoldenVectorPath is the repository-relative location of the vector.
// Both consumers resolve it from their own module root; the gate in
// services/gorules enumerates them.
const sessionInfoGoldenVectorPath = "protocol/fixtures/session-info-golden.json"

// goldenSessionInfo is the canonical payload. It deliberately populates every
// field the CLI reads — identity, working directory, launch command, lifecycle
// status, and the card title it prints as the session's display name — so that
// dropping or renaming any of them changes these bytes.
func goldenSessionInfo() proto.SessionInfo {
	changedAt := time.Date(2026, 8, 14, 9, 30, 0, 0, time.UTC)
	return proto.SessionInfo{
		ID:                 "s_0a1b2c3d4e5f",
		Project:            "/home/operator/repos/agent-grid",
		Workspace:          "default",
		Command:            "claude --permission-mode acceptEdits",
		RootDriver:         "claude",
		RootDriverForkable: true,
		CreatedAt:          "2026-08-14T09:00:00Z",
		Status: stateview.CompositeStatus(
			stateview.StatusRunning,
			changedAt,
			false,
			false,
			&stateview.StatusSignal{Kind: "bell", Epoch: "e1", Sequence: 3},
		),
		View: stateview.View{
			Card: stateview.Card{
				Title: "Wire up the terminal client",
				Tags:  []stateview.Tag{{Text: "claude", Foreground: "#c0ffee"}},
			},
			DisplayName:    "claude",
			StatusLine:     "Editing wire.go",
			LastUserPrompt: "wire up the terminal client",
			Model:          "claude-opus-5",
			Effort:         "high",
		},
		Frames: []proto.FrameInfo{
			{ID: "f_root", Command: "claude"},
		},
		HeadFrameID:     "f_root",
		WorkspaceRoot:   "/home/operator/repos/agent-grid",
		FrameGeneration: 2,
	}
}

// TestSessionInfoGoldenVectorIsStable is
// ver-server-serialization-matches-golden-vector.
func TestSessionInfoGoldenVectorIsStable(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", sessionInfoGoldenVectorPath)

	encoded, err := json.MarshalIndent(goldenSessionInfo(), "", "  ")
	if err != nil {
		t.Fatalf("marshal session info: %v", err)
	}
	encoded = append(encoded, '\n')

	if *updateSessionInfoGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir fixtures: %v", err)
		}
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			t.Fatalf("write golden vector: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden vector: %v", err)
	}
	if string(want) != string(encoded) {
		t.Errorf("the server serializer no longer reproduces %s.\n"+
			"A field the terminal CLI reads was renamed, moved or dropped.\n"+
			"golden:\n%s\nserializer:\n%s", sessionInfoGoldenVectorPath, want, encoded)
	}
}
