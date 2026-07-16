package state

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"
)

type fuzzDriver struct{}

type fuzzDriverState struct {
	DriverStateBase
	Status   Status
	StartDir string
	Counter  int
}

func (fuzzDriver) Name() string        { return "fuzzstub" }
func (fuzzDriver) DisplayName() string { return "fuzzstub" }

func (fuzzDriver) NewState(now time.Time) DriverState {
	return fuzzDriverState{Status: StatusIdle}
}

func (fuzzDriver) Step(prev DriverState, ctx FrameContext, ev DriverEvent) (DriverState, []Effect, View) {
	s := prev.(fuzzDriverState)
	s.Counter++
	switch e := ev.(type) {
	case DEvWorktreeResolved:
		s.StartDir = e.StartDir
	case DEvCommandExited:
		s.Status = StatusStopped
		if e.ExitCode == 77 {
			return s, []Effect{EffStartJob{Input: fuzzJobInput("exit")}}, View{}
		}
	case DEvHook:
		if e.Event == "push" {
			return s, []Effect{EffPushDriver{Command: "fuzzstub"}}, View{}
		}
		if e.Event == "job" {
			return s, []Effect{EffStartJob{Input: fuzzJobInput("hook")}}, View{}
		}
	case DEvSubsystem:
		if e.Kind == SubsystemToolCompleted {
			return s, []Effect{EffStartJob{Input: fuzzJobInput("subsystem")}}, View{}
		}
	case DEvFramePrompt:
		if e.Phase == PromptPhaseComplete {
			s.Status = StatusIdle
		}
	case DEvTick:
		if e.N%11 == 0 {
			return s, []Effect{EffWatchFile{Path: ctx.Project + "/watch.txt"}}, View{}
		}
	case DEvFileChanged:
		return s, []Effect{EffUnwatchFile{}}, View{}
	case DEvJobResult:
		if e.Err == nil {
			s.Status = StatusRunning
		}
	}
	return s, nil, View{}
}

func (fuzzDriver) Status(s DriverState) Status { return s.(fuzzDriverState).Status }
func (fuzzDriver) Persist(s DriverState) map[string]string {
	ds := s.(fuzzDriverState)
	return map[string]string{
		"status":    ds.Status.String(),
		"start_dir": ds.StartDir,
	}
}

func (fuzzDriver) Restore(bag map[string]string, now time.Time) DriverState {
	status, ok := ParseStatus(bag["status"])
	if !ok {
		status = StatusIdle
	}
	return fuzzDriverState{
		Status:   status,
		StartDir: bag["start_dir"],
	}
}

func (fuzzDriver) View(s DriverState) View { return View{} }

func (fuzzDriver) PrepareLaunch(s DriverState, mode LaunchMode, project, baseCommand string, options LaunchOptions, sandboxed bool) (LaunchPlan, error) {
	return LaunchPlan{
		Command:   baseCommand,
		StartDir:  project,
		Project:   project,
		Options:   options,
		Subsystem: LaunchSubsystemCLI,
	}, nil
}

func (fuzzDriver) StartDir(s DriverState) string { return s.(fuzzDriverState).StartDir }

func (fuzzDriver) WithStartDir(s DriverState, dir string) DriverState {
	ds := s.(fuzzDriverState)
	ds.StartDir = dir
	return ds
}

type fuzzJobInput string

func (f fuzzJobInput) JobKind() string { return string(f) }

func init() {
	if _, exists := driverRegistry["fuzzstub"]; !exists {
		Register(fuzzDriver{})
	}
}

func FuzzReduce(f *testing.F) {
	f.Add([]byte{0, 0, 10, 0, 12, 0, 0, 1, 10, 0, 2, 2})
	f.Add([]byte{0, 0, 10, 0, 12, 0, 0, 3, 13, 0, 0, 0, 14, 0, 0, 0})
	f.Add([]byte{0, 0, 10, 0, 12, 0, 0, 2, 1, 0, 0, 0, 15, 0, 0, 1})
	f.Add([]byte{0, 0, 10, 0, 12, 0, 0, 4, 8, 0, 0, 2, 16, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		state := newReduceFuzzState()
		now := state.Now
		for i := 0; i+3 < len(data) && i < 128; i += 4 {
			ev := decodeReduceFuzzEvent(state, now, data[i:i+4])
			snapshot := cloneReduceFuzzState(state)

			var (
				next State
				effs []Effect
			)
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Reduce panicked on %T at step %d: %v", ev, i/4, r)
					}
				}()
				next, effs = Reduce(state, ev)
			}()

			if !reflect.DeepEqual(state, snapshot) {
				t.Fatalf("input state mutated on %T\nbefore=%#v\nafter=%#v", ev, snapshot, state)
			}
			assertReduceFuzzStateInvariants(t, next)
			assertReduceFuzzEffects(t, effs)

			state = next
			now = now.Add(time.Second)
		}
	})
}

func newReduceFuzzState() State {
	s := New()
	s.Now = time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	s.DefaultCommand = "fuzzstub"
	s.Aliases = map[string]string{"fz": "fuzzstub"}
	return s
}

func cloneReduceFuzzState(s State) State {
	clone := s
	clone.Sessions = cloneReduceFuzzSessions(s.Sessions)
	clone.Subscribers = cloneSubscribers(s.Subscribers)
	clone.SurfaceSubs = cloneSurfaceSubs(s.SurfaceSubs)
	clone.Jobs = cloneJobs(s.Jobs)
	if s.Aliases != nil {
		clone.Aliases = make(map[string]string, len(s.Aliases))
		for k, v := range s.Aliases {
			clone.Aliases[k] = v
		}
	}
	return clone
}

func cloneReduceFuzzSessions(src map[SessionID]Session) map[SessionID]Session {
	if src == nil {
		return nil
	}
	dst := make(map[SessionID]Session, len(src))
	for id, sess := range src {
		copied := sess
		copied.Frames = append([]SessionFrame(nil), sess.Frames...)
		copied.MRUFrameIDs = append([]FrameID(nil), sess.MRUFrameIDs...)
		dst[id] = copied
	}
	return dst
}

func decodeReduceFuzzEvent(s State, now time.Time, chunk []byte) Event {
	op, a, b, c := chunk[0], chunk[1], chunk[2], chunk[3]
	sessionID, hasSession := pickReduceFuzzSessionID(s, a)
	frameID, _ := pickReduceFuzzFrameID(s, b)
	reqID := fmt.Sprintf("r-%02x-%02x", a, b)

	switch op % 18 {
	case 0:
		payload := mustReduceFuzzJSON(CreateSessionParams{
			Project: fmt.Sprintf("/tmp/fuzz/%d", a%4),
			Command: pickReduceFuzzCommand(c),
			Options: LaunchOptions{
				Cols: uint16(40 + a%80),
				Rows: uint16(10 + b%40),
			},
		})
		return EvEvent{ConnID: ConnID(a%4 + 1), ReqID: reqID, Event: EventCreateSession, Payload: payload}
	case 1:
		payload := mustReduceFuzzJSON(StopSessionParams{SessionID: string(sessionID)})
		return EvEvent{ConnID: ConnID(a%4 + 1), ReqID: reqID, Event: EventStopSession, Payload: payload}
	case 2:
		payload := mustReduceFuzzJSON(PushDriverParams{
			SessionID: string(sessionID),
			Project:   fmt.Sprintf("/tmp/fuzz/%d", c%3),
			Command:   pickReduceFuzzCommand(c),
			Options:   LaunchOptions{InitialInput: []byte{a, b, c}},
		})
		return EvEvent{ConnID: ConnID(a%4 + 1), ReqID: reqID, Event: EventPushDriver, Payload: payload}
	case 3:
		payload := mustReduceFuzzJSON(SetHeadFrameParams{SessionID: string(sessionID), FrameID: string(frameID)})
		return EvEvent{ConnID: ConnID(a%4 + 1), ReqID: reqID, Event: EventSetHeadFrame, Payload: payload}
	case 4:
		payload := mustReduceFuzzJSON(ForkSessionParams{SessionID: string(sessionID)})
		return EvEvent{ConnID: ConnID(a%4 + 1), ReqID: reqID, Event: EventForkSession, Payload: payload}
	case 5:
		return EvCmdSubscribe{ConnID: ConnID(a%4 + 1), ReqID: reqID, Filters: pickReduceFuzzFilters(b)}
	case 6:
		return EvCmdUnsubscribe{ConnID: ConnID(a%4 + 1), ReqID: reqID}
	case 7:
		return EvConnClosed{ConnID: ConnID(a%4 + 1)}
	case 8:
		return EvCmdSurfaceSubscribe{Cols: 80, Rows: 24, ConnID: ConnID(a%4 + 1), ReqID: reqID, SessionID: sessionID}
	case 9:
		return EvCmdSurfaceUnsubscribe{ConnID: ConnID(a%4 + 1), ReqID: reqID, SessionID: sessionID}
	case 10:
		if !hasSession {
			return EvTick{Now: now, N: uint64(a) + 1}
		}
		frame := sessionRootFrameID(s.Sessions[sessionID])
		return EvFrameSpawned{
			SessionID:        sessionID,
			FrameID:          frame,
			SubsystemID:      SubsystemID(fmt.Sprintf("sub-%d", b%4)),
			WorktreeStartDir: pickReduceFuzzWorktreePath(c),
			WorktreeName:     fmt.Sprintf("wt-%d", c%5),
			ReplyConn:        ConnID(a%4 + 1),
			ReplyReqID:       reqID,
		}
	case 11:
		if !hasSession {
			return EvTick{Now: now, N: uint64(a) + 1}
		}
		frame := sessionRootFrameID(s.Sessions[sessionID])
		return EvSpawnFailed{
			SessionID:  sessionID,
			FrameID:    frame,
			Err:        fmt.Sprintf("spawn-%d", c%3),
			ReplyConn:  ConnID(a%4 + 1),
			ReplyReqID: reqID,
		}
	case 12:
		return EvTick{Now: now, N: uint64(a)<<8 | uint64(b)}
	case 13:
		return EvDriverEvent{
			ConnID:    ConnID(a%4 + 1),
			ReqID:     reqID,
			Event:     pickReduceFuzzHookEvent(c),
			Timestamp: now,
			SenderID:  frameID,
			Payload:   json.RawMessage(`{"step":"fuzz"}`),
		}
	case 14:
		return EvSubsystem{
			ConnID:    ConnID(a%4 + 1),
			ReqID:     reqID,
			FrameID:   frameID,
			Source:    pickReduceFuzzSubsystemSource(a),
			Kind:      pickReduceFuzzSubsystemKind(b),
			Timestamp: now,
			Payload: SubsystemPayload{
				SessionID: string(sessionID),
				TargetID:  string(frameID),
				Tool:      &SubsystemTool{ID: reqID, Name: "tool"},
				Plan:      &SubsystemPlan{Summary: "fuzz"},
			},
		}
	case 15:
		return EvFrameCommandExited{FrameID: frameID, ExitCode: pickReduceFuzzExitCode(c)}
	case 16:
		return EvFrameVanished{FrameID: frameID}
	default:
		return EvFramePrompt{
			FrameID: frameID,
			Phase:   PromptPhase(a%4 + 1),
			Now:     now,
		}
	}
}

func pickReduceFuzzSessionID(s State, b byte) (SessionID, bool) {
	if len(s.Sessions) == 0 || b%4 == 0 {
		return SessionID(fmt.Sprintf("missing-%d", b%3)), false
	}
	ids := make([]SessionID, 0, len(s.Sessions))
	for id := range s.Sessions {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids[int(b)%len(ids)], true
}

func pickReduceFuzzFrameID(s State, b byte) (FrameID, bool) {
	frames := make([]FrameID, 0)
	for _, sess := range s.Sessions {
		for _, frame := range sess.Frames {
			frames = append(frames, frame.ID)
		}
	}
	if len(frames) == 0 || b%4 == 0 {
		return FrameID(fmt.Sprintf("missing-frame-%d", b%3)), false
	}
	slices.Sort(frames)
	return frames[int(b)%len(frames)], true
}

func pickReduceFuzzCommand(b byte) string {
	if b%3 == 0 {
		return ""
	}
	if b%3 == 1 {
		return "fuzzstub"
	}
	return "fz"
}

func pickReduceFuzzFilters(b byte) []string {
	switch b % 3 {
	case 0:
		return nil
	case 1:
		return []string{"sessions-changed"}
	default:
		return []string{"surface-output", "prompt-event"}
	}
}

func pickReduceFuzzHookEvent(b byte) string {
	switch b % 4 {
	case 0:
		return "noop"
	case 1:
		return "push"
	case 2:
		return "job"
	default:
		return "noop"
	}
}

func pickReduceFuzzSubsystemSource(b byte) SubsystemKind {
	if b%2 == 0 {
		return SubsystemCLI
	}
	return SubsystemStream
}

func pickReduceFuzzSubsystemKind(b byte) SubsystemEventKind {
	switch b % 4 {
	case 0:
		return SubsystemTurnStarted
	case 1:
		return SubsystemToolCompleted
	case 2:
		return SubsystemPlanUpdated
	default:
		return SubsystemMessageUpdated
	}
}

func pickReduceFuzzExitCode(b byte) int {
	codes := []int{0, 1, 77, 129, 139}
	return codes[int(b)%len(codes)]
}

func pickReduceFuzzWorktreePath(b byte) string {
	if b%2 == 0 {
		return ""
	}
	return fmt.Sprintf("/tmp/worktree/%d", b%5)
}

func sessionRootFrameID(sess Session) FrameID {
	if len(sess.Frames) == 0 {
		return ""
	}
	return sess.Frames[0].ID
}

func mustReduceFuzzJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func assertReduceFuzzStateInvariants(t *testing.T, s State) {
	t.Helper()

	for sessID, sess := range s.Sessions {
		if len(sess.Frames) == 0 {
			if sess.HeadFrameID != "" {
				t.Fatalf("session %s has empty Frames but HeadFrameID=%q", sessID, sess.HeadFrameID)
			}
			if len(sess.MRUFrameIDs) != 0 {
				t.Fatalf("session %s has empty Frames but MRU=%v", sessID, sess.MRUFrameIDs)
			}
			continue
		}

		frameSet := make(map[FrameID]struct{}, len(sess.Frames))
		for _, frame := range sess.Frames {
			frameSet[frame.ID] = struct{}{}
		}
		if sess.HeadFrameID != "" {
			if _, ok := frameSet[sess.HeadFrameID]; !ok {
				t.Fatalf("session %s HeadFrameID=%q missing from Frames", sessID, sess.HeadFrameID)
			}
		}

		seenMRU := make(map[FrameID]struct{}, len(sess.MRUFrameIDs))
		for _, id := range sess.MRUFrameIDs {
			if _, ok := frameSet[id]; !ok {
				t.Fatalf("session %s MRU frame %q missing from Frames", sessID, id)
			}
			if _, dup := seenMRU[id]; dup {
				t.Fatalf("session %s MRU duplicate %q", sessID, id)
			}
			seenMRU[id] = struct{}{}
		}
	}
}

func assertReduceFuzzEffects(t *testing.T, effs []Effect) {
	t.Helper()
	for _, eff := range effs {
		switch eff.(type) {
		case EffSpawnFrame,
			EffKillFrame,
			EffRegisterFrame,
			EffUnregisterFrame,
			EffSetSessionEnv,
			EffUnsetSessionEnv,
			EffReleaseFrameSandboxes,
			EffReleaseFrameSandbox,
			EffSendResponse,
			EffSendResponseSync,
			EffSendError,
			EffBroadcastSessionsChanged,
			EffBroadcastEvent,
			EffCloseConn,
			EffSendFrameKeys,
			EffPersistSnapshot,
			EffWatchFile,
			EffUnwatchFile,
			EffEventLogAppend,
			EffToolLogAppend,
			EffReconcileWindows,
			EffStartJob,
			EffRecordNotification,
			EffSurfaceSubscribeStart,
			EffSurfaceSubscribeStop,
			EffSurfaceResize,
			EffSurfaceWriteRaw,
			EffBroadcastSurfaceOutput,
			EffBroadcastPromptEvent:
		default:
			t.Fatalf("unexpected effect type %T", eff)
		}
	}
}
