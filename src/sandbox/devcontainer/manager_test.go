package devcontainer

import (
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/sandbox"
	"github.com/takezoh/agent-roost/state"
)

func TestBuildLaunchCommand_subdir(t *testing.T) {
	const project = "/home/take/code/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		WorkspaceFolder: "/workspaces/myapp",
	}
	cs := &ContainerState{containerID: "abc123", spec: spec}

	got := translateWorkDir(project+"/backend", project, spec.workspaceTarget())
	want := "/workspaces/myapp/backend"
	if got != want {
		t.Errorf("workDir = %q, want %q", got, want)
	}
	_ = cs
}

func TestDevcontainerSpec_buildCreateArgs_defaults(t *testing.T) {
	spec := &DevcontainerSpec{
		ProjectPath:  "/workspace/myapp",
		ContainerEnv: map[string]string{"FOO": "bar"},
	}
	args := spec.BuildCreateArgs("roost-abc123:latest")

	mustContain := func(needle string) {
		t.Helper()
		for _, a := range args {
			if a == needle {
				return
			}
		}
		t.Errorf("args missing %q: %v", needle, args)
	}

	mustContain("roost-abc123:latest")
	mustContain("roost-managed=1")
	mustContain("roost-project=/workspace/myapp")
	mustContain("FOO=bar")
	// -w should set the workspace target as default cwd (replaces Dockerfile WORKDIR).
	mustContain("-w")
	mustContain("/workspaces/myapp")
	// workspace mount should be present as --mount arg value
	found := false
	for _, a := range args {
		if a == "type=bind,source=/workspace/myapp,target=/workspaces/myapp,consistency=cached" {
			found = true
		}
	}
	if !found {
		t.Errorf("workspace mount not found in args: %v", args)
	}
}

func TestBuildLaunchCommand_shellUsesLoginShell(t *testing.T) {
	const project = "/workspace/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		WorkspaceFolder: "/workspaces/myapp",
		RemoteUser:      "ubuntu",
	}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: project,
		Internal:    &ContainerState{containerID: "abc123", spec: spec},
	}
	m := &Manager{}
	plan := state.LaunchPlan{Project: project, StartDir: project, Command: "shell"}

	got, _, err := m.BuildLaunchCommand(inst, plan, nil)
	if err != nil {
		t.Fatalf("BuildLaunchCommand error: %v", err)
	}
	if strings.Contains(got, "/bin/bash") {
		t.Errorf("command must not hardcode /bin/bash: %s", got)
	}
	if !strings.Contains(got, "getent passwd") {
		t.Errorf("command must look up login shell via getent passwd: %s", got)
	}
	if !strings.Contains(got, "sh -c ") {
		t.Errorf("command must wrap snippet in sh -c: %s", got)
	}
}

func TestBuildLaunchCommand_RemoteEnv(t *testing.T) {
	const project = "/workspace/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		RemoteEnv:       map[string]string{"MY_VAR": "hello", "PATH": "/extra/bin:/usr/bin"},
		WorkspaceFolder: "/workspaces/myapp",
	}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: project,
		Internal:    &ContainerState{containerID: "ctr123", spec: spec},
	}
	m := &Manager{}
	plan := state.LaunchPlan{Project: project, StartDir: project, Command: "bash"}

	got, _, err := m.BuildLaunchCommand(inst, plan, nil)
	if err != nil {
		t.Fatalf("BuildLaunchCommand error: %v", err)
	}
	for _, want := range []string{"MY_VAR=hello", "PATH=/extra/bin:/usr/bin"} {
		if !strings.Contains(got, want) {
			t.Errorf("command missing %q: %s", want, got)
		}
	}
}

func TestResolveContainerEnvPlaceholders(t *testing.T) {
	imageEnv := map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/bin",
		"TERM": "xterm",
	}

	t.Run("containerEnv placeholder resolved from image", func(t *testing.T) {
		spec := &DevcontainerSpec{
			ContainerEnv: map[string]string{
				"PATH": "/home/ubuntu/.local/bin:${containerEnv:PATH}",
			},
			RemoteEnv: map[string]string{},
		}
		spec.ResolveContainerEnvPlaceholders(imageEnv)
		got := spec.ContainerEnv["PATH"]
		want := "/home/ubuntu/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/bin"
		if got != want {
			t.Errorf("ContainerEnv[PATH] = %q, want %q", got, want)
		}
	})

	t.Run("remoteEnv sees imageEnv union containerEnv (containerEnv wins)", func(t *testing.T) {
		spec := &DevcontainerSpec{
			ContainerEnv: map[string]string{
				"PATH": "/mise/shims:/usr/bin",
			},
			RemoteEnv: map[string]string{
				"PATH": "/extra:${containerEnv:PATH}",
				"TERM": "${containerEnv:TERM}",
			},
		}
		spec.ResolveContainerEnvPlaceholders(imageEnv)
		if got, want := spec.RemoteEnv["PATH"], "/extra:/mise/shims:/usr/bin"; got != want {
			t.Errorf("RemoteEnv[PATH] = %q, want %q", got, want)
		}
		if got, want := spec.RemoteEnv["TERM"], "xterm"; got != want {
			t.Errorf("RemoteEnv[TERM] = %q, want %q", got, want)
		}
	})

	t.Run("undefined var becomes empty string", func(t *testing.T) {
		spec := &DevcontainerSpec{
			ContainerEnv: map[string]string{"X": "${containerEnv:UNDEFINED}"},
			RemoteEnv:    map[string]string{},
		}
		spec.ResolveContainerEnvPlaceholders(imageEnv)
		if got := spec.ContainerEnv["X"]; got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("no placeholder is unchanged", func(t *testing.T) {
		spec := &DevcontainerSpec{
			ContainerEnv: map[string]string{"FOO": "bar"},
			RemoteEnv:    map[string]string{"BAZ": "qux"},
		}
		spec.ResolveContainerEnvPlaceholders(imageEnv)
		if spec.ContainerEnv["FOO"] != "bar" {
			t.Errorf("ContainerEnv[FOO] changed unexpectedly")
		}
		if spec.RemoteEnv["BAZ"] != "qux" {
			t.Errorf("RemoteEnv[BAZ] changed unexpectedly")
		}
	})
}

func TestDevcontainerSpec_effectiveUser(t *testing.T) {
	cases := []struct {
		name      string
		remote    string
		container string
		want      string
	}{
		{"both set", "vscode", "root", "vscode"},
		{"only container", "", "ubuntu", "ubuntu"},
		{"neither", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &DevcontainerSpec{RemoteUser: tc.remote, ContainerUser: tc.container}
			if got := s.EffectiveUser(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
