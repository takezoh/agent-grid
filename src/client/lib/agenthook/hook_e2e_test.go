//go:build e2e

// Hook payload fidelity backstop. Registers the fakeclaude hook JSON schema
// against the same real `claude` binary that stream-json tests use, then
// asserts every hook payload real claude wrote to the recorder file parses
// back into a HookPayload with the expected key set.
//
// Skipped in normal builds by the `e2e` tag. Skipped at runtime unless
// AG_E2E_CLAUDE_BIN is set.

package agenthook

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/takezoh/agent-grid/platform/e2etest"
	"github.com/takezoh/agent-grid/platform/lib/claude/cli"
	"github.com/takezoh/agent-grid/platform/lib/claude/fakeclaude"
	claudehookpayload "github.com/takezoh/agent-grid/platform/lib/claude/hookpayload"
)

// requireHome forces $HOME to a fresh TempDir so Install never
// clobbers the developer's real ~/.claude.
func requireHome(t *testing.T) string {
	t.Helper()
	home := e2etest.NewIsolatedHome(t, ".claude-e2e-home-")
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

// runClaudeShortPTY runs real claude under a PTY so hook delivery is exercised
// through the same terminal-facing path as an interactive session, even though
// we use `-p` for deterministic non-blocking completion.
func runClaudeShort(t *testing.T, bin, prompt string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, cli.AppServerArgs("", "", prompt)...)
	cmd.Env = os.Environ()
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start(claude): %v", err)
	}
	defer ptmx.Close() //nolint:errcheck
	if _, err := io.Copy(io.Discard, ptmx); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) {
		t.Fatalf("read claude pty: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Fatalf("claude exited non-zero: %v\nstderr: %s", err, stderr.String())
		}
		t.Fatalf("claude: %v\nstderr: %s", err, stderr.String())
	}
}

// readHookPayloads parses recordFile as one raw JSON object per line.
func readHookPayloads(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open record: %v", err)
	}
	defer f.Close() //nolint:errcheck

	var out []map[string]any
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			t.Errorf("hook payload not decodable JSON object: %v\nline: %s", err, line)
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
	seen := map[string]map[string]any{}
	for _, p := range payloads {
		name, _ := p["hook_event_name"].(string)
		if name == "" {
			t.Errorf("hook payload has no hook_event_name: %+v", p)
			continue
		}
		if _, ok := seen[name]; !ok {
			seen[name] = p
		}
	}

	// The three lifecycle events we make load-bearing assumptions about.
	for _, want := range []string{"SessionStart", "UserPromptSubmit", "Stop"} {
		p, ok := seen[want]
		if !ok {
			t.Errorf("expected %s to fire at least once (got events: %v)", want, keysOf(seen))
			continue
		}
		if sessionID, _ := p["session_id"].(string); sessionID == "" {
			t.Errorf("%s payload has empty session_id; driver's session-card title fallback depends on it", want)
		}
		if want != "SessionStart" {
			if transcriptPath, _ := p["transcript_path"].(string); transcriptPath == "" {
				// SessionStart may fire before the transcript file exists.
				t.Errorf("%s payload has empty transcript_path; driver's transcript watcher depends on it", want)
			}
		}
	}
}

// TestE2E_HookPayloadKeySubset — every JSON key present in real claude
// payloads must remain raw-decodable JSON objects, and the fields the driver
// reads must be present with the expected JSON shapes.
func TestE2E_HookPayloadRawShape(t *testing.T) {
	bin := fakeclaude.E2EClaudeBin(t)
	recordFile := setupClaudeHooks(t)
	runClaudeShort(t, bin, "Reply with 'hi', then stop.")

	for _, raw := range readHookPayloads(t, recordFile) {
		payloadJSON, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("marshal payload map: %v", err)
		}
		var decoded claudehookpayload.HookPayload
		if err := json.Unmarshal(payloadJSON, &decoded); err != nil {
			t.Fatalf("hook payload does not decode through shared Claude schema: %v\npayload=%+v", err, raw)
		}
		if _, ok := raw["hook_event_name"].(string); !ok {
			t.Fatalf("hook payload missing string hook_event_name: %+v", raw)
		}
		if model, ok := raw["model"]; ok {
			if _, ok := model.(string); !ok {
				t.Fatalf("hook payload model has unexpected shape: %+v", raw)
			}
			if decoded.Model == "" {
				t.Fatalf("hook payload model was silently dropped by shared Claude schema: %+v", raw)
			}
		}
		if effort, ok := raw["effort"]; ok {
			switch effort.(type) {
			case string, map[string]any:
			default:
				t.Fatalf("hook payload effort has unexpected shape: %+v", raw)
			}
			if decoded.Effort == nil || decoded.Effort.Value() == "" {
				t.Fatalf("hook payload effort was silently dropped by shared Claude schema: %+v", raw)
			}
		}
	}
}

func keysOf(m map[string]map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
