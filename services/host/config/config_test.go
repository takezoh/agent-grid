package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	platformconfig "github.com/takezoh/agent-grid/services/platform/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Monitor.PollIntervalMs != 1000 {
		t.Errorf("PollIntervalMs = %d, want 1000", cfg.Monitor.PollIntervalMs)
	}
	if len(cfg.Session.CommandProfiles) != 1 ||
		cfg.Session.CommandProfiles[0].ID != "shell" {
		t.Errorf("CommandProfiles = %v, want default shell profile", cfg.Session.CommandProfiles)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if len(cfg.Session.PushCommands) != 1 || cfg.Session.PushCommands[0] != "shell" {
		t.Errorf("PushCommands = %v, want [shell]", cfg.Session.PushCommands)
	}
	if cfg.Terminal.ScrollbackLines != 10000 {
		t.Errorf("Terminal.ScrollbackLines = %d, want 10000", cfg.Terminal.ScrollbackLines)
	}
}

func TestConfigExcludesClientAndRetiredTUISettings(t *testing.T) {
	assertNoTOMLField(t, reflect.TypeOf(Config{}), "theme")
	assertNoTOMLField(t, reflect.TypeOf(Config{}), "editor")
	assertNoTOMLField(t, reflect.TypeOf(MonitorConfig{}), "fast_poll_interval_ms")
	assertNoTOMLField(t, reflect.TypeOf(SessionConfig{}), "auto_name")
}

func assertNoTOMLField(t *testing.T, typ reflect.Type, tag string) {
	t.Helper()
	for i := range typ.NumField() {
		if got := typ.Field(i).Tag.Get("toml"); got == tag {
			t.Errorf("%s still owns retired TOML setting %q", typ.Name(), tag)
		}
	}
}

// TestLoadFrom_TerminalScrollback pins the TOML wiring for the new
// `[terminal] scrollback_lines = N` knob — late-joining Web UI clients see
// up to this many scrolled-off rows on subscribe.
func TestLoadFrom_TerminalScrollback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[terminal]
scrollback_lines = 500
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Terminal.ScrollbackLines != 500 {
		t.Errorf("Terminal.ScrollbackLines = %d, want 500", cfg.Terminal.ScrollbackLines)
	}
}

func TestLoadFrom_PushCommands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[session]
push_commands = ["shell", "claude"]
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Session.PushCommands) != 2 {
		t.Fatalf("PushCommands = %v, want [shell, claude]", cfg.Session.PushCommands)
	}
	if cfg.Session.PushCommands[0] != "shell" || cfg.Session.PushCommands[1] != "claude" {
		t.Errorf("PushCommands = %v, want [shell, claude]", cfg.Session.PushCommands)
	}
}

func TestLoadFrom_CommandProfiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	err := os.WriteFile(path, []byte(`
[[session.command_profiles]]
id = "sol"
label = "Codex Sol"
executable = "codex"
argv = ["--model", "{{model}}", "-c", "model_reasoning_effort=\"{{effort}}\""]

[[session.command_profiles.parameters]]
name = "model"
label = "Model"
type = "enum"
default = "gpt-5.6-sol"
values = ["gpt-5.6-sol", "gpt-5.6-terra"]

[[session.command_profiles.parameters]]
name = "effort"
label = "Effort"
type = "enum"
default = "low"
values = ["low", "medium", "high"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := cfg.Session.ResolveProfiles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].ID != "sol" {
		t.Fatalf("profiles = %#v", profiles)
	}
	argv, err := MaterializeResolvedProfile(
		profiles, "sol", profiles[0].Revision,
		map[string]any{"model": "gpt-5.6-terra", "effort": "high"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"codex", "--model", "gpt-5.6-terra", "-c",
		`model_reasoning_effort="high"`,
	}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %v, want %v", argv, want)
	}
}

func TestLoadFrom_CommandProfilesRejectProjectsAllowList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	err := os.WriteFile(path, []byte(`
[[session.command_profiles]]
id = "sol"
executable = "codex"
projects = ["/repo/a"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("projects allow-list was silently accepted")
	}
}

func TestLoadFrom_LogLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[log]
level = "debug"
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := platformconfig.ExpandPath("~/foo")
	want := filepath.Join(home, "foo")
	if got != want {
		t.Errorf("ExpandPath(~/foo) = %q, want %q", got, want)
	}
	if got := platformconfig.ExpandPath("/abs/path"); got != "/abs/path" {
		t.Errorf("ExpandPath(/abs/path) = %q, want /abs/path", got)
	}
}

func TestListProjects(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "proj-a"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "proj-b"), 0o755)
	os.MkdirAll(filepath.Join(tmp, ".hidden"), 0o755)
	os.WriteFile(filepath.Join(tmp, "README"), []byte("hi"), 0o644)

	cfg := &Config{Projects: platformconfig.ProjectsConfig{ProjectRoots: []string{tmp}}}
	projects := cfg.ListProjects()
	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2; got %v", len(projects), projects)
	}
	names := map[string]bool{}
	for _, p := range projects {
		names[filepath.Base(p)] = true
	}
	if !names["proj-a"] || !names["proj-b"] {
		t.Errorf("expected proj-a and proj-b, got %v", projects)
	}
}

func TestListProjects_WithProjectPaths(t *testing.T) {
	tmp := t.TempDir()
	direct := filepath.Join(tmp, "direct-proj")
	os.MkdirAll(direct, 0o755)
	nonexistent := filepath.Join(tmp, "does-not-exist")

	cfg := &Config{Projects: platformconfig.ProjectsConfig{ProjectPaths: []string{direct, nonexistent}}}
	projects := cfg.ListProjects()
	if len(projects) != 1 {
		t.Fatalf("len(projects) = %d, want 1; got %v", len(projects), projects)
	}
	if projects[0] != direct {
		t.Errorf("projects[0] = %q, want %q", projects[0], direct)
	}
}

func TestListProjects_RootsAndPaths(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "roots", "proj-a"), 0o755)
	direct := filepath.Join(tmp, "direct-proj")
	os.MkdirAll(direct, 0o755)

	cfg := &Config{Projects: platformconfig.ProjectsConfig{
		ProjectRoots: []string{filepath.Join(tmp, "roots")},
		ProjectPaths: []string{direct},
	}}
	projects := cfg.ListProjects()
	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2; got %v", len(projects), projects)
	}
}

func TestResolveDataDir_Explicit(t *testing.T) {
	t.Setenv("AG_DATA_DIR", "")
	cfg := &Config{DataDir: "/tmp/data"}
	if got := cfg.ResolveDataDir(); got != "/tmp/data" {
		t.Errorf("ResolveDataDir() = %q, want /tmp/data", got)
	}
}

func TestResolveDataDir_Fallback(t *testing.T) {
	t.Setenv("AG_DATA_DIR", "")
	cfg := &Config{}
	want := ConfigDirPath()
	if got := cfg.ResolveDataDir(); got != want {
		t.Errorf("ResolveDataDir() = %q, want %q", got, want)
	}
}

func TestResolveDataDir_EnvOverride(t *testing.T) {
	t.Setenv("AG_DATA_DIR", "/foo")
	cfg := &Config{DataDir: "/bar"}
	if got := cfg.ResolveDataDir(); got != "/foo" {
		t.Errorf("ResolveDataDir() = %q, want /foo", got)
	}
}

func TestResolveDataDir_EnvExpand(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AG_DATA_DIR", "~/x")
	cfg := &Config{}
	want := home + "/x"
	if got := cfg.ResolveDataDir(); got != want {
		t.Errorf("ResolveDataDir() = %q, want %q", got, want)
	}
}

func TestLoadFrom_Missing(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path/settings.toml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Monitor.PollIntervalMs != 1000 {
		t.Fatalf("expected defaults, got PollIntervalMs=%d", cfg.Monitor.PollIntervalMs)
	}
	if len(cfg.Session.CommandProfiles) != 1 {
		t.Fatalf("expected default command profile, got %v", cfg.Session.CommandProfiles)
	}
}

func TestLoadFrom_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[monitor]
poll_interval_ms = 5000
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Monitor.PollIntervalMs != 5000 {
		t.Fatalf("expected 5000, got %d", cfg.Monitor.PollIntervalMs)
	}
	if len(cfg.Session.CommandProfiles) != 1 ||
		cfg.Session.CommandProfiles[0].ID != "shell" {
		t.Fatalf("expected default shell profile, got %v", cfg.Session.CommandProfiles)
	}
}

func TestLoadFrom_DriversSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[drivers.claude]
show_thinking = true
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	claude, ok := cfg.Drivers["claude"]
	if !ok {
		t.Fatal("expected drivers.claude section")
	}
	if claude["show_thinking"] != true {
		t.Errorf("show_thinking = %v, want true", claude["show_thinking"])
	}
}

func TestLoadFrom_FeaturesEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[features.enabled]
example-feature = true
another = false
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Features.Enabled["example-feature"] != true {
		t.Errorf("features.enabled[example-feature] = %v, want true", cfg.Features.Enabled["example-feature"])
	}
	if cfg.Features.Enabled["another"] != false {
		t.Errorf("features.enabled[another] = %v, want false", cfg.Features.Enabled["another"])
	}
}

func TestLoadFrom_FeaturesEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[log]
level = "info"
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Features.Enabled) != 0 {
		t.Errorf("expected empty Features.Enabled, got %v", cfg.Features.Enabled)
	}
}

func TestLoadFrom_DriversEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[log]
level = "info"
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Drivers) != 0 {
		t.Errorf("expected empty Drivers, got %v", cfg.Drivers)
	}
}
