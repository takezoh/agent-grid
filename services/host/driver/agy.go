package driver

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/takezoh/agent-grid/services/host/state"
)

const AgyDriverName = "agy"

type AgyState struct {
	GenericState
	ConversationID    string
	LastInvocation    int
	HasInvocation     bool
	TerminationReason string
}

type AgyDriver struct{ generic GenericDriver }

func NewAgyDriver() AgyDriver {
	return AgyDriver{generic: NewGenericDriver(AgyDriverName, AgyDriverName, 0)}
}

func (AgyDriver) Name() string        { return AgyDriverName }
func (AgyDriver) DisplayName() string { return AgyDriverName }
func (AgyDriver) Status(s state.DriverState) state.Status {
	return s.(AgyState).Status
}
func (AgyDriver) StartDir(s state.DriverState) string {
	if as, ok := s.(AgyState); ok {
		return as.StartDir
	}
	return ""
}
func (AgyDriver) WithStartDir(s state.DriverState, dir string) state.DriverState {
	as, ok := s.(AgyState)
	if ok {
		as.StartDir = dir
	}
	return as
}
func (d AgyDriver) NewState(now time.Time) state.DriverState {
	return AgyState{GenericState: d.generic.NewState(now).(GenericState)}
}
func (d AgyDriver) View(s state.DriverState) state.View {
	as, _ := s.(AgyState)
	return d.generic.View(as.GenericState)
}
func (d AgyDriver) PrepareCreate(s state.DriverState, id state.SessionID, project, command string, options state.LaunchOptions) (state.DriverState, state.CreateLaunch, error) {
	as, _ := s.(AgyState)
	next, launch, err := d.generic.PrepareCreate(as.GenericState, id, project, command, options)
	as.GenericState = next.(GenericState)
	return as, launch, err
}
func (d AgyDriver) PrepareLaunch(s state.DriverState, mode state.LaunchMode, project, command string, options state.LaunchOptions, recovered bool) (state.LaunchPlan, error) {
	as, _ := s.(AgyState)
	return d.generic.PrepareLaunch(as.GenericState, mode, project, command, options, recovered)
}

func (d AgyDriver) Step(prev state.DriverState, ctx state.FrameContext, ev state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	as, ok := prev.(AgyState)
	if !ok {
		as = d.NewState(time.Time{}).(AgyState)
	}
	if hook, ok := ev.(state.DEvHook); ok {
		as = applyAgyHook(as, hook)
		return as, nil, d.View(as)
	}
	next, effects, _ := d.generic.Step(as.GenericState, ctx, ev)
	as.GenericState = next.(GenericState)
	return as, effects, d.View(as)
}

type agyHookPayload struct {
	ConversationID    string `json:"conversationId"`
	InvocationNum     *int   `json:"invocationNum"`
	ExecutionNum      *int   `json:"executionNum"`
	FullyIdle         bool   `json:"fullyIdle"`
	TerminationReason string `json:"terminationReason"`
}

func applyAgyHook(as AgyState, hook state.DEvHook) AgyState {
	var payload agyHookPayload
	if json.Unmarshal(hook.Payload, &payload) != nil {
		return as
	}
	if payload.ConversationID != "" {
		as.ConversationID = payload.ConversationID
	}
	switch hook.Event {
	case "PreInvocation":
		if payload.InvocationNum != nil {
			if as.HasInvocation && *payload.InvocationNum < as.LastInvocation {
				return as
			}
			as.LastInvocation, as.HasInvocation = *payload.InvocationNum, true
		}
		as.ObserveStatus(StatusSourceHook, state.StatusRunning, hook.Timestamp)
	case "PostInvocation", "PostToolUse":
		as.ObserveStatus(StatusSourceHook, state.StatusRunning, hook.Timestamp)
	case "Stop":
		if payload.ExecutionNum != nil && as.HasInvocation && *payload.ExecutionNum < as.LastInvocation {
			return as
		}
		as.TerminationReason = payload.TerminationReason
		if payload.FullyIdle {
			as.ObserveStatus(StatusSourceHook, state.StatusWaiting, hook.Timestamp)
		}
	}
	return as
}

func (AgyDriver) Persist(s state.DriverState) map[string]string {
	as, ok := s.(AgyState)
	if !ok {
		return nil
	}
	out := map[string]string{}
	as.PersistCommon(out)
	if as.ConversationID != "" {
		out["agy_conversation_id"] = as.ConversationID
	}
	if as.HasInvocation {
		out["agy_invocation_num"] = strconv.Itoa(as.LastInvocation)
	}
	if as.TerminationReason != "" {
		out["agy_termination_reason"] = as.TerminationReason
	}
	return out
}

func (d AgyDriver) Restore(bag map[string]string, now time.Time) state.DriverState {
	as := d.NewState(now).(AgyState)
	as.RestoreCommon(bag)
	as.ConversationID = bag["agy_conversation_id"]
	as.TerminationReason = bag["agy_termination_reason"]
	if raw := bag["agy_invocation_num"]; raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			as.LastInvocation, as.HasInvocation = n, true
		}
	}
	return as
}
