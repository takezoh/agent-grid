package state

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

var errFake = errors.New("generation failed")

// capsuleSrcDriver is a fake driver implementing CapsuleSource so tick
// enqueue and job-result reduction can be tested without real drivers.
type capsuleSrcState struct {
	DriverStateBase
	status Status
	turns  []CapsuleTurn
}

type capsuleSrcDriver struct{}

func (capsuleSrcDriver) Name() string                       { return "capsulesrc" }
func (capsuleSrcDriver) DisplayName() string                { return "capsulesrc" }
func (capsuleSrcDriver) Status(s DriverState) Status        { return s.(capsuleSrcState).status }
func (capsuleSrcDriver) NewState(now time.Time) DriverState { return capsuleSrcState{} }
func (capsuleSrcDriver) PrepareLaunch(s DriverState, mode LaunchMode, project, baseCommand string, options LaunchOptions, _ bool) (LaunchPlan, error) {
	return LaunchPlan{Command: baseCommand, StartDir: project}, nil
}
func (capsuleSrcDriver) Persist(s DriverState) map[string]string { return nil }
func (capsuleSrcDriver) Restore(bag map[string]string, now time.Time) DriverState {
	return capsuleSrcState{}
}
func (capsuleSrcDriver) View(s DriverState) View { return View{} }
func (capsuleSrcDriver) Step(prev DriverState, ctx FrameContext, ev DriverEvent) (DriverState, []Effect, View) {
	return prev, nil, View{}
}
func (capsuleSrcDriver) CapsuleTurns(s DriverState) []CapsuleTurn {
	return s.(capsuleSrcState).turns
}

func init() {
	if _, exists := driverRegistry["capsulesrc"]; !exists {
		Register(capsuleSrcDriver{})
	}
}

func capsuleTestSession(status Status, turns []CapsuleTurn) Session {
	ds := capsuleSrcState{status: status, turns: turns}
	return Session{
		ID:      "s1",
		Project: "/repo",
		Command: "capsulesrc",
		Driver:  ds,
		Frames:  []SessionFrame{{ID: "s1-f", Project: "/repo", Command: "capsulesrc", Driver: ds}},
	}
}

// dueTick returns a tick counter N for which the per-session stagger gate
// fires for sessID.
func dueTick(t *testing.T, sessID SessionID) uint64 {
	t.Helper()
	for n := uint64(0); n < capsuleCheckInterval; n++ {
		if capsuleCheckDue(sessID, n) {
			return n
		}
	}
	t.Fatal("no due tick found within interval")
	return 0
}

func findCapsuleJob(effs []Effect) (EffStartJob, bool) {
	for _, e := range effs {
		if job, ok := e.(EffStartJob); ok {
			if _, ok := job.Input.(CapsuleJobInput); ok {
				return job, true
			}
		}
	}
	return EffStartJob{}, false
}

func TestCapsuleTickEnqueuesJobOnNewTurns(t *testing.T) {
	s := New()
	s.ResumeCapsulesEnabled = true
	turns := []CapsuleTurn{{Role: "user", Text: "fix the login bug"}}
	s.Sessions["s1"] = capsuleTestSession(StatusIdle, turns)

	next, effs := Reduce(s, EvTick{Now: time.Now(), N: dueTick(t, "s1")})

	job, ok := findCapsuleJob(effs)
	if !ok {
		t.Fatal("expected a capsule EffStartJob")
	}
	input := job.Input.(CapsuleJobInput)
	if input.SessionID != "s1" || input.SourceDigest == "" || input.SourceTurns != 1 {
		t.Errorf("unexpected job input: %+v", input)
	}
	if input.Prompt == "" {
		t.Error("prompt must not be empty")
	}
	if job.JobID == 0 {
		t.Error("JobID must be allocated")
	}
	if next.CapsuleGenerating["s1"] != input.SourceDigest {
		t.Errorf("in-flight digest not recorded: %q", next.CapsuleGenerating["s1"])
	}
	if _, ok := next.Jobs[job.JobID]; !ok {
		t.Error("JobMeta must be registered")
	}
}

func TestCapsuleTickSkips(t *testing.T) {
	turns := []CapsuleTurn{{Role: "user", Text: "hello"}}
	digest := capsuleDigest(turns)
	cases := []struct {
		name  string
		mutup func(*State)
	}{
		{"disabled", func(s *State) { s.ResumeCapsulesEnabled = false }},
		{"in-flight", func(s *State) { s.CapsuleGenerating["s1"] = "x" }},
		{"running status", func(s *State) {
			s.Sessions["s1"] = capsuleTestSession(StatusRunning, turns)
		}},
		{"no turns", func(s *State) {
			s.Sessions["s1"] = capsuleTestSession(StatusIdle, nil)
		}},
		{"digest unchanged", func(s *State) {
			sess := s.Sessions["s1"]
			sess.ResumeCapsule = &ResumeCapsule{GeneratedAt: time.Now(), SourceDigest: digest}
			s.Sessions["s1"] = sess
		}},
		{"error backoff", func(s *State) {
			sess := s.Sessions["s1"]
			sess.ResumeCapsule = &ResumeCapsule{
				GeneratedAt: time.Now(), SourceDigest: "old",
				LastError: "boom", LastErrorAt: time.Now(),
			}
			s.Sessions["s1"] = sess
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.ResumeCapsulesEnabled = true
			s.Sessions["s1"] = capsuleTestSession(StatusIdle, turns)
			tc.mutup(&s)
			now := time.Now()
			s.Now = now
			_, effs := Reduce(s, EvTick{Now: now, N: dueTick(t, "s1")})
			if _, ok := findCapsuleJob(effs); ok {
				t.Error("expected no capsule job")
			}
		})
	}
}

func TestCapsuleJobResultAppliesCapsule(t *testing.T) {
	now := time.Now()
	s := New()
	s.Now = now
	sess := capsuleTestSession(StatusIdle, nil)
	sess.ResumeCapsule = &ResumeCapsule{GeneratedAt: now.Add(-time.Hour), FlaggedWrong: true}
	s.Sessions["s1"] = sess
	s.Jobs[7] = JobMeta{SessionID: "s1", FrameID: "s1-f"}
	s.CapsuleGenerating["s1"] = "digest-1"

	next, effs := Reduce(s, EvJobResult{JobID: 7, Result: CapsuleJobResult{
		Objective:    "ship the login fix",
		CurrentState: "root cause identified",
		NextAction:   "write the regression test",
		Unresolved:   []string{"flaky CI"},
		Changed:      []string{"fixed token refresh"},
		Verified:     []string{"unit tests pass"},
		Remaining:    []string{"regression test"},
		Attention:    "review the fix",
		SourceDigest: "digest-1",
		SourceTurns:  4,
	}})

	capsule := next.Sessions["s1"].ResumeCapsule
	if capsule == nil {
		t.Fatal("capsule must be stored")
	}
	if capsule.Objective != "ship the login fix" || capsule.SourceDigest != "digest-1" || capsule.SourceTurns != 4 {
		t.Errorf("capsule fields wrong: %+v", capsule)
	}
	if len(capsule.Changed) != 1 || len(capsule.Verified) != 1 || len(capsule.Remaining) != 1 ||
		capsule.Attention != "review the fix" {
		t.Errorf("completion fields wrong: %+v", capsule)
	}
	if !capsule.GeneratedAt.Equal(now) {
		t.Errorf("GeneratedAt = %v, want reducer Now", capsule.GeneratedAt)
	}
	if capsule.FlaggedWrong {
		t.Error("successful regeneration must clear the wrong-flag")
	}
	if _, ok := next.CapsuleGenerating["s1"]; ok {
		t.Error("in-flight marker must be cleared")
	}
	var persisted, broadcast bool
	for _, e := range effs {
		switch e.(type) {
		case EffPersistSnapshot:
			persisted = true
		case EffBroadcastSessionsChanged:
			broadcast = true
		}
	}
	if !persisted || !broadcast {
		t.Error("expected persist + broadcast effects")
	}
}

func TestCapsuleJobResultError(t *testing.T) {
	now := time.Now()
	s := New()
	s.Now = now
	sess := capsuleTestSession(StatusIdle, nil)
	sess.ResumeCapsule = &ResumeCapsule{GeneratedAt: now.Add(-time.Hour), CurrentState: "old state"}
	s.Sessions["s1"] = sess
	s.Jobs[7] = JobMeta{SessionID: "s1", FrameID: "s1-f"}
	s.CapsuleGenerating["s1"] = "digest-1"

	next, _ := Reduce(s, EvJobResult{JobID: 7, Result: CapsuleJobResult{}, Err: errFake})

	capsule := next.Sessions["s1"].ResumeCapsule
	if capsule.LastError == "" || !capsule.LastErrorAt.Equal(now) {
		t.Errorf("error must be recorded: %+v", capsule)
	}
	if capsule.CurrentState != "old state" {
		t.Error("failed generation must keep previous content")
	}
	if _, ok := next.CapsuleGenerating["s1"]; ok {
		t.Error("in-flight marker must be cleared on error")
	}
}

func TestCapsuleJobResultUnconfiguredIsNoop(t *testing.T) {
	s := New()
	s.Sessions["s1"] = capsuleTestSession(StatusIdle, nil)
	s.Jobs[7] = JobMeta{SessionID: "s1", FrameID: "s1-f"}
	s.CapsuleGenerating["s1"] = "digest-1"

	next, effs := Reduce(s, EvJobResult{JobID: 7, Result: CapsuleJobResult{}})

	if next.Sessions["s1"].ResumeCapsule != nil {
		t.Error("no capsule must be created for an unconfigured runner")
	}
	if len(effs) != 0 {
		t.Errorf("expected no effects, got %d", len(effs))
	}
}

func TestFlagResumeCapsule(t *testing.T) {
	now := time.Now()
	s := New()
	s.Now = now
	sess := capsuleTestSession(StatusIdle, nil)
	sess.ResumeCapsule = &ResumeCapsule{GeneratedAt: now.Add(-time.Minute)}
	s.Sessions["s1"] = sess

	payload, _ := json.Marshal(ResumeCapsuleFlagParams{SessionID: "s1", Flagged: true})
	next, effs := Reduce(s, EvEvent{ConnID: 1, ReqID: "r1", Event: EventFlagResumeCapsule, Payload: payload})

	capsule := next.Sessions["s1"].ResumeCapsule
	if !capsule.FlaggedWrong || !capsule.FlaggedAt.Equal(now) {
		t.Errorf("flag not recorded: %+v", capsule)
	}
	var logged bool
	for _, e := range effs {
		if l, ok := e.(EffEventLogAppend); ok && l.Line == "resume-capsule flagged wrong" {
			logged = true
		}
	}
	if !logged {
		t.Error("expected durable event-log line for the flag")
	}
}

func TestFlagResumeCapsuleNotFound(t *testing.T) {
	s := New()
	s.Sessions["s1"] = capsuleTestSession(StatusIdle, nil)

	payload, _ := json.Marshal(ResumeCapsuleFlagParams{SessionID: "s1", Flagged: true})
	_, effs := Reduce(s, EvEvent{ConnID: 1, ReqID: "r1", Event: EventFlagResumeCapsule, Payload: payload})

	if len(effs) != 1 {
		t.Fatalf("expected single error effect, got %d", len(effs))
	}
	errEff, ok := effs[0].(EffSendError)
	if !ok || errEff.Code != ErrCodeNotFound {
		t.Errorf("expected not-found error, got %+v", effs[0])
	}
}

func TestListResumeCapsulesReturnsMarkerReply(t *testing.T) {
	s := New()
	_, effs := Reduce(s, EvEvent{ConnID: 1, ReqID: "r1", Event: EventListResumeCapsules})
	if len(effs) != 1 {
		t.Fatalf("expected single effect, got %d", len(effs))
	}
	resp, ok := effs[0].(EffSendResponseSync)
	if !ok {
		t.Fatalf("expected EffSendResponseSync, got %T", effs[0])
	}
	if _, ok := resp.Body.(ResumeCapsulesReply); !ok {
		t.Errorf("expected ResumeCapsulesReply body, got %T", resp.Body)
	}
}
