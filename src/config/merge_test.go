package config

import (
	"testing"
)

func TestMergeSandbox_NilProject(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer", Devcontainer: DevcontainerConfig{CLIPath: "devcontainer"}}
	got := MergeSandbox(user, nil)
	if got.Mode != "devcontainer" {
		t.Errorf("Mode = %q, want devcontainer", got.Mode)
	}
	if got.Devcontainer.CLIPath != "devcontainer" {
		t.Errorf("CLIPath = %q, want devcontainer", got.Devcontainer.CLIPath)
	}
}

func TestMergeSandbox_ModeOverride(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	project := &SandboxConfig{Mode: "direct"}
	got := MergeSandbox(user, project)
	if got.Mode != "direct" {
		t.Errorf("Mode = %q, want direct (project wins)", got.Mode)
	}
}

func TestMergeSandbox_ModeEmpty_UserWins(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	project := &SandboxConfig{} // no mode set
	got := MergeSandbox(user, project)
	if got.Mode != "devcontainer" {
		t.Errorf("Mode = %q, want devcontainer (project empty, user wins)", got.Mode)
	}
}

func TestMergeSandbox_CLIPathOverride(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{CLIPath: "devcontainer"}}
	project := &SandboxConfig{Devcontainer: DevcontainerConfig{CLIPath: "/usr/local/bin/devcontainer"}}
	got := MergeSandbox(user, project)
	if got.Devcontainer.CLIPath != "/usr/local/bin/devcontainer" {
		t.Errorf("CLIPath = %q, want /usr/local/bin/devcontainer (project wins)", got.Devcontainer.CLIPath)
	}
}

func TestMergeSandbox_CLIPathEmpty_UserWins(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{CLIPath: "devcontainer"}}
	project := &SandboxConfig{}
	got := MergeSandbox(user, project)
	if got.Devcontainer.CLIPath != "devcontainer" {
		t.Errorf("CLIPath = %q, want devcontainer (project empty, user wins)", got.Devcontainer.CLIPath)
	}
}

func TestMergeSandbox_ExtraBuildArgsConcat(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{ExtraBuildArgs: []string{"--remove-existing-container"}}}
	project := &SandboxConfig{Devcontainer: DevcontainerConfig{ExtraBuildArgs: []string{"--skip-non-blocking-commands"}}}
	got := MergeSandbox(user, project)
	if len(got.Devcontainer.ExtraBuildArgs) != 2 {
		t.Errorf("ExtraBuildArgs = %v, want 2 items (user+project concat)", got.Devcontainer.ExtraBuildArgs)
	}
}

func TestMergeSandbox_ExtraBuildArgs_ProjectEmpty(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{ExtraBuildArgs: []string{"--remove-existing-container"}}}
	project := &SandboxConfig{}
	got := MergeSandbox(user, project)
	if len(got.Devcontainer.ExtraBuildArgs) != 1 {
		t.Errorf("ExtraBuildArgs = %v, want 1 item (user only)", got.Devcontainer.ExtraBuildArgs)
	}
}

func TestMergeSandbox_EnvScriptOverride(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{EnvScript: "~/bin/roost-env.sh"}}
	project := &SandboxConfig{Devcontainer: DevcontainerConfig{EnvScript: "./local-env.sh"}}
	got := MergeSandbox(user, project)
	if got.Devcontainer.EnvScript != "./local-env.sh" {
		t.Errorf("EnvScript = %q, want ./local-env.sh (project wins)", got.Devcontainer.EnvScript)
	}
}

func TestMergeSandbox_ProxyNilProject(t *testing.T) {
	user := SandboxConfig{Proxy: ProxyConfig{Enabled: true}}
	got := MergeSandbox(user, nil)
	if !got.Proxy.Enabled {
		t.Errorf("proxy config lost on nil project: %+v", got.Proxy)
	}
}

func TestMergeSandbox_DoesNotMutateInput(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{ExtraBuildArgs: []string{"--a"}}}
	project := &SandboxConfig{Devcontainer: DevcontainerConfig{ExtraBuildArgs: []string{"--b"}}}
	got := MergeSandbox(user, project)
	got.Devcontainer.ExtraBuildArgs = append(got.Devcontainer.ExtraBuildArgs, "--c")
	if len(user.Devcontainer.ExtraBuildArgs) != 1 {
		t.Errorf("user ExtraBuildArgs mutated: %v", user.Devcontainer.ExtraBuildArgs)
	}
	if len(project.Devcontainer.ExtraBuildArgs) != 1 {
		t.Errorf("project ExtraBuildArgs mutated: %v", project.Devcontainer.ExtraBuildArgs)
	}
}
