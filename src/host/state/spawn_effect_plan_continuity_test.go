package state

import (
	"reflect"
	"testing"
	"time"
)

// TestSpawnEffect_PlanFieldContinuity is the Tier T0 reflection-driven guard
// against future field-drop regressions of the same class as the
// ManagedFrameMessaging silent-drop defect (spec-20260714, FR-001).
//
// The recursive walker populates every reachable exported field of
// state.LaunchPlan (including nested exported structs LaunchOptions,
// StreamLaunchOptions, WorktreeOption, ResumeTarget) with a unique
// sentinel per leaf kind, then asserts reflect.DeepEqual between the plan
// passed into spawnEffect and the Plan field of the returned EffSpawnFrame.
//
// Currently-failing sentinels this test would have caught: Argv,
// PreCommands, PreCommandTimeout, ManagedFrameMessaging. Those fields were
// silently dropped by the pre-fix interpret_spawn.go reconstruction literal
// (masked in production only because stream/backend.go's BindFrame overwrote
// Argv / PreCommands on the codex stream path). This test asserts the
// invariant categorically: no field is dropped, defaulted, or reconstructed
// along the spawnEffect boundary.
func TestSpawnEffect_PlanFieldContinuity(t *testing.T) {
	var plan LaunchPlan
	populateSentinel(t, reflect.ValueOf(&plan).Elem(), "LaunchPlan")

	eff := spawnEffect("s-sentinel", "f-sentinel", plan, 42, "r-sentinel")

	if !reflect.DeepEqual(eff.Plan, plan) {
		t.Fatalf("EffSpawnFrame.Plan drifted from input plan.\n input: %+v\n got:   %+v", plan, eff.Plan)
	}
}

// TestSpawnEffect_PlanFieldContinuity_WalkerCoverage proves the walker fixture
// itself is not silently incomplete — every reachable exported field must be
// non-zero after populateSentinel returns. If a future LaunchPlan field lands
// with a leaf kind the walker does not handle, this test fails immediately
// naming the zero field-path, forcing the fixture to be extended.
func TestSpawnEffect_PlanFieldContinuity_WalkerCoverage(t *testing.T) {
	var plan LaunchPlan
	populateSentinel(t, reflect.ValueOf(&plan).Elem(), "LaunchPlan")

	walkFields(reflect.ValueOf(plan), "LaunchPlan", func(path string, v reflect.Value) {
		if v.IsZero() {
			t.Errorf("walker left field %q at zero value — extend populateSentinel to cover %s", path, v.Kind())
		}
	})
}

// populateSentinel recursively fills every reachable exported field with a
// distinct sentinel value keyed by the field-path. The path label doubles as
// the string sentinel, so a silent write to the wrong field surfaces as a
// diff that names the exact traversal step.
func populateSentinel(t *testing.T, v reflect.Value, path string) {
	t.Helper()
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			populateSentinel(t, v.Field(i), path+"."+field.Name)
		}
	case reflect.String:
		v.SetString(path)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// time.Duration is Int64; distinct per path via the primeFor hash.
		v.SetInt(int64(primeFor(path)))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(primeFor(path)))
	case reflect.Slice:
		et := v.Type().Elem()
		switch et.Kind() {
		case reflect.Uint8: // []byte
			v.SetBytes([]byte(path))
		case reflect.String: // []string
			v.Set(reflect.ValueOf([]string{path}))
		case reflect.Slice: // [][]string (or similar)
			if et.Elem().Kind() == reflect.String {
				v.Set(reflect.ValueOf([][]string{{path}}))
			} else {
				t.Fatalf("unhandled nested slice kind at %s: %s", path, et.Elem().Kind())
			}
		default:
			t.Fatalf("unhandled slice element kind at %s: %s", path, et.Kind())
		}
	case reflect.Map:
		mt := reflect.MakeMap(v.Type())
		keyVal := reflect.New(v.Type().Key()).Elem()
		valVal := reflect.New(v.Type().Elem()).Elem()
		populateSentinel(t, keyVal, path+".<key>")
		populateSentinel(t, valVal, path+".<val>")
		mt.SetMapIndex(keyVal, valVal)
		v.Set(mt)
	default:
		t.Fatalf("populateSentinel: unhandled kind %s at %s (extend fixture)", v.Kind(), path)
	}
}

// walkFields visits every reachable exported field. Used by the walker
// coverage self-test to confirm no field is silently skipped.
func walkFields(v reflect.Value, path string, visit func(string, reflect.Value)) {
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			walkFields(v.Field(i), path+"."+field.Name, visit)
		}
		return
	}
	visit(path, v)
}

// primeFor returns a distinct non-zero integer per field-path. Small primes
// suffice — reflect.DeepEqual is what proves equality; distinctness only
// matters for zero-value detection in the walker self-test.
func primeFor(path string) int {
	h := 2166136261 // FNV-1a offset basis (uint32)
	for _, b := range []byte(path) {
		h ^= int(b)
		h *= 16777619
	}
	// Fold to a positive non-zero int with room for time.Duration.
	if h < 0 {
		h = -h
	}
	if h == 0 {
		h = 1
	}
	// Keep values under a second so a Duration-typed field does not blow
	// out into pathological wait times if a future test uses it.
	const maxNs = int(time.Second)
	return (h % maxNs) + 1
}
