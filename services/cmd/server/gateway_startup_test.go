package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogStartup_NoAuthNonLoopbackWarns(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	logStartup("0.0.0.0:8443", true, true, "/tmp/agent-grid.sock")

	got := logs.String()
	for _, want := range []string{
		"level=WARN",
		"-no-auth on NON-LOOPBACK bind",
		"REST/WS surface is reachable from the network",
		"addr=0.0.0.0:8443",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("startup logs = %q, want substring %q", got, want)
		}
	}
}
