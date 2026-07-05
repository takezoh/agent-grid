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
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/e2etest"
	"github.com/takezoh/agent-reactor/platform/lib/claude/cli"
	"github.com/takezoh/agent-reactor/platform/lib/claude/streamjson"
)

// runClaudeOnce runs `claude` with the given prompt and returns every parsed
// stream-json event on stdout, plus stderr for diagnostics.
func runClaudeOnce(t *testing.T, bin, resumeSessionID, prompt string) (events []streamjson.Event, rawLines []string, stderr string, err error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := cli.AppServerArgs(resumeSessionID, "", prompt)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, "", err
	}
	stderrBuf := &strings.Builder{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, nil, "", err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64<<10), 64<<20)
	for scanner.Scan() {
		line := scanner.Text()
		ev, parseErr := streamjson.Parse([]byte(line))
		if parseErr != nil || ev == nil {
			continue
		}
		events = append(events, ev)
		rawLines = append(rawLines, line)
	}
	waitErr := cmd.Wait()
	if scanErr := scanner.Err(); scanErr != nil {
		return events, rawLines, stderrBuf.String(), scanErr
	}
	return events, rawLines, stderrBuf.String(), waitErr
}

// TestE2E_ArgvContract is the smallest guard: does the argv produced by
// cli.AppServerArgs get accepted by the real claude binary? If claude added a
// new required flag or deprecated stream-json, this fails first.
func TestE2E_ArgvContract(t *testing.T) {
	bin := E2EClaudeBin(t)
	events, _, stderr, err := runClaudeOnce(t, bin, "", "Reply with a single word: pong")
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
	events, _, stderr, err := runClaudeOnce(t, bin, "", "Say 'hi'.")
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
	events1, _, stderr1, err := runClaudeOnce(t, bin, "", "Say 'first'.")
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

	events2, _, stderr2, err := runClaudeOnce(t, bin, sid, "Say 'second'.")
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
	events, _, stderr, err := runClaudeOnce(t, bin, "", "Reply with a single word: pong")
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
	events, _, stderr, err := runClaudeOnce(t, bin, "",
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

func TestE2E_RecordedMinimalFixture(t *testing.T) {
	bin := E2EClaudeBin(t)
	_, rawLines, stderr, err := runClaudeOnce(t, bin, "", "Reply with exactly one word: pong")
	if err != nil {
		t.Fatalf("run claude: %v\nstderr: %s", err, stderr)
	}
	var (
		initEntry      any
		assistantEntry any
		resultEntry    any
	)
	for _, line := range rawLines {
		projected, ok := projectedMinimalClaudeEvent(t, []byte(line))
		if !ok {
			continue
		}
		switch projected.(map[string]any)["type"] {
		case "system":
			if initEntry == nil {
				initEntry = projected
			}
		case "assistant":
			if assistantEntry == nil {
				assistantEntry = projected
			}
		case "result":
			if resultEntry == nil {
				resultEntry = projected
			}
		}
	}
	if initEntry == nil || assistantEntry == nil || resultEntry == nil {
		t.Fatalf("missing projected minimal entries: init=%v assistant=%v result=%v", initEntry != nil, assistantEntry != nil, resultEntry != nil)
	}
	e2etest.AssertJSONLFixture(t, filepath.Join("testdata", "recordings", "minimal.jsonl"), []any{initEntry, assistantEntry, resultEntry})
}

func TestE2E_RecordedToolUseFixture(t *testing.T) {
	bin := E2EClaudeBin(t)
	_, rawLines, stderr, err := runClaudeOnce(t, bin, "",
		"Run the shell command 'echo tool-ok' via the Bash tool and stop.")
	if err != nil {
		t.Fatalf("run claude: %v\nstderr: %s", err, stderr)
	}
	var entries []any
	for _, line := range rawLines {
		projected, ok := projectedToolClaudeEvent(t, []byte(line))
		if ok {
			entries = append(entries, projected)
		}
	}
	if len(entries) < 2 {
		t.Skip("claude did not emit both tool_use and tool_result")
	}
	e2etest.AssertJSONLFixture(t, filepath.Join("testdata", "recordings", "tool-use.jsonl"), entries[:2])
}

func projectedMinimalClaudeEvent(t *testing.T, raw []byte) (any, bool) {
	t.Helper()
	norm, err := e2etest.NormalizeJSON(raw)
	if err != nil {
		t.Fatalf("NormalizeJSON: %v", err)
	}
	switch norm["type"] {
	case "system":
		if norm["subtype"] == "init" {
			return map[string]any{
				"type":       "system",
				"subtype":    "init",
				"session_id": norm["session_id"],
			}, true
		}
	case "assistant":
		if text, ok := firstContentBlockField(norm, "text"); ok {
			return map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"content": []any{
						map[string]any{"type": "text", "text": text},
					},
				},
			}, true
		}
	case "result":
		usage := map[string]any{}
		if rawUsage, ok := norm["usage"].(map[string]any); ok {
			if _, ok := rawUsage["input_tokens"]; ok {
				usage["input_tokens"] = "<count>"
			}
			if _, ok := rawUsage["output_tokens"]; ok {
				usage["output_tokens"] = "<count>"
			}
		}
		return map[string]any{
			"type":     "result",
			"subtype":  norm["subtype"],
			"result":   norm["result"],
			"is_error": norm["is_error"],
			"usage":    usage,
		}, true
	}
	return nil, false
}

func projectedToolClaudeEvent(t *testing.T, raw []byte) (any, bool) {
	t.Helper()
	norm, err := e2etest.NormalizeJSON(raw)
	if err != nil {
		t.Fatalf("NormalizeJSON: %v", err)
	}
	if tool, ok := firstContentBlock(norm, "tool_use"); ok {
		input, _ := tool["input"].(map[string]any)
		projected := map[string]any{
			"type": "assistant",
			"message": map[string]any{
				"content": []any{
					map[string]any{
						"type":  "tool_use",
						"id":    tool["id"],
						"name":  tool["name"],
						"input": map[string]any{"command": input["command"]},
					},
				},
			},
		}
		return projected, true
	}
	if tool, ok := firstContentBlock(norm, "tool_result"); ok {
		projected := map[string]any{
			"type": "user",
			"message": map[string]any{
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": tool["tool_use_id"],
						"content":     tool["content"],
						"is_error":    tool["is_error"],
					},
				},
			},
		}
		return projected, true
	}
	return nil, false
}

func firstContentBlock(entry map[string]any, want string) (map[string]any, bool) {
	message, _ := entry["message"].(map[string]any)
	content, _ := message["content"].([]any)
	for _, item := range content {
		block, _ := item.(map[string]any)
		if block["type"] == want {
			return block, true
		}
	}
	return nil, false
}

func firstContentBlockField(entry map[string]any, field string) (any, bool) {
	block, ok := firstContentBlock(entry, "text")
	if !ok {
		return nil, false
	}
	value, ok := block[field]
	return value, ok
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
