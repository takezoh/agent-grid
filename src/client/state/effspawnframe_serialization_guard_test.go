package state

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestEffSpawnFrameIsNotSerialized pins NFR-004 from
// spec-20260714-launchplan-effect-continuity: EffSpawnFrame is an
// in-process effect value only. It must not acquire wire-serialization
// affordances (json / msgpack / proto struct tags, or json.Marshaler /
// json.Unmarshaler implementations) — future serialization tests / tools
// would then quietly rely on the struct shape and lock us out of the
// zero-cost struct-shape refactors adr-20260714-launchplan-effect-embedding
// depends on.
//
// If this test fails after a legitimate wire-boundary change, revisit the
// NFR before deleting the assertion.
func TestEffSpawnFrameIsNotSerialized(t *testing.T) {
	typ := reflect.TypeOf(EffSpawnFrame{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		for _, tag := range []string{"json", "msgpack", "proto", "yaml", "xml", "toml"} {
			if v, ok := field.Tag.Lookup(tag); ok {
				t.Errorf("EffSpawnFrame.%s has %q struct tag = %q — this type must stay in-process only (NFR-004)", field.Name, tag, v)
			}
		}
	}

	// EffSpawnFrame value type — check both value and pointer receivers, since
	// json.Marshaler is satisfied at whichever level a future author declares it.
	marshalerType := reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	unmarshalerType := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	valueType := reflect.TypeOf(EffSpawnFrame{})
	ptrType := reflect.PointerTo(valueType)
	for _, tp := range []reflect.Type{valueType, ptrType} {
		if tp.Implements(marshalerType) {
			t.Errorf("EffSpawnFrame (%s) implements json.Marshaler — remove it or update this guard (NFR-004)", tp)
		}
		if tp.Implements(unmarshalerType) {
			t.Errorf("EffSpawnFrame (%s) implements json.Unmarshaler — remove it or update this guard (NFR-004)", tp)
		}
	}
}
