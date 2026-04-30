package runtime

import (
	"context"
	"errors"
	"testing"

	credproxy "github.com/takezoh/agent-roost/auth/credproxy"
	"github.com/takezoh/agent-roost/config"
	credproxylib "github.com/takezoh/credproxy/pkg/credproxy"
)

// stubProvider is a test-only Provider that returns fixed Spec values.
type stubProvider struct {
	name string
	spec credproxy.Spec
	err  error
}

func (s *stubProvider) Name() string                 { return s.name }
func (s *stubProvider) Init() error                  { return nil }
func (s *stubProvider) Routes() []credproxylib.Route { return nil }
func (s *stubProvider) ContainerSpec(_ context.Context, _ string, _ config.SandboxConfig) (credproxy.Spec, error) {
	return s.spec, s.err
}

func TestCredProxyRunner_ContainerSpec_MergesProviders(t *testing.T) {
	r := &CredProxyRunner{
		providers: []credproxy.Provider{
			&stubProvider{name: "p1", spec: credproxy.Spec{
				Env:    map[string]string{"KEY_A": "val_a"},
				Mounts: []string{"/host/a:/container/a"},
			}},
			&stubProvider{name: "p2", spec: credproxy.Spec{
				Env:    map[string]string{"KEY_B": "val_b"},
				Mounts: []string{"/host/b:/container/b"},
			}},
		},
	}

	out, err := r.ContainerSpec(context.Background(), "/project", config.SandboxConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Env["KEY_A"] != "val_a" {
		t.Errorf("KEY_A = %q, want val_a", out.Env["KEY_A"])
	}
	if out.Env["KEY_B"] != "val_b" {
		t.Errorf("KEY_B = %q, want val_b", out.Env["KEY_B"])
	}
	if len(out.Mounts) != 2 {
		t.Errorf("Mounts len = %d, want 2: %v", len(out.Mounts), out.Mounts)
	}
}

func TestCredProxyRunner_ContainerSpec_SkipsFailingProvider(t *testing.T) {
	r := &CredProxyRunner{
		providers: []credproxy.Provider{
			&stubProvider{name: "good", spec: credproxy.Spec{
				Env: map[string]string{"KEY_OK": "ok"},
			}},
			&stubProvider{name: "bad", err: errors.New("provider down")},
		},
	}

	out, err := r.ContainerSpec(context.Background(), "/project", config.SandboxConfig{})
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if out.Env["KEY_OK"] != "ok" {
		t.Errorf("KEY_OK = %q, want ok", out.Env["KEY_OK"])
	}
}

func TestCredProxyRunner_ContainerSpec_EmptyProviders(t *testing.T) {
	r := &CredProxyRunner{}
	out, err := r.ContainerSpec(context.Background(), "/project", config.SandboxConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Env) != 0 {
		t.Errorf("Env = %v, want empty", out.Env)
	}
}
