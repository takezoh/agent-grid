package agentlaunch

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestInstallExecInRunDir_OverRunningBinary reproduces the ETXTBSY incident:
// the rundir roost-bridge is bind-mounted into a live container and currently
// executing, while a daemon rebuild leaves the source binary with the same
// bytes but a newer mtime. The size+mtime short-circuit misses on mtime, so a
// reinstall is attempted over the in-use inode. A naive O_TRUNC copy fails with
// ETXTBSY; installExecInRunDir must use rename semantics so the running inode is
// left untouched and new launches pick up the swapped directory entry.
func TestInstallExecInRunDir_OverRunningBinary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ETXTBSY is Linux-specific")
	}
	// Any small ELF binary works as a stand-in for roost-bridge.
	sleepBin, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not found:", err)
	}

	runDir := t.TempDir()
	dst := filepath.Join(runDir, "roost-bridge")

	// Install once, then launch it so dst's inode is an in-use ELF text segment.
	if err := installExecInRunDir(sleepBin, dst); err != nil {
		t.Fatalf("initial install: %v", err)
	}
	cmd := exec.Command(dst, "600")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start binary: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	// Reproduce the production trigger: identical bytes, but dst's mtime predates
	// the source (as if rundir was populated by an earlier build). This defeats
	// the size+mtime short-circuit and forces a reinstall over the running inode.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(dst, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := installExecInRunDir(sleepBin, dst); err != nil {
		t.Errorf("reinstall over running binary: %v", err)
	}
}
