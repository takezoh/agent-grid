package fakeclaude

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/takezoh/agent-reactor/platform/e2etest"
	"github.com/takezoh/agent-reactor/platform/lib/claude/streamjson"
)

func TestRecordedMinimalFixtureMatchesBuilderContracts(t *testing.T) {
	recorded := loadClaudeContracts(t, filepath.Join("testdata", "recordings", "minimal.jsonl"))
	fake := contractsFromLines(t, []string{
		SystemInit("fixture-session"),
		AssistantText("pong"),
		ResultOK("pong", streamjson.Usage{InputTokens: 1, OutputTokens: 1}),
	})
	if !reflect.DeepEqual(recorded, fake) {
		t.Fatalf("minimal recording contract mismatch:\nrecorded=%#v\nfake=%#v", recorded, fake)
	}
}

func TestRecordedToolUseFixtureMatchesBuilderContracts(t *testing.T) {
	recorded := loadClaudeContracts(t, filepath.Join("testdata", "recordings", "tool-use.jsonl"))
	fake := contractsFromLines(t, []string{
		ToolUse("tool-1", "Bash", map[string]any{"command": "echo tool-ok"}),
		ToolResult("tool-1", "tool-ok", false),
	})
	if !reflect.DeepEqual(recorded, fake) {
		t.Fatalf("tool-use recording contract mismatch:\nrecorded=%#v\nfake=%#v", recorded, fake)
	}
}

func contractsFromLines(t *testing.T, lines []string) []any {
	t.Helper()
	out := make([]any, 0, len(lines))
	for _, line := range lines {
		norm, err := e2etest.NormalizeJSON([]byte(line))
		if err != nil {
			t.Fatalf("NormalizeJSON: %v", err)
		}
		if norm["type"] == "result" {
			if usage, ok := norm["usage"].(map[string]any); ok {
				if _, ok := usage["input_tokens"]; ok {
					usage["input_tokens"] = "<count>"
				}
				if _, ok := usage["output_tokens"]; ok {
					usage["output_tokens"] = "<count>"
				}
			}
		}
		out = append(out, e2etest.Contract(norm))
	}
	return out
}

func loadClaudeContracts(t *testing.T, path string) []any {
	t.Helper()
	items := e2etest.ReadJSONLFixture(t, path)
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, e2etest.Contract(item))
	}
	return out
}
