package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/services/platform/wslhost"
)

type fixedDefaults struct {
	d   wslhost.Defaults
	err error
}

func (f fixedDefaults) Defaults() (wslhost.Defaults, error) { return f.d, f.err }

func TestRegisterCLIExplicitBindingDoesNotResolveElevatedUserDefaults(t *testing.T) {
	root := t.TempDir()
	err := runRegister([]string{
		"-program-data", root,
		"-linux-user", "agent",
		"-server-executable", "/usr/local/bin/server",
		"-data-dir", "/var/lib/agent-grid",
		"-windows-identity", `CONTOSO\agentgrid`,
		"-distribution", "Ubuntu-24.04",
	}, &bytes.Buffer{}, fixedDefaults{err: errors.New("administrator has no WSL default")},
		wslhost.StaticServiceGate{IsStopped: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRegisterCLIBindsDefaultsOnce(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	err := runRegister([]string{
		"-program-data", root,
		"-linux-user", "agent",
		"-server-executable", "/usr/local/bin/server",
		"-data-dir", "/var/lib/agent-grid",
		"-addr", "127.0.0.1:8443",
	}, &buf, fixedDefaults{d: wslhost.Defaults{
		WindowsIdentity: "CONTOSO\\alice",
		Distribution:    "Ubuntu",
	}}, wslhost.StaticServiceGate{IsStopped: true})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "identity=CONTOSO\\alice") || !strings.Contains(out, "distribution=Ubuntu") {
		t.Fatalf("output = %q", out)
	}
	// Runtime load uses only the snapshot file; drifted env defaults are ignored.
	t.Setenv("AG_WSL_DEFAULT_IDENTITY", "CONTOSO\\other")
	t.Setenv("AG_WSL_DEFAULT_DISTRIBUTION", "OtherDistro")
	store := &wslhost.FileConfigStore{Path: wslhost.ConfigPath(root)}
	snap, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.WindowsIdentity != "CONTOSO\\alice" || snap.Distribution != "Ubuntu" {
		t.Fatalf("runtime re-resolved defaults: %+v", snap)
	}
	// file exists at canonical path
	if _, err := os.Stat(filepath.Join(root, "AgentGrid", "wsl-host-supervisor.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterCLIRequiresStopped(t *testing.T) {
	root := t.TempDir()
	err := runRegister([]string{
		"-program-data", root,
		"-linux-user", "agent",
		"-server-executable", "/usr/local/bin/server",
		"-data-dir", "/var/lib/agent-grid",
		"-windows-identity", "A",
		"-distribution", "D",
	}, &bytes.Buffer{}, fixedDefaults{}, wslhost.StaticServiceGate{IsStopped: false})
	if err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultServiceGateFailsClosed(t *testing.T) {
	t.Setenv("AG_WSL_SERVICE_STOPPED", "")
	stopped, err := newServiceGate().Stopped()
	if err != nil {
		t.Fatal(err)
	}
	if stopped {
		t.Fatal("unset service state must not authorize registration")
	}
}

func TestFirstNonEmptyLinePreservesDistributionSpaces(t *testing.T) {
	got := firstNonEmptyLine("\r\nUbuntu 24.04\r\nDebian\r\n")
	if got != "Ubuntu 24.04" {
		t.Fatalf("distribution = %q", got)
	}
}
