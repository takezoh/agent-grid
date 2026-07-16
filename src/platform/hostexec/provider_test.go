package hostexec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/platform/appid"
	"github.com/takezoh/agent-grid/platform/config"
)

// TestProvider_DoesNotContributePATH pins the case-D regression guard: neither
// the empty-config nor the fully-configured ContainerSpec return value may set
// Env["PATH"]. framelaunch is the sole owner of the runtime PATH invariant
// (adr-20260716-framelaunch-runtime-path-owner). A regression that re-adds
// Env["PATH"] here would restore the vestigial 3-stage relay
// (provider → origPath → framelaunch) that case D eliminated.
func TestProvider_DoesNotContributePATH(t *testing.T) {
	t.Run("no config → empty spec still has no PATH", func(t *testing.T) {
		b, _ := newTestSpecBuilder(t, "/workspace/proj")
		b.cfgFor = func(string) config.HostExecConfig { return config.HostExecConfig{} }
		spec, err := b.ContainerSpec(context.Background(), "/host/proj")
		if err != nil {
			t.Fatalf("ContainerSpec: %v", err)
		}
		if _, ok := spec.Env["PATH"]; ok {
			t.Fatalf("case-D invariant: spec.Env[PATH] = %q, want absent (empty-config path)", spec.Env["PATH"])
		}
	})
	t.Run("full config → non-empty spec has no PATH", func(t *testing.T) {
		b, _ := newTestSpecBuilder(t, "/workspace/proj")
		b.cfgFor = func(string) config.HostExecConfig {
			return config.HostExecConfig{
				Allow:   []string{"gcloud *"},
				Overlay: []config.OverlayEntry{{Target: "bin/gcloud"}},
			}
		}
		spec, err := b.ContainerSpec(context.Background(), "/host/proj")
		if err != nil {
			t.Fatalf("ContainerSpec: %v", err)
		}
		if _, ok := spec.Env["PATH"]; ok {
			t.Fatalf("case-D invariant: spec.Env[PATH] = %q, want absent (full-config path)", spec.Env["PATH"])
		}
	})
}

// TestSpecBuilder_ContainerRunDirSSOT_Panic pins adr-20260716-provider-shim-root-appid-ssot:
// constructing a SpecBuilder with a container run dir that diverges from
// appid.ContainerRunDir MUST panic. Silent divergence would let a mis-configured
// provider write shims to a directory that framelaunch's
// appid.RuntimeAuthoritativePathList() never covers, reproducing the RCA
// bypass while both provider and framelaunch report "healthy".
func TestSpecBuilder_ContainerRunDirSSOT_Panic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on mismatched ContainerRunDir; got no panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is not string: %T", r)
		}
		if !strings.Contains(msg, "adr-20260716-provider-shim-root-appid-ssot") {
			t.Errorf("panic message %q does not name the ADR", msg)
		}
		if !strings.Contains(msg, "/opt/wrong/run") {
			t.Errorf("panic message %q does not name the offending value", msg)
		}
	}()
	_ = NewSpecBuilder(context.Background(),
		Config{
			RunBase:          t.TempDir(),
			ContainerRunDir:  "/opt/wrong/run",
			ContainerBinPath: "/opt/wrong/run/bridge",
		},
		func(string) config.HostExecConfig { return config.HostExecConfig{} })
}

// TestSpecBuilder_ContainerRunDirSSOT_Matched confirms the matched case is a
// no-op: construction succeeds when cfg.ContainerRunDir == appid.ContainerRunDir.
func TestSpecBuilder_ContainerRunDirSSOT_Matched(t *testing.T) {
	b := NewSpecBuilder(context.Background(),
		Config{
			RunBase:          t.TempDir(),
			ContainerRunDir:  appid.ContainerRunDir,
			ContainerBinPath: appid.ContainerBinaryPath,
		},
		func(string) config.HostExecConfig { return config.HostExecConfig{} })
	if b == nil {
		t.Fatal("NewSpecBuilder returned nil")
	}
}

// TestShimDirName_MatchesAppidSSOT pins the alias contract between hostexec
// and appid. If someone re-hardcodes hostexec.ShimDirName to a literal, this
// test flags the SSOT drift.
func TestShimDirName_MatchesAppidSSOT(t *testing.T) {
	if ShimDirName != appid.HostExecShimsDir {
		t.Fatalf("hostexec.ShimDirName = %q, want %q (appid.HostExecShimsDir SSOT)", ShimDirName, appid.HostExecShimsDir)
	}
}

func newTestSpecBuilder(t *testing.T, wsDir string) (*SpecBuilder, string) {
	t.Helper()
	runBase := t.TempDir()
	b := NewSpecBuilder(context.Background(), Config{
		RunBase:            runBase,
		ContainerRunDir:    "/opt/agent-grid/run",
		ContainerBinPath:   "/opt/agent-grid/bin/roost",
		WorkspaceFolderFor: func(string) string { return wsDir },
	}, func(string) config.HostExecConfig { return config.HostExecConfig{} })
	return b, runBase
}

func TestContainerSpec_OverlayMounts(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/myproject")
	cfg := config.HostExecConfig{
		Allow: []string{"gcloud *"},
		Overlay: []config.OverlayEntry{
			{Target: "bin/gcloud"},
			{Target: "tools/gcloud"},
		},
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
	dst := "/workspace/myproject/bin/gcloud"
	cfg := config.HostExecConfig{
		Allow:   []string{"gcloud *"},
		Overlay: []config.OverlayEntry{{Target: "bin/gcloud"}},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	_, err := b.ContainerSpec(context.Background(), "/host/myproject")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}

	alias := OverlayAlias(dst)
	var shimPath string
	_ = filepath.WalkDir(runBase, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Name() == alias && strings.Contains(path, OverlayDirName) {
			shimPath = path
		}
		return nil
	})
	if shimPath == "" {
		t.Fatalf("overlay shim %q not written", alias)
	}
	content, _ := os.ReadFile(shimPath)
	if !strings.Contains(string(content), "host-exec "+alias) {
		t.Errorf("shim content = %q, want host-exec %s", string(content), alias)
	}
}

func TestContainerSpec_OverlayEmptyTarget_Skipped(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/myproject")
	cfg := config.HostExecConfig{
		Allow:   []string{"gcloud *"},
		Overlay: []config.OverlayEntry{{Target: ""}, {Target: "bin/gcloud"}},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/myproject")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 1 {
		t.Errorf("Mounts = %v, want 1 (empty target skipped)", spec.Mounts)
	}
}

func TestContainerSpec_OverlayAbsolutePath(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/myproject")
	cfg := config.HostExecConfig{
		Overlay: []config.OverlayEntry{{Target: "/mnt/d/dev/SocialVR/UnrealEngine/.claude/skills/plasticscm/bin/plastic.exe"}},
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
		Overlay: []config.OverlayEntry{{Target: "../.claude/skills/foo/bin/foo"}},
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
		Overlay: []config.OverlayEntry{{Target: "/opt/shims/foo"}},
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
		Overlay: []config.OverlayEntry{{Target: "bin/gcloud"}},
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

// TestContainerSpec_OverlayRelative_SharedSentinel_Skipped covers the shared
// container: projectPath is the non-absolute "__shared__" sentinel, so a
// relative overlay target must be skipped rather than producing a relative mount
// target that docker create rejects.
func TestContainerSpec_OverlayRelative_SharedSentinel_Skipped(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "__shared__")
	cfg := config.HostExecConfig{
		Overlay: []config.OverlayEntry{{Target: "bin/gcloud"}},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "__shared__")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 0 {
		t.Errorf("Mounts = %v, want empty when wsDir is the non-absolute sentinel", spec.Mounts)
	}
}

func TestContainerSpec_OverlayOnlyNoAllow(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/proj")
	cfg := config.HostExecConfig{
		Overlay: []config.OverlayEntry{{Target: "/opt/tools/plastic.exe"}},
	}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/proj")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 1 {
		t.Errorf("Mounts = %v, want 1 (overlay without global allow)", spec.Mounts)
	}
}

func TestContainerSpec_NoOverlay_NoMounts(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "/workspace/myproject")
	cfg := config.HostExecConfig{Allow: []string{"gcloud *"}, Overlay: nil}
	b.cfgFor = func(string) config.HostExecConfig { return cfg }

	spec, err := b.ContainerSpec(context.Background(), "/host/myproject")
	if err != nil {
		t.Fatalf("ContainerSpec: %v", err)
	}
	if len(spec.Mounts) != 0 {
		t.Errorf("Mounts = %v, want empty when no overlay configured", spec.Mounts)
	}
}

func TestSpecBuilderRefreshesEntries(t *testing.T) {
	b, _ := newTestSpecBuilder(t, "")
	allow1 := []string{"op.exe *"}
	allow2 := []string{"op.exe *", "cm.exe *"}
	call := 0
	b.cfgFor = func(string) config.HostExecConfig {
		call++
		if call == 1 {
			return config.HostExecConfig{Allow: allow1}
		}
		return config.HostExecConfig{Allow: allow2}
	}

	if _, err := b.ContainerSpec(context.Background(), "/host/proj"); err != nil {
		t.Fatalf("first ContainerSpec: %v", err)
	}
	if _, err := b.ContainerSpec(context.Background(), "/host/proj"); err != nil {
		t.Fatalf("second ContainerSpec: %v", err)
	}

	b.mu.Lock()
	br := b.brokers["/host/proj"]
	b.mu.Unlock()

	if _, ok := br.loadEntries()["op.exe"]; !ok {
		t.Error("op.exe should be in entries after refresh")
	}
	if _, ok := br.loadEntries()["cm.exe"]; !ok {
		t.Error("cm.exe should be in entries after second ContainerSpec")
	}
}
