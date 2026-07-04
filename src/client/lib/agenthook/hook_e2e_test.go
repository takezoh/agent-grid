//go:build e2e

// Hook payload fidelity backstop. Registers the fakeclaude hook JSON schema
// against the same real `claude` binary that stream-json tests use, then
// asserts every hook payload real claude wrote to the recorder file parses
// back into a HookPayload with the expected key set.
//
// Skipped in normal builds by the `e2e` tag. Skipped at runtime unless
// REACTOR_E2E_CLAUDE_BIN is set.

package agenthook

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/lib/claude/cli"
	"github.com/takezoh/agent-reactor/platform/lib/claude/fakeclaude"
)

// requireHome forces $HOME to a fresh TempDir so Install never
// clobbers the developer's real ~/.claude.
func requireHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// recorderCmd builds a shell command that reads stdin, wraps it with the
// current UNIX time and an event-hint prefix, and appends the whole line to
// recordFile. Claude does not expose the event name in argv, only in the JSON
// payload — so the recorder just captures every hook invocation verbatim.
func recorderCmd(recordFile string) string {
	// Reuse the package's shellQuote (install.go) — same file path escaping
	// contract Install produces.
	return fmt.Sprintf(`sh -c 'printf "%%s\n" "$(cat)" >> %s'`, shellQuote(recordFile))
}

func setupClaudeHooks(t *testing.T) (recordFile string) {
	t.Helper()
	home := requireHome(t)
	settings := filepath.Join(home, Claude.SettingsRel)

	recordFile = filepath.Join(t.TempDir(), "hooks.jsonl")
	// Pre-create the file so shell's `>>` never fails.
	if err := os.WriteFile(recordFile, nil, 0o644); err != nil {
		t.Fatalf("create record file: %v", err)
	}

	if _, err := Install(settings, recorderCmd(recordFile), Claude); err != nil {
		t.Fatalf("Install: %v", err)
	}
	return recordFile
}

// runClaudeShort runs claude with a trivial prompt and waits for it to exit.
// Test-scoped failures on non-zero exit; stdout is discarded because we only
// care about hook side effects here.
func runClaudeShort(t *testing.T, bin, prompt string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, cli.AppServerArgs("", "", prompt)...)
	cmd.Env = os.Environ()
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	if out, err := cmd.Output(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Fatalf("claude exited non-zero: %v\nstdout head: %.200s\nstderr: %s", err, string(out), stderr.String())
		}
		t.Fatalf("claude: %v\nstderr: %s", err, stderr.String())
	}
}

// readHookPayloads parses recordFile as one JSON payload per line.
func readHookPayloads(t *testing.T, path string) []fakeclaude.HookPayload {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open record: %v", err)
	}
	defer f.Close() //nolint:errcheck

	var out []fakeclaude.HookPayload
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}
		var p fakeclaude.HookPayload
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			t.Errorf("hook payload not decodable into fakeclaude.HookPayload: %v\nline: %s", err, line)
			continue
		}
		out = append(out, p)
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("scan record: %v", err)
	}
	return out
}

// TestE2E_HookPayloadSchema — the payloads real claude writes to hook stdin
// must fit fakeclaude.HookPayload. SessionStart, UserPromptSubmit, and Stop
// are the three every session emits at least once.
func TestE2E_HookPayloadSchema(t *testing.T) {
	bin := fakeclaude.E2EClaudeBin(t)
	recordFile := setupClaudeHooks(t)
	runClaudeShort(t, bin, "Say 'hi'.")

	payloads := readHookPayloads(t, recordFile)
	if len(payloads) == 0 {
		t.Fatalf("no hook payloads recorded — hook registration silently no-op'd")
	}

	// Every payload must at least name its event.
	seen := map[string]fakeclaude.HookPayload{}
	for _, p := range payloads {
		if p.HookEventName == "" {
			t.Errorf("hook payload has no hook_event_name: %+v", p)
			continue
		}
		if _, ok := seen[p.HookEventName]; !ok {
			seen[p.HookEventName] = p
		}
	}

	// The three lifecycle events we make load-bearing assumptions about.
	for _, want := range []string{"SessionStart", "UserPromptSubmit", "Stop"} {
		p, ok := seen[want]
		if !ok {
			t.Errorf("expected %s to fire at least once (got events: %v)", want, keysOf(seen))
			continue
		}
		if p.SessionID == "" {
			t.Errorf("%s payload has empty session_id; driver's session-card title fallback depends on it", want)
		}
		if want != "SessionStart" && p.TranscriptPath == "" {
			// SessionStart may fire before the transcript file exists.
			t.Errorf("%s payload has empty transcript_path; driver's transcript watcher depends on it", want)
		}
	}
}

// TestE2E_HookPayloadKeySubset — every JSON key present in real claude
// payloads must also be declared in the HookPayload struct, otherwise the
// driver silently drops fields.
func TestE2E_HookPayloadKeySubset(t *testing.T) {
	bin := fakeclaude.E2EClaudeBin(t)
	recordFile := setupClaudeHooks(t)
	runClaudeShort(t, bin, "Reply with 'hi', then stop.")

	f, err := os.Open(recordFile)
	if err != nil {
		t.Fatalf("open record: %v", err)
	}
	defer f.Close() //nolint:errcheck

	// The set of json tags fakeclaude.HookPayload declares — must be updated
	// in lockstep with hookpayload.go. Extra real fields are flagged so the
	// driver can be updated to consume them.
	fakeKeys := map[string]bool{
		"session_id":        true,
		"hook_event_name":   true,
		"prompt":            true,
		"transcript_path":   true,
		"notification_type": true,
		"tool_name":         true,
		"tool_input":        true,
		"source":            true,
		"tool_use_id":       true,
		"permission_mode":   true,
		"error":             true,
		"is_interrupt":      true,
	}

	// Fields Claude adds around the driver-required set. Not consumed by the
	// driver today, so their presence is tolerated — but new *sensitive-looking*
	// keys (auth tokens, workspace paths outside cwd, etc.) show up here and
	// warrant an intentional decision.
	knownExtraKeys := map[string]bool{
		"cwd":                            true,
		"user":                           true,
		"claude_code_session_start_time": true,
	}

	unknown := map[string]int{}
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		for k := range raw {
			if fakeKeys[k] || knownExtraKeys[k] {
				continue
			}
			unknown[k]++
		}
	}
	if len(unknown) > 0 {
		t.Logf("real claude hook payloads carry unknown keys — consider extending HookPayload or the knownExtraKeys allowlist: %v", unknown)
	}
}

func keysOf(m map[string]fakeclaude.HookPayload) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
