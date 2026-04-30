package winexec

import (
	"testing"
)

func TestValidateName_OK(t *testing.T) {
	allowed := []string{"code.exe", "explorer.exe", "clip.exe"}
	for _, name := range allowed {
		if err := validateName(name, allowed); err != nil {
			t.Errorf("validateName(%q): unexpected error: %v", name, err)
		}
	}
}

func TestValidateName_NotInAllowlist(t *testing.T) {
	if err := validateName("cmd.exe", []string{"code.exe"}); err == nil {
		t.Error("validateName(cmd.exe): expected error, got nil")
	}
}

func TestValidateName_Empty(t *testing.T) {
	if err := validateName("", []string{"code.exe"}); err == nil {
		t.Error("validateName(''): expected error, got nil")
	}
}

func TestValidateName_PathTraversal(t *testing.T) {
	cases := []string{
		"../code.exe",
		"/usr/bin/sh",
		"sub/code.exe",
		"..\\code.exe",
		"code.exe\x00",
	}
	for _, name := range cases {
		if err := validateName(name, []string{name}); err == nil {
			t.Errorf("validateName(%q): expected error (path traversal/injection), got nil", name)
		}
	}
}

func TestResolveExe_WithMapping(t *testing.T) {
	resolve := map[string]string{"code.exe": "/mnt/c/Users/take/vscode/bin/code.exe"}
	got := resolveExe("code.exe", resolve)
	if got != "/mnt/c/Users/take/vscode/bin/code.exe" {
		t.Errorf("resolveExe = %q, want /mnt/c/Users/take/vscode/bin/code.exe", got)
	}
}

func TestResolveExe_WithoutMapping(t *testing.T) {
	got := resolveExe("clip.exe", nil)
	if got != "clip.exe" {
		t.Errorf("resolveExe = %q, want clip.exe (passthrough)", got)
	}
}

func TestResolveExe_EmptyMappingValue(t *testing.T) {
	resolve := map[string]string{"code.exe": ""}
	got := resolveExe("code.exe", resolve)
	if got != "code.exe" {
		t.Errorf("resolveExe = %q, want code.exe (empty value treated as unset)", got)
	}
}
