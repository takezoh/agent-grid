package credproxy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
)

type stubProvider struct {
	name           string
	spec           container.Spec
	err            error
	materializeErr error
}

func (s *stubProvider) Name() string                 { return s.name }
func (s *stubProvider) Init() error                  { return nil }
func (s *stubProvider) Routes() []credproxylib.Route { return nil }
func (s *stubProvider) ContainerSpec(_ context.Context, _ string) (container.Spec, error) {
	return s.spec, s.err
}
func (s *stubProvider) Materialize(_ context.Context, _ string) error { return s.materializeErr }

// closerProvider is a stubProvider that also implements io.Closer, used to
// verify Shutdown invokes the optional Closer hook.
type closerProvider struct {
	stubProvider
	closed *bool
}

func (c *closerProvider) Close() error { *c.closed = true; return nil }

func TestRunnerShutdownCancelsWaitsAndCloses(t *testing.T) {
	done := make(chan struct{})
	cancelled := false
	closed := false
	r := &Runner{
		srvCancel:  func() { cancelled = true; close(done) },
		serverDone: done,
		providers:  []container.Provider{&closerProvider{stubProvider: stubProvider{name: "c"}, closed: &closed}},
	}

	r.Shutdown(context.Background())

	if !cancelled {
		t.Error("srvCancel was not called")
	}
	if !closed {
		t.Error("provider Close was not called")
	}
}

func TestRunnerShutdownNilSafe(t *testing.T) {
	// A Runner whose Start never completed has nil srvCancel/serverDone.
	(&Runner{}).Shutdown(context.Background())
}

func TestRunnerShutdownRespectsCtxWhenServerHangs(t *testing.T) {
	r := &Runner{srvCancel: func() {}, serverDone: make(chan struct{})} // never closes
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { r.Shutdown(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not return when ctx was cancelled and server hung")
	}
}

func TestRunner_ContainerSpec_MergesProviders(t *testing.T) {
	r := &Runner{
		providers: []container.Provider{
			&stubProvider{name: "p1", spec: container.Spec{
				Env:    map[string]string{"KEY_A": "val_a"},
				Mounts: []string{"/host/a:/container/a"},
			}},
			&stubProvider{name: "p2", spec: container.Spec{
				Env:    map[string]string{"KEY_B": "val_b"},
				Mounts: []string{"/host/b:/container/b"},
			}},
		},
	}

	out, err := r.ContainerSpec(context.Background(), "/project")
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

func TestRunner_ContainerSpec_SkipsFailingProvider(t *testing.T) {
	r := &Runner{
		providers: []container.Provider{
			&stubProvider{name: "good", spec: container.Spec{
				Env: map[string]string{"KEY_OK": "ok"},
			}},
			&stubProvider{name: "bad", err: errors.New("provider down")},
		},
	}

	out, err := r.ContainerSpec(context.Background(), "/project")
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if out.Env["KEY_OK"] != "ok" {
		t.Errorf("KEY_OK = %q, want ok", out.Env["KEY_OK"])
	}
}

func TestRunner_ContainerSpec_EmptyProviders(t *testing.T) {
	r := &Runner{}
	out, err := r.ContainerSpec(context.Background(), "/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Env) != 0 {
		t.Errorf("Env = %v, want empty", out.Env)
	}
}

// TestRunner_Materialize_RecordsSuccessAndFailure pins the invariant that
// ReadinessSnapshot is built solely from the Runner's own Materialize
// outcomes — per adr-20260715-credproxy-runner-readonly-aggregation.
func TestRunner_Materialize_RecordsSuccessAndFailure(t *testing.T) {
	failErr := errors.New("boom")
	r := &Runner{
		readiness: make(map[readinessKey]ProjectReadiness),
		providers: []container.Provider{
			&stubProvider{name: "ok-provider"},
			&stubProvider{name: "bad-provider", materializeErr: failErr},
		},
	}

	err := r.Materialize(context.Background(), "/project")
	if err == nil || err.Error() != failErr.Error() {
		t.Fatalf("Materialize err = %v, want %v", err, failErr)
	}

	snap := r.ReadinessSnapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot len = %d, want 2, got %+v", len(snap), snap)
	}
	byProvider := map[string]ProjectReadiness{}
	for _, pr := range snap {
		byProvider[pr.ProviderName] = pr
	}
	if !byProvider["ok-provider"].Materialized {
		t.Error("ok-provider should have Materialized=true")
	}
	if byProvider["bad-provider"].Materialized {
		t.Error("bad-provider should have Materialized=false")
	}
	if byProvider["bad-provider"].LastError != failErr.Error() {
		t.Errorf("bad-provider LastError = %q, want %q", byProvider["bad-provider"].LastError, failErr.Error())
	}
}

// TestRunner_ReadinessSnapshot_DefensiveCopy pins the invariant that mutations
// on the returned slice do not affect subsequent snapshots.
func TestRunner_ReadinessSnapshot_DefensiveCopy(t *testing.T) {
	r := &Runner{
		readiness: make(map[readinessKey]ProjectReadiness),
		providers: []container.Provider{&stubProvider{name: "p"}},
	}
	_ = r.Materialize(context.Background(), "/project")

	snap := r.ReadinessSnapshot()
	if len(snap) != 1 {
		t.Fatalf("first snapshot len = %d, want 1", len(snap))
	}
	snap[0].Materialized = false
	snap[0].LastError = "tampered"

	snap2 := r.ReadinessSnapshot()
	if !snap2[0].Materialized {
		t.Error("caller mutation to snap[0].Materialized leaked back into Runner state")
	}
	if snap2[0].LastError != "" {
		t.Errorf("caller mutation to snap[0].LastError leaked back: %q", snap2[0].LastError)
	}
}

// TestRunner_Materialize_OptOutProviderIsSilentlyHealthy: providers that
// return nil (no host-side credential state, no-op) do NOT appear in the
// snapshot as absent — they DO appear with Materialized=true (their nil
// return is a positive assertion). silence = healthy is expressed by the
// map itself not gaining entries for providers that were never invoked,
// not by opt-out providers being excluded post-hoc.
func TestRunner_Materialize_OptOutProviderIsPositive(t *testing.T) {
	r := &Runner{
		readiness: make(map[readinessKey]ProjectReadiness),
		providers: []container.Provider{&stubProvider{name: "noop"}},
	}
	if err := r.Materialize(context.Background(), "/project"); err != nil {
		t.Fatalf("Materialize err = %v, want nil", err)
	}
	snap := r.ReadinessSnapshot()
	if len(snap) != 1 || !snap[0].Materialized {
		t.Errorf("snapshot = %+v, want [{Materialized:true}]", snap)
	}
}
