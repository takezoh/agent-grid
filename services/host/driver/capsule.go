package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/takezoh/agent-grid/services/host/state"
)

// Resume-capsule support: CapsuleSource implementations for the agent
// drivers plus the worker runner that pipes the reducer-built prompt
// into the configured capsule command and parses the JSON reply.

func capsuleTurnsFromSummary(turns []SummaryTurn) []state.CapsuleTurn {
	if len(turns) == 0 {
		return nil
	}
	out := make([]state.CapsuleTurn, len(turns))
	for i, t := range turns {
		out[i] = state.CapsuleTurn{Role: t.Role, Text: t.Text}
	}
	return out
}

func (d ClaudeDriver) CapsuleTurns(s state.DriverState) []state.CapsuleTurn {
	cs, ok := s.(ClaudeState)
	if !ok {
		return nil
	}
	return capsuleTurnsFromSummary(cs.RecentTurns)
}

func (d CodexDriver) CapsuleTurns(s state.DriverState) []state.CapsuleTurn {
	cs, ok := s.(CodexState)
	if !ok {
		return nil
	}
	return capsuleTurnsFromSummary(cs.RecentTurns)
}

func (d GeminiDriver) CapsuleTurns(s state.DriverState) []state.CapsuleTurn {
	cs, ok := s.(GeminiState)
	if !ok {
		return nil
	}
	return capsuleTurnsFromSummary(cs.RecentTurns)
}

// newCapsuleRunner builds the worker-pool runner for resume-capsule
// generation. Empty command → permanent no-op (zero result, no digest),
// which the reducer treats as "not configured".
func newCapsuleRunner(capsuleCmd string) func(context.Context, state.CapsuleJobInput) (state.CapsuleJobResult, error) {
	return func(ctx context.Context, in state.CapsuleJobInput) (state.CapsuleJobResult, error) {
		if capsuleCmd == "" || strings.TrimSpace(in.Prompt) == "" {
			return state.CapsuleJobResult{}, nil
		}
		jobCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		out, err := summarizeWithCommand(jobCtx, in.Prompt, capsuleCmd)
		if err != nil {
			return state.CapsuleJobResult{}, err
		}
		parsed, err := parseCapsuleJSON(out)
		if err != nil {
			return state.CapsuleJobResult{}, err
		}
		parsed.SourceDigest = in.SourceDigest
		parsed.SourceTurns = in.SourceTurns
		return parsed, nil
	}
}

type capsuleWireJSON struct {
	Objective    string   `json:"objective"`
	CurrentState string   `json:"current_state"`
	NextAction   string   `json:"next_action"`
	Unresolved   []string `json:"unresolved"`
}

// parseCapsuleJSON extracts the capsule JSON object from LLM output.
// Models frequently wrap JSON in code fences or prose; the parser takes
// the outermost {...} span. Anything unparseable is a hard error — a
// capsule that cannot be parsed must never be displayed (§29.2).
func parseCapsuleJSON(out string) (state.CapsuleJobResult, error) {
	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return state.CapsuleJobResult{}, fmt.Errorf("capsule output contains no JSON object")
	}
	var wire capsuleWireJSON
	if err := json.Unmarshal([]byte(out[start:end+1]), &wire); err != nil {
		return state.CapsuleJobResult{}, fmt.Errorf("capsule JSON parse: %w", err)
	}
	if strings.TrimSpace(wire.Objective) == "" && strings.TrimSpace(wire.CurrentState) == "" {
		return state.CapsuleJobResult{}, fmt.Errorf("capsule JSON has no objective or current_state")
	}
	return state.CapsuleJobResult{
		Objective:    strings.TrimSpace(wire.Objective),
		CurrentState: strings.TrimSpace(wire.CurrentState),
		NextAction:   strings.TrimSpace(wire.NextAction),
		Unresolved:   trimNonEmpty(wire.Unresolved),
	}, nil
}

func trimNonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
