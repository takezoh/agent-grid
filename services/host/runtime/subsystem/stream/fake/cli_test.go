package fake

import (
	"strings"
	"testing"
	"time"
)

// TestSpawnCLIEmitsEventsOnPromptSubmit ties the whole fake stack together:
// spawn a FakeAppServer, spawn a FakeCLI in a pty against it, submit a
// prompt via ptmx, then verify the CLI observed the app-server's broadcast
// events. This is the smoke test for the interactive flow the production
// Backend depends on.
func TestSpawnCLIEmitsEventsOnPromptSubmit(t *testing.T) {
	srv := startFake(t, Config{})
	cli := SpawnCLI(t, "--remote", "unix://"+srv.SockPath(), "--cd", "/work")

	// The CLI created its own thread via thread/start on connect; wait for the
	// [READY] line and remember the id to correlate against later events.
	threadID := cli.Ready(t, 3*time.Second)
	if threadID == "" {
		t.Fatal("Ready() returned empty threadId")
	}

	// Submit one prompt; default TurnHandler emits turn/started +
	// thread/status active + turn/completed + thread/status idle.
	cli.SendPrompt(t, "hello world")

	needed := map[string]bool{
		`method=turn/started`:          true,
		`method=turn/completed`:        true,
		`method=thread/status/changed`: true,
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && anyRemaining(needed) {
		line := cli.WaitFor(t, time.Second, func(l string) bool {
			for k := range needed {
				if strings.Contains(l, k) {
					return true
				}
			}
			return false
		}, "one of turn/thread notifications")
		for k := range needed {
			if strings.Contains(line, k) {
				delete(needed, k)
				break
			}
		}
	}
	if len(needed) > 0 {
		t.Fatalf("missing events: %v", keys(needed))
	}

	// The fake also persists the CLI-created thread — assert Threads() sees
	// the same id the CLI reported.
	found := false
	for _, tr := range srv.Threads() {
		if tr.ID == threadID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("app-server threads %+v missing %q", srv.Threads(), threadID)
	}
}

func anyRemaining(m map[string]bool) bool { return len(m) > 0 }
func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
