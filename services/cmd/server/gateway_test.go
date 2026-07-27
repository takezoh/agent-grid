package main

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartGatewayNotifiesReadiness(t *testing.T) {
	prev := notifyReady
	defer func() { notifyReady = prev }()

	called := make(chan struct{}, 1)
	notifyReady = func() error {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	df := &daemonFlagSet{
		addr:     "127.0.0.1:0",
		insecure: true,
		noAuth:   true,
	}
	h, err := startGateway(ctx, cancel, filepath.Join(t.TempDir(), "server.sock"), t.TempDir(), df, nil)
	if err != nil {
		t.Fatalf("startGateway() error = %v", err)
	}
	defer h.Close()

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for readiness notify")
	}
}

func TestStartGatewayKeepsRunningWhenNotifyFails(t *testing.T) {
	prev := notifyReady
	defer func() { notifyReady = prev }()

	notifyReady = func() error {
		return context.Canceled
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	df := &daemonFlagSet{
		addr:     "127.0.0.1:0",
		insecure: true,
		noAuth:   true,
	}
	h, err := startGateway(ctx, cancel, filepath.Join(t.TempDir(), "server.sock"), t.TempDir(), df, nil)
	if err != nil {
		t.Fatalf("startGateway() error = %v", err)
	}
	h.Close()
}

func TestStartGatewayWritesEffectiveStartupStatus(t *testing.T) {
	prev := notifyReady
	defer func() { notifyReady = prev }()
	notifyReady = func() error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout bytes.Buffer
	df := &daemonFlagSet{
		addr:     "127.0.0.1:0",
		insecure: true,
		token:    "must-not-leak",
	}
	h, err := startGateway(ctx, cancel, filepath.Join(t.TempDir(), "server.sock"), t.TempDir(), df, &stdout)
	if err != nil {
		t.Fatalf("startGateway() error = %v", err)
	}
	defer h.Close()

	got := stdout.String()
	for _, want := range []string{
		"server: listening address=127.0.0.1",
		"tls=false",
		"insecure=true",
		"auth=bearer-token",
		"url=http://127.0.0.1:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout = %q, want substring %q", got, want)
		}
	}
	if strings.Contains(got, "port=0 ") {
		t.Errorf("stdout = %q, want effective bound port instead of requested port 0", got)
	}
	if strings.Contains(got, df.token) {
		t.Errorf("stdout = %q, must not contain token without -print-token", got)
	}
}

func TestStartGatewayPrintsTokenOnlyWhenRequested(t *testing.T) {
	prev := notifyReady
	defer func() { notifyReady = prev }()
	notifyReady = func() error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout bytes.Buffer
	df := &daemonFlagSet{
		addr:       "127.0.0.1:0",
		insecure:   true,
		token:      "explicit-token",
		printToken: true,
	}
	h, err := startGateway(ctx, cancel, filepath.Join(t.TempDir(), "server.sock"), t.TempDir(), df, &stdout)
	if err != nil {
		t.Fatalf("startGateway() error = %v", err)
	}
	defer h.Close()

	if got := stdout.String(); !strings.Contains(got, "server: bearer token=explicit-token\n") {
		t.Errorf("stdout = %q, want explicitly requested token", got)
	}
}

func TestWriteStartupStatusDescribesTLSAndDisabledAuth(t *testing.T) {
	var stdout bytes.Buffer
	writeStartupStatus(
		&stdout,
		&net.TCPAddr{IP: net.ParseIP("::1"), Port: 8443},
		false,
		true,
		true,
		"must-not-leak",
	)

	got := stdout.String()
	for _, want := range []string{
		"port=8443",
		"tls=true",
		"insecure=false",
		"auth=disabled",
		"url=https://[::1]:8443",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout = %q, want substring %q", got, want)
		}
	}
	if strings.Contains(got, "must-not-leak") {
		t.Errorf("stdout = %q, must not print a token when auth is disabled", got)
	}
}
