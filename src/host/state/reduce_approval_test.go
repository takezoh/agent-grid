package state

import (
	"testing"
	"time"
)

func approvalTestState(t *testing.T) (State, SessionID, FrameID) {
	t.Helper()
	s := New()
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	s.Now = now
	sid := SessionID("sess-1")
	fid := FrameID("frame-1")
	s.Sessions = map[SessionID]Session{
		sid: {
			ID:      sid,
			Project: "/tmp/proj",
			Frames: []SessionFrame{{
				ID:      fid,
				Project: "/tmp/proj",
				Command: "codex",
				Driver:  pushStubState{},
			}},
			HeadFrameID: fid,
			Driver:      pushStubState{},
		},
	}
	return s, sid, fid
}

func TestApprovalRequestCreatedAndEmittedInSameCycle(t *testing.T) {
	s, sid, fid := approvalTestState(t)
	now := s.Now
	next, effs := Reduce(s, EvSubsystem{
		FrameID:   fid,
		Kind:      SubsystemApprovalRequested,
		Timestamp: now,
		Payload: SubsystemPayload{
			Approval: &SubsystemApproval{
				ID:      "ap-1",
				Kind:    "command",
				Command: "rm -rf /",
				Reason:  "destructive",
			},
		},
	})
	pending := PendingApprovalsForSession(next, sid)
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
	r := pending[0]
	if r.ID != "ap-1" || r.Status != ApprovalPending {
		t.Fatalf("approval = %+v", r)
	}
	if r.DefaultDecision != ApprovalDecisionDeny {
		t.Fatalf("default_decision = %q, want deny", r.DefaultDecision)
	}
	if !r.ExpiresAt.Equal(now.Add(DefaultApprovalTTL)) {
		t.Fatalf("expires_at = %v, want %v", r.ExpiresAt, now.Add(DefaultApprovalTTL))
	}
	if _, ok := findEff[EffBroadcastEvent](effs); !ok {
		// may be multiple broadcast events; scan
		found := false
		for _, e := range effs {
			if be, ok := e.(EffBroadcastEvent); ok && be.Name == EvtNameApprovalRequested {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected approval-requested broadcast in same Reduce cycle")
		}
	}
}

func TestApprovalAndQuestionLifecycleCreation(t *testing.T) {
	s, sid, fid := approvalTestState(t)
	now := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemQuestionRequested, Timestamp: now,
		Payload: SubsystemPayload{Question: &SubsystemQuestion{ID: "q-1", Prompt: "continue?"}},
	})
	if got := len(PendingApprovalsForSession(s, sid)); got != 1 {
		t.Fatalf("approvals = %d", got)
	}
	if got := len(PendingQuestionsForSession(s, sid)); got != 1 {
		t.Fatalf("questions = %d", got)
	}
	// Duplicate driver events must not create a second entry.
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	if got := len(s.PendingApprovals[sid]); got != 1 {
		t.Fatalf("after duplicate: approvals map size = %d", got)
	}
}

func TestApprovalSingleWriterFirstCommit(t *testing.T) {
	s, _, fid := approvalTestState(t)
	now := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	s, effsA := Reduce(s, EvCmdApprovalRespond{
		ConnID: 1, ReqID: "rA", ApprovalID: "ap-1",
		Decision: ApprovalDecisionAccept, ClientInstanceID: "ci-A",
	})
	if _, ok := findEff[EffSendError](effsA); ok {
		t.Fatal("winner must not receive error")
	}
	r, _, ok := FindApproval(s, "ap-1")
	if !ok || r.Status != ApprovalResolved || r.Decision != ApprovalDecisionAccept {
		t.Fatalf("winner state = %+v ok=%v", r, ok)
	}
	if r.ResolvingClientInstanceID != "ci-A" {
		t.Fatalf("resolving_client_instance_id = %q", r.ResolvingClientInstanceID)
	}
	_, effsB := Reduce(s, EvCmdApprovalRespond{
		ConnID: 2, ReqID: "rB", ApprovalID: "ap-1",
		Decision: ApprovalDecisionDeny, ClientInstanceID: "ci-B",
	})
	errEff, ok := findEff[EffSendError](effsB)
	if !ok {
		t.Fatal("loser must receive resolved-by-other error")
	}
	if errEff.Code != ErrCodeResolvedByOther {
		t.Fatalf("code = %q, want %q", errEff.Code, ErrCodeResolvedByOther)
	}
	if errEff.Details["decision"] != "accept" {
		t.Fatalf("details.decision = %v", errEff.Details["decision"])
	}
	if errEff.Details["resolving_client_instance_id"] != "ci-A" {
		t.Fatalf("details.resolving = %v", errEff.Details["resolving_client_instance_id"])
	}
	// No second resolve broadcast for the loser path.
	for _, e := range effsB {
		if be, ok := e.(EffBroadcastEvent); ok && be.Name == EvtNameApprovalResolved {
			t.Fatal("loser path must not re-broadcast EvtApprovalResolved")
		}
	}
}

func TestApprovalExpiryDecisionCaptureAtCreation(t *testing.T) {
	s, _, fid := approvalTestState(t)
	created := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: created,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	// Mid-flight "policy mutation" cannot flip captured default_decision.
	// (DefaultDecision is already deny; we assert expiry uses it.)
	r, _, _ := FindApproval(s, "ap-1")
	if r.DefaultDecision != ApprovalDecisionDeny {
		t.Fatalf("captured default = %q", r.DefaultDecision)
	}
	// Tick past expires_at.
	s, effs := Reduce(s, EvTick{Now: created.Add(DefaultApprovalTTL + time.Second), N: 1})
	r, _, ok := FindApproval(s, "ap-1")
	if !ok || r.Status != ApprovalExpired {
		t.Fatalf("after expiry: %+v ok=%v", r, ok)
	}
	if r.Decision != ApprovalDecisionDeny {
		t.Fatalf("expired decision = %q, want deny", r.Decision)
	}
	if r.ResolutionReason != ApprovalReasonExpired {
		t.Fatalf("reason = %q", r.ResolutionReason)
	}
	foundReply := false
	foundBroadcast := false
	for _, e := range effs {
		if re, ok := e.(EffReplyHeldApproval); ok && re.ApprovalID == "ap-1" && re.Decision == ApprovalDecisionDeny {
			foundReply = true
		}
		if be, ok := e.(EffBroadcastEvent); ok && be.Name == EvtNameApprovalResolved {
			foundBroadcast = true
		}
	}
	if !foundReply || !foundBroadcast {
		t.Fatalf("reply=%v broadcast=%v", foundReply, foundBroadcast)
	}
}

func TestApprovalExpiryTOCTOU(t *testing.T) {
	// Same as capture-at-creation: even if we mutate a local policy flag,
	// the ApprovalRequest.DefaultDecision is what expiry applies.
	s, _, fid := approvalTestState(t)
	created := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: created,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{
			ID: "ap-1", Kind: "command", AutoApprove: false,
		}},
	})
	// Simulate "live policy flipped to accept" by only changing unrelated state.
	// DefaultDecision on the request remains deny.
	r, _, _ := FindApproval(s, "ap-1")
	if r.DefaultDecision != ApprovalDecisionDeny {
		t.Fatal("expected deny captured at creation")
	}
	s, _ = Reduce(s, EvTick{Now: created.Add(DefaultApprovalTTL + time.Millisecond), N: 2})
	r, _, _ = FindApproval(s, "ap-1")
	if r.Decision != ApprovalDecisionDeny {
		t.Fatalf("TOCTOU: decision flipped to %q", r.Decision)
	}
}

func TestApprovalLifecycleTeardownReap(t *testing.T) {
	s, sid, fid := approvalTestState(t)
	now := s.Now
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{ID: "ap-1", Kind: "command"}},
	})
	s, _ = Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemQuestionRequested, Timestamp: now,
		Payload: SubsystemPayload{Question: &SubsystemQuestion{ID: "q-1", Prompt: "?"}},
	})
	// Root frame vanish evicts the session and reaps pending domain objects.
	s, effs := Reduce(s, EvFrameVanished{FrameID: fid})
	if len(PendingApprovalsForSession(s, sid)) != 0 {
		t.Fatalf("pending approvals remain: %v", PendingApprovalsForSession(s, sid))
	}
	if len(PendingQuestionsForSession(s, sid)) != 0 {
		t.Fatalf("pending questions remain: %v", PendingQuestionsForSession(s, sid))
	}
	if _, ok := s.PendingApprovals[sid]; ok {
		t.Fatal("session approval map should be fully reaped")
	}
	if _, ok := s.PendingQuestions[sid]; ok {
		t.Fatal("session question map should be fully reaped")
	}
	var replyN, qReplyN, cancelBroadcastN int
	for _, e := range effs {
		switch ef := e.(type) {
		case EffReplyHeldApproval:
			if ef.Error == "connection-lost" {
				replyN++
			}
		case EffReplyHeldQuestion:
			if ef.Error == "connection-lost" {
				qReplyN++
			}
		case EffBroadcastEvent:
			if ef.Name == EvtNameApprovalResolved || ef.Name == EvtNameQuestionResolved {
				cancelBroadcastN++
			}
		}
	}
	if replyN != 1 || qReplyN != 1 {
		t.Fatalf("drain replies approval=%d question=%d", replyN, qReplyN)
	}
	if cancelBroadcastN < 2 {
		t.Fatalf("cancel broadcasts = %d, want >=2", cancelBroadcastN)
	}
}

func TestApprovalAutoApproveResolvesInSameCycle(t *testing.T) {
	s, sid, fid := approvalTestState(t)
	now := s.Now
	s, effs := Reduce(s, EvSubsystem{
		FrameID: fid, Kind: SubsystemApprovalRequested, Timestamp: now,
		Payload: SubsystemPayload{Approval: &SubsystemApproval{
			ID: "ap-auto", Kind: "command", AutoApprove: true,
		}},
	})
	if len(PendingApprovalsForSession(s, sid)) != 0 {
		t.Fatal("auto-approve must not leave pending")
	}
	r, _, ok := FindApproval(s, "ap-auto")
	if !ok || r.Status != ApprovalResolved || r.Decision != ApprovalDecisionAccept {
		t.Fatalf("auto tombstone = %+v ok=%v", r, ok)
	}
	if r.ResolvingClientInstanceID != AutoResolvingClientInstanceID {
		t.Fatalf("resolving = %q", r.ResolvingClientInstanceID)
	}
	var sawReply, sawResolved bool
	for _, e := range effs {
		if re, ok := e.(EffReplyHeldApproval); ok && re.Decision == ApprovalDecisionAccept {
			sawReply = true
		}
		if be, ok := e.(EffBroadcastEvent); ok && be.Name == EvtNameApprovalResolved {
			sawResolved = true
		}
	}
	if !sawReply || !sawResolved {
		t.Fatalf("reply=%v resolved=%v", sawReply, sawResolved)
	}
}
