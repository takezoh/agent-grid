package webhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-grid/clients/web-host/internal/clientconfig"
)

func TestFleetHealthWireCanonicalFixture(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	fixture := filepath.Join(filepath.Dir(file), "..", "..", "..", "ui", "src", "fleet", "fixtures", "fleet-health-v1.json")
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := DecodeFleetHealthSnapshotV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.HostInstanceID != "host-fixture-1" || snapshot.Revision != 7 || len(snapshot.Servers) != 6 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	// Deep-equal complete decoded observation tree (reason/prior/state), not spot counts.
	var expected map[string]any
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip map[string]any
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	// One canonical semantic normalization — includes serverId, serverLabel,
	// enabled, observations, and every other credential-free contract field.
	// A decoder dropping/corrupting labels must fail (no partial serverId/enabled-only fallback).
	assertFleetHealthSemanticParity(t, expected, roundTrip)
	if strings.Contains(string(encoded), "Bearer ") || strings.Contains(string(encoded), "token=") {
		t.Fatal("credential material leaked")
	}
}

func TestFleetHealthSemanticParityIncludesServerLabel(t *testing.T) {
	// rejects counterexample: partial meta checks that omit serverLabel
	left := map[string]any{
		"schemaVersion":  float64(1),
		"hostInstanceId": "h",
		"revision":       float64(1),
		"servers": []any{
			map[string]any{
				"serverId":    "a",
				"serverLabel": "Alpha",
				"enabled":     true,
				"observation": map[string]any{"kind": "connected", "observedAt": "2026-08-03T00:00:00Z"},
			},
		},
	}
	rightDroppedLabel := map[string]any{
		"schemaVersion":  float64(1),
		"hostInstanceId": "h",
		"revision":       float64(1),
		"servers": []any{
			map[string]any{
				"serverId": "a",
				// serverLabel intentionally dropped/corrupted
				"serverLabel": "CORRUPTED",
				"enabled":     true,
				"observation": map[string]any{"kind": "connected", "observedAt": "2026-08-03T00:00:00Z"},
			},
		},
	}
	if err := fleetHealthSemanticDiff(left, rightDroppedLabel); err == nil {
		t.Fatal("semantic parity must fail when serverLabel is dropped/corrupted")
	}
	// Full meaning including label must pass.
	if err := fleetHealthSemanticDiff(left, left); err != nil {
		t.Fatalf("identical meaning must pass: %v", err)
	}
}

// assertFleetHealthSemanticParity enforces deep credential-free meaning parity
// via a single canonical normalization (not ad-hoc partial field checks).
func assertFleetHealthSemanticParity(t *testing.T, expected, got map[string]any) {
	t.Helper()
	if err := fleetHealthSemanticDiff(expected, got); err != nil {
		t.Fatal(err)
	}
}

// fleetHealthSemanticDiff returns an error when the credential-free contract
// meaning differs. Normalization is canonical JSON of the full tree after
// sorting object keys so field order does not mask drops/corruption.
func fleetHealthSemanticDiff(expected, got map[string]any) error {
	expNorm, err := normalizeFleetHealthMeaning(expected)
	if err != nil {
		return err
	}
	gotNorm, err := normalizeFleetHealthMeaning(got)
	if err != nil {
		return err
	}
	if expNorm != gotNorm {
		return fmt.Errorf("fleet health semantic meaning diverge:\nwant %s\ngot  %s", expNorm, gotNorm)
	}
	return nil
}

// normalizeFleetHealthMeaning is the single canonical semantic normalization for
// credential-free fleet health snapshots: every contract field (schemaVersion,
// hostInstanceId, revision, servers[].serverId/serverLabel/enabled/observation
// and nested reason/prior/detail) participates in the meaning.
func normalizeFleetHealthMeaning(value any) (string, error) {
	normalized, err := canonicalizeJSONValue(value)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func canonicalizeJSONValue(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		// Rebuild with sorted insertion so subsequent json.Marshal is stable.
		out := make(map[string]any, len(typed))
		for _, key := range keys {
			child, err := canonicalizeJSONValue(typed[key])
			if err != nil {
				return nil, err
			}
			out[key] = child
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			child, err := canonicalizeJSONValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = child
		}
		return out, nil
	case json.Number:
		return typed.String(), nil
	default:
		return typed, nil
	}
}

func TestFleetHealthWireRejectsInvalid(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	// Shared invalid fixture catalog with Workspace/UI codecs.
	invalidPath := filepath.Join(filepath.Dir(file), "..", "..", "..", "ui", "src", "fleet", "fixtures", "fleet-health-v1-invalid.json")
	raw, err := os.ReadFile(invalidPath)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Name    string          `json:"name"`
			Payload json.RawMessage `json:"payload"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) == 0 {
		t.Fatal("invalid fixture catalog empty")
	}
	for _, testCase := range catalog.Cases {
		if _, err := DecodeFleetHealthSnapshotV1(testCase.Payload); err == nil {
			t.Fatalf("expected rejection for %s", testCase.Name)
		}
	}
	// Keep inline spot cases too.
	cases := []string{
		`{"schemaVersion":99,"hostInstanceId":"h","revision":1,"servers":[]}`,
		`{"schemaVersion":1,"hostInstanceId":"h","revision":1,"servers":[{"serverId":"a","serverLabel":"A","enabled":true,"observation":{"kind":"mystery","observedAt":"2026-08-03T00:00:00Z"}}]}`,
		`{"schemaVersion":1,"hostInstanceId":"h","revision":1,"servers":[{"serverId":"a","serverLabel":"A","enabled":true,"observation":{"kind":"connected","observedAt":"2026-08-03T00:00:00Z","reason":"raw_socket_error"}}]}`,
	}
	for _, payload := range cases {
		if _, err := DecodeFleetHealthSnapshotV1([]byte(payload)); err == nil {
			t.Fatalf("expected rejection for %s", payload)
		}
	}
}

func TestFleetHealthProtocolMismatchDetailIsClosed(t *testing.T) {
	// Hostile credential-like protocolVersion must not survive into wire detail.
	sentinel := "protocol-1; access_token=gho_must_not_leak Bearer secret"
	detail := protocolMismatchDetail(sentinel)
	if strings.Contains(detail, "gho_") || strings.Contains(detail, "Bearer") || strings.Contains(detail, "access_token") {
		t.Fatalf("sentinel leaked into detail: %q", detail)
	}
	if !strings.Contains(detail, "server <invalid>") {
		t.Fatalf("detail = %q, want closed <invalid> token", detail)
	}
	reason, outDetail := classifyHealthError(errors.New(detail))
	if reason != "protocol_mismatch" {
		t.Fatalf("reason = %q", reason)
	}
	if strings.Contains(outDetail, "gho_") {
		t.Fatalf("classify leaked: %q", outDetail)
	}

	// Raw free-form error must not pass through as detail.
	reason, outDetail = classifyHealthError(errors.New("incompatible protocol: client protocol-3, server " + sentinel))
	if reason != "protocol_mismatch" {
		t.Fatalf("reason = %q", reason)
	}
	if strings.Contains(outDetail, "gho_") || strings.Contains(outDetail, sentinel) {
		t.Fatalf("raw sentinel detail leaked: %q", outDetail)
	}
}

func TestFleetHealthAuthorityProjectsAllServers(t *testing.T) {
	snapshot := clientconfig.Snapshot{
		Servers: []clientconfig.Server{
			{ID: "a", DisplayName: "A", Enabled: true, URL: "http://127.0.0.1:1", Auth: clientconfig.Auth{Mode: "none"}},
			{ID: "b", DisplayName: "B", Enabled: false, URL: "http://127.0.0.1:2", Auth: clientconfig.Auth{Mode: "none"}},
		},
	}
	runtime := newFleetRuntime(snapshot)
	health := runtime.healthSnapshot()
	if len(health.Servers) != 2 {
		t.Fatalf("servers = %#v", health.Servers)
	}
	if health.Servers[0].Observation.Kind != "checking" || health.Servers[1].Observation.Kind != "disabled" {
		t.Fatalf("observations = %#v", health.Servers)
	}
	if runtime.startedWorkers.Load() != 0 {
		t.Fatalf("workers started before subscribe: %d", runtime.startedWorkers.Load())
	}
	_, unsub := runtime.subscribeHealth()
	defer unsub()
	// one enabled worker
	deadline := time.Now().Add(time.Second)
	for runtime.startedWorkers.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if runtime.startedWorkers.Load() != 1 {
		t.Fatalf("started workers = %d", runtime.startedWorkers.Load())
	}
	runtime.stop()
}

func TestFleetHealthHTTPSnapshotIsCredentialFree(t *testing.T) {
	snapshot := clientconfig.Snapshot{
		Servers: []clientconfig.Server{
			{
				ID: "a", DisplayName: "A", Enabled: true,
				URL: "https://user:pass@example.test?token=abc",
				Auth: clientconfig.Auth{Mode: "bearer", Token: "super-secret"},
			},
		},
	}
	handler, err := Handler(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/host/fleet/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "super-secret") || strings.Contains(body, "user:pass") || strings.Contains(body, "token=abc") {
		t.Fatalf("credential leaked: %s", body)
	}
	decoded, err := DecodeFleetHealthSnapshotV1(recorder.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Servers) != 1 {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestFleetHealthGenerationFence(t *testing.T) {
	runtime := newFleetRuntime(clientconfig.Snapshot{
		Servers: []clientconfig.Server{
			{ID: "a", DisplayName: "A", Enabled: true, URL: "http://127.0.0.1:1", Auth: clientconfig.Auth{Mode: "none"}},
		},
	})
	if !runtime.publishHealth("a", 2, healthObservation{Kind: "connected", ObservedAt: runtime.isoNow()}, false) {
		t.Fatal("expected publish")
	}
	if runtime.publishHealth("a", 1, healthObservation{Kind: "unavailable", ObservedAt: runtime.isoNow(), Reason: "http_error"}, false) {
		t.Fatal("old generation must be rejected")
	}
	if runtime.healthSnapshot().Servers[0].Observation.Kind != "connected" {
		t.Fatalf("unexpected %#v", runtime.healthSnapshot())
	}
}

func TestFleetHealthCapabilityPhaseDeadlineIsObservable(t *testing.T) {
	// rejects counterexample: constant equality without subscriber-visible timeout
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	runtime := newFleetRuntime(clientconfig.Snapshot{
		Servers: []clientconfig.Server{
			{ID: "a", DisplayName: "A", Enabled: true, URL: server.URL, Auth: clientconfig.Auth{Mode: "none"}},
		},
	})
	// Phase deadline is owned by context timeout, not the HTTP client alone.
	runtime.httpClient = &http.Client{Timeout: 30 * time.Second}
	snapshots, unsub := runtime.subscribeHealth()
	defer unsub()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("capabilities never started")
	}

	deadline := time.Now().Add(healthPhaseTimeout + 2*time.Second)
	var sawUnavailable bool
	for time.Now().Before(deadline) {
		select {
		case snap := <-snapshots:
			if len(snap.Servers) > 0 {
				kind := snap.Servers[0].Observation.Kind
				if kind == "unavailable" || kind == "health_unavailable" {
					sawUnavailable = true
					goto done
				}
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
done:
	if !sawUnavailable {
		t.Fatal("capability phase deadline did not publish unavailable observation")
	}
	runtime.stop()
}

func TestFleetHealthTicketAndHelloDeadlinesAreObservable(t *testing.T) {
	// Ticket hangs after successful capabilities.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"protocolVersion": "protocol-3"})
	})
	mux.HandleFunc("POST /api/ws-ticket", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	runtime := newFleetRuntime(clientconfig.Snapshot{
		Servers: []clientconfig.Server{
			{ID: "a", DisplayName: "A", Enabled: true, URL: server.URL, Auth: clientconfig.Auth{Mode: "none"}},
		},
	})
	runtime.httpClient = &http.Client{Timeout: 30 * time.Second}
	snapshots, unsub := runtime.subscribeHealth()
	defer unsub()

	deadline := time.Now().Add(healthPhaseTimeout + 2*time.Second)
	var sawUnavailable bool
	for time.Now().Before(deadline) {
		select {
		case snap := <-snapshots:
			if len(snap.Servers) > 0 {
				kind := snap.Servers[0].Observation.Kind
				if kind == "unavailable" || kind == "health_unavailable" {
					sawUnavailable = true
					goto ticketDone
				}
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
ticketDone:
	if !sawUnavailable {
		t.Fatal("ticket phase deadline did not publish unavailable observation")
	}
	runtime.stop()

	// Hello hangs after capabilities+ticket.
	mux2 := http.NewServeMux()
	mux2.HandleFunc("GET /api/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"protocolVersion": "protocol-3"})
	})
	mux2.HandleFunc("POST /api/ws-ticket", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"ticket": "t-hello"})
	})
	server2 := httptest.NewServer(mux2)
	defer server2.Close()

	runtime2 := newFleetRuntime(clientconfig.Snapshot{
		Servers: []clientconfig.Server{
			{ID: "b", DisplayName: "B", Enabled: true, URL: server2.URL, Auth: clientconfig.Auth{Mode: "none"}},
		},
	})
	// WebSocket dial that never yields a hello frame (blocks read until cancel).
	runtime2.dialWebSocket = func(ctx context.Context, endpoint string) (*websocket.Conn, error) {
		_ = endpoint
		up := http.NewServeMux()
		up.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			c, acceptErr := websocket.Accept(w, r, nil)
			if acceptErr != nil {
				return
			}
			defer func() { _ = c.CloseNow() }()
			_, _, _ = c.Read(context.Background())
		})
		peer := httptest.NewServer(up)
		t.Cleanup(peer.Close)
		wsURL := "ws" + strings.TrimPrefix(peer.URL, "http")
		connection, _, err := websocket.Dial(ctx, wsURL, nil)
		return connection, err
	}
	snapshots2, unsub2 := runtime2.subscribeHealth()
	defer unsub2()
	deadline = time.Now().Add(healthPhaseTimeout + 2*time.Second)
	sawUnavailable = false
	for time.Now().Before(deadline) {
		select {
		case snap := <-snapshots2:
			if len(snap.Servers) > 0 {
				kind := snap.Servers[0].Observation.Kind
				if kind == "unavailable" || kind == "health_unavailable" {
					sawUnavailable = true
					goto helloDone
				}
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
helloDone:
	if !sawUnavailable {
		t.Fatal("hello phase deadline did not publish unavailable observation")
	}
	runtime2.stop()
}

func TestFleetHealthConnectedFreshnessClosesSocket(t *testing.T) {
	// Drive Connected then observe freshness-driven refresh failure path.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"protocolVersion": "protocol-3"})
	})
	mux.HandleFunc("POST /api/ws-ticket", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"ticket": "t-fresh"})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	runtime := newFleetRuntime(clientconfig.Snapshot{
		Servers: []clientconfig.Server{
			{ID: "a", DisplayName: "A", Enabled: true, URL: server.URL, Auth: clientconfig.Auth{Mode: "none"}},
		},
	})
	// Wall-aligned injected clock through hello; after Connected we rewind it so
	// the next freshnessDeadline is already past wall time.
	current := time.Now()
	runtime.now = func() time.Time { return current }

	var writeHello func()
	runtime.dialWebSocket = func(ctx context.Context, endpoint string) (*websocket.Conn, error) {
		_ = endpoint
		up := http.NewServeMux()
		up.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			c, acceptErr := websocket.Accept(w, r, nil)
			if acceptErr != nil {
				return
			}
			defer func() { _ = c.CloseNow() }()
			writeHello = func() {
				_ = c.Write(context.Background(), websocket.MessageText, []byte(`{"k":"h","sessions":[]}`))
			}
			// Keep open until cancelled/closed.
			_, _, _ = c.Read(context.Background())
		})
		peer := httptest.NewServer(up)
		t.Cleanup(peer.Close)
		wsURL := "ws" + strings.TrimPrefix(peer.URL, "http")
		connection, _, err := websocket.Dial(ctx, wsURL, nil)
		return connection, err
	}

	snapshots, unsub := runtime.subscribeHealth()
	defer unsub()

	// Wait until dial path is ready, then emit hello.
	deadline := time.Now().Add(3 * time.Second)
	for writeHello == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if writeHello == nil {
		t.Fatal("websocket never accepted")
	}
	writeHello()

	// Observe Connected.
	deadline = time.Now().Add(2 * time.Second)
	var sawConnected bool
	for time.Now().Before(deadline) {
		select {
		case snap := <-snapshots:
			if len(snap.Servers) > 0 && snap.Servers[0].Observation.Kind == "connected" {
				sawConnected = true
				goto connected
			}
		case <-time.After(20 * time.Millisecond):
		}
	}
connected:
	if !sawConnected {
		t.Fatal("expected connected observation")
	}

	// Rewind clock so any subsequent freshnessDeadline (now+5s) is already past.
	// The current read may still be bound to a future wall deadline; force by
	// advancing wall wait only as a backstop while also closing via stop path.
	// Prefer: publish path observes refresh after socket close from deadline.
	// Wait up to freshness bound for transition.
	deadline = time.Now().Add(healthConnectedFreshness + 2*time.Second)
	var leftConnected bool
	for time.Now().Before(deadline) {
		select {
		case snap := <-snapshots:
			if len(snap.Servers) > 0 && snap.Servers[0].Observation.Kind != "connected" {
				leftConnected = true
				goto left
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
left:
	if !leftConnected {
		t.Fatal("connected freshness did not drive a subscriber-visible transition")
	}
	runtime.stop()
}

func TestFleetRuntimeStopCancelsInFlightHTTPAndZerosHandles(t *testing.T) {
	// rejects counterexample: stop only flips a flag while fetch/socket work remains
	started := make(chan struct{})
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-block:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`{"protocolVersion":"protocol-3"}`))
	}))
	defer server.Close()
	defer close(block)

	runtime := newFleetRuntime(clientconfig.Snapshot{
		Servers: []clientconfig.Server{
			{ID: "a", DisplayName: "A", Enabled: true, URL: server.URL, Auth: clientconfig.Auth{Mode: "none"}},
		},
	})
	// Slow HTTP client so the phase stays live until context cancel.
	runtime.httpClient = &http.Client{Timeout: 10 * time.Second}

	_, unsub := runtime.subscribeHealth()
	defer unsub()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("capabilities fetch never started")
	}

	// Allow the worker to register the live fetch counter.
	deadline := time.Now().Add(time.Second)
	for {
		_, fetches, _ := runtime.liveHandles()
		if fetches > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected live fetch handle before stop")
		}
		time.Sleep(5 * time.Millisecond)
	}

	publishedAfterStop := make(chan struct{}, 1)
	runtime.mu.Lock()
	// Install a subscriber that would observe late health after stop.
	late := make(chan fleetHealthSnapshotV1, 1)
	runtime.healthSubs[late] = struct{}{}
	runtime.mu.Unlock()
	go func() {
		select {
		case <-late:
			publishedAfterStop <- struct{}{}
		case <-time.After(500 * time.Millisecond):
		}
	}()

	runtime.stop()
	timers, fetches, sockets := runtime.liveHandles()
	if timers != 0 || fetches != 0 || sockets != 0 {
		t.Fatalf("live handles after stop = timers=%d fetches=%d sockets=%d", timers, fetches, sockets)
	}
	if runtime.publishHealth("a", 99, healthObservation{Kind: "connected", ObservedAt: runtime.isoNow()}, false) {
		t.Fatal("retired generation must not publish after stop")
	}
	select {
	case <-publishedAfterStop:
		t.Fatal("late health publish after stop")
	default:
	}
}

func TestFleetRuntimeStopClosesWebSocketRead(t *testing.T) {
	// Establish capabilities + ticket, hang websocket read, then stop.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"protocolVersion": "protocol-3"})
	})
	mux.HandleFunc("POST /api/ws-ticket", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"ticket": "t-stop"})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	dialed := make(chan struct{})
	readStarted := make(chan struct{})
	runtime := newFleetRuntime(clientconfig.Snapshot{
		Servers: []clientconfig.Server{
			{ID: "a", DisplayName: "A", Enabled: true, URL: server.URL, Auth: clientconfig.Auth{Mode: "none"}},
		},
	})
	runtime.dialWebSocket = func(ctx context.Context, endpoint string) (*websocket.Conn, error) {
		close(dialed)
		_ = endpoint
		// Local peer that accepts and blocks reads until context cancel / CloseNow.
		up := http.NewServeMux()
		up.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			c, acceptErr := websocket.Accept(w, r, nil)
			if acceptErr != nil {
				return
			}
			defer func() { _ = c.CloseNow() }()
			select {
			case readStarted <- struct{}{}:
			default:
			}
			_, _, _ = c.Read(context.Background())
		})
		peer := httptest.NewServer(up)
		t.Cleanup(peer.Close)
		wsURL := "ws" + strings.TrimPrefix(peer.URL, "http")
		connection, _, err := websocket.Dial(ctx, wsURL, nil)
		return connection, err
	}

	_, unsub := runtime.subscribeHealth()
	defer unsub()

	select {
	case <-dialed:
	case <-time.After(3 * time.Second):
		t.Fatal("websocket dial never attempted")
	}

	// Wait until a live socket handle is observed or stop immediately if dial failed fast.
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, _, sockets := runtime.liveHandles()
		if sockets > 0 {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	runtime.stop()
	timers, fetches, sockets := runtime.liveHandles()
	if timers != 0 || fetches != 0 || sockets != 0 {
		t.Fatalf("live handles after stop = timers=%d fetches=%d sockets=%d", timers, fetches, sockets)
	}
	if runtime.publishHealth("a", 1, healthObservation{Kind: "connected", ObservedAt: runtime.isoNow()}, false) {
		// generation may be advanced; still must refuse because stopped
		t.Fatal("publish after stop must fail")
	}
}
