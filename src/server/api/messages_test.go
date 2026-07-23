package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takezoh/agent-grid/host/proto"
	"github.com/takezoh/agent-grid/host/state"
)

func TestMux_GetSessionMessages(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	done := make(chan struct{})
	go func() {
		defer close(done)
		env := daemon.recv()
		cmd, err := proto.DecodeCommand(env)
		if err != nil {
			t.Errorf("decode command: %v", err)
			return
		}
		ev, ok := cmd.(proto.CmdEvent)
		if !ok {
			t.Errorf("command = %T, want proto.CmdEvent", cmd)
			return
		}
		if ev.Event != state.EventListSessionMessages {
			t.Errorf("event = %q, want %q", ev.Event, state.EventListSessionMessages)
		}
		daemon.sendResp(env.ReqID, proto.RespSessionMessages{
			SessionID: "s1",
			Summary: &proto.FrameMessagingSummary{
				UnreadCount:          2,
				PendingDeliveryCount: 1,
				LastDeliveryStatus:   "pending",
			},
			Messages: []proto.SessionMessage{{
				ID:            "m1",
				SourceFrameID: "frame-a",
				TargetFrameID: "frame-b",
				Topic:         "Need review",
				Body:          "Full message body",
				BodyPreview:   "Full message body",
				CreatedAt:     "2026-07-06T00:00:00Z",
				ReplyStatus:   "pending",
			}},
		})
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/s1/messages", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	<-done

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, has := got["raw_transcript"]; has {
		t.Fatal("messages API must not include raw_transcript")
	}
	if got["session_id"] != "s1" {
		t.Fatalf("session_id = %v, want s1", got["session_id"])
	}
	msgs, ok := got["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages = %#v, want len=1", got["messages"])
	}
}

func TestMux_ReadSessionMessages(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	done := make(chan struct{})
	go func() {
		defer close(done)
		env := daemon.recv()
		cmd, err := proto.DecodeCommand(env)
		if err != nil {
			t.Errorf("decode command: %v", err)
			return
		}
		ev, ok := cmd.(proto.CmdEvent)
		if !ok {
			t.Errorf("command = %T, want proto.CmdEvent", cmd)
			return
		}
		if ev.Event != state.EventReadSessionMessages {
			t.Errorf("event = %q, want %q", ev.Event, state.EventReadSessionMessages)
		}
		var params state.SessionMessagesParams
		if err := json.Unmarshal(ev.Payload, &params); err != nil {
			t.Errorf("unmarshal payload: %v", err)
			return
		}
		if params.LastReadMessageID != "m1" {
			t.Errorf("LastReadMessageID = %q, want m1", params.LastReadMessageID)
		}
		daemon.sendResp(env.ReqID, proto.RespOK{})
	}()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/sessions/s1/messages/read",
		bytes.NewBufferString(`{"last_read_message_id":"m1"}`),
	)
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	<-done

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}
