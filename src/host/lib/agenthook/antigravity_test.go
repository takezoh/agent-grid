package agenthook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallAntigravityPluginsCreatesPassiveLifecycleHooks(t *testing.T) {
	home := t.TempDir()
	changed, err := InstallAntigravityPlugins(home, "/opt/agent-grid/server", "/data")
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 2 {
		t.Fatalf("changed plugins = %v, want desktop and CLI", changed)
	}
	for _, rel := range AntigravityPluginDirs {
		raw, err := os.ReadFile(filepath.Join(home, rel, "hooks.json"))
		if err != nil {
			t.Fatal(err)
		}
		var root map[string]map[string]json.RawMessage
		if err := json.Unmarshal(raw, &root); err != nil {
			t.Fatal(err)
		}
		hooks := root[AntigravityHookNamespace]
		for _, event := range []string{"PreInvocation", "PostInvocation", "PostToolUse", "Stop"} {
			if _, ok := hooks[event]; !ok {
				t.Errorf("%s missing %s", rel, event)
			}
		}
		if _, ok := hooks["PreToolUse"]; ok {
			t.Errorf("%s registers policy-changing PreToolUse", rel)
		}
	}
}

func TestInstallAntigravityPluginsPreservesUnmanagedPlugin(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, AntigravityPluginDirs[0])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(dir, "plugin.json")
	if err := os.WriteFile(plugin, []byte(`{"name":"user-owned"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallAntigravityPlugins(home, "/bin/server", ""); err == nil {
		t.Fatal("expected unmanaged plugin ownership error")
	}
	raw, err := os.ReadFile(plugin)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"name":"user-owned"}` {
		t.Fatalf("unmanaged plugin overwritten: %s", raw)
	}
}

func TestInstallAntigravityPluginsIsIdempotent(t *testing.T) {
	home := t.TempDir()
	if _, err := InstallAntigravityPlugins(home, "/bin/server", "/data"); err != nil {
		t.Fatal(err)
	}
	changed, err := InstallAntigravityPlugins(home, "/bin/server", "/data")
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 0 {
		t.Fatalf("second install changed %v", changed)
	}
}
