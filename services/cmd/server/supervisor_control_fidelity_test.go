package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/services/platform/appid"
	"golang.org/x/sys/unix"
)

// fakeHostControl is the in-process HostControl seam used by pure tests.
// FakeVsRealHostControlLockAndSignal asserts that scripted classes match the
// real flock + /proc + SIGTERM path for the same scenarios.
type fakeHostControl struct {
	held       bool
	pid        int
	ident      processIdentity
	identErr   error
	inspectErr error
	signals    []unix.Signal
}

func (f *fakeHostControl) install(t *testing.T) func() {
	t.Helper()
	restore := stubHostControl(t)
	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		if f.inspectErr != nil {
			return lockHolderInspect{}, f.inspectErr
		}
		return lockHolderInspect{Held: f.held, PID: f.pid}, nil
	}
	readProcessIdentityFn = func(int) (processIdentity, error) {
		if f.identErr != nil {
			return processIdentity{}, f.identErr
		}
		return f.ident, nil
	}
	signalProcessFn = func(pid int, sig unix.Signal) error {
		if pid != f.pid {
			t.Fatalf("fake signal pid = %d, want %d", pid, f.pid)
		}
		f.signals = append(f.signals, sig)
		return nil
	}
	return restore
}

// TestFakeVsRealHostControlLockAndSignal is the T3-style fidelity backstop for
// the Linux process/lock dependency. The fake HostControl and the real
// inspect/identity/signal path must agree on classification and stop authority.
// If this fails, fix the fake (or production adapter) — do not weaken asserts.
func TestFakeVsRealHostControlLockAndSignal(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, appid.PidFileName)

	// --- Real absent ---
	realAbsent, err := inspectDaemonLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if realAbsent.Held {
		t.Fatal("real: expected absent before acquire")
	}

	fake := &fakeHostControl{held: false}
	restore := fake.install(t)
	fakeProbe := classifyProbe(probeRequest{DataDir: dir})
	restore()
	if fakeProbe.Class != probeClassAbsent {
		t.Fatalf("fake absent class = %q", fakeProbe.Class)
	}

	// --- Real matched holder ---
	lock, err := acquireDaemonLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lock.release)

	selfPID := os.Getpid()
	realIdent, err := readProcessIdentity(selfPID)
	if err != nil {
		t.Fatalf("read self identity: %v", err)
	}
	realHeld, err := inspectDaemonLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if !realHeld.Held || realHeld.PID != selfPID {
		t.Fatalf("real inspect = %+v, want held by self", realHeld)
	}

	// Fake mirrors the real observation.
	fake = &fakeHostControl{
		held:  true,
		pid:   selfPID,
		ident: realIdent,
	}
	restore = fake.install(t)
	fakeMatched := classifyProbe(probeRequest{
		DataDir:     dir,
		ExpectedExe: realIdent.Executable,
		ExpectedArgv: realIdent.Argv,
	})
	restore()
	if fakeMatched.Class != probeClassMatchedRunning {
		t.Fatalf("fake matched class = %q (%s)", fakeMatched.Class, fakeMatched.Reason)
	}

	// Real classify with production seams (no stub).
	realMatched := classifyProbe(probeRequest{
		DataDir:      dir,
		ExpectedExe:  realIdent.Executable,
		ExpectedArgv: realIdent.Argv,
	})
	if realMatched.Class != probeClassMatchedRunning {
		t.Fatalf("real matched class = %q (%s)", realMatched.Class, realMatched.Reason)
	}
	if realMatched.StartToken != realIdent.StartToken {
		t.Fatalf("start_token fake/real diverge: fake-source=%s real=%s", realIdent.StartToken, realMatched.StartToken)
	}
	if fakeMatched.Class != realMatched.Class {
		t.Fatalf("class mismatch fake=%s real=%s", fakeMatched.Class, realMatched.Class)
	}

	// --- Stop revalidation: real path would SIGTERM self; use fake signal only
	// for delivery proof, then real identity revalidation against a stale token.
	fake = &fakeHostControl{held: true, pid: selfPID, ident: realIdent}
	restore = fake.install(t)
	stop := executeStop(dir, realMatched.OwnershipToken)
	restore()
	if stop.Class != stopClassSignaled {
		t.Fatalf("fake stop class = %q (%s)", stop.Class, stop.Reason)
	}
	if len(fake.signals) != 1 || fake.signals[0] != unix.SIGTERM {
		t.Fatalf("fake signals = %v, want SIGTERM", fake.signals)
	}

	// Real revalidation without signalling: wrong start token is rejected.
	staleTok, err := encodeOwnershipToken(ownershipToken{
		PID:        selfPID,
		StartToken: realIdent.StartToken + "-stale",
		DataDir:    dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Temporarily no-op signal so a logic bug cannot kill the test process.
	restore = stubHostControl(t)
	signalProcessFn = func(int, unix.Signal) error {
		t.Fatal("stale real stop must not call signal")
		return nil
	}
	// Keep real inspect + identity.
	inspectDaemonLockFn = inspectDaemonLock
	readProcessIdentityFn = readProcessIdentity
	stale := executeStop(dir, staleTok)
	restore()
	if stale.Class != stopClassRejected {
		t.Fatalf("real stale stop class = %q, want rejected", stale.Class)
	}

	// Stability: start token remains stable for a live process over a short window.
	time.Sleep(5 * time.Millisecond)
	again, err := readProcessIdentity(selfPID)
	if err != nil {
		t.Fatal(err)
	}
	if again.StartToken != realIdent.StartToken {
		t.Fatalf("start token unstable: %s -> %s", realIdent.StartToken, again.StartToken)
	}
}
