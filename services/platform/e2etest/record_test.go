package e2etest

import (
	"reflect"
	"testing"
)

func TestNormalizeJSONRedactsVolatileFields(t *testing.T) {
	raw := []byte(`{
		"session_id":"019e727e-fde4-7432-9036-ae6604ce1b27",
		"thread":{"id":"thread-123","path":"/tmp/work/rollout.jsonl"},
		"created_at":"2026-07-05T10:11:12Z",
		"api_token":"secret",
		"text":"keep me"
	}`)
	got, err := NormalizeJSON(raw)
	if err != nil {
		t.Fatalf("NormalizeJSON: %v", err)
	}
	if got["session_id"] != "<id>" {
		t.Fatalf("session_id = %v", got["session_id"])
	}
	thread := got["thread"].(map[string]any)
	if thread["id"] != "<id>" {
		t.Fatalf("thread.id = %v", thread["id"])
	}
	if thread["path"] != "<path>" {
		t.Fatalf("thread.path = %v", thread["path"])
	}
	if got["created_at"] != "<timestamp>" {
		t.Fatalf("created_at = %v", got["created_at"])
	}
	if got["api_token"] != "<secret>" {
		t.Fatalf("api_token = %v", got["api_token"])
	}
	if got["text"] != "keep me" {
		t.Fatalf("text = %v", got["text"])
	}
}

func TestContractKeepsDiscriminatorsAndDropsConcreteValues(t *testing.T) {
	in := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "tool_use", "id": "tu-1", "name": "Bash"},
			},
		},
	}
	got := Contract(in)
	want := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "tool_use", "id": "string", "name": "string"},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Contract() = %#v, want %#v", got, want)
	}
}
