package hostexec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/config"
)

func newTestSpecBuilder(t *testing.T, wsDir string) (*SpecBuilder, string) {
	t.Helper()
	runBase := t.TempDir()
	b := NewSpecBuilder(context.Background(), Config{
		RunBase:          runBase,
		ContainerRunDir:  "/opt/roost/run",
		ContainerBinPath: "/opt/roost/bin/roost",
		WorkspaceFolderFor: func(string) string { return wsDir },
	}, func(string) config.HostExecConfig { return config.HostExecConfig{} })
	return b, runBase
}

func TestContainerSpec_OverlayMounts(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/myproject")
	cfg := config.HostExecConfig{
		Allow:   []string{"gcloud *"},
		Overlay: []string{"bin/gcloud", "tools/gcloud"},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/myproject")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}

	if len(spec.Mounts) != 2 {
		t.Fatalf("Mounts = %v, want 2", spec.Mounts)
	}
	if !strings.Contains(spec.Mounts[0], "target=/workspace/myproject/bin/gcloud") {
		t.Errorf("mount[0] = %q, want target .../bin/gcloud", spec.Mounts[0])
	}
	if !strings.Contains(spec.Mounts[0], "readonly") {
		t.Errorf("mount[0] = %q, want readonly", spec.Mounts[0])
	}
	if !strings.Contains(spec.Mounts[1], "target=/workspace/myproject/tools/gcloud") {
		t.Errorf("mount[1] = %q, want target .../tools/gcloud", spec.Mounts[1])
	}
}

func TestContainerSpec_OverlayShimWritten(t *testing.T) {
	b, runBase := newTestSpecBuilder(t, "/workspace/myproject")
	cfg := config.HostExecConfig{
		Allow:   []string{"gcloud *"},
		Overlay: []string{"bin/gcloud"},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	_, err := b.ContainerSpec(context.Background(), "/host/myproject")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}

	// find overlay shim file
	entries, _ := os.ReadDir(filepath.Join(runBase, filepath.Base("/host/myproject")+"_"+strings.ReplaceAll("/host/myproject", "/", "_")))
	_ = entries

	// locate any hostexec-overlay dir under runBase
	var shimPath string
	_ = filepath.WalkDir(runBase, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Name() == "gcloud" && strings.Contains(path, OverlayDirName) {
			shimPath = path
		}
		return nil
	})
	if shimPath == "" {
		t.Fatal("overlay shim gcloud not written")
	}
	content, _ := os.ReadFile(shimPath)
	if !strings.Contains(string(content), "host-exec gcloud") {
		t.Errorf("shim content = %q, want host-exec gcloud", string(content))
	}
}

func TestContainerSpec_OverlayEmptyPath_Skipped(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/myproject")
	cfg := config.HostExecConfig{
		Allow:   []string{"gcloud *"},
		Overlay: []string{"", "bin/gcloud"},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/myproject")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 1 {
		t.Errorf("Mounts = %v, want 1 (empty path skipped)", spec.Mounts)
	}
}

func TestContainerSpec_OverlayAbsolutePath(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/myproject")
	cfg := config.HostExecConfig{
		Allow:   []string{"plastic *"},
		Overlay: []string{"/mnt/d/dev/SocialVR/UnrealEngine/.claude/skills/plasticscm/bin/plastic.exe"},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/myproject")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 1 {
		t.Fatalf("Mounts = %v, want 1", spec.Mounts)
	}
	if !strings.Contains(spec.Mounts[0], "target=/mnt/d/dev/SocialVR/UnrealEngine/.claude/skills/plasticscm/bin/plastic.exe") {
		t.Errorf("mount[0] = %q, want absolute target", spec.Mounts[0])
	}
}

func TestContainerSpec_OverlayParentRelative(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/proj")
	cfg := config.HostExecConfig{
		Allow:   []string{"foo *"},
		Overlay: []string{"../.claude/skills/foo/bin/foo"},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/proj")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 1 {
		t.Fatalf("Mounts = %v, want 1", spec.Mounts)
	}
	if !strings.Contains(spec.Mounts[0], "target=/workspace/.claude/skills/foo/bin/foo") {
		t.Errorf("mount[0] = %q, want target resolving via parent", spec.Mounts[0])
	}
}

func TestContainerSpec_OverlayAbsolute_NoWsDir(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "")
	cfg := config.HostExecConfig{
		Allow:   []string{"foo *"},
		Overlay: []string{"/opt/shims/foo"},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/proj")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 1 {
		t.Errorf("Mounts = %v, want 1 (absolute path ok without wsDir)", spec.Mounts)
	}
}

func TestContainerSpec_OverlayRelative_NoWsDir_Skipped(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "")
	cfg := config.HostExecConfig{
		Allow:   []string{"gcloud *"},
		Overlay: []string{"bin/gcloud"},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/myproject")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 0 {
		t.Errorf("Mounts = %v, want empty when wsDir is empty and path is relative", spec.Mounts)
	}
}

func TestContainerSpec_NoOverlay_NoMounts(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/myproject")
	cfg := config.HostExecConfig{Allow: []string{"gcloud *"}}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/myproject")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 0 {
		t.Errorf("Mounts = %v, want empty when no overlay configured", spec.Mounts)
	}
}

