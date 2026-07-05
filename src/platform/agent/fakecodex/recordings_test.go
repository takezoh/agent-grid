package fakecodex

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
	"github.com/takezoh/agent-reactor/platform/e2etest"
)

func TestRecordedDefaultTurnFixtureMatchesPresetContract(t *testing.T) {
	recorded := loadCodexContracts(t, filepath.Join("testdata", "recordings", "default-turn.jsonl"))
	fake := recordFakeContracts(t, Config{}, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodTurnCompleted,
	}, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})
	if !reflect.DeepEqual(recorded, fake) {
		t.Fatalf("default-turn recording contract mismatch:\nrecorded=%#v\nfake=%#v", recorded, fake)
	}
}

func recordFakeContracts(t *testing.T, cfg Config, wanted []string, expectedPrefix []string) []any {
	t.Helper()
	server := New(cfg)
	client, cleanup := pipeToClient(t, server)
	defer cleanup()

	rec := &notifRecorder{}
	go func() { _ = client.Run(context.Background(), rec) }()

	if err := codexclient.Initialize(client); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if _, err := codexclient.StartThread(client, "/tmp/work", nil, codexclient.ThreadOptions{}); err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if err := codexclient.StartTurn(client, DefaultThreadID, "/tmp/work", []byte("ping"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	allow := map[string]bool{}
	for _, method := range wanted {
		allow[method] = true
	}
	waitFor(t, rec, expectedPrefix)

	rec.mu.Lock()
	defer rec.mu.Unlock()
	out := make([]any, 0, len(wanted))
	for _, event := range rec.events {
		if !allow[event.method] {
			continue
		}
		out = append(out, e2etest.Contract(projectFakeDefaultTurnEvent(t, event.method, event.params)))
	}
	return out
}

func loadCodexContracts(t *testing.T, path string) []any {
	t.Helper()
	items := e2etest.ReadJSONLFixture(t, path)
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, e2etest.Contract(item))
	}
	return out
}

func projectFakeDefaultTurnEvent(t *testing.T, method string, raw json.RawMessage) any {
	t.Helper()
	norm, err := e2etest.NormalizeJSON(raw)
	if err != nil {
		t.Fatalf("NormalizeJSON(%s): %v", method, err)
	}
	params := norm
	switch method {
	case codexschema.MethodThreadStarted:
		thread, _ := params["thread"].(map[string]any)
		path := thread["path"]
		if path == nil {
			path = thread["cwd"]
		}
		return map[string]any{
			"method": method,
			"params": map[string]any{
				"thread": map[string]any{
					"id":   thread["id"],
					"path": path,
				},
			},
		}
	case codexschema.MethodTurnStarted:
		return map[string]any{
			"method": method,
			"params": map[string]any{
				"threadId": params["threadId"],
				"turnId":   params["turnId"],
			},
		}
	case codexschema.MethodThreadSettingsUpdated, codexschema.MethodTurnCompleted:
		return map[string]any{"method": method, "params": params}
	default:
		t.Fatalf("unsupported fake projection method: %s", method)
		return nil
	}
}
