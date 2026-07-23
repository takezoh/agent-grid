package api

import (
	"encoding/base64"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/takezoh/agent-grid/host/proto"
	stateview "github.com/takezoh/agent-grid/host/state/view"
)

var updateWireFixtures = flag.Bool("update", false, "rewrite committed wire fixtures")

func TestWireFixtures(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "clients", "ui", "src", "wire", "testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	fixtures := []struct {
		name string
		path string
		got  []byte
	}{
		{
			name: "hello",
			path: filepath.Join(dir, "hello.json"),
			got: encodeHelloFrame(proto.EvtSessionsChanged{
				Sessions: []proto.SessionInfo{
					{
						ID:            "s1",
						Project:       "p",
						Command:       "claude",
						WorkspaceRoot: "/workspace/p",
						CreatedAt:     "2026-06-20T00:00:00Z",
						View: stateview.View{
							Card:       stateview.Card{Title: "T1", Tags: []stateview.Tag{{Text: "tag"}}},
							Status:     stateview.StatusRunning,
							StatusLine: "thinking",
							LogTabs: []stateview.LogTab{
								{Label: "events", Path: "/tmp/x", Kind: stateview.TabKindText},
							},
							StatusChangedAt: time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
						},
					},
				},
				Features: []string{"surface"},
			}, 1700000001),
		},
		{
			name: "view-update",
			path: filepath.Join(dir, "view-update.json"),
			got: encodeFromSessionsChanged(proto.EvtSessionsChanged{
				Sessions: []proto.SessionInfo{
					wireFixtureSession("s1", "T1", stateview.StatusRunning, "gpt-5", "high"),
					wireFixtureSession("s2", "T2", stateview.StatusWaiting, "claude-sonnet", "medium"),
					wireFixtureSession("s3", "T3", stateview.StatusIdle, "", ""),
					wireFixtureSession("s4", "T4", stateview.StatusStopped, "", ""),
					wireFixtureSession("s5", "T5", stateview.StatusPending, "codex", "low"),
				},
			}),
		},
		{
			name: "output",
			path: filepath.Join(dir, "output.json"),
			got: outputFrameFromSurface(proto.EvtSurfaceOutput{
				SessionID: "s1",
				TimeSec:   1.5,
				DataB64:   base64.StdEncoding.EncodeToString([]byte("hi")),
			}),
		},
		{
			name: "control",
			path: filepath.Join(dir, "control.json"),
			got:  controlFrame(0, "daemon-disconnected"),
		},
		{
			name: "control-with-code",
			path: filepath.Join(dir, "control-with-code.json"),
			got:  controlFrame(9, "t | b"),
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if *updateWireFixtures {
				if err := os.WriteFile(fixture.path, fixture.got, 0o644); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
			}

			want, err := os.ReadFile(fixture.path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if diff := cmp.Diff(string(want), string(fixture.got)); diff != "" {
				t.Fatalf("fixture drift (-want +got):\n%s", diff)
			}
		})
	}
}

func wireFixtureSession(
	id string,
	title string,
	status stateview.Status,
	model string,
	effort string,
) proto.SessionInfo {
	view := stateview.View{
		Card: stateview.Card{Title: title},
		// Distinct timestamps keep JSON deterministic while still exercising
		// the RFC3339 status_changed_at field on every status variant.
		Status:          status,
		StatusChangedAt: time.Date(2026, 6, 20, 0, len(id), 0, 0, time.UTC),
	}
	if model != "" {
		view.Model = model
	}
	if effort != "" {
		view.Effort = effort
	}
	return proto.SessionInfo{
		ID:        id,
		Project:   "p",
		Command:   "claude",
		CreatedAt: "2026-06-20T00:00:00Z",
		View:      view,
	}
}
