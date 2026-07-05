package devcontainer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/takezoh/agent-reactor/platform/sandbox"
	"github.com/takezoh/agent-reactor/platform/sandbox/devcontainer/fakedocker"
)

func TestManagerLifecycle_FakeDocker(t *testing.T) {
	project := writeLifecycleSpec(t)
	recordPath, binDir := setupFakeDocker(t, fakedocker.Config{
		Responses: map[string][]fakedocker.Response{
			"ps":            {{Stdout: ""}},
			"image inspect": {{Stdout: "[]\n"}},
			"create":        {{Stdout: fakedocker.DefaultContainerID + "\n"}},
			"start":         {{}},
			"exec": {
				{},
				{Stdout: "postcreate ok\n"},
				{Stdout: "run ok\n"},
			},
			"rm": {{}},
		},
	})
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	m := New(nil)
	inst, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{})
	if err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}

	runCmd, _, err := m.BuildLaunchCommand(
		inst,
		sandbox.LaunchSpec{StartDir: filepath.Join(project, "backend"), Command: "printf run ok"},
		sandbox.FrameContext{Env: map[string]string{"FRAME_ENV": "frame"}},
		map[string]string{"TOKEN": "abc"},
	)
	if err != nil {
		t.Fatalf("BuildLaunchCommand: %v", err)
	}
	cmd := exec.Command("sh", "-c", runCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run launch command: %v\n%s", err, out)
	}
	if string(out) != "run ok\n" {
		t.Fatalf("launch output = %q, want %q", out, "run ok\n")
	}

	if err := m.DestroyInstance(context.Background(), inst); err != nil {
		t.Fatalf("DestroyInstance: %v", err)
	}

	records, err := fakedocker.ReadInvocations(recordPath)
	if err != nil {
		t.Fatalf("ReadInvocations: %v", err)
	}
	if got, want := subcommands(records), []string{
		"ps", "image inspect", "create", "start", "exec", "exec", "exec", "rm",
	}; !slices.Equal(got, want) {
		t.Fatalf("subcommands = %v, want %v", got, want)
	}

	assertCreateArgs(t, project, firstInvocation(t, records, "create").Args)
	assertPostCreateArgs(t, nthInvocation(t, records, "exec", 1).Args)
	assertRunArgs(t, nthInvocation(t, records, "exec", 2).Args)
	assertArgsEqual(t, firstInvocation(t, records, "rm").Args, []string{"rm", "-f", fakedocker.DefaultContainerID})
}

func TestManagerLifecycle_FakeDockerCreateFailurePropagates(t *testing.T) {
	project := writeLifecycleSpec(t)
	_, binDir := setupFakeDocker(t, fakedocker.Config{
		Responses: map[string][]fakedocker.Response{
			"ps":            {{Stdout: ""}},
			"image inspect": {{Stdout: "[]\n"}},
			"create":        {{ExitCode: 31, Stderr: "create exploded\n"}},
		},
	})
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := New(nil).EnsureInstance(context.Background(), project, "", sandbox.StartOptions{})
	if err == nil {
		t.Fatal("EnsureInstance succeeded, want error")
	}
	if !strings.Contains(err.Error(), "docker create") {
		t.Fatalf("EnsureInstance error = %v, want docker create failure", err)
	}
}

func TestManagerLifecycle_FakeDockerRunFailurePropagates(t *testing.T) {
	project := writeLifecycleSpec(t)
	_, binDir := setupFakeDocker(t, fakedocker.Config{
		Responses: map[string][]fakedocker.Response{
			"ps":            {{Stdout: ""}},
			"image inspect": {{Stdout: "[]\n"}},
			"create":        {{Stdout: fakedocker.DefaultContainerID + "\n"}},
			"start":         {{}},
			"exec": {
				{},
				{},
				{ExitCode: 42, Stderr: "container died\n"},
			},
		},
	})
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	m := New(nil)
	inst, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{})
	if err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	runCmd, _, err := m.BuildLaunchCommand(
		inst,
		sandbox.LaunchSpec{StartDir: filepath.Join(project, "backend"), Command: "printf boom"},
		sandbox.FrameContext{},
		nil,
	)
	if err != nil {
		t.Fatalf("BuildLaunchCommand: %v", err)
	}
	cmd := exec.Command("sh", "-c", runCmd)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("run launch command succeeded, want error")
	}
	if !strings.Contains(string(out), "container died") {
		t.Fatalf("launch stderr = %q, want container died", out)
	}
}

func setupFakeDocker(t *testing.T, cfg fakedocker.Config) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	recordPath := filepath.Join(tmp, "docker-records.jsonl")
	configPath := filepath.Join(tmp, "docker-config.json")
	if err := fakedocker.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	binPath := buildFakeDocker(t, filepath.Join(tmp, "docker"))
	t.Setenv(fakedocker.EnvConfigPath, configPath)
	t.Setenv(fakedocker.EnvRecordPath, recordPath)
	return recordPath, filepath.Dir(binPath)
}

func buildFakeDocker(t *testing.T, out string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pkgDir := filepath.Dir(file)
	cmd := exec.Command("go", "build", "-o", out, "./fakedocker/cmd/docker")
	cmd.Dir = pkgDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake docker: %v\n%s", err, out)
	}
	return out
}

func writeLifecycleSpec(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".devcontainer"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(project, "backend"), 0o755); err != nil {
		t.Fatalf("MkdirAll backend: %v", err)
	}
	spec := `{
  "image": "test:latest",
  "workspaceFolder": "/workspaces/app",
  "remoteUser": "ubuntu",
  "remoteEnv": {"REMOTE_ENV": "spec-remote"},
  "postCreateCommand": ["bash", "-lc", "echo postcreate"]
}`
	if err := os.WriteFile(filepath.Join(project, ".devcontainer", "devcontainer.json"), []byte(spec), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return project
}

func subcommands(records []fakedocker.Invocation) []string {
	out := make([]string, 0, len(records))
	for _, rec := range records {
		out = append(out, rec.Subcommand)
	}
	return out
}

func firstInvocation(t *testing.T, records []fakedocker.Invocation, subcommand string) fakedocker.Invocation {
	t.Helper()
	return nthInvocation(t, records, subcommand, 0)
}

func nthInvocation(t *testing.T, records []fakedocker.Invocation, subcommand string, index int) fakedocker.Invocation {
	t.Helper()
	seen := 0
	for _, rec := range records {
		if rec.Subcommand != subcommand {
			continue
		}
		if seen == index {
			return rec
		}
		seen++
	}
	t.Fatalf("invocation %q[%d] not found in %v", subcommand, index, subcommands(records))
	return fakedocker.Invocation{}
}

func assertCreateArgs(t *testing.T, project string, args []string) {
	t.Helper()
	for _, want := range []string{
		"--label", "reactor-managed=1",
		"reactor-project=" + project,
		"test:latest",
		"-w", "/workspaces/app",
	} {
		if !slices.Contains(args, want) {
			t.Fatalf("create args missing %q: %v", want, args)
		}
	}
	wantMount := "type=bind,source=" + project + ",target=/workspaces/app,consistency=cached"
	if !slices.Contains(args, wantMount) {
		t.Fatalf("create args missing workspace mount %q: %v", wantMount, args)
	}
	if !containsPrefix(args, "reactor-mount-hash=") {
		t.Fatalf("create args missing mount-hash label: %v", args)
	}
}

func assertPostCreateArgs(t *testing.T, args []string) {
	t.Helper()
	assertArgsEqual(t, args, []string{
		"exec", "-u", "ubuntu", fakedocker.DefaultContainerID, "bash", "-lc", "echo postcreate",
	})
}

func assertRunArgs(t *testing.T, args []string) {
	t.Helper()
	for _, want := range []string{
		"exec", "-i", "-u", "ubuntu", "-w", "/workspaces/app/backend",
		"-e", "REMOTE_ENV=spec-remote",
		"-e", "FRAME_ENV=frame",
		"-e", "TOKEN=abc",
		fakedocker.DefaultContainerID, "printf", "run", "ok",
	} {
		if !slices.Contains(args, want) {
			t.Fatalf("run args missing %q: %v", want, args)
		}
	}
}

func assertArgsEqual(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func containsPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}
