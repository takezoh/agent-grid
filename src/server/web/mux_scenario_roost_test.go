package web

import (
	"testing"
	"time"
)

func TestE2E_GatewayScenarioFakeClaudeLifecycleAndSurface_SanitizesAmbientRoostEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon scenario e2e in -short mode")
	}

	t.Setenv("AG_SOCKET_TOKEN", "ambient-token")
	t.Setenv("AG_SOCKET", "/tmp/ambient.sock")
	t.Setenv("AG_FRAME_ID", "ambient-frame")

	daemon := startScenarioServer(t, installFakeAgents(t))
	lifecycle := dialGatewayWS(t, daemon, "")
	readJSONFrame(t, lifecycle) // initial empty hello

	project := t.TempDir()
	sessionID := createSessionViaAPI(t, daemon, project, "claude")
	initial := waitForSessionFrame(t, lifecycle, sessionID, func(session map[string]any) bool {
		view, _ := session["view"].(map[string]any)
		return view["model"] == "claude-sonnet-4-5" && view["effort"] == "high"
	})
	assertSessionView(t, initial, sessionID, "idle")

	surface := dialGatewayWS(t, daemon, sessionID)
	output := waitForOutputFrame(t, surface, 5*time.Second)
	assertOutputFrameShapeFromFixture(t, output)

	sendSurfaceInput(t, surface, "summarize this\n")
	running := waitForSessionFrame(t, lifecycle, sessionID, func(session map[string]any) bool {
		view, _ := session["view"].(map[string]any)
		return view["status"] == "running"
	})
	assertSessionView(t, running, sessionID, "running")

	waiting := waitForSessionFrame(t, lifecycle, sessionID, func(session map[string]any) bool {
		view, _ := session["view"].(map[string]any)
		return view["status"] == "waiting"
	})
	assertSessionView(t, waiting, sessionID, "waiting")

	deleteSessionViaAPI(t, daemon, sessionID)
	waitForSessionAbsent(t, lifecycle, 5*time.Second, sessionID)
}
