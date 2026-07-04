//go:build e2e

// Opt-in fidelity backstop for the fakeclaude Launcher and Line constants.
//
// This file runs the same stream-json parser production uses against a REAL
// `claude` binary, then compares the shape of the events the real binary emits
// with the shape the fakeclaude helpers produce. When it fails, the fake is
// the thing to update — not the assertion.
//
// Skipped in normal builds by the `e2e` tag. Skipped at runtime unless
// REACTOR_E2E_CLAUDE_BIN points at an executable. Successful runs require a
// live Anthropic API key (ANTHROPIC_API_KEY or claude's existing login).

package fakeclaude

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/lib/claude/cli"
	"github.com/takezoh/agent-reactor/platform/lib/claude/streamjson"
)

// runClaudeOnce runs `claude` with the given prompt and returns every parsed
// stream-json event on stdout, plus stderr for diagnostics.
func runClaudeOnce(t *testing.T, bin, resumeSessionID, prompt string) (events []streamjson.Event, stderr string, err error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := cli.AppServerArgs(resumeSessionID, "", prompt)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", err
	}
	stderrBuf := &strings.Builder{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, "", err
	}

	scanner := streamjson.NewScanner(stdout)
	for scanner.Scan() {
		events = append(events, scanner.Event())
	}
	waitErr := cmd.Wait()
	if scanErr := scanner.Err(); scanErr != nil {
		return events, stderrBuf.String(), scanErr
	}
	return events, stderrBuf.String(), waitErr
}

// TestE2E_ArgvContract is the smallest guard: does the argv produced by
// cli.AppServerArgs get accepted by the real claude binary? If claude added a
// new required flag or deprecated stream-json, this fails first.
func TestE2E_ArgvContract(t *testing.T) {
	bin := E2EClaudeBin(t)
	events, stderr, err := runClaudeOnce(t, bin, "", "Reply with a single word: pong")
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Fatalf("claude exited non-zero: %v\nstderr: %s", err, stderr)
		}
		t.Fatalf("run claude: %v\nstderr: %s", err, stderr)
	}
	if len(events) == 0 {
		t.Fatalf("no stream-json events — argv or output-format likely rejected. stderr: %s", stderr)
	}
}

// TestE2E_StreamJSONLexicon asserts the two events the shim depends on
// (SystemInit with a non-empty session id, and Result) both appear.
func TestE2E_StreamJSONLexicon(t *testing.T) {
	bin := E2EClaudeBin(t)
	events, stderr, err := runClaudeOnce(t, bin, "", "Say 'hi'.")
	if err != nil {
		t.Fatalf("run claude: %v\nstderr: %s", err, stderr)
	}

	var (
		gotInit   streamjson.SystemInit
		gotInitOk bool
		gotResult streamjson.Result
		gotResOk  bool
	)
	for _, ev := range events {
		switch v := ev.(type) {
		case streamjson.SystemInit:
			gotInit, gotInitOk = v, true
		case streamjson.Result:
			gotResult, gotResOk = v, true
		}
	}
	if !gotInitOk {
		t.Fatalf("no SystemInit event in real claude output; got %d events", len(events))
	}
	if gotInit.SessionID == "" {
		t.Errorf("real SystemInit.SessionID is empty — shim's resume propagation will break")
	}
	if !gotResOk {
		t.Fatalf("no Result event — shim treats missing result as failure")
	}
	if gotResult.Subtype == "" {
		t.Errorf("real Result.Subtype is empty; expected 'success' or 'error_*'")
	}
	if gotResult.Usage.InputTokens == 0 && gotResult.Usage.OutputTokens == 0 {
		t.Errorf("real Result.Usage is entirely zero; token accounting is broken")
	}
}

// TestE2E_SessionResume verifies that a session id obtained from turn 1 is
// accepted by --resume for turn 2. This is the shim's continuation contract.
func TestE2E_SessionResume(t *testing.T) {
	bin := E2EClaudeBin(t)
	events1, stderr1, err := runClaudeOnce(t, bin, "", "Say 'first'.")
	if err != nil {
		t.Fatalf("turn1: %v\nstderr: %s", err, stderr1)
	}
	var sid string
	for _, ev := range events1 {
		if init, ok := ev.(streamjson.SystemInit); ok {
			sid = init.SessionID
			break
		}
	}
	if sid == "" {
		t.Fatalf("turn1 has no session id — cannot exercise resume")
	}

	events2, stderr2, err := runClaudeOnce(t, bin, sid, "Say 'second'.")
	if err != nil {
		t.Fatalf("turn2 resume: %v\nstderr: %s", err, stderr2)
	}
	var resumed bool
	for _, ev := range events2 {
		if _, ok := ev.(streamjson.Result); ok {
			resumed = true
			break
		}
	}
	if !resumed {
		t.Fatalf("turn2 with --resume produced no Result event")
	}
}

// TestE2E_FakeVsRealShape is the fake-fidelity assertion. For each event the
// fake produces, at least one event of the same Go type must exist in the real
// output. If real claude changes the type breakdown, the fake needs updating.
func TestE2E_FakeVsRealShape(t *testing.T) {
	bin := E2EClaudeBin(t)
	events, stderr, err := runClaudeOnce(t, bin, "", "Reply with a single word: pong")
	if err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr)
	}
	realTypes := map[string]bool{}
	for _, ev := range events {
		realTypes[reflect.TypeOf(ev).String()] = true
	}

	// The fake's canonical minimal sequence.
	fakeLines := []string{
		LineSystemInit,
		LineResultOK,
	}
	for _, line := range fakeLines {
		ev, err := streamjson.Parse([]byte(line))
		if err != nil {
			t.Fatalf("fake line unparseable — that is the fake's job to keep valid: %v", err)
		}
		typ := reflect.TypeOf(ev).String()
		if !realTypes[typ] {
			t.Errorf("fake emits %s but real claude never did; real types = %v",
				typ, keys(realTypes))
		}
	}
}

// TestE2E_ToolUseLexicon exercises the tool_use / tool_result pair only when
// the prompt actually triggers a tool — otherwise skipped. Used to keep the
// fake's LineToolUse / LineToolResult honest.
func TestE2E_ToolUseLexicon(t *testing.T) {
	bin := E2EClaudeBin(t)
	// Explicitly ask claude to run a shell command so tool_use shows up.
	events, stderr, err := runClaudeOnce(t, bin, "",
		"Run the shell command 'echo tool-ok' via the Bash tool and stop.")
	if err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr)
	}
	var haveToolUse, haveToolResult bool
	for _, ev := range events {
		switch v := ev.(type) {
		case streamjson.AssistantMessage:
			if len(v.ToolUses) > 0 {
				haveToolUse = true
			}
		case streamjson.ToolResult:
			haveToolResult = true
		case streamjson.ToolResults:
			if len(v.Results) > 0 {
				haveToolResult = true
			}
		}
	}
	if !haveToolUse {
		t.Skip("claude did not invoke a tool — cannot validate the tool_use fake path")
	}
	if !haveToolResult {
		t.Errorf("saw tool_use but no tool_result — fake would emit both, real did not")
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
