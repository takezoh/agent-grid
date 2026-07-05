package driver

import "testing"

func TestHasFlagToken(t *testing.T) {
	cases := []struct {
		command string
		flag    string
		want    bool
	}{
		{"claude --foo", "--foo", true},
		{"claude --foo=bar", "--foo", true},
		{"claude --foobar", "--foo", false},
		{"claude", "--foo", false},
		{"", "--foo", false},
	}
	for _, tc := range cases {
		got := hasFlagToken(tc.command, tc.flag)
		if got != tc.want {
			t.Errorf("hasFlagToken(%q, %q) = %v, want %v", tc.command, tc.flag, got, tc.want)
		}
	}
}

func TestStripFlagToken(t *testing.T) {
	cases := []struct {
		command string
		flag    string
		want    string
	}{
		// basic removal
		{"claude --enable-auto-mode", "--enable-auto-mode", "claude"},
		// multiple occurrences removed
		{"claude --enable-auto-mode --foo --enable-auto-mode", "--enable-auto-mode", "claude --foo"},
		// = form is NOT removed
		{"claude --enable-auto-mode=true", "--enable-auto-mode", "claude --enable-auto-mode=true"},
		// flag not present: no-op
		{"claude --foo", "--enable-auto-mode", "claude --foo"},
		// empty command
		{"", "--enable-auto-mode", ""},
		// flag in the middle
		{"claude --enable-auto-mode --worktree", "--enable-auto-mode", "claude --worktree"},
	}
	for _, tc := range cases {
		got := stripFlagToken(tc.command, tc.flag)
		if got != tc.want {
			t.Errorf("stripFlagToken(%q, %q) = %q, want %q", tc.command, tc.flag, got, tc.want)
		}
	}
}

func TestReplaceFlagValue(t *testing.T) {
	cases := []struct {
		name    string
		command string
		flags   []string
		value   string
		want    string
	}{
		{
			name:    "replaces equals form and preserves unrelated flags",
			command: "claude --worktree feature --model=sonnet",
			flags:   []string{"--model"},
			value:   "opus",
			want:    "claude --worktree feature --model opus",
		},
		{
			name:    "replaces spaced form and preserves sandbox flags",
			command: "claude --model sonnet --allow-dangerously-skip-permissions",
			flags:   []string{"--model"},
			value:   "opus",
			want:    "claude --model opus --allow-dangerously-skip-permissions",
		},
		{
			name:    "replaces codex short flag alias",
			command: "codex -m gpt-4.1 --effort high",
			flags:   []string{"--model", "-m"},
			value:   "gpt-5-codex",
			want:    "codex --model gpt-5-codex --effort high",
		},
		{
			name:    "preserves quoted unrelated args",
			command: `claude -c 'foo = "bar baz"' --model=sonnet`,
			flags:   []string{"--model"},
			value:   "opus",
			want:    `claude -c 'foo = "bar baz"' --model opus`,
		},
		{
			name:    "preserves original shell quoting for unrelated apostrophe literal",
			command: `claude -c 'it'\''s $HOME $(printf boom)' --model=sonnet`,
			flags:   []string{"--model"},
			value:   "opus",
			want:    `claude -c 'it'\''s $HOME $(printf boom)' --model opus`,
		},
	}
	for _, tc := range cases {
		got := replaceFlagValue(tc.command, tc.flags, tc.value)
		if got != tc.want {
			t.Errorf("replaceFlagValue(%q, %v, %q) = %q, want %q", tc.command, tc.flags, tc.value, got, tc.want)
		}
	}
}

func TestStripFlagValues(t *testing.T) {
	cases := []struct {
		command string
		flags   []string
		want    string
	}{
		{
			command: `claude -c "$HOME/conf" --model=sonnet --effort high`,
			flags:   []string{"--model"},
			want:    `claude -c "$HOME/conf" --effort high`,
		},
		{
			command: `codex -m gpt-5-codex -c 'foo = "bar baz"' --effort=high`,
			flags:   []string{"--model", "-m"},
			want:    `codex -c 'foo = "bar baz"' --effort=high`,
		},
	}
	for _, tc := range cases {
		got := stripFlagValues(tc.command, tc.flags)
		if got != tc.want {
			t.Errorf("stripFlagValues(%q, %v) = %q, want %q", tc.command, tc.flags, got, tc.want)
		}
	}
}

func TestParseWorktreeFlagsPreservesQuotedArgs(t *testing.T) {
	req, got := parseWorktreeFlags(`claude -c 'foo = "bar baz"' --worktree feature --model sonnet`, "--worktree")
	if !req.Enabled || req.Name != "feature" {
		t.Fatalf("req = %+v", req)
	}
	want := `claude -c 'foo = "bar baz"' --model sonnet`
	if got != want {
		t.Fatalf("parseWorktreeFlags(...) = %q, want %q", got, want)
	}
}
