package hookpayload

import (
	"encoding/json"
	"testing"
)

func TestHookPayloadTracksMetadataPresence(t *testing.T) {
	var got HookPayload
	if err := json.Unmarshal([]byte(`{"model":null,"effort":null}`), &got); err != nil {
		t.Fatalf("Unmarshal = %v", err)
	}
	if !got.ModelSet || !got.EffortSet {
		t.Fatalf("presence flags = %v/%v, want true/true", got.ModelSet, got.EffortSet)
	}
	if got.Model != "" || got.Effort.Value() != "" {
		t.Fatalf("cleared values = %q/%q, want empty", got.Model, got.Effort.Value())
	}
}

func TestHookPayloadLeavesAbsentMetadataUnset(t *testing.T) {
	var got HookPayload
	if err := json.Unmarshal([]byte(`{"session_id":"abc"}`), &got); err != nil {
		t.Fatalf("Unmarshal = %v", err)
	}
	if got.ModelSet || got.EffortSet {
		t.Fatalf("presence flags = %v/%v, want false/false", got.ModelSet, got.EffortSet)
	}
}
