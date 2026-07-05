package driver

import (
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

const (
	claudeKeyForkParentID = "fork_parent_id"
	claudeKeyModel        = "claude_model"
	claudeKeyEffort       = "claude_effort"
	claudeLegacyKeyModel  = "model"
	claudeLegacyKeyEffort = "effort"
)

// Persist writes the Claude driver state into the bag SessionService
// round-trips through sessions.json. Status is persisted so warm /
// cold restart restores the prior status without resetting to Idle,
// plus the cached branch tag so the user sees the prior branch
// immediately on restart, plus the rolling haiku summary.
func (ClaudeDriver) Persist(s state.DriverState) map[string]string {
	cs, ok := s.(ClaudeState)
	if !ok {
		return nil
	}
	out := make(map[string]string, 10)
	cs.PersistCommon(out)
	if cs.ClaudeSessionID != "" {
		out[claudeKeyClaudeSessionID] = cs.ClaudeSessionID
	}
	if cs.Model != "" {
		out[claudeKeyModel] = cs.Model
	}
	if cs.Effort != "" {
		out[claudeKeyEffort] = cs.Effort
	}
	if cs.ModelSet {
		out[claudeKeyModelSet] = "1"
	}
	if cs.EffortSet {
		out[claudeKeyEffortSet] = "1"
	}
	if cs.ModelAuthoritative {
		out[claudeKeyModelAuthoritative] = "1"
	}
	if cs.EffortAuthoritative {
		out[claudeKeyEffortAuthoritative] = "1"
	}
	if cs.ForkParentID != "" {
		out[claudeKeyForkParentID] = cs.ForkParentID
	}
	return out
}

// Restore rehydrates ClaudeState from a persisted bag. Empty bags
// produce a fresh state stamped with `now`.
func (d ClaudeDriver) Restore(bag map[string]string, now time.Time) state.DriverState {
	cs := ClaudeState{
		CommonState: CommonState{
			Status:          state.StatusIdle,
			StatusChangedAt: now,
		},
	}
	if len(bag) == 0 {
		return cs
	}
	cs.RestoreCommon(bag)
	cs.ClaudeSessionID = bag[claudeKeyClaudeSessionID]
	cs.Model = firstPersistedValue(bag, claudeKeyModel, claudeLegacyKeyModel)
	cs.Effort = firstPersistedValue(bag, claudeKeyEffort, claudeLegacyKeyEffort)
	cs.ModelSet = bag[claudeKeyModelSet] == "1" || cs.Model != ""
	cs.EffortSet = bag[claudeKeyEffortSet] == "1" || cs.Effort != ""
	cs.ModelAuthoritative = bag[claudeKeyModelAuthoritative] == "1"
	cs.EffortAuthoritative = bag[claudeKeyEffortAuthoritative] == "1"
	cs.ForkParentID = bag[claudeKeyForkParentID]
	return cs
}

func firstPersistedValue(bag map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := bag[key]; value != "" {
			return value
		}
	}
	return ""
}
