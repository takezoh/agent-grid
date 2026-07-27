package agentlaunch

import (
	"os/exec"
	"strings"
	"testing"
)

func TestJoinArgs(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{"empty", nil, ""},
		{"safe args stay plain", []string{"claude", "--model", "sonnet"}, "claude --model sonnet"},
		{"spaces are quoted", []string{"codex", "-c", `foo = "bar baz"`}, `codex -c 'foo = "bar baz"'`},
		{"single quote is escaped", []string{"echo", "it's"}, `echo 'it'\''s'`},
		{"empty arg", []string{"echo", ""}, "echo ''"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinArgs(tt.argv)
			if got != tt.want {
				t.Fatalf("JoinArgs(%v) = %q, want %q", tt.argv, got, tt.want)
			}
			roundTrip, err := SplitArgs(got)
			if err != nil {
				t.Fatalf("SplitArgs(JoinArgs(%v)) error = %v", tt.argv, err)
			}
			if len(roundTrip) != len(tt.argv) {
				t.Fatalf("roundTrip len = %d, want %d (%v)", len(roundTrip), len(tt.argv), roundTrip)
			}
			for i := range tt.argv {
				if roundTrip[i] != tt.argv[i] {
					t.Fatalf("roundTrip[%d] = %q, want %q", i, roundTrip[i], tt.argv[i])
				}
			}
		})
	}
}

func TestJoinArgsPreservesLiteralShellMeaningForApostrophes(t *testing.T) {
	argv := []string{"printf", "%s", "it's $HOME $(printf boom)"}
	cmd := exec.Command("sh", "-c", JoinArgs(argv))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("sh -c JoinArgs(...) failed: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != argv[2] {
		t.Fatalf("shell output = %q, want %q", got, argv[2])
	}
}
