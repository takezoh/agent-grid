//go:build e2e

// The external test package is required rather than stylistic: internal/app
// imports internal/attach, so an in-package test that reads the detach-byte
// constant from its single owner would close an import cycle.
package attach_test

import (
	"bytes"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/takezoh/agent-grid/clients/cli/internal/app"
)

// detach_key_probe_e2e_test.go answers evidence-detach-key-chord-collision by
// measurement: does a real agent bind the CLI's default detach byte?
//
// The CLI consumes the detach byte locally and never forwards it, so a binding
// on the agent's side would be unreachable for anyone attached through the CLI
// — the user could not press it at all. That is a product problem, and it is
// not answerable from the CLI's own source.

// ptyRecorder accumulates everything an agent writes to its pty. Reads run on
// their own goroutine because a pty master does not honour read deadlines here:
// an earlier version used SetReadDeadline and blocked until the test timeout.
type ptyRecorder struct {
	mu   sync.Mutex
	seen bytes.Buffer
}

func (r *ptyRecorder) run(file *os.File) {
	buffer := make([]byte, 8192)
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			r.mu.Lock()
			r.seen.Write(buffer[:n])
			r.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (r *ptyRecorder) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.seen.Len()
}

func (r *ptyRecorder) tail(n int) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	raw := r.seen.Bytes()
	if len(raw) > n {
		raw = raw[len(raw)-n:]
	}
	return string(raw)
}

// keyReaction is how many bytes an agent emitted in response to one key.
type keyReaction struct {
	control int // a key the agent's UI certainly handles
	subject int // the detach byte
	tail    string
}

// probeAgentKeyReaction sends a control key and then the subject key, measuring
// what each provokes.
//
// The control is what makes the result discriminating. Without it, "the agent
// emitted nothing" is equally explained by "the agent has no binding" and by
// "the probe cannot see reactions at all" — and only one of those is an answer.
func probeAgentKeyReaction(t *testing.T, bin string, control, subject []byte) keyReaction {
	t.Helper()
	command := exec.Command(bin)
	command.Env = append(os.Environ(), "TERM=xterm-256color")
	file, err := pty.Start(command)
	if err != nil {
		t.Fatalf("start %s in a pty: %v", bin, err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
		_ = file.Close()
	})
	_ = pty.Setsize(file, &pty.Winsize{Cols: 100, Rows: 30})

	recorder := &ptyRecorder{}
	go recorder.run(file)

	time.Sleep(5 * time.Second) // start-up and first paint

	send := func(key []byte) int {
		before := recorder.len()
		if _, err := file.Write(key); err != nil {
			t.Fatalf("write %q: %v", key, err)
		}
		time.Sleep(2 * time.Second)
		return recorder.len() - before
	}

	return keyReaction{
		control: send(control),
		subject: send(subject),
		tail:    recorder.tail(200),
	}
}

// TestE2E_DetachByteIsNotBoundByRealAgents requires the default detach byte to
// provoke no reaction from an agent that demonstrably reacts to other keys.
//
// A failure here is a product finding rather than a broken test: it would mean
// the default in adr-20260814-cli-raw-mode-critical-section shadows a chord the
// agent owns, and the default — not this assertion — is what should change.
func TestE2E_DetachByteIsNotBoundByRealAgents(t *testing.T) {
	// Down-arrow: every terminal UI these agents ship drives a selection or a
	// history with it, so it is a reaction the probe should always see.
	control := []byte{0x1b, '[', 'B'}

	for _, agent := range []struct{ name, env string }{
		{"claude", "AG_E2E_CLAUDE_BIN"},
		{"codex", "AG_E2E_CODEX_BIN"},
	} {
		t.Run(agent.name, func(t *testing.T) {
			bin := os.Getenv(agent.env)
			if bin == "" {
				t.Skipf("%s is unset; the detach-key collision probe needs a real agent", agent.env)
			}
			got := probeAgentKeyReaction(t, bin, control, []byte{app.DefaultDetachByte})
			t.Logf("%s: control=%d bytes, detach byte=%d bytes\ntail: %q",
				agent.name, got.control, got.subject, got.tail)

			if got.control == 0 {
				t.Skipf("%s produced no reaction to the control key either; "+
					"the probe cannot see this agent's redraws, so it cannot answer", agent.name)
			}
			if got.subject != 0 {
				t.Errorf("%s reacted to the detach byte 0x%02x with %d bytes; "+
					"the default shadows a chord this agent owns and should be reconsidered",
					agent.name, app.DefaultDetachByte, got.subject)
			}
		})
	}
}
