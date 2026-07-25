package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/takezoh/agent-grid/platform/wslhost"
)

// programDataRoot resolves %ProgramData% or AG_PROGRAMDATA override (tests).
func programDataRoot() string {
	if v := strings.TrimSpace(os.Getenv("AG_PROGRAMDATA")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("ProgramData")); v != "" {
		return v
	}
	return ""
}

// defaultSource is injected for registration-time identity/distro resolution.
type defaultSource interface {
	Defaults() (wslhost.Defaults, error)
}

func runRegister(args []string, stdout io.Writer, src defaultSource, gate wslhost.ServiceGate) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		identity, distro, linuxUser, server, dataDir, addr, tlsMode, health, logPath, programData string
	)
	fs.StringVar(&identity, "windows-identity", "", "Windows service identity (default: registration-time login)")
	fs.StringVar(&distro, "distribution", "", "WSL distribution (default: registration-time wsl default)")
	fs.StringVar(&linuxUser, "linux-user", "", "Linux user inside the distribution")
	fs.StringVar(&server, "server-executable", "", "server path inside WSL")
	fs.StringVar(&dataDir, "data-dir", "", "server -data-dir")
	fs.StringVar(&addr, "addr", "127.0.0.1:8443", "server -addr")
	fs.StringVar(&tlsMode, "tls-mode", wslhost.TLSModeTLS, "tls|insecure")
	fs.StringVar(&health, "health-url", "", "optional health URL override")
	fs.StringVar(&logPath, "log-path", "", "optional log path override")
	fs.StringVar(&programData, "program-data", "", "ProgramData root (default: env)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := strings.TrimSpace(programData)
	if root == "" {
		root = programDataRoot()
	}
	if root == "" {
		return errors.New("ProgramData root is required (set ProgramData or -program-data)")
	}
	defaults, err := src.Defaults()
	if err != nil {
		return err
	}
	acl := wslhost.NewOSACLApplier()
	store := &wslhost.FileConfigStore{Path: wslhost.ConfigPath(root), ACL: acl}
	reg := &wslhost.Registrar{
		Store:           store,
		ACL:             acl,
		Gate:            gate,
		ProgramDataRoot: root,
		Now:             func() time.Time { return time.Now().UTC() },
	}
	snap, err := reg.Register(wslhost.RegistrationRequest{
		WindowsIdentity:  identity,
		Distribution:     distro,
		LinuxUser:        linuxUser,
		ServerExecutable: server,
		DataDir:          dataDir,
		Addr:             addr,
		TLSMode:          tlsMode,
		HealthURL:        health,
		LogPath:          logPath,
	}, defaults)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "registered generation=%s identity=%s distribution=%s\n",
		snap.Generation, snap.WindowsIdentity, snap.Distribution)
	return err
}

func runApplyCheck(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("apply-check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var programData string
	fs.StringVar(&programData, "program-data", "", "ProgramData root")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := strings.TrimSpace(programData)
	if root == "" {
		root = programDataRoot()
	}
	if root == "" {
		return errors.New("ProgramData root is required")
	}
	store := &wslhost.FileConfigStore{Path: wslhost.ConfigPath(root)}
	snap, err := store.Load()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "ok generation=%s schema=%d\n", snap.Generation, snap.SchemaVersion)
	return err
}

func runPrintRuntime(args []string, stdout io.Writer) error {
	return runApplyCheck(args, stdout)
}
