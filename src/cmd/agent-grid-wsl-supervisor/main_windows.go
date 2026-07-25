//go:build windows

package main

import (
	"errors"
	"fmt"

	"github.com/takezoh/agent-grid/platform/wslhost"
)

func runService(args []string) error {
	root := programDataRoot()
	if root == "" {
		return errors.New("ProgramData root is required")
	}
	acl := wslhost.NewOSACLApplier()
	store := &wslhost.FileConfigStore{Path: wslhost.ConfigPath(root), ACL: acl}
	snap, err := store.Load()
	if err != nil {
		return fmt.Errorf("load registration snapshot: %w", err)
	}
	sink, err := wslhost.OpenFileRecordSink(snap.LogPath, snap.WindowsIdentity, acl, wslhost.DefaultLogRetention())
	if err != nil {
		return fmt.Errorf("open lifecycle record sink: %w", err)
	}
	defer sink.Close()
	deps := wslhost.Deps{
		WSL: &wslhost.ExecWSLRunner{}, Host: &wslhost.ExecHostControl{Snap: snap},
		// The server's default TLS path uses a per-process self-signed localhost
		// certificate; this bound loopback probe verifies health semantics, not PKI.
		Health: &wslhost.HTTPHealthProber{InsecureSkipVerify: snap.TLSMode == wslhost.TLSModeTLS},
		Clock:  wslhost.RealClock{}, Timers: wslhost.RealScheduler{},
		Resume: wslhost.NewWindowsResumeSource(), Record: wslhost.NewSinkRecorder(snap, sink),
	}
	host, err := wslhost.NewServiceHost(snap, deps, nil, nil)
	if err != nil {
		return err
	}
	wslhost.BindWindowsStopWaiters(host.Stop)
	name := wslhost.WindowsServiceName
	if len(args) > 0 && args[0] != "" {
		name = args[0]
	}
	return wslhost.RunWindowsService(name, host)
}
