package editor

import "testing"

func TestWslDistro_set(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "MyDistro")
	if got := wslDistro(); got != "MyDistro" {
		t.Errorf("wslDistro() = %q, want %q", got, "MyDistro")
	}
}

func TestWslDistro_unset(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	if got := wslDistro(); got != "" {
		t.Errorf("wslDistro() = %q, want empty", got)
	}
}

func TestToEditorTarget_nonWSL(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	const want = "/workspace/my-project"
	if got := toEditorTarget(want); got != want {
		t.Errorf("toEditorTarget(%q) = %q, want unchanged path in non-WSL", want, got)
	}
}

func TestToEditorTarget_wslpathUnavailable(t *testing.T) {
	// When WSL_DISTRO_NAME is set but wslpath is unavailable (e.g. in CI),
	// toEditorTarget must fall back to the original path rather than error.
	t.Setenv("WSL_DISTRO_NAME", "TestDistro")
	t.Setenv("PATH", "") // make wslpath unlookable
	const want = "/workspace/fallback"
	if got := toEditorTarget(want); got != want {
		t.Errorf("toEditorTarget fallback = %q, want %q", got, want)
	}
}
