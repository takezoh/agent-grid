package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-grid/services/platform/appid"
	"golang.org/x/sys/unix"
)

func TestClassifyProbe_Absent(t *testing.T) {
	restore := stubHostControl(t)
	defer restore()

	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		return lockHolderInspect{Held: false}, nil
	}
	result := classifyProbe(probeRequest{DataDir: "/data/ag"})
	if result.Class != probeClassAbsent {
		t.Fatalf("class = %q, want %s", result.Class, probeClassAbsent)
	}
	if result.OwnershipToken != "" {
		t.Fatalf("absent must not grant ownership token, got %q", result.OwnershipToken)
	}
}

func TestClassifyProbe_MatchedRunning(t *testing.T) {
	restore := stubHostControl(t)
	defer restore()

	const pid = 4242
	ident := processIdentity{
		PID:        pid,
		Executable: "/usr/local/bin/server",
		Argv:       []string{"server", "-data-dir", "/data/ag"},
		StartToken: "4242:999",
	}
	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		return lockHolderInspect{Held: true, PID: pid}, nil
	}
	readProcessIdentityFn = func(got int) (processIdentity, error) {
		if got != pid {
			t.Fatalf("pid = %d, want %d", got, pid)
		}
		return ident, nil
	}

	result := classifyProbe(probeRequest{
		DataDir:      "/data/ag",
		ExpectedExe:  "/usr/local/bin/server",
		ExpectedArgv: []string{"server", "-data-dir", "/data/ag"},
	})
	if result.Class != probeClassMatchedRunning {
		t.Fatalf("class = %q, want %s (%s)", result.Class, probeClassMatchedRunning, result.Reason)
	}
	if result.OwnershipToken == "" {
		t.Fatal("matched_running must bind an ownership token")
	}
	tok, err := decodeOwnershipToken(result.OwnershipToken)
	if err != nil {
		t.Fatalf("token decode: %v", err)
	}
	if tok.PID != pid || tok.StartToken != ident.StartToken {
		t.Fatalf("token = %+v, want pid/start from identity", tok)
	}
}

func TestClassifyProbe_Mismatch(t *testing.T) {
	restore := stubHostControl(t)
	defer restore()

	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		return lockHolderInspect{Held: true, PID: 7}, nil
	}
	readProcessIdentityFn = func(int) (processIdentity, error) {
		return processIdentity{
			PID:        7,
			Executable: "/other/bin",
			Argv:       []string{"other"},
			StartToken: "7:1",
		}, nil
	}

	result := classifyProbe(probeRequest{
		DataDir:     "/data/ag",
		ExpectedExe: "/usr/local/bin/server",
	})
	if result.Class != probeClassMismatch {
		t.Fatalf("class = %q, want %s", result.Class, probeClassMismatch)
	}
	if result.OwnershipToken != "" {
		t.Fatal("mismatch must not grant stop authority")
	}
}

func TestIdentityMatchesResolvedExecutableSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "server-real")
	if err := os.WriteFile(target, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "server")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	argv := []string{link, "-data-dir", "/data/ag"}
	if !identityMatches(probeRequest{ExpectedExe: link, ExpectedArgv: argv}, processIdentity{
		Executable: target,
		Argv:       argv,
	}) {
		t.Fatal("resolved /proc executable target should match registered symlink")
	}
}

func TestClassifyProbe_UnknownOnIdentityFailure(t *testing.T) {
	restore := stubHostControl(t)
	defer restore()

	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		return lockHolderInspect{Held: true, PID: 9}, nil
	}
	readProcessIdentityFn = func(int) (processIdentity, error) {
		return processIdentity{}, errors.New("no /proc")
	}

	result := classifyProbe(probeRequest{DataDir: "/data/ag"})
	if result.Class != probeClassUnknown {
		t.Fatalf("class = %q, want %s", result.Class, probeClassUnknown)
	}
}

func TestExecuteStop_RejectsStaleToken(t *testing.T) {
	restore := stubHostControl(t)
	defer restore()

	signaled := false
	signalProcessFn = func(int, unix.Signal) error {
		signaled = true
		return nil
	}
	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		return lockHolderInspect{Held: true, PID: 100}, nil
	}
	readProcessIdentityFn = func(int) (processIdentity, error) {
		return processIdentity{PID: 100, StartToken: "100:current"}, nil
	}

	tok, err := encodeOwnershipToken(ownershipToken{PID: 100, StartToken: "100:stale", DataDir: "/data/ag"})
	if err != nil {
		t.Fatal(err)
	}
	result := executeStop("/data/ag", tok)
	if result.Class != stopClassRejected {
		t.Fatalf("class = %q, want %s", result.Class, stopClassRejected)
	}
	if signaled {
		t.Fatal("stale token must never reach SIGTERM")
	}
}

func TestExecuteStop_SignaledOnMatchedToken(t *testing.T) {
	restore := stubHostControl(t)
	defer restore()

	var gotPID int
	var gotSig unix.Signal
	signalProcessFn = func(pid int, sig unix.Signal) error {
		gotPID, gotSig = pid, sig
		return nil
	}
	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		return lockHolderInspect{Held: true, PID: 55}, nil
	}
	readProcessIdentityFn = func(int) (processIdentity, error) {
		return processIdentity{PID: 55, StartToken: "55:abc"}, nil
	}

	tok, err := encodeOwnershipToken(ownershipToken{PID: 55, StartToken: "55:abc", DataDir: "/data/ag"})
	if err != nil {
		t.Fatal(err)
	}
	result := executeStop("/data/ag", tok)
	if result.Class != stopClassSignaled {
		t.Fatalf("class = %q, want %s (%s)", result.Class, stopClassSignaled, result.Reason)
	}
	if gotPID != 55 || gotSig != unix.SIGTERM {
		t.Fatalf("signal = pid=%d sig=%v, want 55 SIGTERM", gotPID, gotSig)
	}
}

func TestExecuteStop_RejectsMismatchedDataDir(t *testing.T) {
	restore := stubHostControl(t)
	defer restore()

	signaled := false
	signalProcessFn = func(int, unix.Signal) error {
		signaled = true
		return nil
	}
	tok, err := encodeOwnershipToken(ownershipToken{PID: 1, StartToken: "1:1", DataDir: "/other"})
	if err != nil {
		t.Fatal(err)
	}
	result := executeStop("/data/ag", tok)
	if result.Class != stopClassRejected {
		t.Fatalf("class = %q, want %s", result.Class, stopClassRejected)
	}
	if signaled {
		t.Fatal("data-dir mismatch must not signal")
	}
}

func TestRunSupervisorProbe_CLIWritesJSON(t *testing.T) {
	restore := stubHostControl(t)
	defer restore()

	inspectDaemonLockFn = func(string) (lockHolderInspect, error) {
		return lockHolderInspect{Held: false}, nil
	}
	var buf bytes.Buffer
	if err := runSupervisorProbe([]string{"-data-dir", t.TempDir()}, &buf); err != nil {
		t.Fatal(err)
	}
	var result probeResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("json: %v (%q)", err, buf.String())
	}
	if result.Class != probeClassAbsent {
		t.Fatalf("class = %q", result.Class)
	}
}

func TestInspectDaemonLock_AbsentAndHeld(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, appid.PidFileName)

	got, err := inspectDaemonLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Held {
		t.Fatal("missing lock file should be absent")
	}

	lock, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()

	got, err = inspectDaemonLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Held || got.PID <= 0 {
		t.Fatalf("inspect = %+v, want held with pid", got)
	}
}

func TestParseProcStatStarttime(t *testing.T) {
	// Synthetic line: pid (comm with spaces) state ... starttime at field 22.
	stat := "1 (some name) S 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 12345 0 0 0"
	got, err := parseProcStatStarttime(stat)
	if err != nil {
		t.Fatal(err)
	}
	if got != "12345" {
		t.Fatalf("starttime = %q, want 12345", got)
	}
}

func stubHostControl(t *testing.T) func() {
	t.Helper()
	prevInspect := inspectDaemonLockFn
	prevIdent := readProcessIdentityFn
	prevSignal := signalProcessFn
	prevHealth := probeHealthFn
	return func() {
		inspectDaemonLockFn = prevInspect
		readProcessIdentityFn = prevIdent
		signalProcessFn = prevSignal
		probeHealthFn = prevHealth
	}
}
