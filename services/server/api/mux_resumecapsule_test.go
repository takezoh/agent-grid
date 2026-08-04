package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takezoh/agent-grid/services/host/proto"
	"github.com/takezoh/agent-grid/services/host/state"
)

func TestMux_ListResumeCapsules(t *testing.T) {
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
		if ev.Event != state.EventListResumeCapsules {
			t.Errorf("event = %q, want %q", ev.Event, state.EventListResumeCapsules)
		}
		daemon.sendResp(env.ReqID, proto.RespResumeCapsules{ResumeCapsules: []proto.ResumeCapsuleInfo{{
			SessionID:    "s1",
			Project:      "/repo",
			Objective:    "ship it",
			CurrentState: "tests green",
			NextAction:   "commit",
			GeneratedAt:  "2026-08-04T00:00:00Z",
			SourceTurns:  4,
		}}})
	}()

	req := protocolRequest(http.MethodGet, "/api/resume-capsules", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	<-done

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got struct {
		ResumeCapsules []map[string]any `json:"resume_capsules"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(got.ResumeCapsules) != 1 {
		t.Fatalf("resume_capsules len = %d, want 1", len(got.ResumeCapsules))
	}
	if got.ResumeCapsules[0]["session_id"] != "s1" || got.ResumeCapsules[0]["objective"] != "ship it" {
		t.Errorf("unexpected capsule payload: %+v", got.ResumeCapsules[0])
	}
}

func TestMux_ListResumeCapsulesEmptyIsArray(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	daemon.pumpResponses(func(reqID string) {
		daemon.sendResp(reqID, proto.RespResumeCapsules{})
	})

	req := protocolRequest(http.MethodGet, "/api/resume-capsules", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"resume_capsules":[]`)) {
		t.Fatalf("empty list must serialize as [], got %s", w.Body.String())
	}
}

func TestMux_FlagResumeCapsule(t *testing.T) {
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
		if ev.Event != state.EventFlagResumeCapsule {
			t.Errorf("event = %q, want %q", ev.Event, state.EventFlagResumeCapsule)
		}
		var params state.ResumeCapsuleFlagParams
		if err := json.Unmarshal(ev.Payload, &params); err != nil {
			t.Errorf("unmarshal payload: %v", err)
			return
		}
		if params.SessionID != "s1" || !params.Flagged {
			t.Errorf("params = %+v, want s1/flagged", params)
		}
		daemon.sendResp(env.ReqID, proto.RespOK{})
	}()

	// Empty body defaults to flagged=true (the one-action wrong-flag).
	req := protocolRequest(http.MethodPost, "/api/sessions/s1/resume-capsule/flag", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	<-done

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestMux_FlagResumeCapsuleInvalidSessionID(t *testing.T) {
	t.Parallel()
	d, _ := newDaemonPair(t)
	mux := NewMux(d, "tok")

	req := protocolRequest(http.MethodPost, "/api/sessions/bad%2Fid/resume-capsule/flag", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
