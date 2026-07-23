package api

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/takezoh/agent-grid/host/proto"
)

func TestDaemonClient_PerTabSeverDoesNotAffectOthers(t *testing.T) {
	t.Parallel()

	c1, s1 := net.Pipe()
	c2, s2 := net.Pipe()
	defer s1.Close()
	defer s2.Close()

	var callCount int
	d := NewDaemonClientWithDialer(func() (*proto.Client, error) {
		callCount++
		if callCount == 1 {
			return proto.DialConn(c1), nil
		}
		return proto.DialConn(c2), nil
	}, testMinDelay, testMaxDelay)
	defer d.Close()

	if !waitHealth(d, true, time.Second) {
		t.Fatal("daemon not healthy")
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	ch1 := d.SubscribeEvents(ctx1)
	ch2 := d.SubscribeEvents(ctx2)

	// Saturate tab 1 without draining it.
	for i := 0; i < perSubscriberBuf+2; i++ {
		d.broadcastEvent(proto.EvtAgentNotification{SessionID: "s1", Cmd: 1})
	}

	severed := false
	deadline := time.After(time.Second)
	for !severed {
		select {
		case _, ok := <-ch1:
			if !ok {
				severed = true
			}
		case <-deadline:
			t.Fatal("timeout waiting for slow tab severance")
		}
	}

	select {
	case ev := <-ch2:
		if ev == nil {
			t.Fatal("fast tab channel closed unexpectedly")
		}
	case <-time.After(time.Second):
		t.Fatal("fast tab starved after slow tab severance")
	}
}
