package runtime

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/fsnotify/fsnotify"
	"github.com/takezoh/agent-grid/client/driver"
	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/state"
)

// newTestRelayAttached builds a Runtime with a standalone FileRelay
// (no background goroutines) attached for inspecting registration.
func newTestRelayAttached(t *testing.T) (*Runtime, *FileRelay) {
	t.Helper()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })

	fr := &FileRelay{
		watcher: w,
		files:   map[string]*relayFile{},
	}
	r := New(Config{
		Backend: newFakeBackend(),
	})
	r.relay = fr
	return r, fr
}

// TestSyncRelayWatchesRegistersNewSessionLogTabs verifies that
// syncRelayWatches registers all LogTab paths from a newly-injected
// session into the FileRelay. This is the core fix for the bug where
// sessions created at runtime had their log tabs excluded from push updates.
func TestSyncRelayWatchesRegistersNewSessionLogTabs(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")

	r, fr := newTestRelayAttached(t)

	// Inject a session directly into runtime state — mimics a session that
	// was created after SetRelay ran (internalSetRelay would miss it).
	sessID := state.SessionID("sess-1")
	r.state.Sessions = map[state.SessionID]state.Session{
		sessID: {
			ID:        sessID,
			Command:   "codex",
			CreatedAt: time.Now(),
			Driver: driver.CodexState{
				CommonState: driver.CommonState{
					TranscriptPath: transcriptPath,
				},
			},
		},
	}

	r.syncRelayWatches()

	if _, ok := fr.files[transcriptPath]; !ok {
		t.Errorf("syncRelayWatches did not register transcript path %s", transcriptPath)
	}
}

// TestSyncRelayWatchesNoRelayIsNoop verifies that syncRelayWatches is
// safe to call when no FileRelay has been attached.
func TestSyncRelayWatchesNoRelayIsNoop(t *testing.T) {
	r := New(Config{
		Backend: newFakeBackend(),
	})
	// r.relay == nil; must not panic
	r.syncRelayWatches()
}

// newTestRuntimeWithConns creates a Runtime with fake connections pre-wired.
// Returns the runtime and a map of ConnID → outbox channel for assertions.
func newTestRuntimeWithConns(t *testing.T, ids ...state.ConnID) (*Runtime, map[state.ConnID]chan []byte) {
	t.Helper()
	r := New(Config{
		Backend: newFakeBackend(),
	})
	outboxes := make(map[state.ConnID]chan []byte, len(ids))
	for _, id := range ids {
		srv, _ := net.Pipe()
		t.Cleanup(func() { srv.Close() })
		cc := newIPCConn(id, srv)
		r.conns[id] = cc
		outboxes[id] = cc.outbox
	}
	return r, outboxes
}

// decodeSurfaceOutput decodes a raw wire frame from the outbox into EvtSurfaceOutput.
func decodeSurfaceOutput(t *testing.T, wire []byte) proto.EvtSurfaceOutput {
	t.Helper()
	var env proto.Envelope
	if err := json.Unmarshal(wire, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	var out proto.EvtSurfaceOutput
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatalf("decode EvtSurfaceOutput: %v", err)
	}
	return out
}

// TestBroadcastSurfaceOutput_PerSessionSubsOnly verifies that broadcastSurfaceOutput
// delivers to connIDs subscribed to the target SessionID only.
func TestBroadcastSurfaceOutput_PerSessionSubsOnly(t *testing.T) {
	r, outboxes := newTestRuntimeWithConns(t, 1, 2, 3)

	// s1 has connIDs 1 and 2; s2 has connID 3.
	r.state.SurfaceSubs = map[state.ConnID]map[state.SessionID]struct{}{
		1: {"s1": {}},
		2: {"s1": {}},
		3: {"s2": {}},
	}

	r.broadcastSurfaceOutput(state.EffBroadcastSurfaceOutput{
		SessionID: "s1",
		Data:      []byte("hi"),
		TimeSec:   0.5,
	})

	// ConnID 1 and 2 must have received a message.
	for _, id := range []state.ConnID{1, 2} {
		select {
		case msg := <-outboxes[id]:
			out := decodeSurfaceOutput(t, msg)
			if out.DataB64 != base64.StdEncoding.EncodeToString([]byte("hi")) {
				t.Errorf("conn %d: unexpected DataB64 %q", id, out.DataB64)
			}
			if out.SessionID != "s1" {
				t.Errorf("conn %d: unexpected SessionID %q", id, out.SessionID)
			}
		default:
			t.Errorf("conn %d: expected message in outbox but got none", id)
		}
	}

	// ConnID 3 must NOT have received anything.
	select {
	case <-outboxes[3]:
		t.Error("conn 3 received unexpected surface output")
	default:
	}
}

// TestBroadcastSurfaceFromInternal_SingleConn verifies that broadcastSurfaceFromInternal
// delivers exactly one message to the specified ConnID with correct fields.
func TestBroadcastSurfaceFromInternal_SingleConn(t *testing.T) {
	r, outboxes := newTestRuntimeWithConns(t, 1, 2)

	r.broadcastSurfaceFromInternal(internalBroadcastSurface{
		ConnID:    1,
		SessionID: "s1",
		Data:      []byte("ab"),
		Sequence:  2,
		TimeSec:   1.0,
	})

	// ConnID 1 must receive exactly one message.
	select {
	case msg := <-outboxes[1]:
		out := decodeSurfaceOutput(t, msg)
		wantB64 := base64.StdEncoding.EncodeToString([]byte("ab"))
		if out.DataB64 != wantB64 {
			t.Errorf("DataB64: got %q want %q", out.DataB64, wantB64)
		}
		if out.Sequence != 2 {
			t.Errorf("Sequence: got %d want 2", out.Sequence)
		}
		if out.SessionID != "s1" {
			t.Errorf("SessionID: got %q want %q", out.SessionID, "s1")
		}
	default:
		t.Error("conn 1: expected message in outbox but got none")
	}

	// ConnID 2 must NOT receive anything.
	select {
	case <-outboxes[2]:
		t.Error("conn 2 received unexpected surface output")
	default:
	}
}

// TestBroadcastPromptEvent_PerSessionSubs verifies that broadcastPromptEvent
// delivers EvtPromptEvent only to ConnIDs subscribed to the frame's session.
func TestBroadcastPromptEvent_PerSessionSubs(t *testing.T) {
	r, outboxes := newTestRuntimeWithConns(t, 1, 2, 3)

	// Session "s1" has frame "f1"; session "s2" has frame "f2".
	r.state.Sessions = map[state.SessionID]state.Session{
		"s1": {
			ID:      "s1",
			Command: "codex",
			Frames: []state.SessionFrame{
				{ID: "f1"},
			},
			CreatedAt: time.Now(),
		},
		"s2": {
			ID:      "s2",
			Command: "codex",
			Frames: []state.SessionFrame{
				{ID: "f2"},
			},
			CreatedAt: time.Now(),
		},
	}

	// ConnID 1 subscribed to s1; connID 2 subscribed to s1; connID 3 to s2.
	r.state.SurfaceSubs = map[state.ConnID]map[state.SessionID]struct{}{
		1: {"s1": {}},
		2: {"s1": {}},
		3: {"s2": {}},
	}

	r.broadcastPromptEvent(state.EffBroadcastPromptEvent{
		FrameID:  "f1",
		Phase:    "end",
		ExitCode: 0,
	})

	// ConnIDs 1 and 2 must receive the prompt event.
	for _, id := range []state.ConnID{1, 2} {
		select {
		case msg := <-outboxes[id]:
			var env proto.Envelope
			if err := json.Unmarshal(msg, &env); err != nil {
				t.Fatalf("conn %d: decode envelope: %v", id, err)
			}
			if env.Name != proto.EvtNamePromptEvent {
				t.Errorf("conn %d: expected event %q, got %q", id, proto.EvtNamePromptEvent, env.Name)
			}
			var ev proto.EvtPromptEvent
			if err := json.Unmarshal(env.Data, &ev); err != nil {
				t.Fatalf("conn %d: decode EvtPromptEvent: %v", id, err)
			}
			if ev.FrameID != "f1" {
				t.Errorf("conn %d: FrameID: got %q want %q", id, ev.FrameID, "f1")
			}
			if ev.Phase != "end" {
				t.Errorf("conn %d: Phase: got %q want %q", id, ev.Phase, "end")
			}
		default:
			t.Errorf("conn %d: expected prompt event but got none", id)
		}
	}

	// ConnID 3 must NOT receive anything.
	select {
	case <-outboxes[3]:
		t.Error("conn 3 received unexpected prompt event")
	default:
	}
}

func TestStateMessagePreview_TruncatesByRune(t *testing.T) {
	body := strings.Repeat("あ", 119) + "終端"

	preview := stateMessagePreview(body)

	if !utf8.ValidString(preview) {
		t.Fatal("preview must remain valid UTF-8")
	}
	if got := utf8.RuneCountInString(preview); got != 120 {
		t.Fatalf("preview rune count = %d, want 120", got)
	}
	if !strings.HasSuffix(preview, "...") {
		t.Fatalf("preview = %q, want suffix ...", preview)
	}
	if strings.ContainsRune(preview, '終') {
		t.Fatalf("preview must be truncated before trailing runes: %q", preview)
	}
}

func TestBuildSessionMessagesIncludesReplyMetadataOnly(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	replyAt := now.Add(time.Minute)
	r := New(Config{Backend: newFakeBackend()})
	r.state.Sessions = map[state.SessionID]state.Session{
		"s1": {
			ID:        "s1",
			CreatedAt: now,
			FrameMessaging: &state.SessionFrameMessaging{
				Summary: state.FrameMessagingSummary{
					UnreadCount:          1,
					LatestMessagePreview: "Need review",
					LatestReplyPreview:   "done",
					PendingDeliveryCount: 0,
					LastDeliveryStatus:   "delivered",
				},
				Messages: []state.FrameMessage{{
					ID:             "m1",
					SourceFrameID:  "frame-a",
					TargetFrameID:  "frame-b",
					Topic:          "Review",
					Body:           "Please review this patch",
					CreatedAt:      now,
					Read:           true,
					DeliveryStatus: "delivered",
					Reply: &state.FrameReply{
						ID:                 "r1",
						SourceFrameID:      "frame-b",
						Body:               "done",
						CreatedAt:          replyAt,
						Resolution:         "resolved",
						FinalAnswerPreview: "done",
					},
				}},
			},
		},
	}

	resp := r.buildSessionMessages(state.SessionMessagesReply{SessionID: "s1"})
	got, ok := resp.(proto.RespSessionMessages)
	if !ok {
		t.Fatalf("response = %T, want proto.RespSessionMessages", resp)
	}
	if got.Summary == nil {
		t.Fatal("summary missing")
	}
	if got.Summary.UnreadCount != 1 {
		t.Fatalf("UnreadCount = %d, want 1", got.Summary.UnreadCount)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(got.Messages))
	}
	msg := got.Messages[0]
	if msg.Reply == nil {
		t.Fatal("reply missing from response")
	}
	if msg.ReplyStatus != "resolved" {
		t.Fatalf("ReplyStatus = %q, want resolved", msg.ReplyStatus)
	}
	if msg.FinalAnswerPreview != "done" {
		t.Fatalf("FinalAnswerPreview = %q, want done", msg.FinalAnswerPreview)
	}
	if msg.Reply.BodyPreview != "done" {
		t.Fatalf("BodyPreview = %q, want done", msg.Reply.BodyPreview)
	}
	if msg.Reply.CreatedAt != "2026-07-06T00:01:00Z" {
		t.Fatalf("Reply.CreatedAt = %q, want 2026-07-06T00:01:00Z", msg.Reply.CreatedAt)
	}

	replyType := reflect.TypeOf(*msg.Reply)
	if _, ok := replyType.FieldByName("RawTranscript"); ok {
		t.Fatal("SessionMessageReply must not expose RawTranscript")
	}
	msgType := reflect.TypeOf(msg)
	if _, ok := msgType.FieldByName("RawTranscript"); ok {
		t.Fatal("SessionMessage must not expose RawTranscript")
	}
}
