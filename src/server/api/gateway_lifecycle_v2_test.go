package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-grid/host/proto"
)

func TestGatewayLifecycleV2DesiredUsesPublicCorrelation(t *testing.T) {
	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)
	t.Cleanup(func() { _ = c.Close(websocket.StatusNormalClosure, "done") })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	frame := map[string]any{
		"k": "ld", "reqId": "v2-1", "sessionId": "s1", "cols": 120, "rows": 40,
		"desired": true,
		"correlation": map[string]any{
			"clientInstanceID": "client-1", "connectionGeneration": 2, "clientRevision": 7,
		},
	}
	payload, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatal(err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	if response["k"] != "r" || response["reqId"] != "v2-1" {
		t.Fatalf("response = %v", response)
	}
	if fake.subscribeCalls != 1 || fake.subscribeCols != 120 || fake.subscribeRows != 40 {
		t.Fatalf("subscribe = %d (%dx%d)", fake.subscribeCalls, fake.subscribeCols, fake.subscribeRows)
	}
}

func TestEncodeLifecycleEvidenceExposesOnlyPublicCorrelation(t *testing.T) {
	b := encodeServerEvent(proto.EvtLifecycleOutcome{RevisionOutcome: proto.RevisionOutcome{
		Correlation: proto.PublicCorrelation{ClientInstanceID: "c1", ConnectionGeneration: 2, ClientRevision: 7},
		Status:      proto.RevisionApplied,
	}})
	if string(b) == "" || string(b) == "null" {
		t.Fatal("lifecycle outcome was not encoded")
	}
	if string(b) == "" || containsAny(string(b), "owner", "ticket", "nonce", "ipc") {
		t.Fatalf("private lifecycle material leaked: %s", b)
	}
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
