package wslhost

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"
)

// TestContractProbeBeforeSpawnAndTokenRevalidation pins probe-before-spawn and
// stop-token authority at the supervisor seam.
//
// rejects counterexample: speculative spawn on unknown/matched, or stop without token.
func TestContractProbeBeforeSpawnAndTokenRevalidation(t *testing.T) {
	h := newHarness(t)
	h.host.SetProbe(ProbeResult{Class: ProbeClassUnknown, Reason: "indeterminate"})
	h.sup.Start(context.Background())
	if h.wsl.SpawnCount() != 0 {
		t.Fatal("unknown must not spawn")
	}
	if h.host.ProbeCount() == 0 {
		t.Fatal("probe must run before spawn decision")
	}

	h2 := newHarness(t)
	h2.host.SetProbe(ProbeResult{
		Class: ProbeClassMatchedRunning, OwnershipToken: "own-1", Reason: "matched",
	})
	// Stop revalidation via ScriptedHostControl
	script := &ScriptedHostControl{
		Results: []ProbeResult{{
			Class: ProbeClassMatchedRunning, OwnershipToken: "own-1", Reason: "matched",
		}},
	}
	h2.host = nil
	sup, err := NewSupervisor(h2.snap, Deps{
		WSL: h2.wsl, Host: script, Health: h2.health,
		Clock: h2.clock, Timers: h2.clock, Record: h2.rec,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sup.Close)
	sup.Start(context.Background())
	if h2.wsl.SpawnCount() != 0 {
		t.Fatal("matched must not spawn")
	}
	stop, err := script.Stop(context.Background(), StopRequest{
		DataDir: h2.snap.DataDir, OwnershipToken: "own-1",
	})
	if err != nil || stop.Class != StopClassSignaled {
		t.Fatalf("stop = %+v err=%v", stop, err)
	}
	rej, _ := script.Stop(context.Background(), StopRequest{
		DataDir: h2.snap.DataDir, OwnershipToken: "stale",
	})
	if rej.Class != StopClassRejected {
		t.Fatalf("stale token class = %s", rej.Class)
	}
}

// TestContractNoShutdownAndBoundTarget pins WSL argv invariants.
func TestContractNoShutdownAndBoundTarget(t *testing.T) {
	snap := testSnap(t)
	spec := BuildLaunchSpec(snap)
	argv := BuildWSLExecArgv(spec)
	if err := ValidateLaunchArgv(argv, snap.Distribution, snap.LinuxUser); err != nil {
		t.Fatal(err)
	}
	if ArgvContainsShutdown(argv) {
		t.Fatal("launch contains --shutdown")
	}
	term := BuildTerminateArgv(snap.Distribution)
	if ArgvContainsShutdown(term) {
		t.Fatal("terminate contains --shutdown")
	}
	if term[2] != snap.Distribution {
		t.Fatalf("terminate target = %q", term[2])
	}
	// adversarial: refuse shutdown argv
	bad := []string{"wsl.exe", "--shutdown"}
	if !ArgvContainsShutdown(bad) {
		t.Fatal("detector missed --shutdown")
	}
	wsl := &FakeWSLRunner{}
	if err := wsl.TerminateDistro(context.Background(), snap.Distribution); err != nil {
		t.Fatal(err)
	}
	if len(wsl.Terminates) != 1 || wsl.Terminates[0] != snap.Distribution {
		t.Fatalf("terminates = %v", wsl.Terminates)
	}
}

// TestContractHealthStatusOKOnly pins Ready classification.
func TestContractHealthStatusOKOnly(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"ok", 200, `{"status":"ok"}`, true},
		{"unavailable", 200, `{"status":"unavailable"}`, false},
		{"unhealthy", 200, `{"status":"unhealthy"}`, false},
		{"http_500", 500, `{"status":"ok"}`, false},
		{"empty", 200, `{}`, false},
		{"plain", 200, `ok`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := ClassifyHealthBody(tc.status, []byte(tc.body))
			if r.Healthy != tc.want {
				t.Fatalf("healthy=%v want %v (%s)", r.Healthy, tc.want, r.Reason)
			}
			if tc.want && r.Status != HealthStatusOK {
				t.Fatalf("status = %q", r.Status)
			}
		})
	}
}

// TestContractGenerationInvalidatesHealth pins resume/epoch linearization.
func TestContractGenerationInvalidatesHealth(t *testing.T) {
	h := newHarness(t)
	h.sup.Start(context.Background())
	readyEpoch := h.sup.Epoch()
	h.resume.Resume(h.clock.Now())
	// epoch bumps on resume
	if h.sup.Epoch() == readyEpoch {
		t.Fatal("resume must bump epoch to invalidate in-flight health")
	}
	h.sup.DeliverHealth(HealthResult{
		Status: HealthStatusUnavailable, Healthy: false, Reason: "stale",
	}, readyEpoch)
	if h.rec.CountKind(KindStaleDiscard) == 0 {
		t.Fatal("stale health must be discarded")
	}
	if h.rec.CountKind(KindHealthInvalidate) == 0 {
		t.Fatal("resume must record health invalidation")
	}
}

// TestContractThresholdProjection ensures snapshot thresholds match ADR defaults.
func TestContractThresholdProjection(t *testing.T) {
	th := DefaultThresholds()
	if th.HealthProbeTimeoutMS != 2_000 || th.HealthCadenceMS != 5_000 ||
		th.StartupBudgetMS != 60_000 || th.ResumeRecheckMS != 10_000 ||
		th.HealthRestartThreshold != 3 || th.ConfigTerminalThreshold != 5 ||
		th.TransientTerminalThreshold != 10 {
		t.Fatalf("threshold drift: %+v", th)
	}
	if len(th.BackoffSeconds) != 5 || th.BackoffSeconds[4] != 60 {
		t.Fatalf("backoff = %v", th.BackoffSeconds)
	}
}

// JSON round-trip for probe wire used by ExecHostControl.
func TestProbeStopJSONParse(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"class": ProbeClassMatchedRunning, "ownership_token": "x", "pid": 1, "reason": "ok",
	})
	pr, err := parseProbeJSON(raw)
	if err != nil || pr.Class != ProbeClassMatchedRunning || pr.OwnershipToken != "x" {
		t.Fatalf("%+v %v", pr, err)
	}
	raw, _ = json.Marshal(map[string]any{"class": StopClassSignaled, "pid": 1})
	st, err := parseStopJSON(raw)
	if err != nil || st.Class != StopClassSignaled {
		t.Fatalf("%+v %v", st, err)
	}
	// invalid JSON → unknown, not panic
	pr, _ = parseProbeJSON([]byte("{"))
	if pr.Class != ProbeClassUnknown {
		t.Fatalf("class = %s", pr.Class)
	}
}

func TestBuildLaunchSpecBoundTarget(t *testing.T) {
	snap := testSnap(t)
	spec := BuildLaunchSpec(snap)
	if spec.Distribution != snap.Distribution || spec.LinuxUser != snap.LinuxUser {
		t.Fatalf("%+v", spec)
	}
	argv := BuildWSLExecArgv(spec)
	// no shell, has --exec
	if ArgvHasShell(argv) {
		t.Fatal(argv)
	}
	probe := BuildProbeLinuxArgv(snap, time.Second)
	if probe[1] != "supervisor-probe" {
		t.Fatalf("probe argv = %v", probe)
	}
	wantArgv := append([]string{spec.ServerExecutable}, spec.ServerArgs...)
	var gotArgv []string
	for i := 0; i+1 < len(probe); i++ {
		if probe[i] == "-arg" {
			gotArgv = append(gotArgv, probe[i+1])
		}
	}
	if !slices.Equal(gotArgv, wantArgv) {
		t.Fatalf("probe fingerprint argv = %v, want %v", gotArgv, wantArgv)
	}
}

// TestContractLaunchSpecSelectsExplicitNoAuth pins the Local WSL client/server
// authentication contract. The registered Workspace endpoint uses auth.mode=none,
// while the server refuses to start unless exactly one explicit authentication
// selector is supplied. The supervisor therefore has to launch this loopback
// endpoint with -no-auth; TLS selection alone is not an auth selector.
//
// rejects counterexample: a supervised server exits before listening with
// "authentication mode is required" even though direct operator launch works.
func TestContractLaunchSpecSelectsExplicitNoAuth(t *testing.T) {
	spec := BuildLaunchSpec(testSnap(t))
	if !slices.Contains(spec.ServerArgs, "-no-auth") {
		t.Fatalf("supervised server args %v omit required -no-auth selector", spec.ServerArgs)
	}
}
