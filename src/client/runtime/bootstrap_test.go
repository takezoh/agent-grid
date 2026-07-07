package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/driver"
	"github.com/takezoh/agent-grid/client/state"
)

func TestRecoverWarmStartSessions_ReinstallsTranscriptWatch(t *testing.T) {
	watcher := &recordingWatcher{}
	persist := &recordingPersist{}
	r := New(Config{
		TickInterval: 10 * time.Second,
		Backend:      newFakeBackend(),
		Watcher:      watcher,
		Persist:      persist,
	})
	now := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	d := driver.NewCodexDriver("/tmp/events")
	r.state.Sessions["s1"] = state.Session{
		ID:        "s1",
		Project:   "/repo",
		CreatedAt: now,
		Frames: []state.SessionFrame{{
			ID:        "f1",
			Project:   "/repo",
			Command:   "codex",
			CreatedAt: now,
			Driver: d.Restore(map[string]string{
				"transcript_path":  "/tmp/t.jsonl",
				"codex_session_id": "sess-1",
			}, now),
		}},
	}

	r.RecoverWarmStartSessions()

	watcher.mu.Lock()
	gotPath := watcher.watches["f1"]
	watcher.mu.Unlock()
	if gotPath != "/tmp/t.jsonl" {
		t.Fatalf("watch path = %q, want /tmp/t.jsonl", gotPath)
	}
	if len(r.state.Jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(r.state.Jobs))
	}
	got := r.state.Sessions["s1"].Frames[0].Driver.(driver.CodexState)
	if !got.TranscriptInFlight {
		t.Fatal("TranscriptInFlight should be true")
	}
	if persist.saves == 0 {
		t.Fatal("expected persist on rehydrate")
	}
}

func TestLoadSnapshot_ColdStartConvertsRunningToWaiting(t *testing.T) {
	snaps := []SessionSnapshot{
		{
			ID: "s1",
			Frames: []SessionFrameSnapshot{
				{
					ID:      "f1",
					Command: "generic",
					DriverState: map[string]string{
						"status": "running",
					},
				},
				{
					ID:      "f2",
					Command: "generic",
					DriverState: map[string]string{
						"status": "waiting",
					},
				},
			},
			FrameMessaging: &SessionFrameMessagingSnapshot{
				Summary: FrameMessagingSummarySnapshot{
					UnreadCount:          1,
					LatestMessagePreview: "Need review",
					PendingDeliveryCount: 0,
				},
				Messages: []FrameMessageSnapshot{{
					ID:            "m1",
					SourceFrameID: "f1",
					TargetFrameID: "f2",
					Body:          "hello",
					CreatedAt:     "2026-07-06T00:00:00Z",
				}},
			},
		},
	}
	persist := &snapLoader{snaps: snaps}
	r := New(Config{
		Persist: persist,
	})

	// Cold start: should convert to waiting
	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	s1 := r.state.Sessions["s1"]
	drv := state.GetDriver("generic")
	if drv.Status(s1.Driver) != state.StatusWaiting {
		t.Errorf("Cold start status = %v, want waiting", drv.Status(s1.Driver))
	}
	if s1.FrameMessaging == nil || len(s1.FrameMessaging.Messages) != 1 {
		t.Fatalf("FrameMessaging not restored on cold start: %+v", s1.FrameMessaging)
	}
	if !s1.FrameMessaging.Messages[0].CreatedAt.Equal(time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("message CreatedAt = %v, want 2026-07-06T00:00:00Z", s1.FrameMessaging.Messages[0].CreatedAt)
	}

	// Reset and try warm start with a fresh snap map
	r.state.Sessions = make(map[state.SessionID]state.Session)
	persist.snaps = []SessionSnapshot{
		{
			ID: "s1",
			Frames: []SessionFrameSnapshot{
				{
					ID:      "f1",
					Command: "generic",
					DriverState: map[string]string{
						"status": "running",
					},
				},
				{
					ID:      "f2",
					Command: "generic",
					DriverState: map[string]string{
						"status": "waiting",
					},
				},
			},
			FrameMessaging: &SessionFrameMessagingSnapshot{
				Summary: FrameMessagingSummarySnapshot{
					UnreadCount:          3,
					LatestMessagePreview: "Warm",
					PendingDeliveryCount: 1,
				},
				Messages: []FrameMessageSnapshot{
					{
						ID:            "m1",
						SourceFrameID: "f2",
						TargetFrameID: "f1",
						Body:          "Warm one",
						CreatedAt:     "2026-07-06T00:00:00Z",
					},
					{
						ID:             "m2",
						SourceFrameID:  "f2",
						TargetFrameID:  "f1",
						Body:           "Warm two",
						CreatedAt:      "2026-07-06T00:01:00Z",
						DeliveryStatus: "pending",
					},
					{
						ID:            "m3",
						SourceFrameID: "f2",
						TargetFrameID: "f1",
						Body:          "Warm",
						CreatedAt:     "2026-07-06T00:02:00Z",
					},
				},
			},
		},
	}
	if err := r.LoadSnapshot(false); err != nil {
		t.Fatalf("LoadSnapshot(false): %v", err)
	}
	s1 = r.state.Sessions["s1"]
	if drv.Status(s1.Driver) != state.StatusRunning {
		t.Errorf("Warm start status = %v, want running", drv.Status(s1.Driver))
	}
	if s1.FrameMessaging == nil || s1.FrameMessaging.Summary.UnreadCount != 3 {
		t.Fatalf("FrameMessaging not restored on warm start: %+v", s1.FrameMessaging)
	}
}

func TestLoadSnapshot_RestoresFrameMessagingReplyAndReadState(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "s1",
		Frames: []SessionFrameSnapshot{
			{
				ID:      "f1",
				Command: "generic",
				DriverState: map[string]string{
					"status": "waiting",
				},
			},
			{
				ID:      "f2",
				Command: "generic",
				DriverState: map[string]string{
					"status": "waiting",
				},
			},
			{
				ID:      "f3",
				Command: "generic",
				DriverState: map[string]string{
					"status": "waiting",
				},
			},
		},
		FrameMessaging: &SessionFrameMessagingSnapshot{
			Summary: FrameMessagingSummarySnapshot{
				UnreadCount:          1,
				LatestMessagePreview: "Need review",
				LatestReplyPreview:   "done",
				PendingDeliveryCount: 0,
				LastDeliveryStatus:   "delivered",
			},
			Messages: []FrameMessageSnapshot{
				{
					ID:             "m1",
					SourceFrameID:  "f1",
					TargetFrameID:  "f2",
					Body:           "first",
					CreatedAt:      "2026-07-06T00:00:00Z",
					Read:           true,
					ReplyStatus:    "resolved",
					DeliveryStatus: "delivered",
					Reply: &FrameReplySnapshot{
						ID:                 "r1",
						SourceFrameID:      "f2",
						Body:               "done",
						CreatedAt:          "2026-07-06T00:01:00Z",
						Resolution:         "resolved",
						FinalAnswerPreview: "done",
					},
				},
				{
					ID:            "m2",
					SourceFrameID: "f3",
					TargetFrameID: "f1",
					Body:          "second",
					CreatedAt:     "2026-07-06T00:02:00Z",
				},
			},
		},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}

	s1 := r.state.Sessions["s1"]
	if s1.FrameMessaging == nil {
		t.Fatal("FrameMessaging not restored")
	}
	if got := s1.FrameMessaging.Summary.UnreadCount; got != 1 {
		t.Fatalf("UnreadCount = %d, want 1", got)
	}
	if got := s1.FrameMessaging.Summary.LatestReplyPreview; got != "done" {
		t.Fatalf("LatestReplyPreview = %q, want done", got)
	}
	if len(s1.FrameMessaging.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(s1.FrameMessaging.Messages))
	}
	if !s1.FrameMessaging.Messages[0].Read {
		t.Fatal("first message read state not restored")
	}
	if s1.FrameMessaging.Messages[1].Read {
		t.Fatal("second message should remain unread")
	}
	if s1.FrameMessaging.Messages[0].Reply == nil {
		t.Fatal("reply not restored")
	}
	if got := s1.FrameMessaging.Messages[0].Reply.FinalAnswerPreview; got != "done" {
		t.Fatalf("FinalAnswerPreview = %q, want done", got)
	}
	if got := s1.FrameMessaging.Messages[0].Reply.Resolution; got != "resolved" {
		t.Fatalf("Resolution = %q, want resolved", got)
	}
}

func TestLoadSnapshot_ColdStartFiltersGhostFrameMessages(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "s1",
		Frames: []SessionFrameSnapshot{
			{
				ID:      "f-live",
				Command: "generic",
				DriverState: map[string]string{
					"status": "waiting",
				},
			},
			{
				ID:      "f-dead",
				Command: "generic",
				DriverState: map[string]string{
					"status": "stopped",
				},
			},
		},
		FrameMessaging: &SessionFrameMessagingSnapshot{
			Messages: []FrameMessageSnapshot{
				{
					ID:            "m-dead-src",
					SourceFrameID: "f-dead",
					TargetFrameID: "f-live",
					Body:          "from dead frame",
					CreatedAt:     "2026-07-06T00:00:00Z",
				},
				{
					ID:            "m-dead-target",
					SourceFrameID: "f-live",
					TargetFrameID: "f-dead",
					Body:          "to dead frame",
					CreatedAt:     "2026-07-06T00:01:00Z",
				},
			},
		},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}

	s1 := r.state.Sessions["s1"]
	if len(s1.Frames) != 1 || s1.Frames[0].ID != "f-live" {
		t.Fatalf("restored frames = %+v, want only f-live", s1.Frames)
	}
	if s1.FrameMessaging == nil {
		t.Fatal("FrameMessaging not restored")
	}
	if len(s1.FrameMessaging.Messages) != 0 {
		t.Fatalf("messages len = %d, want 0 after ghost filtering", len(s1.FrameMessaging.Messages))
	}
	if s1.FrameMessaging.Summary.UnreadCount != 0 {
		t.Fatalf("UnreadCount = %d, want 0", s1.FrameMessaging.Summary.UnreadCount)
	}
}

type snapLoader struct {
	noopPersist
	snaps   []SessionSnapshot
	deleted []string
}

func (s *snapLoader) Load() ([]SessionSnapshot, error) {
	return s.snaps, nil
}

func (s *snapLoader) Delete(id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

// codexThreadID is a representative resumable thread id (alphanumeric+hyphen),
// matching the format codex persists for a started/resumed thread.
const codexThreadID = "019e727e-fde4-7432-9036-ae6604ce1b27"

// TestLoadSnapshot_ColdStartKeepsRecoverableStoppedCodexFrame guards the cold
// start regression where a stopped codex session was dropped (and deleted from
// disk) even though its conversation lives in a host-mounted thread that can be
// resumed against a fresh app-server. Codex implements ColdStartRecoverer, so a
// stopped frame with a resumable thread must survive cold start.
func TestLoadSnapshot_ColdStartKeepsRecoverableStoppedCodexFrame(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "codex-sess",
		Frames: []SessionFrameSnapshot{{
			ID:      "f1",
			Command: "codex",
			DriverState: map[string]string{
				"status":    "stopped",
				"thread_id": codexThreadID,
			},
		}},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	sess, ok := r.state.Sessions["codex-sess"]
	if !ok {
		t.Fatal("recoverable stopped codex session dropped on cold start; want kept for thread resume")
	}
	if len(sess.Frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(sess.Frames))
	}
	for _, id := range persist.deleted {
		if id == "codex-sess" {
			t.Error("recoverable snapshot must not be deleted from disk")
		}
	}
}

// TestLoadSnapshot_ColdStartDropsStoppedCodexFrameWithoutLocator ensures the
// recovery is gated on an actual resume locator: with no thread/session/path
// there is nothing to resume, so the stopped frame is dropped like any other.
func TestLoadSnapshot_ColdStartDropsStoppedCodexFrameWithoutLocator(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "codex-nothread",
		Frames: []SessionFrameSnapshot{{
			ID:          "f1",
			Command:     "codex",
			DriverState: map[string]string{"status": "stopped"},
		}},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	if _, ok := r.state.Sessions["codex-nothread"]; ok {
		t.Error("stopped codex frame without a resume locator should be dropped on cold start")
	}
}

func TestLoadSnapshot_ColdStartKeepsStoppedCodexFrameWithRolloutPath(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "codex-rollout",
		Frames: []SessionFrameSnapshot{{
			ID:      "f1",
			Command: "codex",
			DriverState: map[string]string{
				"status":       "stopped",
				"rollout_path": "/repo/rollout.jsonl",
			},
		}},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	if _, ok := r.state.Sessions["codex-rollout"]; !ok {
		t.Error("stopped codex frame with rollout_path must remain after cold start")
	}
	for _, id := range persist.deleted {
		if id == "codex-rollout" {
			t.Error("codex snapshot with rollout_path must not be deleted from disk")
		}
	}
}

// TestLoadSnapshot_ColdStartDropsStoppedGenericFrame ensures the default policy
// is unchanged for drivers without durable state: a stopped frame is dropped.
func TestLoadSnapshot_ColdStartDropsStoppedGenericFrame(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "generic-sess",
		Frames: []SessionFrameSnapshot{{
			ID:          "f1",
			Command:     "generic",
			DriverState: map[string]string{"status": "stopped"},
		}},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	if _, ok := r.state.Sessions["generic-sess"]; ok {
		t.Error("stopped generic frame (no durable state) must still be dropped on cold start")
	}
}

func TestLoadSnapshot_ColdStartDropDeletesFrameMessagingStore(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "frame-messaging", "generic-sess")
	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "messages.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "generic-sess",
		Frames: []SessionFrameSnapshot{{
			ID:          "f1",
			Command:     "generic",
			DriverState: map[string]string{"status": "stopped"},
		}},
	}}}
	r := New(Config{Persist: persist, DataDir: dir})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	if _, err := os.Stat(storeDir); !os.IsNotExist(err) {
		t.Fatalf("frame messaging store still exists, err=%v", err)
	}
}

// TestLoadSnapshot_WarmStartKeepsStoppedCodexFrame ensures warm start is
// unaffected — it keeps every frame, recoverable or not, since the live backend
// frame is still attached for inspection.
func TestLoadSnapshot_WarmStartKeepsStoppedCodexFrame(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "codex-warm",
		Frames: []SessionFrameSnapshot{{
			ID:          "f1",
			Command:     "codex",
			DriverState: map[string]string{"status": "stopped"},
		}},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(false); err != nil {
		t.Fatalf("LoadSnapshot(false): %v", err)
	}
	if _, ok := r.state.Sessions["codex-warm"]; !ok {
		t.Error("warm start must keep stopped frames (dead frame still attached for inspection)")
	}
}
