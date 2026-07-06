package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

var (
	fakeAgentsMu  sync.Mutex
	fakeAgentsDir string
	fakeAgentsErr error
)

func buildFakeAgentsOnce(t *testing.T) string {
	t.Helper()
	fakeAgentsMu.Lock()
	defer fakeAgentsMu.Unlock()
	if fakeAgentsDir != "" || fakeAgentsErr != nil {
		if fakeAgentsErr != nil {
			t.Fatalf("fake agents binary unavailable: %v", fakeAgentsErr)
		}
		return fakeAgentsDir
	}

	dir, err := os.MkdirTemp("", "fake-agents-bin-")
	if err != nil {
		fakeAgentsErr = err
		t.Fatalf("mkdir tempdir: %v", err)
	}
	bin := filepath.Join(dir, "fake-agents")
	cmd := exec.Command("go", "build", "-o", bin, "./server/web/testsupport/fakeagents")
	cmd.Dir = moduleRoot(t)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fakeAgentsErr = err
		_ = os.RemoveAll(dir)
		t.Fatalf("go build fake-agents failed: %v\nstderr:\n%s", err, stderr.String())
	}
	for _, name := range []string{"claude", "codex"} {
		dst := filepath.Join(dir, name)
		data, err := os.ReadFile(bin)
		if err != nil {
			fakeAgentsErr = err
			t.Fatalf("read fake agents binary: %v", err)
		}
		if err := os.WriteFile(dst, data, 0o755); err != nil {
			fakeAgentsErr = err
			t.Fatalf("write fake agent %s: %v", name, err)
		}
	}
	fakeAgentsDir = dir
	return fakeAgentsDir
}

func installFakeAgents(t *testing.T) string {
	t.Helper()
	return buildFakeAgentsOnce(t)
}

func startScenarioServer(t *testing.T, fakeBinDir string) daemonInstance {
	t.Helper()
	pathEnv := fakeBinDir
	if oldPath := os.Getenv("PATH"); oldPath != "" {
		pathEnv += string(os.PathListSeparator) + oldPath
	}
	return startServerDaemonWithOptions(t, daemonLaunchOptions{
		addr:     reserveLoopbackAddr(t),
		extraEnv: []string{"PATH=" + pathEnv},
	})
}

func dialGatewayWS(t *testing.T, daemon daemonInstance, sessionID string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(daemon.httpURL, "http") + "/ws"
	if sessionID != "" {
		wsURL += "?session=" + sessionID
	}
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

func createSessionViaAPI(t *testing.T, daemon daemonInstance, project, command string) string {
	t.Helper()
	body := `{"project":"` + project + `","command":"` + command + `"}`
	req, err := http.NewRequest(http.MethodPost, daemon.httpURL+"/api/sessions", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusGatewayTimeout {
			if sessionID := waitForSessionIDByProject(t, daemon, project, command, 2*time.Second); sessionID != "" {
				return sessionID
			}
		}
		logTail, _ := os.ReadFile(filepath.Join(filepath.Dir(daemon.sockPath), "server.log"))
		t.Fatalf("create session status = %d, want 201 (body %q)\nserver.log:\n%s",
			resp.StatusCode, string(body), string(logTail))
	}
	var created apiSessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("create response returned empty session id")
	}
	return created.ID
}

func waitForSessionIDByProject(
	t *testing.T,
	daemon daemonInstance,
	project, command string,
	timeout time.Duration,
) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, daemon.httpURL+"/api/sessions", nil)
		if err != nil {
			t.Fatalf("new list request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			var sessions []apiSessionInfo
			if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&sessions) == nil {
				_ = resp.Body.Close()
				for _, session := range sessions {
					if session.Project == project && session.Command == command {
						return session.ID
					}
				}
			} else {
				_ = resp.Body.Close()
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ""
}

func listSessionsViaAPI(t *testing.T, daemon daemonInstance) []apiSessionInfo {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, daemon.httpURL+"/api/sessions", nil)
	if err != nil {
		t.Fatalf("new list request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("list sessions status = %d, want 200 (body %q)", resp.StatusCode, string(body))
	}
	var sessions []apiSessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	return sessions
}

func waitForSessionListed(
	t *testing.T,
	daemon daemonInstance,
	sessionID string,
	timeout time.Duration,
	wantPresent bool,
) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sessions := listSessionsViaAPI(t, daemon)
		found := false
		for _, session := range sessions {
			if session.ID == sessionID {
				found = true
				break
			}
		}
		if found == wantPresent {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for session %q presence=%v", sessionID, wantPresent)
}

func deleteSessionViaAPI(t *testing.T, daemon daemonInstance, sessionID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, daemon.httpURL+"/api/sessions/"+sessionID, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete session: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("delete session status = %d, want 200/204 (body %q)", resp.StatusCode, string(body))
	}
}

func sendSurfaceInput(t *testing.T, c *websocket.Conn, input string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	payload := `{"k":"i","d":` + strconvQuote(input) + `}`
	if err := c.Write(ctx, websocket.MessageText, []byte(payload)); err != nil {
		t.Fatalf("write surface input: %v", err)
	}
}

func readWSPayload(t *testing.T, c *websocket.Conn, timeout time.Duration) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read ws frame: %v", err)
	}
	return data
}

func tryReadWSPayload(c *websocket.Conn, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := c.Read(ctx)
	return data, err
}

func waitForSessionFrame(t *testing.T, c *websocket.Conn, sessionID string, pred func(map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		frame := readJSONFrame(t, c)
		session := sessionFromFrame(frame, sessionID)
		if session != nil && pred(session) {
			return frame
		}
	}
	t.Fatalf("timed out waiting for lifecycle frame for session %q", sessionID)
	return nil
}

func waitForSessionAbsent(t *testing.T, c *websocket.Conn, timeout time.Duration, sessionID string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		frame := readJSONFrame(t, c)
		if sessionFromFrame(frame, sessionID) == nil {
			return
		}
	}
	t.Fatalf("timed out waiting for session %q disappearance", sessionID)
}

func waitForOutputFrame(t *testing.T, c *websocket.Conn, timeout time.Duration) []any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data := readWSPayload(t, c, time.Until(deadline))
		var arr []any
		if err := json.Unmarshal(data, &arr); err == nil && len(arr) == 4 {
			return arr
		}
	}
	t.Fatal("timed out waiting for output frame")
	return nil
}

func decodeOutputFrameText(t *testing.T, frame []any) string {
	t.Helper()
	if len(frame) != 4 {
		t.Fatalf("output length = %d, want 4", len(frame))
	}
	dataB64, _ := frame[2].(string)
	if dataB64 == "" {
		t.Fatal("output frame missing base64 payload")
	}
	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		t.Fatalf("decode output payload: %v", err)
	}
	return string(data)
}

func collectSurfaceOutputOrLog(
	t *testing.T,
	daemon daemonInstance,
	c *websocket.Conn,
	timeout time.Duration,
	frames int,
) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var b strings.Builder
	for i := 0; i < frames && time.Now().Before(deadline); i++ {
		data, err := tryReadWSPayload(c, time.Until(deadline))
		if err != nil {
			logTail, _ := os.ReadFile(filepath.Join(filepath.Dir(daemon.sockPath), "server.log"))
			t.Fatalf("read ws frame: %v\nserver.log:\n%s", err, string(logTail))
		}
		var frame []any
		if err := json.Unmarshal(data, &frame); err != nil {
			t.Fatalf("unmarshal output frame %q: %v", data, err)
		}
		assertOutputFrameShapeFromFixture(t, frame)
		b.WriteString(decodeOutputFrameText(t, frame))
	}
	return b.String()
}

func sessionFromFrame(frame map[string]any, sessionID string) map[string]any {
	sessions, _ := frame["sessions"].([]any)
	for _, raw := range sessions {
		session, _ := raw.(map[string]any)
		if session["id"] == sessionID {
			return session
		}
	}
	return nil
}

func assertSessionView(t *testing.T, frame map[string]any, sessionID, status string) {
	t.Helper()
	session := sessionFromFrame(frame, sessionID)
	if session == nil {
		t.Fatalf("session %q missing from frame", sessionID)
	}
	view, _ := session["view"].(map[string]any)
	if view == nil {
		t.Fatalf("session %q missing view", sessionID)
	}
	card, _ := view["card"].(map[string]any)
	if card == nil {
		t.Fatalf("session %q missing view.card", sessionID)
	}
	if got, ok := card["title"]; ok && got != "" {
		t.Fatalf("title = %v, want empty/absent", got)
	}
	if got := view["status"]; got != status {
		t.Fatalf("status = %v, want %q", got, status)
	}
	if got := view["model"]; got != "claude-sonnet-4-5" {
		t.Fatalf("model = %v, want %q", got, "claude-sonnet-4-5")
	}
	if got := view["effort"]; got != "high" {
		t.Fatalf("effort = %v, want %q", got, "high")
	}
}

func assertOutputFrameShapeFromFixture(t *testing.T, frame []any) {
	t.Helper()
	fixture := loadFixtureJSON(t, "output.json")
	want, _ := fixture.([]any)
	if want == nil {
		t.Fatal("output fixture is not an array")
	}
	if len(frame) != len(want) {
		t.Fatalf("output length = %d, want %d", len(frame), len(want))
	}
	for i := range want {
		if jsonTypeName(want[i]) != jsonTypeName(frame[i]) {
			t.Fatalf("output[%d] type = %T, want %T", i, frame[i], want[i])
		}
	}
}

func loadFixtureJSON(t *testing.T, name string) any {
	t.Helper()
	path := filepath.Join("..", "..", "client", "web", "src", "wire", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
	return v
}

func jsonTypeName(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	default:
		return "other"
	}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestE2E_GatewayScenarioFakeClaudeLifecycleAndSurface(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon scenario e2e in -short mode")
	}
	t.Parallel()

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

func TestE2E_GatewayScenarioFakeCodexSurfaceAndSessionState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon scenario e2e in -short mode")
	}
	t.Parallel()

	daemon := startScenarioServer(t, installFakeAgents(t))
	project := t.TempDir()
	sessionID := createSessionViaAPI(t, daemon, project, "codex")
	waitForSessionListed(t, daemon, sessionID, 5*time.Second, true)

	surface := dialGatewayWS(t, daemon, sessionID)
	sendSurfaceInput(t, surface, "implement wire test\n")
	output := collectSurfaceOutputOrLog(t, daemon, surface, 10*time.Second, 6)
	if !strings.Contains(output, "implement wire test") {
		t.Fatalf("surface output missing prompt text: %q", output)
	}
	if !strings.Contains(output, "method=turn/started") {
		t.Fatalf("surface output missing turn/started event: %q", output)
	}
	if !strings.Contains(output, "[READY] threadId=") {
		t.Fatalf("surface output missing ready marker: %q", output)
	}

	deleteSessionViaAPI(t, daemon, sessionID)
	waitForSessionListed(t, daemon, sessionID, 5*time.Second, false)
}
