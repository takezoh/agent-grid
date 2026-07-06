package drivertest

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

type MetadataSnapshot struct {
	Model     string
	ModelSet  bool
	Effort    string
	EffortSet bool
}

type MetadataScenario struct {
	Name               string
	SeedFallback       func(now time.Time) state.DriverState
	ApplyAuthoritative func(prev state.DriverState, now time.Time) state.DriverState
	ApplyFallback      func(prev state.DriverState, now time.Time) state.DriverState
	ClearAuthoritative func(prev state.DriverState, now time.Time) state.DriverState
	Read               func(s state.DriverState) MetadataSnapshot
}

func Conformance(t *testing.T, drv state.Driver) {
	t.Helper()

	now := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	ctx := state.FrameContext{
		ID:        "root",
		Project:   "/repo",
		Command:   drv.Name(),
		CreatedAt: now,
		IsRoot:    true,
	}
	evs := state.ConformanceDriverEvents(now)

	current := drv.NewState(now)
	reached := []state.DriverState{current}
	assertViewAndStatusTotality(t, drv, current, "new_state")

	for i, ev := range evs {
		snapshot := cloneDriverState(current)
		next := callStep(t, drv, current, ctx, ev, i)
		if !reflect.DeepEqual(snapshot, current) {
			t.Fatalf("Step mutated previous state for %T", ev)
		}
		reached = append(reached, next)
		assertViewAndStatusTotality(t, drv, next, eventName(ev))
		current = next
	}

	restored := drv.Restore(drv.Persist(current), now.Add(time.Hour))
	if got, want := drv.Status(restored), drv.Status(current); got != want {
		t.Fatalf("Status after round-trip = %v, want %v", got, want)
	}
	if got, want := drv.View(restored), drv.View(current); !reflect.DeepEqual(got, want) {
		t.Fatalf("View after round-trip mismatch: got %+v want %+v", got, want)
	}
	assertViewAndStatusTotality(t, drv, restored, "restored")
}

func MetadataSourcePriority(t *testing.T, drv state.Driver, scenario MetadataScenario) {
	t.Helper()

	now := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)

	t.Run(scenario.Name+"/authoritative_beats_fallback", func(t *testing.T) {
		seed := scenario.SeedFallback(now)
		authoritative := scenario.ApplyAuthoritative(seed, now.Add(time.Second))
		next := scenario.ApplyFallback(authoritative, now.Add(2*time.Second))

		if got, want := scenario.Read(next), scenario.Read(authoritative); !reflect.DeepEqual(got, want) {
			t.Fatalf("fallback overrode authoritative metadata: got %+v want %+v", got, want)
		}
	})

	t.Run(scenario.Name+"/clear_round_trip", func(t *testing.T) {
		cleared := scenario.ClearAuthoritative(scenario.SeedFallback(now), now.Add(time.Second))
		restored := drv.Restore(drv.Persist(cleared), now.Add(2*time.Second))
		if got, want := scenario.Read(restored), scenario.Read(cleared); !reflect.DeepEqual(got, want) {
			t.Fatalf("clear did not survive round-trip: got %+v want %+v", got, want)
		}
	})

	t.Run(scenario.Name+"/tri_state", func(t *testing.T) {
		unset := scenario.Read(drv.NewState(now))
		cleared := scenario.Read(scenario.ClearAuthoritative(drv.NewState(now), now.Add(time.Second)))
		if unset.ModelSet == cleared.ModelSet {
			t.Fatalf("model tri-state collapsed: unset=%+v cleared=%+v", unset, cleared)
		}
		if unset.EffortSet == cleared.EffortSet {
			t.Fatalf("effort tri-state collapsed: unset=%+v cleared=%+v", unset, cleared)
		}
	})
}

func callStep(t *testing.T, drv state.Driver, prev state.DriverState, ctx state.FrameContext, ev state.DriverEvent, index int) state.DriverState {
	t.Helper()

	var (
		next state.DriverState
		pan  any
	)
	func() {
		defer func() {
			pan = recover()
		}()
		next, _, _ = drv.Step(prev, ctx, ev)
	}()
	if pan != nil {
		t.Fatalf("Step panicked for event[%d] %T: %v", index, ev, pan)
	}
	return next
}

func assertViewAndStatusTotality(t *testing.T, drv state.Driver, s state.DriverState, label string) {
	t.Helper()

	defer func() {
		if pan := recover(); pan != nil {
			t.Fatalf("View/Status panicked for %s: %v", label, pan)
		}
	}()
	_ = drv.Status(s)
	_ = drv.View(s)
}

func cloneDriverState(in state.DriverState) state.DriverState {
	if in == nil {
		return nil
	}
	// Step purity is checked by value via a JSON round-trip clone so the
	// previous state cannot alias the post-Step state. DriverState therefore
	// needs to remain JSON-round-trippable, which matches the Persist/Restore
	// contract this suite also checks.
	raw, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	out := reflect.New(reflect.TypeOf(in))
	if err := json.Unmarshal(raw, out.Interface()); err != nil {
		panic(err)
	}
	return out.Elem().Interface().(state.DriverState)
}

func eventName(ev state.DriverEvent) string {
	return reflect.TypeOf(ev).Name()
}
