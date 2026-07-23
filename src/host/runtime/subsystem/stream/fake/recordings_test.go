package fake

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/platform/agent/codexclient"
	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	"github.com/takezoh/agent-grid/platform/e2etest"
)

func TestRecordedDefaultTurnFixtureMatchesAppServerContract(t *testing.T) {
	recorded := loadRecordedContracts(t, filepath.Join("..", "..", "..", "..", "..", "platform", "agent", "fakecodex", "testdata", "recordings", "default-turn.jsonl"))
	fake := recordAppServerContracts(t, Config{}, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodTurnCompleted,
	}, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadSettingsUpdated,
		codexschema.MethodThreadStatusChanged,
		codexschema.MethodItemAgentMessageDelta,
		codexschema.MethodTurnCompleted,
	})
	if !reflect.DeepEqual(recorded, fake) {
		t.Fatalf("default-turn recording contract mismatch:\nrecorded=%#v\nfake=%#v", recorded, fake)
	}
}

func recordAppServerContracts(t *testing.T, cfg Config, wanted []string, expectedPrefix []string) []any {
	t.Helper()
	srv := startFake(t, cfg)
	client, rec := dialClient(t, srv.SockPath())
	if _, err := codexclient.StartThread(client, "/tmp/work", nil, codexclient.ThreadOptions{}); err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if err := codexclient.StartTurn(client, "fake-thread-001", "/tmp/work", []byte("ping"), codexclient.TurnOptions{}); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		if len(rec.events) < len(expectedPrefix) {
			return false
		}
		for i, method := range expectedPrefix {
			if rec.events[i].Method != method {
				return false
			}
		}
		return true
	}, "default lifecycle sequence")

	allow := map[string]bool{}
	for _, method := range wanted {
		allow[method] = true
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	out := make([]any, 0, len(wanted))
	for _, event := range rec.events {
		if !allow[event.Method] {
			continue
		}
		out = append(out, e2etest.Contract(projectAppServerDefaultTurnEvent(t, event.Method, event.Params)))
	}
	return out
}

func loadRecordedContracts(t *testing.T, path string) []any {
	t.Helper()
	items := e2etest.ReadJSONLFixture(t, path)
	out := make([]any, 0, len(items))
	for _, item := range items {
		method, _ := item["method"].(string)
		params, _ := item["params"].(map[string]any)
		if params == nil {
			t.Fatalf("fixture missing params for %s: %v", method, item)
		}
		out = append(out, e2etest.Contract(projectAppServerDefaultTurnParams(t, method, params)))
	}
	return out
}

func projectAppServerDefaultTurnEvent(t *testing.T, method string, raw json.RawMessage) any {
	t.Helper()
	norm, err := e2etest.NormalizeJSON(raw)
	if err != nil {
		t.Fatalf("NormalizeJSON(%s): %v", method, err)
	}
	params := fakecodexNormalizeRecordedDefaultTurnParams(norm)
	return projectAppServerDefaultTurnParams(t, method, params)
}

func projectAppServerDefaultTurnParams(t *testing.T, method string, params map[string]any) any {
	t.Helper()
	switch method {
	case codexschema.MethodThreadStarted:
		thread, _ := params["thread"].(map[string]any)
		path := thread["path"]
		if path == nil {
			t.Fatalf("thread/started missing thread.path: %v", thread)
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
				"turn":     params["turn"],
			},
		}
	case codexschema.MethodTurnCompleted:
		return map[string]any{
			"method": method,
			"params": map[string]any{
				"threadId": params["threadId"],
				"turn":     params["turn"],
			},
		}
	default:
		t.Fatalf("unsupported app-server projection method: %s", method)
		return nil
	}
}

func fakecodexNormalizeRecordedDefaultTurnParams(v any) map[string]any {
	params, _ := v.(map[string]any)
	return normalizeRecordedDefaultTurnParamsLocal(params).(map[string]any)
}

func normalizeRecordedDefaultTurnParamsLocal(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, value := range x {
			if isVolatileRecordedNumberKeyLocal(k) {
				if _, ok := value.(float64); ok {
					out[k] = "number"
					continue
				}
			}
			if k == "cliVersion" {
				if _, ok := value.(string); ok {
					out[k] = "string"
					continue
				}
			}
			out[k] = normalizeRecordedDefaultTurnParamsLocal(value)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = normalizeRecordedDefaultTurnParamsLocal(x[i])
		}
		return out
	default:
		return v
	}
}

func isVolatileRecordedNumberKeyLocal(key string) bool {
	switch key {
	case "createdAt", "updatedAt", "recencyAt", "startedAt", "completedAt", "durationMs":
		return true
	default:
		return false
	}
}
