package driver

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/client/driver/drivertest"
	"github.com/takezoh/agent-grid/client/state"
)

func TestDriverRegistryConformanceCoversBuiltins(t *testing.T) {
	registerConformanceDrivers()

	names := runRegistryConformance(t)
	want := []string{"claude", "codex", "gemini", "generic", "grok", "shell"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("registry conformance names = %v, want %v", names, want)
	}
}

func TestDriverRegistryConformanceAutomaticallyIncludesNewRegistrations(t *testing.T) {
	registerConformanceDrivers()
	state.Register(conformanceStubDriver{name: "stub"})

	names := runRegistryConformance(t)
	if !slices.Contains(names, "stub") {
		t.Fatalf("registry conformance names = %v, want stub included", names)
	}
}

func TestDriverMetadataSourcePriorityConformance(t *testing.T) {
	d, _, _ := newClaude(t)
	drivertest.MetadataSourcePriority(t, d, drivertest.MetadataScenario{
		Name: "claude_hook",
		SeedFallback: func(now time.Time) state.DriverState {
			cs := d.NewState(now).(ClaudeState)
			cs.Model = "seed-model"
			cs.ModelSet = true
			cs.Effort = "seed-effort"
			cs.EffortSet = true
			return cs
		},
		ApplyAuthoritative: func(prev state.DriverState, now time.Time) state.DriverState {
			next, _, _ := d.Step(prev, state.FrameContext{IsRoot: true}, hookEventAny("Stop", map[string]any{
				"session_id":      "uuid",
				"hook_event_name": "Stop",
				"model":           "authoritative-model",
				"effort":          map[string]any{"level": "high"},
			}, now))
			return next
		},
		ApplyFallback: func(prev state.DriverState, now time.Time) state.DriverState {
			next := d.handleJobResult(prev.(ClaudeState), state.DEvJobResult{
				Now:    now,
				Result: TranscriptParseResult{Model: "fallback-model"},
			})
			plan, err := d.PrepareLaunch(next, state.LaunchModeColdStart, "/repo", "claude --model fallback-model --effort low", state.LaunchOptions{}, false)
			if err != nil {
				panic(err)
			}
			if plan.Command != "claude --model authoritative-model --effort high" {
				panic("claude launch fallback overrode authoritative metadata: " + plan.Command)
			}
			return next
		},
		ClearAuthoritative: func(prev state.DriverState, now time.Time) state.DriverState {
			next, _, _ := d.Step(prev, state.FrameContext{IsRoot: true}, hookEventAny("Stop", map[string]any{
				"session_id":      "uuid",
				"hook_event_name": "Stop",
				"model":           "",
				"effort":          map[string]any{"level": ""},
			}, now))
			return next
		},
		Read: func(s state.DriverState) drivertest.MetadataSnapshot {
			cs := s.(ClaudeState)
			return drivertest.MetadataSnapshot{
				Model:     cs.Model,
				ModelSet:  cs.ModelSet,
				Effort:    cs.Effort,
				EffortSet: cs.EffortSet,
			}
		},
	})

	codex, _, _ := newCodex(t)
	drivertest.MetadataSourcePriority(t, codex, drivertest.MetadataScenario{
		Name: "codex_thread_settings_updated",
		SeedFallback: func(now time.Time) state.DriverState {
			cs := codex.NewState(now).(CodexState)
			cs.Model = "seed-model"
			cs.ModelSet = true
			cs.Effort = "seed-effort"
			cs.EffortSet = true
			return cs
		},
		ApplyAuthoritative: func(prev state.DriverState, now time.Time) state.DriverState {
			next, _, _ := codex.Step(prev, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
				Source:    state.SubsystemStream,
				Kind:      state.SubsystemMetadataUpdated,
				Timestamp: now,
				Payload: state.SubsystemPayload{
					SessionID: "thread-1",
					TargetID:  "thread-1",
					Model:     "authoritative-model",
					ModelSet:  true,
					Effort:    "high",
					EffortSet: true,
				},
			})
			return next
		},
		ApplyFallback: func(prev state.DriverState, now time.Time) state.DriverState {
			next := codex.handleJobResult(prev.(CodexState), state.DEvJobResult{
				Now: now,
				Result: CodexTranscriptParseResult{
					Model:     "fallback-model",
					ModelSet:  true,
					Effort:    "low",
					EffortSet: true,
				},
			})
			return next
		},
		ClearAuthoritative: func(prev state.DriverState, now time.Time) state.DriverState {
			next, _, _ := codex.Step(prev, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
				Source:    state.SubsystemStream,
				Kind:      state.SubsystemMetadataUpdated,
				Timestamp: now,
				Payload: state.SubsystemPayload{
					SessionID: "thread-1",
					TargetID:  "thread-1",
					Model:     "",
					ModelSet:  true,
					Effort:    "",
					EffortSet: true,
				},
			})
			return next
		},
		Read: func(s state.DriverState) drivertest.MetadataSnapshot {
			cs := s.(CodexState)
			return drivertest.MetadataSnapshot{
				Model:     cs.Model,
				ModelSet:  cs.ModelSet,
				Effort:    cs.Effort,
				EffortSet: cs.EffortSet,
			}
		},
	})
}

func registerConformanceDrivers() {
	state.ClearRegistry()
	for _, drv := range builtinDrivers(RegisterOptions{
		Home:          testHome,
		EventLogDir:   testEventLogDir,
		IdleThreshold: 30 * time.Second,
		Pager:         "less",
	}) {
		state.Register(drv)
	}
	state.Register(NewShellDriver(ShellDriverName, ShellDriverName, 30*time.Second))
}

func runRegistryConformance(t *testing.T) []string {
	t.Helper()

	registry := state.GetRegistry()
	names := make([]string, 0, len(registry))
	for name, drv := range registry {
		label := name
		if label == "" {
			label = "generic"
		}
		names = append(names, label)
		t.Run(label, func(t *testing.T) {
			drivertest.Conformance(t, drv)
		})
	}
	slices.Sort(names)
	return names
}

type conformanceStubDriver struct {
	name string
}

func (d conformanceStubDriver) Name() string        { return d.name }
func (d conformanceStubDriver) DisplayName() string { return d.name }
func (d conformanceStubDriver) NewState(now time.Time) state.DriverState {
	return GenericState{
		Name: d.name,
		CommonState: CommonState{
			Status:          state.StatusWaiting,
			StatusChangedAt: now,
		},
	}
}

func (d conformanceStubDriver) Step(prev state.DriverState, _ state.FrameContext, _ state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	gs, ok := prev.(GenericState)
	if !ok {
		gs = d.NewState(time.Time{}).(GenericState)
	}
	return gs, nil, state.View{Status: gs.Status, StatusChangedAt: gs.StatusChangedAt}
}

func (conformanceStubDriver) Status(s state.DriverState) state.Status {
	return s.(GenericState).Status
}

func (d conformanceStubDriver) View(s state.DriverState) state.View {
	gs := s.(GenericState)
	return state.View{Status: gs.Status, StatusChangedAt: gs.StatusChangedAt}
}

func (conformanceStubDriver) Persist(s state.DriverState) map[string]string {
	return NewGenericDriver("", "", 0).Persist(s)
}

func (d conformanceStubDriver) Restore(bag map[string]string, now time.Time) state.DriverState {
	gs := NewGenericDriver(d.name, d.name, 0).Restore(bag, now).(GenericState)
	gs.Name = d.name
	return gs
}

func (conformanceStubDriver) PrepareLaunch(_ state.DriverState, _ state.LaunchMode, project, baseCommand string, options state.LaunchOptions, _ bool) (state.LaunchPlan, error) {
	return state.LaunchPlan{
		Command:  baseCommand,
		StartDir: project,
		Options:  options,
	}, nil
}
