package wslhost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

// TestFakeVsRealWSLBoundDistribution compares scripted FakeWSLRunner argv/targets
// against the pure production argv builders (real Windows exec is opt-in T3).
// If this fails, fix the fake or builders — do not weaken asserts.
func TestFakeVsRealWSLBoundDistribution(t *testing.T) {
	snap := testSnap(t)
	spec := BuildLaunchSpec(snap)
	wantArgv := BuildWSLExecArgv(spec)
	wantTerm := BuildTerminateArgv(snap.Distribution)

	fake := &FakeWSLRunner{}
	unit, err := fake.StartForeground(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLaunchArgv(unit.Argv(), snap.Distribution, snap.LinuxUser); err != nil {
		t.Fatal(err)
	}
	got := unit.Argv()
	if len(got) != len(wantArgv) {
		t.Fatalf("argv len fake=%d real-builder=%d", len(got), len(wantArgv))
	}
	for i := range wantArgv {
		if got[i] != wantArgv[i] {
			t.Fatalf("argv[%d] fake=%q builder=%q", i, got[i], wantArgv[i])
		}
	}
	if ArgvContainsShutdown(got) || ArgvHasShell(got) {
		t.Fatalf("forbidden tokens in %v", got)
	}
	if !slices.Contains(got, "-no-auth") {
		t.Fatalf("fake/real launch argv %v omit required -no-auth selector", got)
	}
	if err := fake.TerminateDistro(context.Background(), snap.Distribution); err != nil {
		t.Fatal(err)
	}
	if len(fake.Terminates) != 1 || fake.Terminates[0] != snap.Distribution {
		t.Fatalf("terminate target = %v", fake.Terminates)
	}
	// Real builder agreement on terminate argv (no --shutdown).
	if ArgvContainsShutdown(wantTerm) || wantTerm[2] != snap.Distribution {
		t.Fatalf("builder terminate = %v", wantTerm)
	}
	// Adversarial: other distro must not be implied by builders.
	other := BuildTerminateArgv("Other-Distro")
	if other[2] == snap.Distribution {
		t.Fatal("bound distro leaked into other terminate")
	}
}

// TestFakeVsRealHostControlLockAndSignal compares FakeHostControl scripted
// classes with JSON-decoded production wire (parseProbeJSON/parseStopJSON).
func TestFakeVsRealHostControlLockAndSignal(t *testing.T) {
	// Real wire samples as the Linux CLI would emit.
	realMatched, _ := json.Marshal(map[string]any{
		"class": ProbeClassMatchedRunning, "ownership_token": "tok-abc",
		"pid": 9, "start_token": "9:x", "reason": "fingerprint_matched_lock_holder",
	})
	realAbsent, _ := json.Marshal(map[string]any{
		"class": ProbeClassAbsent, "reason": "no_exclusive_lock_holder",
	})
	realStop, _ := json.Marshal(map[string]any{
		"class": StopClassSignaled, "pid": 9, "reason": "sigterm_delivered",
	})
	realReject, _ := json.Marshal(map[string]any{
		"class": StopClassRejected, "reason": "start_token_mismatch",
	})

	parsedMatched, _ := parseProbeJSON(realMatched)
	parsedAbsent, _ := parseProbeJSON(realAbsent)
	parsedStop, _ := parseStopJSON(realStop)
	parsedReject, _ := parseStopJSON(realReject)

	fake := &FakeHostControl{}
	fake.SetProbe(ProbeResult{
		Class: ProbeClassMatchedRunning, OwnershipToken: "tok-abc",
		PID: 9, StartToken: "9:x", Reason: "fingerprint_matched_lock_holder",
	})
	got, err := fake.Probe(context.Background(), ProbeRequest{DataDir: "/var/lib/agent-grid"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != parsedMatched.Class || got.OwnershipToken != parsedMatched.OwnershipToken {
		t.Fatalf("matched drift fake=%+v real=%+v", got, parsedMatched)
	}

	fake.SetProbe(ProbeResult{Class: ProbeClassAbsent, Reason: "no_exclusive_lock_holder"})
	got, _ = fake.Probe(context.Background(), ProbeRequest{DataDir: "/var/lib/agent-grid"})
	if got.Class != parsedAbsent.Class {
		t.Fatalf("absent drift fake=%+v real=%+v", got, parsedAbsent)
	}

	// Scripted stop authority vs real rejected/signaled classes.
	script := &ScriptedHostControl{
		Results: []ProbeResult{{Class: ProbeClassMatchedRunning, OwnershipToken: "tok-abc"}},
	}
	pr, _ := script.Probe(context.Background(), ProbeRequest{DataDir: "/d"})
	if pr.OwnershipToken != "tok-abc" {
		t.Fatal(pr)
	}
	st, _ := script.Stop(context.Background(), StopRequest{DataDir: "/d", OwnershipToken: "tok-abc"})
	if st.Class != parsedStop.Class {
		t.Fatalf("stop class fake=%s real=%s", st.Class, parsedStop.Class)
	}
	st, _ = script.Stop(context.Background(), StopRequest{DataDir: "/d", OwnershipToken: "nope"})
	if st.Class != parsedReject.Class {
		t.Fatalf("reject class fake=%s real=%s", st.Class, parsedReject.Class)
	}
}

// TestFakeVsRealHealthzSemantics compares FakeHealthProber with HTTPHealthProber
// against a real local /healthz server.
func TestFakeVsRealHealthzSemantics(t *testing.T) {
	mux := http.NewServeMux()
	var status string
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	real := &HTTPHealthProber{}
	fake := &FakeHealthProber{}

	for _, st := range []string{HealthStatusOK, HealthStatusUnavailable, HealthStatusUnhealthy} {
		status = st
		fake.Next = ClassifyHealthBody(200, []byte(`{"status":"`+st+`"}`))
		fr, err := fake.Probe(context.Background(), srv.URL+"/healthz", time.Second)
		if err != nil {
			t.Fatal(err)
		}
		rr, err := real.Probe(context.Background(), srv.URL+"/healthz", time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if fr.Healthy != rr.Healthy || fr.Status != rr.Status {
			t.Fatalf("status %s: fake=%+v real=%+v", st, fr, rr)
		}
		if st == HealthStatusOK && !rr.Healthy {
			t.Fatal("real did not accept status ok")
		}
		if st != HealthStatusOK && rr.Healthy {
			t.Fatal("real accepted non-ok")
		}
	}
	// Adversarial: HTTP 200 without ok body is non-Ready for both.
	status = "weird"
	fake.Next = ClassifyHealthBody(200, []byte(`{"status":"weird"}`))
	fr, _ := fake.Probe(context.Background(), srv.URL+"/healthz", time.Second)
	rr, _ := real.Probe(context.Background(), srv.URL+"/healthz", time.Second)
	if fr.Healthy || rr.Healthy {
		t.Fatalf("non-ok body accepted fake=%v real=%v", fr.Healthy, rr.Healthy)
	}
}

// TestFakeVsRealWindowsResumeNotification compares FakeResumeSource delivery
// against ChannelResumeSource (production-shaped adapter) under ManualClock policy.
func TestFakeVsRealWindowsResumeNotification(t *testing.T) {
	snap := testSnap(t)

	run := func(t *testing.T, name string, resume ResumeSource, fire func(time.Time), drain func()) {
		t.Helper()
		clock := NewManualClock(time.Date(2026, 7, 26, 18, 0, 0, 0, time.UTC))
		rec := &MemoryRecorder{}
		host := &FakeHostControl{NextProbe: ProbeResult{Class: ProbeClassAbsent}}
		health := &FakeHealthProber{}
		health.SetOK()
		sup, err := NewSupervisor(snap, Deps{
			WSL: &FakeWSLRunner{}, Host: host, Health: health,
			Clock: clock, Timers: clock, Resume: resume, Record: rec,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(sup.Close)
		sup.Start(context.Background())
		if sup.Phase() != PhaseReady {
			t.Fatalf("%s: phase = %s", name, sup.Phase())
		}
		at := clock.Now().Add(2 * time.Second)
		fire(at)
		if drain != nil {
			drain()
		}
		clock.Advance(0)
		if rec.CountKind(KindResume) == 0 {
			t.Fatalf("%s: missing resume record", name)
		}
		if rec.CountKind(KindHealthInvalidate) == 0 {
			t.Fatalf("%s: missing invalidate", name)
		}
	}

	fake := &FakeResumeSource{}
	run(t, "fake", fake, fake.Resume, nil)

	ch := make(chan time.Time, 1)
	realSrc := NewChannelResumeSource(ch)
	t.Cleanup(realSrc.Close)
	run(t, "channel", realSrc, func(at time.Time) { ch <- at }, func() {
		// Allow fan-out goroutine to deliver before fake-clock advance.
		time.Sleep(30 * time.Millisecond)
	})

	if DefaultThresholds().ResumeRecheckMS > 10_000 {
		t.Fatal("resume budget exceeds 10s")
	}
}
