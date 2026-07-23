package driver

import (
	"time"

	"github.com/takezoh/agent-grid/host/state"
)

func (CodexDriver) Persist(s state.DriverState) map[string]string {
	cs, ok := s.(CodexState)
	if !ok {
		return nil
	}
	out := make(map[string]string, 13)
	cs.PersistCommon(out)
	if cs.ThreadID != "" {
		out[codexKeyThreadID] = cs.ThreadID
	}
	if cs.SessionID != "" {
		out[codexKeySessionID] = cs.SessionID
	}
	if cs.RolloutPath != "" {
		out[codexKeyRolloutPath] = cs.RolloutPath
	}
	if cs.Model != "" {
		out[codexKeyModel] = cs.Model
	}
	if cs.Effort != "" {
		out[codexKeyEffort] = cs.Effort
	}
	if cs.ModelSet {
		out[codexKeyModelSet] = "1"
	}
	if cs.EffortSet {
		out[codexKeyEffortSet] = "1"
	}
	if cs.ModelAuthoritative {
		out[codexKeyModelAuthoritative] = "1"
	}
	if cs.EffortAuthoritative {
		out[codexKeyEffortAuthoritative] = "1"
	}
	if cs.RequestedThreadID != "" {
		out[codexKeyRequestedThreadID] = cs.RequestedThreadID
	}
	if cs.ObservedThreadID != "" {
		out[codexKeyObservedThreadID] = cs.ObservedThreadID
	}
	if cs.ResumePhase != "" {
		out[codexKeyResumePhase] = cs.ResumePhase
	}
	if cs.Preview != "" {
		out[codexKeyPreview] = cs.Preview
	}
	if cs.DisplayFallback != "" {
		out[codexKeyDisplayFallback] = cs.DisplayFallback
	}
	return out
}

func (d CodexDriver) Restore(bag map[string]string, now time.Time) state.DriverState {
	cs := CodexState{
		CommonState: CommonState{
			Status:          state.StatusIdle,
			StatusChangedAt: now,
		},
	}
	if len(bag) == 0 {
		return cs
	}
	cs.RestoreCommon(bag)
	cs.ThreadID = bag[codexKeyThreadID]
	cs.SessionID = bag[codexKeySessionID]
	cs.RolloutPath = bag[codexKeyRolloutPath]
	cs.Model = firstPersistedValue(bag, codexKeyModel, codexLegacyKeyModel)
	cs.Effort = firstPersistedValue(bag, codexKeyEffort, codexLegacyKeyEffort)
	cs.ModelSet = bag[codexKeyModelSet] == "1" || cs.Model != ""
	cs.EffortSet = bag[codexKeyEffortSet] == "1" || cs.Effort != ""
	cs.ModelAuthoritative = bag[codexKeyModelAuthoritative] == "1"
	cs.EffortAuthoritative = bag[codexKeyEffortAuthoritative] == "1"
	if cs.RolloutPath == "" && cs.TranscriptPath != "" {
		cs.RolloutPath = cs.TranscriptPath
	}
	cs.RequestedThreadID = bag[codexKeyRequestedThreadID]
	cs.ObservedThreadID = bag[codexKeyObservedThreadID]
	cs.ResumePhase = bag[codexKeyResumePhase]
	cs.Preview = bag[codexKeyPreview]
	cs.DisplayFallback = bag[codexKeyDisplayFallback]
	if cs.DisplayFallback == "" {
		cs.DisplayFallback = codexDisplayFallback(cs.Preview, cs.LastPrompt)
	}
	if cs.TranscriptPath == "" && cs.RolloutPath != "" {
		cs.TranscriptPath = cs.RolloutPath
	}
	return cs
}
