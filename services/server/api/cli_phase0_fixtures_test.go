package api

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/host/config"
	"github.com/takezoh/agent-grid/services/host/proto"
	stateview "github.com/takezoh/agent-grid/services/host/state/view"
	platformconfig "github.com/takezoh/agent-grid/services/platform/config"
)

// Contract: contract-create-and-attach, contract-exit-classification,
// contract-attach-transport-topology (unit-phase0-contract-pin).
//
// Phase 0 of the terminal CLI replaces the remaining unmeasured wire facts with
// captured fixtures before any attach code exists. Everything written here comes
// out of a production handler, encoder or serializer in this package — never out
// of a paraphrase. clients/cli reads these back and cannot invent a dialect the
// server does not speak, and a server-side change to any of these shapes fails
// here rather than in a client at runtime.
//
// One substitution is applied: the project directory must exist on disk for
// config.ListProjects to return it, so the capture runs against a temporary
// directory whose path is replaced by phase0ProjectPath. That is the only edit
// between what the handler wrote and what is checked in.

var updateCLIPhase0Fixtures = flag.Bool(
	"update-cli-phase0", false, "rewrite the checked-in Phase 0 fixtures for clients/cli")

// phase0ProjectPath is the stable stand-in for the capture's temporary project
// directory. It is not a path any machine is expected to have.
const phase0ProjectPath = "/projects/agent-grid"

func TestCLIPhase0Fixtures(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "clients", "cli", "internal", "gateway", "testdata")

	project := t.TempDir()
	fixtures := []struct {
		name string
		got  []byte
	}{
		{"session-config.json", capturedSessionConfig(t, project)},
		{"create-session-request.json", capturedCreateRequest(t)},
		{"create-session-response.json", capturedCreateResponse(t)},
		{"lifecycle-session-end.json", capturedSessionEndFrame()},
		{"departure-capacity.json", capturedCapacityDeparture()},
		{"rejection-unauthenticated.json", capturedUnauthenticatedRejection(t)},
	}

	for _, fixture := range fixtures {
		got := []byte(strings.ReplaceAll(string(fixture.got), project, phase0ProjectPath))
		path := filepath.Join(dir, fixture.name)
		if *updateCLIPhase0Fixtures {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir testdata: %v", err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatalf("write %s: %v", fixture.name, err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", fixture.name, err)
		}
		if string(want) != string(got) {
			t.Errorf("%s drifted from what this gateway produces.\ncaptured:\n%s\nchecked in:\n%s",
				fixture.name, got, want)
		}
	}
}

// capturedSessionConfig answers evidence-create-request-vocabulary: which
// vocabulary carries the agent kind and the working directory a creation names.
// It is the production GET /api/session-config handler, driven through a
// recorder rather than a socket.
func capturedSessionConfig(t *testing.T, project string) []byte {
	t.Helper()
	loader := func() (*config.Config, error) {
		return &config.Config{
			Session: config.SessionConfig{
				PushCommands: []string{"/compact"},
				CommandProfiles: []config.CommandProfileConfig{
					{
						ID:         "claude",
						Label:      "Claude Code",
						Executable: "claude",
						Argv:       []string{"claude", "--permission-mode", "{{permission}}"},
						Parameters: []config.CommandParameterConfig{{
							Name:     "permission",
							Label:    "Permission mode",
							Type:     config.ParameterEnum,
							Required: true,
							Default:  "acceptEdits",
							Values:   []string{"acceptEdits", "plan"},
						}},
					},
					{ID: "shell", Label: "Shell", Executable: "bash", Argv: []string{"bash", "-l"}},
				},
			},
			Projects: platformconfig.ProjectsConfig{ProjectPaths: []string{project}},
		}, nil
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/session-config", nil)
	handleSessionConfig(loader)(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("session-config status = %d, want 200", recorder.Code)
	}
	return indentJSON(t, recorder.Body.Bytes())
}

// capturedCreateRequest is the POST /api/sessions body. It is written only
// after the production decoder admits it, so the fixture cannot record a body
// the gateway would reject.
func capturedCreateRequest(t *testing.T) []byte {
	t.Helper()
	body := `{"version":2,` +
		`"project":"` + phase0ProjectPath + `",` +
		`"profile_id":"claude","profile_revision":"` + phase0ProfileRevision + `",` +
		`"parameters":{"permission":"acceptEdits"},"cols":120,"rows":40,"worktree":false}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(body))
	if _, ok := decodeCreateRequest(recorder, request); !ok {
		t.Fatalf("the gateway rejected the captured create body: %s", recorder.Body.String())
	}
	return indentJSON(t, []byte(body))
}

// phase0ProfileRevision is any well-formed revision token; the gateway compares
// it against the profile set it resolves at request time, which is what
// unit-lifecycle-commands must therefore read from the target server rather
// than cache.
const phase0ProfileRevision = "sha256:0000000000000000"

// capturedCreateResponse is the 201 body the gateway writes, produced by the
// same apiSessionInfo serializer the handler uses.
func capturedCreateResponse(t *testing.T) []byte {
	t.Helper()
	recorder := httptest.NewRecorder()
	writeJSON(recorder, http.StatusCreated, apiSessionInfo{
		ID:        "s_0a1b2c3d4e5f",
		Project:   phase0ProjectPath,
		Command:   "claude --permission-mode acceptEdits",
		CreatedAt: time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC).Format(time.RFC3339),
	})
	return indentJSON(t, recorder.Body.Bytes())
}

// capturedSessionEndFrame answers evidence-session-end-frame-shape. The answer
// is positive, and it is an ordinary view update: the execution plane retains
// the ended session with a terminal lifecycle state
// (change-20260814-cli-prerequisite-contracts), so an attached CLI attributes
// its departure to the session ending by reading status.lifecycle == "stopped"
// on a row that is still published — not by watching a row vanish.
func capturedSessionEndFrame() []byte {
	changedAt := time.Date(2026, 8, 14, 9, 45, 0, 0, time.UTC)
	return mustIndent(encodeFromSessionsChanged(proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{{
			ID:        "s_0a1b2c3d4e5f",
			Project:   phase0ProjectPath,
			Command:   "claude --permission-mode acceptEdits",
			CreatedAt: "2026-08-14T09:00:00Z",
			Status: stateview.CompositeStatus(
				stateview.StatusStopped, changedAt, false, false, nil),
			View: stateview.View{Card: stateview.Card{Title: "Wire up the terminal client"}},
		}},
	}))
}

// capturedCapacityDeparture is the control frame a refused attach receives. The
// CLI maps reason to an exit class; it never authors the sentence itself.
func capturedCapacityDeparture() []byte {
	err := &proto.ErrorBody{Code: proto.ErrResourceExhausted, Message: "subscriber budget exhausted"}
	return mustIndent(departureControlFrame(departureReason(err), legacyDepartureData(err)))
}

// capturedUnauthenticatedRejection is the pre-upgrade body. It carries the same
// vocabulary as a close frame, which is what lets the CLI tell a rejected
// ticket from a dropped connection.
func capturedUnauthenticatedRejection(t *testing.T) []byte {
	t.Helper()
	recorder := httptest.NewRecorder()
	writeTransportRejection(recorder, http.StatusUnauthorized, CloseUnauthenticated,
		"missing, unknown, already-used or expired ticket")
	return indentJSON(t, recorder.Body.Bytes())
}

func indentJSON(t *testing.T, raw []byte) []byte {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("captured payload is not JSON: %v\n%s", err, raw)
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("re-encode captured payload: %v", err)
	}
	return append(encoded, '\n')
}

func mustIndent(raw []byte) []byte {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return append(raw, '\n')
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return append(raw, '\n')
	}
	return append(encoded, '\n')
}
