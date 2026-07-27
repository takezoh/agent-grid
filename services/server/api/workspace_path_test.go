package api

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspacePathGuard(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "in-root.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	linkInside := filepath.Join(sub, "link-out")
	if err := os.Symlink(outside, linkInside); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	linkAtRoot := filepath.Join(root, "escape-link")
	if err := os.Symlink(outside, linkAtRoot); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	cases := []struct {
		name    string
		rel     string
		wantErr bool
		wantIn  string
	}{
		{name: "empty is root", rel: "", wantIn: root},
		{name: "relative in-root", rel: "in-root.txt", wantIn: root},
		{name: "nested dir", rel: "sub", wantIn: root},
		{name: "absolute rejected", rel: filepath.Join(root, "in-root.txt"), wantErr: true},
		{name: "dotdot rejected", rel: "../outside.txt", wantErr: true},
		{name: "encoded traversal shape", rel: "sub/../../" + filepath.Base(outside) + "/outside.txt", wantErr: true},
		{name: "intermediate symlink outside", rel: "sub/link-out/outside.txt", wantErr: true},
		{name: "final symlink outside", rel: "escape-link/outside.txt", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GuardWorkspacePath(root, tc.rel)
			if tc.wantErr {
				if !errors.Is(err, ErrWorkspacePathNotFound) {
					t.Fatalf("err = %v, want ErrWorkspacePathNotFound", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.wantIn != "" && !stringsHasPrefix(got, filepath.Clean(tc.wantIn)) {
				t.Fatalf("resolved %q not under %q", got, tc.wantIn)
			}
			if tc.rel == "in-root.txt" {
				data, readErr := os.ReadFile(got)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if string(data) != "secret" {
					t.Fatalf("read %q", data)
				}
			}
		})
	}

	t.Run("outside bytes never returned", func(t *testing.T) {
		for _, rel := range []string{"../" + filepath.Base(outside) + "/outside.txt", "escape-link/outside.txt"} {
			_, err := GuardWorkspacePath(root, rel)
			if !errors.Is(err, ErrWorkspacePathNotFound) {
				t.Fatalf("rel %q: err = %v, want not found", rel, err)
			}
		}
		data, err := os.ReadFile(filepath.Join(outside, "outside.txt"))
		if err != nil || string(data) != "outside" {
			t.Fatalf("fixture sanity check failed")
		}
	})
}

func stringsHasPrefix(s, prefix string) bool {
	if s == prefix {
		return true
	}
	sep := string(filepath.Separator)
	return len(s) > len(prefix) && s[:len(prefix)] == prefix && s[len(prefix)] == sep[0]
}

func FuzzGuardWorkspacePathRejectsTraversal(f *testing.F) {
	root := f.TempDir()
	f.Add("..")
	f.Add("../etc/passwd")
	f.Add("/absolute")
	f.Add("ok/..")

	f.Fuzz(func(t *testing.T, rel string) {
		if rel == "" || rel == "." || (!filepath.IsAbs(rel) && !strings.Contains(filepath.Clean(rel), "..")) {
			return
		}
		_, err := GuardWorkspacePath(root, rel)
		if err == nil {
			t.Fatalf("rel %q should be rejected", rel)
		}
		if !errors.Is(err, ErrWorkspacePathNotFound) {
			t.Fatalf("rel %q: err = %v, want ErrWorkspacePathNotFound", rel, err)
		}
	})
}
