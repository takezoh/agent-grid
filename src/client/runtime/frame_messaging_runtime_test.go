package runtime

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/state"
)

func seedFrameMessagingSession(r *Runtime) state.SessionID {
	sid := state.SessionID("s-msg")
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	r.state.Sessions[sid] = state.Session{
		ID:        sid,
		Project:   "/repo",
		CreatedAt: now,
		Frames: []state.SessionFrame{
			{ID: "frame-claude", Project: "/repo", Command: "claude", CreatedAt: now},
			{ID: "frame-codex", Project: "/repo", Command: "codex", CreatedAt: now},
			{ID: "frame-shell", Project: "/repo", Command: "shell", CreatedAt: now},
		},
		HeadFrameID: "frame-claude",
	}
	return sid
}

func readAuditRecords(t *testing.T, path string) []frameMessagingAuditRecord {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit: %v", err)
	}
	defer f.Close()
	var out []frameMessagingAuditRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var rec frameMessagingAuditRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("unmarshal audit: %v", err)
		}
		out = append(out, rec)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan audit: %v", err)
	}
	return out
}

func TestFrameMessagingBroker_ReplayAndAudit(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{DataDir: dir, Persist: noopPersist{}})
	sid := seedFrameMessagingSession(r)

	sendRes := r.frameMessagingSend("frame-claude", "frame-codex", "Need review", "Please check this", "urgent")
	if sendRes.err != nil {
		t.Fatalf("send: %v", sendRes.err)
	}
	send := sendRes.resp
	if send.Message.Priority != "urgent" {
		t.Fatalf("send priority = %q, want urgent", send.Message.Priority)
	}
	readRes := r.frameMessagingRead("frame-codex", "frame-claude")
	if readRes.err != nil {
		t.Fatalf("read: %v", readRes.err)
	}
	if got := readRes.resp.Messages[0].Priority; got != "urgent" {
		t.Fatalf("read priority = %q, want urgent", got)
	}
	replyRes := r.frameMessagingReply("frame-codex", send.Message.ID, "Done", "Ship it", "resolved", "high")
	if replyRes.err != nil {
		t.Fatalf("reply: %v", replyRes.err)
	}

	rawAudit, err := os.ReadFile(filepath.Join(dir, "frame-messaging", string(sid), "audit.jsonl"))
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	if strings.Contains(string(rawAudit), "Please check this") || strings.Contains(string(rawAudit), "Ship it") {
		t.Fatal("audit file must not store raw message bodies or final answers")
	}
	records := readAuditRecords(t, filepath.Join(dir, "frame-messaging", string(sid), "audit.jsonl"))
	if len(records) < 3 {
		t.Fatalf("audit records = %d, want >= 3", len(records))
	}
	if records[0].BodyHash == "" {
		t.Fatal("send audit must include body hash")
	}

	r2 := New(Config{DataDir: dir, Persist: noopPersist{}})
	sid2 := seedFrameMessagingSession(r2)
	if sid2 != sid {
		t.Fatalf("session seed mismatch: %q vs %q", sid2, sid)
	}
	r2.restoreFrameMessagingStores()
	got := r2.state.Sessions[sid].FrameMessaging
	if got == nil || len(got.Messages) != 1 {
		t.Fatalf("restored messages = %+v, want 1", got)
	}
	if !got.Messages[0].Read {
		t.Fatal("read state was not replayed from messages.jsonl")
	}
	if got.Messages[0].Reply == nil {
		t.Fatal("reply was not replayed from messages.jsonl")
	}
	if got.Messages[0].Reply.FinalAnswer != "Ship it" {
		t.Fatalf("final answer = %q, want Ship it", got.Messages[0].Reply.FinalAnswer)
	}
}

func TestRestoreFrameMessagingStores_FiltersGhostFrameMessages(t *testing.T) {
	r := New(Config{Persist: noopPersist{}})
	sid := state.SessionID("s-msg")
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	r.state.Sessions[sid] = state.Session{
		ID:        sid,
		Project:   "/repo",
		CreatedAt: now,
		Frames: []state.SessionFrame{
			{ID: "frame-live", Project: "/repo", Command: "claude", CreatedAt: now},
		},
		FrameMessaging: &state.SessionFrameMessaging{
			Messages: []state.FrameMessage{
				{
					ID:            "m1",
					SourceFrameID: "frame-live",
					TargetFrameID: "frame-missing",
					Body:          "ghost target",
					CreatedAt:     now,
				},
				{
					ID:            "m2",
					SourceFrameID: "frame-missing",
					TargetFrameID: "frame-live",
					Body:          "ghost source",
					CreatedAt:     now.Add(time.Minute),
				},
			},
		},
	}

	r.restoreFrameMessagingStores()

	got := r.state.Sessions[sid].FrameMessaging
	if got == nil {
		t.Fatal("frame messaging missing")
	}
	if len(got.Messages) != 0 {
		t.Fatalf("messages len = %d, want 0 after ghost filtering", len(got.Messages))
	}
	if got.Summary.UnreadCount != 0 {
		t.Fatalf("UnreadCount = %d, want 0", got.Summary.UnreadCount)
	}
}

func TestFrameMessagingBroker_RejectsSelfTargetAndNonAgentTarget(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{DataDir: dir, Persist: noopPersist{}})
	sid := seedFrameMessagingSession(r)

	self := r.frameMessagingSend("frame-claude", "frame-claude", "", "loop", "")
	if self.err == nil {
		t.Fatal("self-target send must fail")
	}
	nonAgent := r.frameMessagingSend("frame-claude", "frame-shell", "", "hello", "")
	if nonAgent.err == nil {
		t.Fatal("non-agent target send must fail")
	}
	var body *proto.ErrorBody
	if !errors.As(nonAgent.err, &body) || body.Code != proto.ErrUnsupported {
		t.Fatalf("non-agent error = %v, want unsupported", nonAgent.err)
	}

	records := readAuditRecords(t, filepath.Join(dir, "frame-messaging", string(sid), "audit.jsonl"))
	if len(records) != 2 {
		t.Fatalf("audit records = %d, want 2", len(records))
	}
	if records[0].Decision != frameMessagingAuditReject || records[0].Reason != "self_target" {
		t.Fatalf("self-target audit = %+v", records[0])
	}
	if records[1].Decision != frameMessagingAuditReject || records[1].Reason != "target_not_agent_frame" {
		t.Fatalf("non-agent audit = %+v", records[1])
	}
}

func TestIsAgentFrame_AcceptsCommandsWithArgs(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    bool
	}{
		{command: "codex -m gpt-5", want: true},
		{command: "claude --print", want: true},
		{command: "shell -lc 'echo hi'", want: false},
	} {
		got := isAgentFrame(state.SessionFrame{Command: tc.command})
		if got != tc.want {
			t.Fatalf("isAgentFrame(%q) = %v, want %v", tc.command, got, tc.want)
		}
	}
}

func TestFrameMessagingBroker_RejectsCrossSessionTarget(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{DataDir: dir, Persist: noopPersist{}})
	sid := seedFrameMessagingSession(r)
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	r.state.Sessions["s-other"] = state.Session{
		ID:        "s-other",
		Project:   "/repo",
		CreatedAt: now,
		Frames: []state.SessionFrame{
			{ID: "frame-other", Project: "/repo", Command: "codex", CreatedAt: now},
		},
		HeadFrameID: "frame-other",
	}

	res := r.frameMessagingSend("frame-claude", "frame-other", "", "hello", "")
	if res.err == nil {
		t.Fatal("cross-session send must fail")
	}
	var body *proto.ErrorBody
	if !errors.As(res.err, &body) || body.Code != proto.ErrInvalidArgument {
		t.Fatalf("cross-session error = %v, want invalid argument", res.err)
	}

	records := readAuditRecords(t, filepath.Join(dir, "frame-messaging", string(sid), "audit.jsonl"))
	if len(records) != 1 {
		t.Fatalf("audit records = %d, want 1", len(records))
	}
	if records[0].Decision != frameMessagingAuditReject || records[0].Reason != "cross_session_target" {
		t.Fatalf("cross-session audit = %+v", records[0])
	}
}

func TestFrameMessagingRead_DoesNotPartiallyPersistOnAppendFailure(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{DataDir: dir, Persist: noopPersist{}})
	sid := seedFrameMessagingSession(r)

	first := r.frameMessagingSend("frame-claude", "frame-codex", "", "one", "")
	if first.err != nil {
		t.Fatalf("first send: %v", first.err)
	}
	second := r.frameMessagingSend("frame-claude", "frame-codex", "", "two", "")
	if second.err != nil {
		t.Fatalf("second send: %v", second.err)
	}

	calls := 0
	r.appendFrameMessagingJSONL = func(path string, record any) error {
		if rec, ok := record.(frameMessagingJournalRecord); ok && rec.Kind == frameMessagingKindRead {
			calls++
			return fmt.Errorf("boom")
		}
		return appendJSONL(path, record)
	}

	res := r.frameMessagingRead("frame-codex", "frame-claude")
	if res.err == nil {
		t.Fatal("read must fail when journal append fails")
	}
	if calls != 1 {
		t.Fatalf("read append calls = %d, want 1 batch append", calls)
	}

	got := r.state.Sessions[sid].FrameMessaging
	if got == nil {
		t.Fatal("frame messaging missing")
	}
	for _, msg := range got.Messages {
		if msg.Read {
			t.Fatalf("message %q became read in memory despite append failure", msg.ID)
		}
	}

	r2 := New(Config{DataDir: dir, Persist: noopPersist{}})
	seedFrameMessagingSession(r2)
	r2.restoreFrameMessagingStores()
	restored := r2.state.Sessions[sid].FrameMessaging
	if restored == nil {
		t.Fatal("restored frame messaging missing")
	}
	for _, msg := range restored.Messages {
		if msg.Read {
			t.Fatalf("message %q became read on replay despite append failure", msg.ID)
		}
	}
}

func TestFrameMessagingReply_RejectsSecondReply(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{DataDir: dir, Persist: noopPersist{}})
	sid := seedFrameMessagingSession(r)

	send := r.frameMessagingSend("frame-claude", "frame-codex", "", "hello", "")
	if send.err != nil {
		t.Fatalf("send: %v", send.err)
	}
	first := r.frameMessagingReply("frame-codex", send.resp.Message.ID, "Done", "Ship it", "resolved", "high")
	if first.err != nil {
		t.Fatalf("first reply: %v", first.err)
	}
	second := r.frameMessagingReply("frame-codex", send.resp.Message.ID, "Overwrite", "Nope", "resolved", "high")
	if second.err == nil {
		t.Fatal("second reply must fail")
	}
	var body *proto.ErrorBody
	if !errors.As(second.err, &body) || body.Code != proto.ErrInvalidArgument {
		t.Fatalf("second reply err = %v, want invalid argument", second.err)
	}

	got := r.state.Sessions[sid].FrameMessaging.Messages[0]
	if got.Reply == nil || got.Reply.FinalAnswer != "Ship it" {
		t.Fatalf("reply overwritten in memory: %+v", got.Reply)
	}

	r2 := New(Config{DataDir: dir, Persist: noopPersist{}})
	seedFrameMessagingSession(r2)
	r2.restoreFrameMessagingStores()
	restored := r2.state.Sessions[sid].FrameMessaging.Messages[0]
	if restored.Reply == nil || restored.Reply.FinalAnswer != "Ship it" {
		t.Fatalf("reply overwritten on replay: %+v", restored.Reply)
	}
}

func TestFrameMessagingReply_RejectsEmptyReply(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{DataDir: dir, Persist: noopPersist{}})
	sid := seedFrameMessagingSession(r)

	send := r.frameMessagingSend("frame-claude", "frame-codex", "", "hello", "")
	if send.err != nil {
		t.Fatalf("send: %v", send.err)
	}

	res := r.frameMessagingReply("frame-codex", send.resp.Message.ID, "", "", "", "high")
	if res.err == nil {
		t.Fatal("empty reply must fail")
	}
	var body *proto.ErrorBody
	if !errors.As(res.err, &body) || body.Code != proto.ErrInvalidArgument {
		t.Fatalf("empty reply err = %v, want invalid argument", res.err)
	}

	got := r.state.Sessions[sid].FrameMessaging.Messages[0]
	if got.Reply != nil {
		t.Fatalf("empty reply was persisted in memory: %+v", got.Reply)
	}

	r2 := New(Config{DataDir: dir, Persist: noopPersist{}})
	seedFrameMessagingSession(r2)
	r2.restoreFrameMessagingStores()
	restored := r2.state.Sessions[sid].FrameMessaging.Messages[0]
	if restored.Reply != nil {
		t.Fatalf("empty reply was persisted on replay: %+v", restored.Reply)
	}
}

func TestRestoreFrameMessagingStores_PreservesSnapshotReadsWhenJournalLags(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{DataDir: dir, Persist: noopPersist{}})
	sid := seedFrameMessagingSession(r)

	send := r.frameMessagingSend("frame-claude", "frame-codex", "", "hello", "")
	if send.err != nil {
		t.Fatalf("send: %v", send.err)
	}

	r2 := New(Config{DataDir: dir, Persist: noopPersist{}})
	sid2 := seedFrameMessagingSession(r2)
	if sid2 != sid {
		t.Fatalf("session seed mismatch: %q vs %q", sid2, sid)
	}
	msg := r2.state.Sessions[sid].FrameMessaging
	if msg != nil {
		t.Fatalf("unexpected frame messaging seed: %+v", msg)
	}
	r2.state.Sessions[sid] = state.Session{
		ID:          sid,
		Project:     "/repo",
		CreatedAt:   r.state.Sessions[sid].CreatedAt,
		Frames:      r.state.Sessions[sid].Frames,
		HeadFrameID: r.state.Sessions[sid].HeadFrameID,
		FrameMessaging: &state.SessionFrameMessaging{
			Summary: state.FrameMessagingSummary{UnreadCount: 0, PendingDeliveryCount: 1},
			Messages: []state.FrameMessage{{
				ID:             send.resp.Message.ID,
				SourceFrameID:  "frame-claude",
				TargetFrameID:  "frame-codex",
				Body:           "hello",
				CreatedAt:      time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC),
				Read:           true,
				ReplyStatus:    "pending",
				DeliveryStatus: "pending",
			}},
		},
	}

	r2.restoreFrameMessagingStores()
	restored := r2.state.Sessions[sid].FrameMessaging
	if restored == nil || len(restored.Messages) != 1 {
		t.Fatalf("restored = %+v, want 1 message", restored)
	}
	if !restored.Messages[0].Read {
		t.Fatal("snapshot read state was lost when journal lacked message_read")
	}
}
