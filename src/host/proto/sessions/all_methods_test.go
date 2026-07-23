package sessions_test

import (
	"testing"

	"github.com/takezoh/agent-grid/host/proto"
)

func reply(t *testing.T, srv *fakeServer, resp proto.Response) {
	t.Helper()
	env := srv.recv()
	wire, _ := proto.EncodeResponse(env.ReqID, resp)
	srv.send(wire)
}

func TestSubscribe(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.Subscribe(); err != nil {
		t.Errorf("Subscribe: %v", err)
	}
}

func TestListSessions(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespSessions{
		Sessions: []proto.SessionInfo{{ID: "s1"}},
		Features: []string{"f1"},
	})
	sessions, features, err := c.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || len(features) != 1 {
		t.Errorf("unexpected: %+v %v", sessions, features)
	}
}

func TestShutdown(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.Shutdown(); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

func TestSetHeadFrame(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.SetHeadFrame("s", "f"); err != nil {
		t.Errorf("SetHeadFrame: %v", err)
	}
}

func TestForkSession(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespCreateSession{SessionID: "new"})
	id, err := c.ForkSession("orig")
	if err != nil || id != "new" {
		t.Errorf("got %q err %v", id, err)
	}
}
