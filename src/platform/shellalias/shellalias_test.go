package shellalias

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		want       map[string]string
		wantClosed bool
	}{
		{
			name:       "block with noise outside markers",
			raw:        "rc banner line\n" + aliasBegin + "\nclaude\tclaude --enable-auto-mode\nsonnet-medium\tclaude --model sonnet\n" + aliasEnd + "\ntrailing noise\n",
			want:       map[string]string{"claude": "claude --enable-auto-mode", "sonnet-medium": "claude --model sonnet"},
			wantClosed: true,
		},
		{
			name:       "empty value omitted",
			raw:        aliasBegin + "\nshell\t\ncodex\t\n" + aliasEnd + "\n",
			want:       map[string]string{},
			wantClosed: true,
		},
		{
			name:       "no markers yields empty and unclosed",
			raw:        "claude\tclaude --enable-auto-mode\n",
			want:       map[string]string{},
			wantClosed: false,
		},
		{
			name:       "truncated block is unclosed even with partial entries",
			raw:        aliasBegin + "\nclaude\tclaude --enable-auto-mode\n",
			want:       map[string]string{"claude": "claude --enable-auto-mode"},
			wantClosed: false,
		},
		{
			name:       "value containing spaces and flags",
			raw:        aliasBegin + "\nopus-xhigh\tclaude --model opus --effort xhigh\n" + aliasEnd,
			want:       map[string]string{"opus-xhigh": "claude --model opus --effort xhigh"},
			wantClosed: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, closed := parse(c.raw)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("parse() map = %v, want %v", got, c.want)
			}
			if closed != c.wantClosed {
				t.Errorf("parse() closed = %v, want %v", closed, c.wantClosed)
			}
		})
	}
}

func TestBuildQuery(t *testing.T) {
	if _, ok := buildQuery("/usr/bin/fish", []string{"a"}); ok {
		t.Error("buildQuery should reject unknown shell")
	}
	zsh, ok := buildQuery("/usr/bin/zsh", []string{"sonnet-medium"})
	if !ok || !strings.Contains(zsh, "${aliases[$n]}") || !strings.Contains(zsh, "sonnet-medium") {
		t.Errorf("zsh query missing parts: %q", zsh)
	}
	bash, ok := buildQuery("/bin/bash", []string{"sonnet-medium"})
	if !ok || !strings.Contains(bash, "${BASH_ALIASES[$n]}") {
		t.Errorf("bash query missing parts: %q", bash)
	}
	if !strings.Contains(zsh, aliasBegin) || !strings.Contains(zsh, aliasEnd) {
		t.Errorf("query missing sentinels: %q", zsh)
	}
}

func TestResolve(t *testing.T) {
	var gotShellArgs []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotShellArgs = append([]string{name}, args...)
		out := aliasBegin + "\n" +
			"claude\tclaude --enable-auto-mode\n" +
			"sonnet-medium\tclaude --model sonnet --effort medium\n" +
			"shell\t\n" +
			aliasEnd + "\n"
		return []byte(out), nil
	}
	got, err := Resolve(context.Background(), "/usr/bin/zsh", []string{"claude", "sonnet-medium", "shell"}, run)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := map[string]string{
		"claude":        "claude --enable-auto-mode",
		"sonnet-medium": "claude --model sonnet --effort medium",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve() = %v, want %v", got, want)
	}
	if len(gotShellArgs) < 3 || gotShellArgs[0] != "/usr/bin/zsh" || gotShellArgs[1] != "-i" || gotShellArgs[2] != "-c" {
		t.Errorf("shell invoked with unexpected args: %v", gotShellArgs)
	}
}

// TestResolveTruncated covers the timeout/kill case: the shell emits the begin
// marker and some entries but is killed before the end marker. The partial map
// must be discarded and an error returned (not silently dropped aliases).
func TestResolveTruncated(t *testing.T) {
	run := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		out := aliasBegin + "\nclaude\tclaude --enable-auto-mode\n" // no end marker
		return []byte(out), errors.New("signal: killed")
	}
	got, err := Resolve(context.Background(), "/usr/bin/zsh", []string{"claude", "sonnet-medium"}, run)
	if err == nil {
		t.Error("expected error on truncated output")
	}
	if len(got) != 0 {
		t.Errorf("partial output must be discarded, got %v", got)
	}
}

func TestResolveUnsupportedShell(t *testing.T) {
	run := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		t.Fatalf("shell should not be invoked for unsupported login shell")
		return nil, nil
	}
	got, err := Resolve(context.Background(), "/usr/bin/fish", []string{"claude"}, run)
	if err == nil {
		t.Error("expected error for unsupported shell")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestResolveEmptyNames(t *testing.T) {
	run := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		t.Fatal("runner must not be called for empty names")
		return nil, nil
	}
	got, err := Resolve(context.Background(), "/usr/bin/zsh", nil, run)
	if err != nil || len(got) != 0 {
		t.Errorf("Resolve(nil) = %v, %v", got, err)
	}
}

func TestLoginShell(t *testing.T) {
	run := func(_ context.Context, name string, _ ...string) ([]byte, error) {
		if name != "getent" {
			t.Fatalf("unexpected command %q", name)
		}
		return []byte("take:x:1000:1000:,,,:/home/take:/usr/bin/zsh\n"), nil
	}
	got, err := LoginShell(context.Background(), run)
	if err != nil || got != "/usr/bin/zsh" {
		t.Errorf("LoginShell() = %q, %v", got, err)
	}

	// A duplicate NSS entry yields multi-line output; only the first line counts.
	multiline := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("take:x:1000:1000::/home/take:/usr/bin/zsh\nother:x:1:1::/home/other:/bin/bash\n"), nil
	}
	if got, err := LoginShell(context.Background(), multiline); err != nil || got != "/usr/bin/zsh" {
		t.Errorf("LoginShell(multiline) = %q, %v; want /usr/bin/zsh", got, err)
	}

	bad := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("take:x:1000:1000:,,,:/home/take:\n"), nil
	}
	if _, err := LoginShell(context.Background(), bad); err == nil {
		t.Error("expected error when passwd has no shell field")
	}

	failRun := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, errors.New("boom")
	}
	if _, err := LoginShell(context.Background(), failRun); err == nil {
		t.Error("expected error when getent fails")
	}
}

// TestQueryAgainstRealBash exercises buildQuery + a real interactive bash +
// parse end to end, injecting an alias via --rcfile. zsh may be absent in CI,
// so the cross-shell mechanism is validated against bash.
func TestQueryAgainstRealBash(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not in PATH")
	}
	rc := filepath.Join(t.TempDir(), "rc")
	if err := os.WriteFile(rc, []byte("alias foo='bar baz'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script, ok := buildQuery(bash, []string{"foo", "missing"})
	if !ok {
		t.Fatal("buildQuery returned ok=false for bash")
	}
	out, _ := exec.Command(bash, "--rcfile", rc, "-i", "-c", script).Output()
	got, closed := parse(string(out))
	if !closed {
		t.Errorf("expected closed block from real bash, raw=%q", string(out))
	}
	want := map[string]string{"foo": "bar baz"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parse(real bash) = %v, want %v (raw=%q)", got, want, string(out))
	}
}
