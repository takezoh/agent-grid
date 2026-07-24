package api

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type blockingLifecycleAttacher struct {
	*fakeLifecycleAttacher

	mu                  sync.Mutex
	blockWriteRaw       chan struct{}
	blockResize         chan struct{}
	writeRawStarted     chan struct{}
	resizeStarted       chan struct{}
	unsubscribeSessions []string
}

func newBlockingLifecycleAttacher() *blockingLifecycleAttacher {
	return &blockingLifecycleAttacher{
		fakeLifecycleAttacher: newFakeLifecycleAttacher(),
		blockWriteRaw:         make(chan struct{}),
		blockResize:           make(chan struct{}),
		writeRawStarted:       make(chan struct{}, 1),
		resizeStarted:         make(chan struct{}, 1),
	}
}

func (f *blockingLifecycleAttacher) WriteRaw(ctx context.Context, _ string, _ []byte) error {
	f.writeRawStarted <- struct{}{}
	select {
	case <-f.blockWriteRaw:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *blockingLifecycleAttacher) Resize(ctx context.Context, _ string, _, _ uint16) error {
	f.resizeStarted <- struct{}{}
	select {
	case <-f.blockResize:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *blockingLifecycleAttacher) SendSurfaceUnsubscribe(
	_ context.Context,
	sessionID string,
	_ string,
) error {
	f.mu.Lock()
	f.unsubscribeSessions = append(f.unsubscribeSessions, sessionID)
	f.mu.Unlock()
	return nil
}

func (f *blockingLifecycleAttacher) unsubscribeSnapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.unsubscribeSessions...)
}

func writeLifecycleFrame(t *testing.T, c *websocket.Conn, frame string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, []byte(frame)); err != nil {
		t.Fatalf("write lifecycle frame: %v", err)
	}
}

func assertLifecycleResponseWithin(
	t *testing.T,
	c *websocket.Conn,
	reqID string,
	timeout time.Duration,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read response for %s within %s: %v", reqID, timeout, err)
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("decode response %q: %v", data, err)
	}
	if response["k"] != "r" || response["reqId"] != reqID {
		t.Fatalf("response = %v, want success for %s", response, reqID)
	}
}

func TestGatewayLifecycle_UnsubscribeIsNotBlockedByEarlierInboundRPC(t *testing.T) {
	tests := []struct {
		name     string
		blocking string
		started  func(*blockingLifecycleAttacher) <-chan struct{}
		release  func(*blockingLifecycleAttacher)
	}{
		{
			name:     "write_raw",
			blocking: `{"k":"i","sessionId":"s1","d":"x"}`,
			started:  func(f *blockingLifecycleAttacher) <-chan struct{} { return f.writeRawStarted },
			release:  func(f *blockingLifecycleAttacher) { close(f.blockWriteRaw) },
		},
		{
			name:     "resize",
			blocking: `{"k":"r","sessionId":"s1","cols":120,"rows":40}`,
			started:  func(f *blockingLifecycleAttacher) <-chan struct{} { return f.resizeStarted },
			release:  func(f *blockingLifecycleAttacher) { close(f.blockResize) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newBlockingLifecycleAttacher()
			srv := startLifecycleServer(t, fake)
			c := dialLifecycleWS(t, srv)

			writeLifecycleFrame(t, c, `{"k":"s","reqId":"subscribe-s1","sessionId":"s1","cols":120,"rows":40}`)
			assertLifecycleResponseWithin(t, c, "subscribe-s1", time.Second)

			writeLifecycleFrame(t, c, tt.blocking)
			select {
			case <-tt.started(fake):
			case <-time.After(time.Second):
				t.Fatal("blocking Attacher RPC did not start")
			}

			writeLifecycleFrame(t, c, `{"k":"u","reqId":"unsubscribe-s1","sessionId":"s1"}`)
			assertLifecycleResponseWithin(t, c, "unsubscribe-s1", 100*time.Millisecond)
			if got := fake.unsubscribeSnapshot(); len(got) != 1 || got[0] != "s1" {
				t.Fatalf("unsubscribe calls = %v, want [s1]", got)
			}

			tt.release(fake)
		})
	}
}

var _ Attacher = (*blockingLifecycleAttacher)(nil)
