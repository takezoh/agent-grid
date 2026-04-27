package main

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/config"
	appruntime "github.com/takezoh/agent-roost/runtime"
)

func TestNewAgentLauncher_direct(t *testing.T) {
	for _, mode := range []string{"", "direct"} {
		l, err := newAgentLauncher(context.Background(), config.SandboxConfig{Mode: mode}, t.TempDir())
		if err != nil {
			t.Errorf("mode=%q: unexpected error: %v", mode, err)
			continue
		}
		d, ok := l.(*appruntime.SandboxDispatcher)
		if !ok {
			t.Errorf("mode=%q: expected *SandboxDispatcher, got %T", mode, l)
			continue
		}
		if d.Devcontainer != nil {
			t.Errorf("mode=%q: expected Devcontainer=nil for direct mode, got %T", mode, d.Devcontainer)
		}
	}
}

func TestNewAgentLauncher_devcontainer_missing(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := newAgentLauncher(context.Background(), config.SandboxConfig{Mode: "devcontainer"}, t.TempDir())
	if err == nil {
		t.Error("expected error when devcontainer CLI is not in PATH, got nil")
	}
}

func TestResolveShellDisplayFromValues(t *testing.T) {
	cases := []struct {
		tmuxDefault string
		envSHELL    string
		want        string
	}{
		{"/usr/bin/zsh", "/bin/bash", "zsh"},
		{"", "/bin/bash", "bash"},
		{"", "/usr/bin/zsh", "zsh"},
		{"", "", "shell"},
		{".", "", "shell"},
		{"", ".", "shell"},
		{".", ".", "shell"},
	}
	for _, c := range cases {
		got := resolveShellDisplayFromValues(c.tmuxDefault, c.envSHELL)
		if got != c.want {
			t.Errorf("resolveShellDisplayFromValues(%q, %q) = %q, want %q",
				c.tmuxDefault, c.envSHELL, got, c.want)
		}
	}
}
