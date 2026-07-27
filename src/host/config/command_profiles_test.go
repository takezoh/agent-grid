package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestCommandProfilesMaterializeTypedValues(t *testing.T) {
	min := 1.0
	max := 10.0
	session := SessionConfig{CommandProfiles: []CommandProfileConfig{{
		ID: "sol", Label: "Sol", Executable: "codex",
		Argv: []string{"--model", "{{model}}", "--verbose", "{{verbose}}", "--limit", "{{limit}}"},
		Parameters: []CommandParameterConfig{
			{Name: "model", Type: ParameterEnum, Values: []string{"sol", "terra"}, Default: "sol"},
			{Name: "verbose", Type: ParameterBoolean, Default: false},
			{Name: "limit", Type: ParameterNumber, Min: &min, Max: &max, Required: true},
		},
	}}}
	if err := session.ValidateCommandProfiles(); err != nil {
		t.Fatal(err)
	}
	profile := session.ProfilesForProject("/repo")[0]
	got, err := session.MaterializeProfile("/repo", "sol", profile.Revision, map[string]any{
		"model": "terra", "verbose": true, "limit": float64(5),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "--model", "terra", "--verbose", "true", "--limit", "5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestCommandProfilesRejectUnknownAndStaleValues(t *testing.T) {
	session := SessionConfig{CommandProfiles: []CommandProfileConfig{{
		ID: "agent", Executable: "agent", Argv: []string{"{{effort}}"},
		Parameters: []CommandParameterConfig{{
			Name: "effort", Type: ParameterEnum, Values: []string{"low", "high"}, Required: true,
		}},
	}}}
	profile := session.ProfilesForProject("/repo")[0]
	if _, err := session.MaterializeProfile("/repo", "agent", "stale", nil); err == nil ||
		!strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale error = %v", err)
	}
	if _, err := session.MaterializeProfile(
		"/repo", "agent", profile.Revision, map[string]any{"effort": "ultra"}); err == nil {
		t.Fatal("invalid enum value accepted")
	}
	if _, err := session.MaterializeProfile(
		"/repo", "agent", profile.Revision, map[string]any{"other": "x"}); err == nil {
		t.Fatal("unknown value accepted")
	}
}

func TestCommandProfilesProjectScope(t *testing.T) {
	session := SessionConfig{
		CommandProfiles: []CommandProfileConfig{{
			ID: "only-a", Executable: "agent", Projects: []string{"/a"},
		}},
	}
	a := SortedProfileIDs(session.ProfilesForProject("/a"))
	b := SortedProfileIDs(session.ProfilesForProject("/b"))
	if !reflect.DeepEqual(a, []string{"only-a"}) {
		t.Fatalf("/a profiles = %v", a)
	}
	if len(b) != 0 {
		t.Fatalf("/b profiles = %v", b)
	}
}

func TestCommandProfilesRenderParameterInsideOneArgvToken(t *testing.T) {
	session := SessionConfig{CommandProfiles: []CommandProfileConfig{{
		ID: "setting", Executable: "agent", Argv: []string{`effort="{{value}}"`},
		Parameters: []CommandParameterConfig{{
			Name: "value", Type: ParameterString, Default: "x",
		}},
	}}}
	if err := session.ValidateCommandProfiles(); err != nil {
		t.Fatal(err)
	}
	profile := session.ProfilesForProject("/repo")[0]
	argv, err := session.MaterializeProfile("/repo", profile.ID, profile.Revision, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(argv, []string{"agent", `effort="x"`}) {
		t.Fatalf("argv = %v", argv)
	}
}

func TestCommandProfilesRejectUnknownAndOptionalArgvParameters(t *testing.T) {
	for _, profile := range []CommandProfileConfig{
		{ID: "unknown", Executable: "agent", Argv: []string{"{{missing}}"}},
		{ID: "malformed", Executable: "agent", Argv: []string{"{{missing"}},
		{
			ID: "optional", Executable: "agent", Argv: []string{"{{note}}"},
			Parameters: []CommandParameterConfig{{Name: "note", Type: ParameterString}},
		},
	} {
		session := SessionConfig{CommandProfiles: []CommandProfileConfig{profile}}
		if err := session.ValidateCommandProfiles(); err == nil {
			t.Fatalf("invalid profile %q accepted", profile.ID)
		}
	}
}
