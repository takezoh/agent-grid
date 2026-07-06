package fakecodex

import (
	"encoding/json"
	"testing"

	"github.com/takezoh/agent-grid/platform/agent/codexschema"
	"github.com/takezoh/agent-grid/platform/e2etest"
)

func defaultTurnRecordedEvent(t *testing.T, method string, raw json.RawMessage) any {
	t.Helper()
	switch method {
	case codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadSettingsUpdated,
		codexschema.MethodTurnCompleted:
	default:
		t.Fatalf("unsupported default-turn method: %s", method)
	}
	params, err := e2etest.NormalizeJSON(raw)
	if err != nil {
		t.Fatalf("NormalizeJSON(%s): %v", method, err)
	}
	params = normalizeRecordedDefaultTurnParams(params).(map[string]any)
	return map[string]any{
		"method": method,
		"params": params,
	}
}

func defaultTurnEventContract(t *testing.T, method string, raw json.RawMessage) any {
	t.Helper()
	params, err := e2etest.NormalizeJSON(raw)
	if err != nil {
		t.Fatalf("NormalizeJSON(%s): %v", method, err)
	}
	return defaultTurnEventContractFromParams(t, method, params)
}

func defaultTurnEventContractFromFixture(t *testing.T, item map[string]any) any {
	t.Helper()
	method, _ := item["method"].(string)
	params, _ := item["params"].(map[string]any)
	if params == nil {
		t.Fatalf("fixture missing params for %s: %v", method, item)
	}
	return defaultTurnEventContractFromParams(t, method, params)
}

func defaultTurnEventContractFromParams(t *testing.T, method string, params map[string]any) any {
	t.Helper()
	switch method {
	case codexschema.MethodThreadStarted:
		thread, _ := params["thread"].(map[string]any)
		if thread == nil {
			t.Fatalf("thread/started missing thread payload: %v", params)
		}
		projected := map[string]any{}
		copyIfPresent(projected, "id", thread, "id")
		copyIfPresent(projected, "cwd", thread, "cwd")
		copyIfPresent(projected, "path", thread, "path")
		return map[string]any{
			"method": method,
			"params": map[string]any{"thread": projected},
		}
	case codexschema.MethodTurnStarted:
		projected := map[string]any{}
		copyIfPresent(projected, "threadId", params, "threadId")
		copyIfPresent(projected, "turn", params, "turn")
		return map[string]any{"method": method, "params": projected}
	case codexschema.MethodThreadSettingsUpdated:
		projected := map[string]any{}
		copyIfPresent(projected, "threadId", params, "threadId")
		copyIfPresent(projected, "threadSettings", params, "threadSettings")
		return map[string]any{"method": method, "params": projected}
	case codexschema.MethodTurnCompleted:
		projected := map[string]any{}
		copyIfPresent(projected, "threadId", params, "threadId")
		copyIfPresent(projected, "turn", params, "turn")
		return map[string]any{"method": method, "params": projected}
	default:
		t.Fatalf("unsupported default-turn method: %s", method)
		return nil
	}
}

func copyIfPresent(dst map[string]any, dstKey string, src map[string]any, srcKey string) {
	if value, ok := src[srcKey]; ok {
		dst[dstKey] = value
	}
}

func normalizeRecordedDefaultTurnParams(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, value := range x {
			if isVolatileRecordedNumberKey(k) {
				if _, ok := value.(float64); ok {
					out[k] = "number"
					continue
				}
			}
			if isVolatileRecordedStringKey(k) {
				if _, ok := value.(string); ok {
					out[k] = "string"
					continue
				}
			}
			out[k] = normalizeRecordedDefaultTurnParams(value)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = normalizeRecordedDefaultTurnParams(x[i])
		}
		return out
	default:
		return v
	}
}

func isVolatileRecordedNumberKey(key string) bool {
	switch key {
	case "createdAt", "updatedAt", "recencyAt", "startedAt", "completedAt", "durationMs":
		return true
	default:
		return false
	}
}

func isVolatileRecordedStringKey(key string) bool {
	switch key {
	case "cliVersion":
		return true
	default:
		return false
	}
}
