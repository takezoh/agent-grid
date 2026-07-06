package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/driver"
	"github.com/takezoh/agent-grid/client/state"
)

func TestFilePersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := NewFilePersist(dir)

	want := []SessionSnapshot{
		{
			ID:        "abc",
			Project:   "/foo",
			CreatedAt: "2026-04-10T12:00:00Z",
			Frames: []SessionFrameSnapshot{{
				ID:          "f1",
				SubsystemID: "cli:f1",
				TargetID:    "f1",
				Project:     "/foo",
				Command:     "claude",
				CreatedAt:   "2026-04-10T12:00:00Z",
				Driver:      "claude",
				DriverState: map[string]string{
					"session_id": "uuid",
				},
			}},
			FrameMessaging: &SessionFrameMessagingSnapshot{
				Summary: FrameMessagingSummarySnapshot{
					UnreadCount:          1,
					LatestMessagePreview: "Need review",
					PendingDeliveryCount: 1,
					LastDeliveryStatus:   "pending",
				},
				Messages: []FrameMessageSnapshot{{
					ID:             "m1",
					SourceFrameID:  "f1",
					TargetFrameID:  "f2",
					Body:           "hello",
					CreatedAt:      "2026-04-10T12:01:00Z",
					ReplyStatus:    "pending",
					DeliveryStatus: "pending",
					Reply: &FrameReplySnapshot{
						ID:                 "r1",
						SourceFrameID:      "f2",
						Body:               "done",
						CreatedAt:          "2026-04-10T12:02:00Z",
						Resolution:         "resolved",
						FinalAnswerPreview: "done",
					},
				}},
			},
		},
	}
	if err := p.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Per-session file exists
	if _, err := os.Stat(filepath.Join(dir, "sessions", "abc.json")); err != nil {
		t.Errorf("abc.json not created: %v", err)
	}
	// No .tmp left over
	if _, err := os.Stat(filepath.Join(dir, "sessions", "abc.json.tmp")); !os.IsNotExist(err) {
		t.Errorf("temp file leaked: %v", err)
	}

	got, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != "abc" || got[0].Frames[0].Driver != "claude" {
		t.Errorf("got = %+v", got[0])
	}
	if got[0].Frames[0].DriverState["session_id"] != "uuid" {
		t.Errorf("DriverState[session_id] = %q", got[0].Frames[0].DriverState["session_id"])
	}
	if got[0].Frames[0].SubsystemID != "cli:f1" || got[0].Frames[0].TargetID != "f1" {
		t.Errorf("frame identity = %+v", got[0].Frames[0])
	}
	if got[0].FrameMessaging == nil || got[0].FrameMessaging.Messages[0].Reply == nil {
		t.Fatalf("FrameMessaging did not round-trip: %+v", got[0].FrameMessaging)
	}
	if got[0].FrameMessaging.Messages[0].Reply.FinalAnswerPreview != "done" {
		t.Errorf("reply preview = %q, want done", got[0].FrameMessaging.Messages[0].Reply.FinalAnswerPreview)
	}
}

func TestSnapshotSessionsPersistsFrameMessaging(t *testing.T) {
	r := New(Config{
		TickInterval: 10 * time.Second,
		Backend:      newFakeBackend(),
		Persist:      noopPersist{},
	})
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	sid := state.SessionID("s-msg")
	r.state.Sessions[sid] = state.Session{
		ID:        sid,
		Project:   "/repo",
		CreatedAt: now,
		Command:   "shell",
		Driver:    driver.NewGenericDriver("shell", "shell", 0).NewState(now),
		Frames: []state.SessionFrame{{
			ID:        "f1",
			Project:   "/repo",
			Command:   "shell",
			CreatedAt: now,
			Driver:    driver.NewGenericDriver("shell", "shell", 0).NewState(now),
		}},
		FrameMessaging: &state.SessionFrameMessaging{
			Summary: state.FrameMessagingSummary{
				UnreadCount:          2,
				LatestMessagePreview: "Need review",
				PendingDeliveryCount: 1,
			},
			Messages: []state.FrameMessage{{
				ID:            "m1",
				SourceFrameID: "f1",
				TargetFrameID: "f2",
				Body:          "hello",
				CreatedAt:     now,
			}},
		},
	}

	snaps := r.snapshotSessions()
	if len(snaps) != 1 || snaps[0].FrameMessaging == nil {
		t.Fatalf("snapshot missing frame messaging: %+v", snaps)
	}
	if snaps[0].FrameMessaging.Summary.UnreadCount != 2 {
		t.Fatalf("UnreadCount = %d, want 2", snaps[0].FrameMessaging.Summary.UnreadCount)
	}
	if snaps[0].FrameMessaging.Messages[0].ID != "m1" {
		t.Fatalf("message id = %q, want m1", snaps[0].FrameMessaging.Messages[0].ID)
	}
}

func TestFilePersistLoadMissing(t *testing.T) {
	dir := t.TempDir()
	p := NewFilePersist(dir)
	got, err := p.Load()
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestFilePersistSaveEmpty(t *testing.T) {
	dir := t.TempDir()
	p := NewFilePersist(dir)
	if err := p.Save(nil); err != nil {
		t.Fatalf("Save nil: %v", err)
	}
	got, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFilePersistExplicitDeleteRemovesFile(t *testing.T) {
	dir := t.TempDir()
	p := NewFilePersist(dir)

	if err := p.Save([]SessionSnapshot{
		{ID: "s1", Project: "/a", Frames: []SessionFrameSnapshot{{ID: "f1", Project: "/a", Command: "claude"}}},
		{ID: "s2", Project: "/b", Frames: []SessionFrameSnapshot{{ID: "f2", Project: "/b", Command: "claude"}}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := p.Delete("s2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "sessions", "s2.json")); !os.IsNotExist(err) {
		t.Error("s2.json should have been deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "sessions", "s1.json")); err != nil {
		t.Errorf("s1.json missing: %v", err)
	}
}

// Save must be upsert-only: a session absent from the input list
// must NOT be removed from disk. Removal is explicit via Delete.
// This guards against the catastrophic-loss path where a transient
// empty in-memory state would otherwise wipe the directory.
func TestFilePersistSaveDoesNotPrune(t *testing.T) {
	dir := t.TempDir()
	p := NewFilePersist(dir)

	if err := p.Save([]SessionSnapshot{
		{ID: "keep", Project: "/k", Frames: []SessionFrameSnapshot{{ID: "fk", Project: "/k", Command: "claude"}}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := p.Save(nil); err != nil {
		t.Fatalf("Save(nil): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sessions", "keep.json")); err != nil {
		t.Errorf("keep.json wiped by empty Save: %v", err)
	}
}

func TestFilePersistMultipleSessions(t *testing.T) {
	dir := t.TempDir()
	p := NewFilePersist(dir)

	sessions := []SessionSnapshot{
		{ID: "aaa", Project: "/p1", Frames: []SessionFrameSnapshot{{ID: "fa", Project: "/p1", Command: "claude"}}},
		{ID: "bbb", Project: "/p2", Frames: []SessionFrameSnapshot{{ID: "fb", Project: "/p2", Command: "gemini"}}},
		{ID: "ccc", Project: "/p3", Frames: []SessionFrameSnapshot{{ID: "fc", Project: "/p3", Command: "codex"}}},
	}
	if err := p.Save(sessions); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	ids := map[string]bool{}
	for _, s := range got {
		ids[s.ID] = true
	}
	for _, id := range []string{"aaa", "bbb", "ccc"} {
		if !ids[id] {
			t.Errorf("missing session %s", id)
		}
	}
}
