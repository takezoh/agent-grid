package gitclone

import (
	"path/filepath"
	"testing"
)

func TestPublishCloneContract_OrderingAndConflict(t *testing.T) {
	fs := NewFakeFilesystemProcess()
	staging, final := "/repo/stage-1", "/repo/final-1"
	_ = fs.MkdirAll(staging, 0o755)
	fs.Trees[filepath.Join(staging, "README")] = []byte("ok")

	res := PublishClone(PublishRequest{StagingDir: staging, FinalDir: final, Marker: "m1", FS: fs})
	if res.Code != CodeAtomicPublishOK {
		t.Fatalf("publish: %+v", res)
	}
	wantOrder := []string{"stat", "sync_tree", "marker_write", "marker_sync", "mkdir", "rename", "parent_sync"}
	if !containsSequence(fs.Calls, wantOrder) {
		t.Fatalf("calls=%v want sequence %v", fs.Calls, wantOrder)
	}
	marker, err := fs.ReadFile(filepath.Join(final, MarkerFileName))
	if err != nil || string(marker) != "m1" {
		t.Fatalf("marker=%q err=%v", marker, err)
	}

	// Conflict: final already exists → no overwrite.
	fs2 := NewFakeFilesystemProcess()
	fs2.FinalExists = map[string]bool{filepath.Clean(final): true}
	res = PublishClone(PublishRequest{StagingDir: staging, FinalDir: final, Marker: "m2", FS: fs2})
	if res.Code != CodeCloneIdentityConflict {
		t.Fatalf("conflict: %+v", res)
	}
	for _, c := range fs2.Calls {
		if c == "rename" {
			t.Fatal("must not rename on conflict")
		}
	}
}

func TestPublishCloneContract_CrashPoints(t *testing.T) {
	for _, step := range []string{"sync_tree", "marker_write", "marker_sync", "rename", "parent_sync"} {
		fs := NewFakeFilesystemProcess()
		fs.FailAt = step
		_ = fs.MkdirAll("/r/s", 0o755)
		res := PublishClone(PublishRequest{StagingDir: "/r/s", FinalDir: "/r/f", Marker: "g1", FS: fs})
		if res.Code == CodeAtomicPublishOK {
			t.Fatalf("%s: unexpected ok", step)
		}
	}
	fs := NewFakeFilesystemProcess()
	fs.DirSyncUnsupported = true
	_ = fs.MkdirAll("/r/s", 0o755)
	res := PublishClone(PublishRequest{StagingDir: "/r/s", FinalDir: "/r/f", Marker: "g", FS: fs})
	if res.Code != CodeAtomicPublishUnsupported {
		t.Fatalf("unsupported: %+v", res)
	}
}

func TestFilesystemProcessContract(t *testing.T) {
	fs := NewFakeFilesystemProcess()
	if err := fs.MkdirAll("/a/b", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFileAtomic("/a/b/f", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fs.SyncFile("/a/b/f"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Rename("/a/b", "/a/c"); err != nil {
		t.Fatal(err)
	}
	if err := fs.KillProcess(0); err != ErrProcessUnavailable {
		t.Fatalf("kill unavailable: %v", err)
	}
	if err := fs.KillProcess(42); err != nil {
		t.Fatal(err)
	}
	if len(fs.Killed) != 1 || fs.Killed[0] != 42 {
		t.Fatalf("killed=%v", fs.Killed)
	}
}

func TestReconcileCloneEvidence(t *testing.T) {
	fs := NewFakeFilesystemProcess()
	final := "/root/final"
	_ = fs.MkdirAll(final, 0o755)
	fs.Trees[filepath.Join(final, MarkerFileName)] = []byte("m1")
	r := ReconcileCloneEvidence(ReconcileRequest{StagingDir: "/root/stage", FinalDir: final, ExpectedMarker: "m1", Root: "/root", FS: fs})
	if r.Disposition != ReconcileCloneReady {
		t.Fatalf("%+v", r)
	}
	r = ReconcileCloneEvidence(ReconcileRequest{StagingDir: "/root/stage", FinalDir: final, ExpectedMarker: "other", Root: "/root", FS: fs})
	if r.Disposition != ReconcileConflict {
		t.Fatalf("conflict: %+v", r)
	}
	fs2 := NewFakeFilesystemProcess()
	_ = fs2.MkdirAll("/root/stage", 0o755)
	r = ReconcileCloneEvidence(ReconcileRequest{StagingDir: "/root/stage", FinalDir: "/root/final", Root: "/root", FS: fs2})
	if r.Disposition != ReconcileRetryClone {
		t.Fatalf("retry: %+v", r)
	}
	r = ReconcileCloneEvidence(ReconcileRequest{StagingDir: "/escape/s", FinalDir: "/escape/f", Root: "/root", FS: NewFakeFilesystemProcess()})
	if r.Disposition != ReconcileQuarantine {
		t.Fatalf("escape: %+v", r)
	}
}

func containsSequence(calls, want []string) bool {
	i := 0
	for _, c := range calls {
		if i < len(want) && c == want[i] {
			i++
		}
	}
	return i == len(want)
}
