package winexec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteShims_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	exes := []string{"code.exe", "clip.exe"}
	shimDir, err := writeShims(dir, "/opt/roost/run/roost", exes)
	if err != nil {
		t.Fatalf("writeShims: %v", err)
	}

	for _, exe := range exes {
		path := filepath.Join(shimDir, exe)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("shim %s not found: %v", exe, err)
			continue
		}
		if string(data[:2]) != "#!" {
			t.Errorf("shim %s missing shebang", exe)
		}
		info, _ := os.Stat(path)
		if info.Mode()&0o100 == 0 {
			t.Errorf("shim %s not executable", exe)
		}
	}
}

func TestWriteShims_Idempotent(t *testing.T) {
	dir := t.TempDir()
	exes := []string{"code.exe"}

	if _, err := writeShims(dir, "/opt/roost/run/roost", exes); err != nil {
		t.Fatalf("first writeShims: %v", err)
	}

	path := filepath.Join(dir, ShimDirName, "code.exe")
	info1, _ := os.Stat(path)

	if _, err := writeShims(dir, "/opt/roost/run/roost", exes); err != nil {
		t.Fatalf("second writeShims: %v", err)
	}
	info2, _ := os.Stat(path)

	if info1.ModTime() != info2.ModTime() {
		t.Error("shim file was rewritten on second call (expected no-op)")
	}
}

func TestWriteShims_ShimContent(t *testing.T) {
	dir := t.TempDir()
	if _, err := writeShims(dir, "/opt/roost/run/roost", []string{"explorer.exe"}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ShimDirName, "explorer.exe"))
	content := string(data)

	if content != "#!/bin/sh\nexec /opt/roost/run/roost win-exec explorer.exe \"$@\"\n" {
		t.Errorf("unexpected shim content:\n%s", content)
	}
}
