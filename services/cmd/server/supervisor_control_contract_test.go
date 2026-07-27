package main

import (
	"testing"

	"golang.org/x/sys/unix"
)

// TestContractProbeBeforeSpawnAndTokenRevalidation pins the integration
// invariant named in implementation.md: adoption/stop authority comes only
// from a fingerprint-matched probe token, and stop revalidates before SIGTERM.
//
// Counterexample rejected: treating a live lock holder as stop-authoritative
// without a matched probe token (or after start-token rotation) would signal
// a non-owned process.
func TestContractProbeBeforeSpawnAndTokenRevalidation(t *testing.T) {
	restore := stubHostControl(t)
	defer restore()

	const (
		pid        = 777
		startToken = "777:start-a"
		dataDir    = "/var/lib/agent-grid"
		exe        = "/opt/agent-grid/server"
	)
	ident := processIdentity{
		PID:        pid,
		Executable: exe,
		Argv:       []string{"server", "-data-dir", dataDir},
		StartToken: startToken,
	}

	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		return lockHolderInspect{Held: true, PID: pid}, nil
	}
	readProcessIdentityFn = func(int) (processIdentity, error) { return ident, nil }

	// 1) Probe before any signal path: matched_running binds the token.
	probe := classifyProbe(probeRequest{
		DataDir:      dataDir,
		ExpectedExe:  exe,
		ExpectedArgv: ident.Argv,
	})
	if probe.Class != probeClassMatchedRunning {
		t.Fatalf("probe class = %q, want matched_running", probe.Class)
	}
	if probe.OwnershipToken == "" {
		t.Fatal("matched probe must emit ownership_token")
	}

	// 2) Stop with the probe token delivers SIGTERM only after revalidation.
	var signals []unix.Signal
	signalProcessFn = func(gotPID int, sig unix.Signal) error {
		if gotPID != pid {
			t.Fatalf("signal pid = %d, want %d", gotPID, pid)
		}
		signals = append(signals, sig)
		return nil
	}
	stop := executeStop(dataDir, probe.OwnershipToken)
	if stop.Class != stopClassSignaled {
		t.Fatalf("stop class = %q, want signaled (%s)", stop.Class, stop.Reason)
	}
	if len(signals) != 1 || signals[0] != unix.SIGTERM {
		t.Fatalf("signals = %v, want single SIGTERM", signals)
	}

	// 3) Adversarial: start-token rotation invalidates prior authority.
	signals = nil
	ident.StartToken = "777:start-b"
	readProcessIdentityFn = func(int) (processIdentity, error) { return ident, nil }
	stale := executeStop(dataDir, probe.OwnershipToken)
	if stale.Class != stopClassRejected {
		t.Fatalf("stale stop class = %q, want rejected", stale.Class)
	}
	if len(signals) != 0 {
		t.Fatalf("stale token signaled: %v", signals)
	}

	// 4) Adversarial: absent lock never grants signal authority.
	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		return lockHolderInspect{Held: false}, nil
	}
	absentProbe := classifyProbe(probeRequest{DataDir: dataDir, ExpectedExe: exe})
	if absentProbe.Class != probeClassAbsent {
		t.Fatalf("absent probe = %q", absentProbe.Class)
	}
	if absentProbe.OwnershipToken != "" {
		t.Fatal("absent probe must not emit ownership_token")
	}
}
