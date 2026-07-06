package dockercli

import (
	"os"
	"path/filepath"
	"testing"
)

func installFakeDocker(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("PATH", dir)
}

func TestSetup_MissingDocker(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	_, err := Setup(func(string) bool { return false })
	if err == nil {
		t.Fatal("Setup() error = nil, want missing docker error")
	}
}

func TestSetup_RootlessSocketDetected(t *testing.T) {
	installFakeDocker(t)
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	got, err := Setup(func(path string) bool {
		return path == "/run/user/1000/docker.sock"
	})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if !got.UsingRootless {
		t.Fatal("Setup() UsingRootless = false, want true")
	}
	if got.DOCKERHOST != "unix:///run/user/1000/docker.sock" {
		t.Fatalf("Setup() DOCKERHOST = %q, want rootless socket", got.DOCKERHOST)
	}
	if got.UsingDefaultSock {
		t.Fatal("Setup() UsingDefaultSock = true, want false")
	}
	if env := os.Getenv("DOCKER_HOST"); env != got.DOCKERHOST {
		t.Fatalf("DOCKER_HOST = %q, want %q", env, got.DOCKERHOST)
	}
}

func TestSetup_ExplicitDockerHostPreserved(t *testing.T) {
	installFakeDocker(t)
	t.Setenv("DOCKER_HOST", "tcp://remote:2376")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	got, err := Setup(func(string) bool {
		t.Fatal("socketExists should not be called when DOCKER_HOST is already set")
		return false
	})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if got.UsingRootless {
		t.Fatal("Setup() UsingRootless = true, want false")
	}
	if got.UsingDefaultSock {
		t.Fatal("Setup() UsingDefaultSock = true, want false")
	}
	if got.DOCKERHOST != "" {
		t.Fatalf("Setup() DOCKERHOST = %q, want empty", got.DOCKERHOST)
	}
	if env := os.Getenv("DOCKER_HOST"); env != "tcp://remote:2376" {
		t.Fatalf("DOCKER_HOST = %q, want preserved explicit host", env)
	}
}
