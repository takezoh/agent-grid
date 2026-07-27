package webhost

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-grid/clients/web-host/internal/clientconfig"
)

func TestProjectionContainsFleetWithoutCredentials(t *testing.T) {
	snapshot := testSnapshot("http://127.0.0.1:1", "secret")
	handler, err := Handler(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/host/config", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Fatal("credential leaked in host projection")
	}
	var projection clientconfig.Projection
	if err := json.Unmarshal(recorder.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	if len(projection.Servers) != 1 || projection.Servers[0].ServerID != snapshot.Servers[0].ID {
		t.Fatalf("projection = %#v", projection)
	}
}

func TestQualifiedProxySelectsServerAndInjectsConfiguredCredential(t *testing.T) {
	var gotAuth, gotOrigin, gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotOrigin = r.Header.Get("Origin")
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()
	snapshot := testSnapshot(upstream.URL, "configured-token")
	handler, err := Handler(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	path := serverPath(snapshot, "/api/sessions")
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer browser-token")
	request.Header.Set("Origin", "http://web.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/api/sessions" {
		t.Fatalf("upstream path = %q", gotPath)
	}
	if gotAuth != "Bearer configured-token" {
		t.Fatalf("upstream auth = %q", gotAuth)
	}
	if gotOrigin != "" {
		t.Fatalf("upstream origin = %q", gotOrigin)
	}
}

func TestUnqualifiedAndUnknownRoutesDoNotSelectDefaultServer(t *testing.T) {
	snapshot := testSnapshot("http://127.0.0.1:1", "")
	handler, err := Handler(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/sessions",
		"/ws",
		"/host/servers/unknown/api/sessions",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, recorder.Code)
		}
	}
}

func TestQualifiedProxyDialRefusedMaps503WithRetryAfter(t *testing.T) {
	snapshot := testSnapshot("http://127.0.0.1:1", "")
	handler, err := Handler(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, serverPath(snapshot, "/api/session-config"), nil),
	)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", recorder.Header().Get("Retry-After"))
	}
}

func TestQualifiedProxyRejectsRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://other.example")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()
	snapshot := testSnapshot(upstream.URL, "")
	handler, err := Handler(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, serverPath(snapshot, "/api/sessions"), nil),
	)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", recorder.Code)
	}
}

func TestQualifiedWebSocketOriginAndRoundTrip(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = connection.CloseNow() }()
		kind, data, err := connection.Read(r.Context())
		if err == nil {
			_ = connection.Write(r.Context(), kind, data)
		}
	}))
	defer upstream.Close()
	snapshot := testSnapshot(upstream.URL, "")
	handler, err := Handler(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	wsPath := serverPath(snapshot, "/ws") + "?ticket=x"
	foreign, _ := http.NewRequest(http.MethodGet, server.URL+wsPath, nil)
	foreign.Header.Set("Origin", "https://evil.example")
	response, err := http.DefaultClient.Do(foreign)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin status = %d", response.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + wsPath
	connection, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {server.URL}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.CloseNow() }()
	if err := connection.Write(ctx, websocket.MessageText, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	_, data, err := connection.Read(ctx)
	if err != nil || string(data) != "ping" {
		t.Fatalf("round trip data=%q err=%v", data, err)
	}
}

func TestStaticServesShellAndSuppressesDirectoryListing(t *testing.T) {
	handler, err := Handler(testSnapshot("http://127.0.0.1:1", ""))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	if code, body := get(t, server.URL+"/"); code != http.StatusOK || !strings.Contains(body, "<html") {
		t.Fatalf("root status=%d body=%q", code, body)
	}
	if code, _ := get(t, server.URL+"/assets/"); code != http.StatusNotFound {
		t.Fatalf("assets directory status=%d", code)
	}
}

func testSnapshot(rawURL, token string) clientconfig.Snapshot {
	auth := clientconfig.Auth{Mode: "none"}
	if token != "" {
		auth = clientconfig.Auth{Mode: "bearer", Token: token}
	}
	return clientconfig.Snapshot{Servers: []clientconfig.Server{{
		ID:          clientconfig.ServerID(rawURL),
		DisplayName: "test",
		Enabled:     true,
		URL:         rawURL,
		Auth:        auth,
	}}}
}

func serverPath(snapshot clientconfig.Snapshot, suffix string) string {
	return serverRoutePrefix + snapshot.Servers[0].ID + suffix
}

func get(t *testing.T, rawURL string) (int, string) {
	t.Helper()
	response, err := http.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(body)
}
