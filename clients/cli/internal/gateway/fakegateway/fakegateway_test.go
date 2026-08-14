package fakegateway

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Contract: contract-external-dependency-registration (unit-fake-gateway).

// TestFakeGatewayContract_ShapesMatchCapturedFixtures is the fake's invariant
// contract test.
//
// A fake drifts by becoming convenient: it starts answering a question the real
// server would refuse, or answering with a field name only this client uses,
// and every test built on it keeps passing while meaning nothing. The defence
// is that the shapes come from the Phase 0 captures — bytes a production
// handler produced — rather than from what was convenient to write here.
func TestFakeGatewayContract_ShapesMatchCapturedFixtures(t *testing.T) {
	t.Parallel()

	fake := New()
	fake.SessionConfig = decodeFixture(t, "session-config.json")

	for _, route := range []struct {
		method  string
		path    string
		fixture string
	}{
		{http.MethodGet, "/api/session-config", "session-config.json"},
	} {
		request, err := http.NewRequestWithContext(
			context.Background(), route.method, "https://gateway.test"+route.path, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		response, err := fake.Doer().Do(request)
		if err != nil {
			t.Fatalf("%s %s: %v", route.method, route.path, err)
		}
		var answered any
		if err := json.NewDecoder(response.Body).Decode(&answered); err != nil {
			t.Fatalf("decode %s: %v", route.path, err)
		}
		var captured any
		if err := json.Unmarshal(readFixture(t, route.fixture), &captured); err != nil {
			t.Fatalf("decode fixture %s: %v", route.fixture, err)
		}
		if !reflect.DeepEqual(answered, captured) {
			t.Errorf("%s answers a different shape than the captured %s", route.path, route.fixture)
		}
	}

	// The rejection body and the departure frame are the two shapes the CLI
	// classifies on, so they are compared field by field against what the real
	// gateway wrote.
	assertMatchesFixture(t, rejectionFromFake(t), "rejection-unauthenticated.json")
	assertMatchesFixture(t, departureFromFake(t), "departure-capacity.json")
}

// rejectionFromFake produces the fake's ticket rejection body.
func rejectionFromFake(t *testing.T) map[string]any {
	t.Helper()
	fake := New()
	fake.RejectTicketMint = true
	request, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, "https://gateway.test/api/ws-ticket", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	response, err := fake.Doer().Do(request)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("rejection status = %d", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode rejection: %v", err)
	}
	return body
}

// departureFromFake produces the fake's departure control frame.
func departureFromFake(t *testing.T) map[string]any {
	t.Helper()
	fake := New()
	socket, err := fake.Dialer()(context.Background(), "wss://gateway.test/ws?ticket=t")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	// Drain the hello frame the real gateway also sends first.
	if _, err := socket.Read(context.Background()); err != nil {
		t.Fatalf("read hello: %v", err)
	}
	fakeSocket, _ := socket.(*Socket)
	go fakeSocket.Depart(gateway.ReasonCapacity)
	raw, err := socket.Read(context.Background())
	if err != nil {
		t.Fatalf("read departure: %v", err)
	}
	var frame map[string]any
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("decode departure: %v", err)
	}
	return frame
}

// assertMatchesFixture compares the keys the fake emits against the captured
// ones. The fake may omit the pre-taxonomy `data` payload — nothing in this
// client reads it — but it must not carry a key the real gateway never wrote,
// and every key it does carry must be spelled the same way.
func assertMatchesFixture(t *testing.T, produced map[string]any, fixture string) {
	t.Helper()
	var captured map[string]any
	if err := json.Unmarshal(readFixture(t, fixture), &captured); err != nil {
		t.Fatalf("decode fixture %s: %v", fixture, err)
	}
	for key := range produced {
		if _, present := captured[key]; !present {
			t.Errorf("%s: the fake emits key %q that the captured gateway response does not",
				fixture, key)
		}
	}
	for key, value := range captured {
		if key == "data" || key == "message" {
			continue
		}
		if produced[key] != value {
			t.Errorf("%s: key %q is %v in the fake and %v in the capture",
				fixture, key, produced[key], value)
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

func decodeFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(readFixture(t, name), &decoded); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return decoded
}

// TestFakeGatewayDrivesEveryRegisteredBehaviour proves the fake covers the
// whole boundary, which is what makes the tests built on it meaningful.
func TestFakeGatewayDrivesEveryRegisteredBehaviour(t *testing.T) {
	t.Parallel()

	fake := New(gateway.SessionInfo{ID: "s1"})
	client, err := gateway.New(testServerEntry(), fake.Doer(), fake.Dialer())
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if err := client.Admit(context.Background()); err != nil {
		t.Fatalf("admission is not driveable: %v", err)
	}
	socket, instance, err := client.Dial(context.Background())
	if err != nil {
		t.Fatalf("ticket minting and dialling are not driveable: %v", err)
	}
	if instance == "" {
		t.Error("the mint returned no client instance identity")
	}
	if fake.TicketMints != 1 {
		t.Errorf("%d tickets minted for one dial", fake.TicketMints)
	}

	raw, err := socket.Read(context.Background())
	if err != nil {
		t.Fatalf("the initial frame is not driveable: %v", err)
	}
	if !strings.Contains(string(raw), `"k":"h"`) {
		t.Errorf("the initial frame is %s, want a hello", raw)
	}

	fakeSocket, _ := socket.(*Socket)
	go fakeSocket.PushOutput("s1", []byte("output"))
	if _, err := socket.Read(context.Background()); err != nil {
		t.Fatalf("output is not driveable: %v", err)
	}
	go fakeSocket.DropAbruptly()
	if _, err := socket.Read(context.Background()); err == nil {
		t.Error("an abrupt drop is not driveable")
	}
	if socket.CloseReason() != gateway.ReasonUnattributed {
		t.Errorf("an abrupt drop reported %q, want the unattributed sentinel", socket.CloseReason())
	}
}

// testServerEntry is a usable configuration entry pointing at the fake.
func testServerEntry() clientconfig.Server {
	return clientconfig.Server{
		Index: 0, ID: "srv_fake", DisplayName: "fake", Enabled: true,
		URL: "https://gateway.test", Auth: clientconfig.Auth{Mode: "none"},
	}
}
