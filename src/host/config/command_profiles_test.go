package config

import (
	"reflect"
	"strings"
	"testing"
)

func mustResolveProfiles(
	t *testing.T, session SessionConfig, project string,
) []ResolvedCommandProfile {
	t.Helper()
	profiles, err := session.ResolveProfiles(project)
	if err != nil {
		t.Fatal(err)
	}
	return profiles
}

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
	profiles := mustResolveProfiles(t, session, t.TempDir())
	got, err := MaterializeResolvedProfile(profiles, "sol", profiles[0].Revision, map[string]any{
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
	profiles := mustResolveProfiles(t, session, t.TempDir())
	profile := profiles[0]
	if _, err := MaterializeResolvedProfile(profiles, "agent", "stale", nil); err == nil ||
		!strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale error = %v", err)
	}
	if _, err := MaterializeResolvedProfile(
		profiles, "agent", profile.Revision, map[string]any{"effort": "ultra"}); err == nil {
		t.Fatal("invalid enum value accepted")
	}
	if _, err := MaterializeResolvedProfile(
		profiles, "agent", profile.Revision, map[string]any{"other": "x"}); err == nil {
		t.Fatal("unknown value accepted")
	}
}

func TestCommandProfilesProjectSettingsReplaceByIDAndAppend(t *testing.T) {
	project := t.TempDir()
	writeProjectSettings(t, project, `
[[session.command_profiles]]
id = "shared"
executable = "project-shared"

[[session.command_profiles]]
id = "local"
executable = "project-local"

[[session.command_profiles]]
id = "removed"
disabled = true
`)
	session := SessionConfig{CommandProfiles: []CommandProfileConfig{
		{ID: "shared", Executable: "server-shared"},
		{ID: "global", Executable: "server-global"},
		{ID: "removed", Executable: "server-removed"},
	}}
	profiles := mustResolveProfiles(t, session, project)
	if ids := SortedProfileIDs(profiles); !reflect.DeepEqual(ids, []string{"global", "local", "shared"}) {
		t.Fatalf("profile ids = %v", ids)
	}
	if profiles[0].ID != "shared" || profiles[0].Executable != "project-shared" {
		t.Fatalf("project replacement = %#v", profiles[0])
	}
}

func TestServerCommandProfilesRejectDisabledEntry(t *testing.T) {
	session := SessionConfig{CommandProfiles: []CommandProfileConfig{{
		ID: "disabled", Disabled: true,
	}}}
	if err := session.ValidateCommandProfiles(); err == nil {
		t.Fatal("server-level disabled profile accepted")
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
	profiles := mustResolveProfiles(t, session, t.TempDir())
	profile := profiles[0]
	argv, err := MaterializeResolvedProfile(profiles, profile.ID, profile.Revision, nil)
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
