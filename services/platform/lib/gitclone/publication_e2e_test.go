package gitclone

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFakeVsRealFilesystemProcess: opt-in host fidelity; absent authority is typed unavailable.
func TestFakeVsRealFilesystemProcess(t *testing.T) {
	if os.Getenv("AG_E2E_FILESYSTEM_PROCESS") == "" {
		fake := NewFakeFilesystemProcess()
		if err := fake.KillProcess(0); err != ErrProcessUnavailable {
			t.Fatalf("fake unavailable: %v", err)
		}
		if err := (OSFilesystem{}).KillProcess(0); err != ErrProcessUnavailable {
			t.Fatalf("real unavailable: %v", err)
		}
		return
	}
	dir := t.TempDir()
	real := OSFilesystem{}
	staging := filepath.Join(dir, "stage")
	final := filepath.Join(dir, "final")
	if err := real.MkdirAll(filepath.Join(staging, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := real.WriteFileAtomic(filepath.Join(staging, "README"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := PublishClone(PublishRequest{StagingDir: staging, FinalDir: final, Marker: "real-1", FS: real})
	if res.Code != CodeAtomicPublishOK {
		t.Fatalf("real publish: %+v", res)
	}
	fake := NewFakeFilesystemProcess()
	_ = fake.MkdirAll("/s", 0o755)
	fakeRes := PublishClone(PublishRequest{StagingDir: "/s", FinalDir: "/f", Marker: "real-1", FS: fake})
	if fakeRes.Code != CodeAtomicPublishOK {
		t.Fatalf("fake publish: %+v", fakeRes)
	}
}
