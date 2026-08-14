package attach

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway/fakegateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/tty"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// harness_test.go builds one attach against the fake gateway and the fake
// terminal. Every test in this package drives the real runtime through the two
// registered boundaries, so an exit path that only a real terminal could reach
// does not exist.

const (
	testSessionID  = "s_attached"
	foreignSession = "s_other"
)

func testServer() clientconfig.Server {
	return clientconfig.Server{
		Index: 0, ID: "srv_test", DisplayName: "one", Enabled: true,
		URL: "https://gateway.test", Auth: clientconfig.Auth{Mode: "none"},
	}
}

func liveSession(id string) gateway.SessionInfo {
	return gateway.SessionInfo{
		ID: id, Project: "/repo", Command: "claude",
		Status: gateway.SessionStatus{Lifecycle: "active", Failure: "none"},
		View:   gateway.View{Card: gateway.Card{Title: "Attached"}},
	}
}

func endedSession(id string) gateway.SessionInfo {
	ended := liveSession(id)
	ended.Status.Lifecycle = "stopped"
	return ended
}

// syncBuffer is a terminal output sink a test can read while the runtime
// writes.
type syncBuffer struct {
	mu      sync.Mutex
	data    []byte
	release chan struct{}
}

func (buffer *syncBuffer) Write(payload []byte) (int, error) {
	if buffer.release != nil {
		<-buffer.release
	}
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.data = append(buffer.data, payload...)
	return len(payload), nil
}

func (buffer *syncBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return string(buffer.data)
}

type harness struct {
	t        *testing.T
	fake     *fakegateway.Fake
	socket   *fakegateway.Socket
	terminal *tty.Fake
	out      *syncBuffer
	signals  chan os.Signal
	result   chan Result
	failure  chan error
}

// newHarness dials one socket through the real transport and starts an attach
// on a 120×40 terminal.
func newHarness(t *testing.T, configure func(*fakegateway.Fake), options ...func(*Options)) *harness {
	t.Helper()
	return newHarnessOn(t, 120, 40, configure, options...)
}

// newHarnessOn is newHarness with the terminal's reported size chosen, so the
// geometry rules can be driven through the path a real frame takes.
func newHarnessOn(
	t *testing.T, cols, rows int, configure func(*fakegateway.Fake), options ...func(*Options),
) *harness {
	t.Helper()

	fake := fakegateway.New(liveSession(testSessionID))
	if configure != nil {
		configure(fake)
	}
	client, err := gateway.New(testServer(), fake.Doer(), fake.Dialer())
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	if err := client.Admit(context.Background()); err != nil {
		t.Fatalf("admit: %v", err)
	}
	socket, instance, err := client.Dial(context.Background())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	fakeSocket, ok := socket.(*fakegateway.Socket)
	if !ok {
		t.Fatalf("the fake dialer returned %T", socket)
	}

	attached := &harness{
		t: t, fake: fake, socket: fakeSocket,
		terminal: tty.NewFake(cols, rows),
		out:      &syncBuffer{},
		signals:  make(chan os.Signal, 1),
		result:   make(chan Result, 1),
		failure:  make(chan error, 1),
	}
	runOptions := Options{
		Ref:              gateway.NewSessionRef(testServer(), liveSession(testSessionID)),
		Socket:           socket,
		ClientInstanceID: instance,
		Terminal:         attached.terminal,
		Out:              attached.out,
		DetachByte:       0x1d,
		Signals:          attached.signals,
		RenewInterval:    20 * time.Millisecond,
	}
	for _, apply := range options {
		apply(&runOptions)
	}
	go func() {
		result, err := Run(context.Background(), runOptions)
		if err != nil {
			attached.failure <- err
			return
		}
		attached.result <- result
	}()
	return attached
}

// waitEstablished blocks until the runtime has entered raw mode, which is the
// observable proof that the subscription reached a terminal accepted outcome.
func (attached *harness) waitEstablished() {
	attached.t.Helper()
	attached.waitFor("the attach never entered raw mode", func() bool {
		return attached.terminal.IsRaw()
	})
}

func (attached *harness) waitFor(what string, condition func() bool) {
	attached.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		select {
		case err := <-attached.failure:
			attached.failure <- err
			attached.t.Fatalf("%s: the attach failed first: %v", what, err)
		default:
		}
		time.Sleep(time.Millisecond)
	}
	attached.t.Fatal(what)
}

// awaitResult blocks for the attach's outcome.
func (attached *harness) awaitResult() Result {
	attached.t.Helper()
	select {
	case result := <-attached.result:
		return result
	case err := <-attached.failure:
		attached.t.Fatalf("the attach failed instead of departing: %v", err)
	case <-time.After(3 * time.Second):
		attached.t.Fatal("the attach never departed")
	}
	return Result{}
}

// awaitFailure blocks for a pre-subscription failure.
func (attached *harness) awaitFailure() error {
	attached.t.Helper()
	select {
	case err := <-attached.failure:
		return err
	case result := <-attached.result:
		attached.t.Fatalf("the attach departed instead of failing: %+v", result)
	case <-time.After(3 * time.Second):
		attached.t.Fatal("the attach neither failed nor departed")
	}
	return nil
}

// outboundFrames decodes every frame the client sent, in order.
func (attached *harness) outboundFrames() []map[string]any {
	written := attached.socket.Written()
	frames := make([]map[string]any, 0, len(written))
	for _, raw := range written {
		var decoded map[string]any
		if json.Unmarshal(raw, &decoded) != nil {
			continue
		}
		frames = append(frames, decoded)
	}
	return frames
}

// framesOfKind filters the outbound frames by their discriminator.
func (attached *harness) framesOfKind(kind string) []map[string]any {
	var selected []map[string]any
	for _, frame := range attached.outboundFrames() {
		if frame["k"] == kind {
			selected = append(selected, frame)
		}
	}
	return selected
}
