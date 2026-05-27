package secretenv

import (
	"testing"
)

func TestGate_allow(t *testing.T) {
	g := NewGate([]string{"/home/user/project/*.env", "/etc/secrets/*"})

	if err := g.Check("/home/user/project/dev.env"); err != nil {
		t.Errorf("expected allow, got %v", err)
	}
	if err := g.Check("/etc/secrets/prod"); err != nil {
		t.Errorf("expected allow, got %v", err)
	}
}

func TestGate_deny(t *testing.T) {
	g := NewGate([]string{"/home/user/project/*.env"})

	if err := g.Check("/etc/passwd"); err == nil {
		t.Error("expected deny, got nil")
	}
	if err := g.Check("/home/user/other/dev.env"); err == nil {
		t.Error("expected deny, got nil")
	}
}

func TestGate_emptyAllowlist(t *testing.T) {
	g := NewGate(nil)
	if err := g.Check("/any/path.env"); err == nil {
		t.Error("expected deny on empty allowlist")
	}
}
