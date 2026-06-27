package state

import (
	"testing"
)

// makeSessionWithFrames creates a state with one session that has two frames:
// root (f1) and a child (f2), with f1 as the active frame.
func makeSessionWithFrames() State {
	s := New()
	s.Sessions["s1"] = Session{
		ID:          "s1",
		Project:     "/foo",
		Command:     "stub",
		Driver:      stubDriverState{},
		HeadFrameID: "f1",
		Frames: []SessionFrame{
			{ID: "f1", Project: "/foo", Command: "stub", Driver: stubDriverState{}},
			{ID: "f2", Project: "/foo", Command: "alt", Driver: stubDriverState{}},
		},
	}
	return s
}

// TestSetHeadFrameSameFrameIsNoOp verifies that activating the already-active
// frame is a no-op: only okResp, no persist / broadcast / mutation.
func TestSetHeadFrameSameFrameIsNoOp(t *testing.T) {
	s := makeSessionWithFrames()

	next, effs := Reduce(s, EvEvent{
		ConnID: 1, ReqID: "r", Event: "set-head-frame",
		Payload: mustPayload(map[string]string{"session_id": "s1", "frame_id": "f1"}),
	})
	if sess := next.Sessions["s1"]; sess.HeadFrameID != "f1" {
		t.Errorf("HeadFrameID = %q, want f1 (unchanged)", sess.HeadFrameID)
	}
	if n := countEff[EffPersistSnapshot](effs); n != 0 {
		t.Errorf("EffPersistSnapshot = %d, want 0", n)
	}
	if n := countEff[EffBroadcastSessionsChanged](effs); n != 0 {
		t.Errorf("EffBroadcastSessionsChanged = %d, want 0", n)
	}
	mustOK(t, effs)
}

// TestActivateDifferentFrameUpdatesAndPersistsAndBroadcasts verifies that
// activating a different frame updates HeadFrameID, persists the snapshot,
// and broadcasts EvtSessionsChanged.
func TestActivateDifferentFrameUpdatesAndPersistsAndBroadcasts(t *testing.T) {
	s := makeSessionWithFrames()

	next, effs := Reduce(s, EvEvent{
		ConnID: 1, ReqID: "r", Event: "set-head-frame",
		Payload: mustPayload(map[string]string{"session_id": "s1", "frame_id": "f2"}),
	})
	if sess := next.Sessions["s1"]; sess.HeadFrameID != "f2" {
		t.Errorf("HeadFrameID = %q, want f2", sess.HeadFrameID)
	}
	if _, ok := findEff[EffPersistSnapshot](effs); !ok {
		t.Error("expected EffPersistSnapshot")
	}
	if _, ok := findEff[EffBroadcastSessionsChanged](effs); !ok {
		t.Error("expected EffBroadcastSessionsChanged")
	}
	mustOK(t, effs)
}
