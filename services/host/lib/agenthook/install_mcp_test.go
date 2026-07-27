package agenthook

import "testing"

func TestMcpServerEqual(t *testing.T) {
	want := map[string]any{
		"type":    "stdio",
		"command": "agent-grid",
		"args":    []string{"mcp", "serve"},
	}
	cases := []struct {
		name     string
		existing any
		want     bool
	}{
		{"not-a-map", "not-a-map", false},
		{"type-mismatch", map[string]any{"type": "sse", "command": "agent-grid", "args": []any{"mcp", "serve"}}, false},
		{"command-mismatch", map[string]any{"type": "stdio", "command": "other", "args": []any{"mcp", "serve"}}, false},
		{"args-length-mismatch", map[string]any{"type": "stdio", "command": "agent-grid", "args": []any{"mcp"}}, false},
		{"args-element-mismatch", map[string]any{"type": "stdio", "command": "agent-grid", "args": []any{"mcp", "other"}}, false},
		{"equal", map[string]any{"type": "stdio", "command": "agent-grid", "args": []any{"mcp", "serve"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mcpServerEqual(tc.existing, want); got != tc.want {
				t.Errorf("mcpServerEqual(%v, %v) = %v, want %v", tc.existing, want, got, tc.want)
			}
		})
	}
}
