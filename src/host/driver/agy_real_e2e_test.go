//go:build e2e

package driver

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/driver/drivertest"
	"github.com/takezoh/agent-grid/host/state"
)

// TestFakeVsRealAgyPassiveLifecycle compares the synthetic hook sequence used
// by unit tests with a real agy --print run. Set AG_E2E_AGY_BIN to opt in; the
// invoking user's existing agy authentication is used, while all hook files
// and the workspace remain isolated under t.TempDir.
func TestFakeVsRealAgyPassiveLifecycle(t *testing.T) {
	bin := os.Getenv("AG_E2E_AGY_BIN")
	sourceHome := os.Getenv("AG_E2E_AGY_HOME")
	if bin == "" || sourceHome == "" {
		t.Skip("AG_E2E_AGY_BIN or AG_E2E_AGY_HOME is unset")
	}
	assertAgySequenceWaiting(t, drivertest.FakeAgyLifecycle(time.Unix(1, 0)))
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(workspace, "hooks.jsonl")
	recorder := filepath.Join(workspace, "record-hook.sh")
	script := `#!/bin/sh
payload=$(cat)
printf '%s\t%s\n' "$1" "$payload" >> "$AGY_E2E_LOG"
if [ "$1" = Stop ]; then printf '%s' '{"decision":"stop"}'; else printf '%s' '{}'; fi
`
	if err := os.WriteFile(recorder, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(workspace, "home")
	copyAgyCredentialFixture(t, sourceHome, home)
	hooksDir := filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "agent-grid-fidelity")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hooks := map[string]any{"agent-grid-fidelity": map[string]any{
		"PreInvocation": []any{map[string]any{"type": "command", "command": recorder + " PreInvocation"}},
		"Stop":          []any{map[string]any{"type": "command", "command": recorder + " Stop"}},
	}}
	raw, _ := json.Marshal(hooks)
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "plugin.json"), []byte(`{"name":"agent-grid-fidelity"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "--print-timeout=90s", "--print", "Reply exactly OK.")
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), "HOME="+home, "AGY_E2E_LOG="+logPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("real agy failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("real agy did not invoke workspace hooks: %v\n%s", err, out)
	}

	events := readAgyHookCapture(t, logPath)
	real := make([]state.DEvHook, 0, len(events))
	for i, event := range events {
		real = append(real, state.DEvHook{
			Event: event.name, Payload: event.payload,
			Timestamp: time.Unix(int64(i+2), 0),
		})
	}
	assertAgySequenceWaiting(t, real)
	if !events[len(events)-1].fullyIdle {
		t.Fatal("real final Stop did not carry fullyIdle=true")
	}
}

func copyAgyCredentialFixture(t *testing.T, sourceHome, targetHome string) {
	t.Helper()
	files := []string{
		".gemini/antigravity-cli/antigravity-oauth-token",
		".gemini/antigravity-cli/settings.json",
		".gemini/config/config.json",
	}
	for _, rel := range files {
		source := filepath.Join(sourceHome, rel)
		raw, err := os.ReadFile(source)
		if err != nil {
			if os.IsNotExist(err) && !strings.Contains(rel, "oauth-token") {
				continue
			}
			t.Fatalf("read agy fidelity fixture %s: %v", rel, err)
		}
		target := filepath.Join(targetHome, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func assertAgySequenceWaiting(t *testing.T, events []state.DEvHook) {
	t.Helper()
	d := NewAgyDriver()
	s := d.NewState(time.Unix(1, 0))
	ctx := state.FrameContext{IsRoot: true}
	for _, event := range events {
		s, _, _ = d.Step(s, ctx, event)
	}
	if got := d.Status(s); got != state.StatusWaiting {
		t.Fatalf("hook sequence final status = %v, want Waiting", got)
	}
}

type capturedAgyHook struct {
	name      string
	payload   json.RawMessage
	fullyIdle bool
}

func readAgyHookCapture(t *testing.T, path string) []capturedAgyHook {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []capturedAgyHook
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		name, payload, ok := strings.Cut(scanner.Text(), "\t")
		if !ok {
			continue
		}
		event := capturedAgyHook{name: name, payload: json.RawMessage(payload)}
		if name == "Stop" {
			var stop struct {
				FullyIdle bool `json:"fullyIdle"`
			}
			if json.Unmarshal(event.payload, &stop) == nil {
				event.fullyIdle = stop.FullyIdle
			}
		}
		out = append(out, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) < 2 || out[0].name != "PreInvocation" || out[len(out)-1].name != "Stop" {
		t.Fatalf("real hook sequence = %#v, want PreInvocation ... Stop", out)
	}
	return out
}
