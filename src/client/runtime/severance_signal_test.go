package runtime

import (
	"testing"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/state"
)

func TestReduceSurfaceUnsubscribeServerInitiatedPush(t *testing.T) {
	s := state.New()
	s.Sessions["sess-1"] = state.Session{
		ID: "sess-1", Frames: []state.SessionFrame{{ID: "f1"}}, HeadFrameID: "f1",
	}
	s1, _ := state.Reduce(s, state.EvCmdSurfaceSubscribe{ConnID: 1, ReqID: "r1", SessionID: "sess-1"})

	_, effs := state.Reduce(s1, state.EvCmdSurfaceUnsubscribe{ConnID: 1, ReqID: "", SessionID: "sess-1"})
	if len(effs) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(effs))
	}
	if _, ok := effs[0].(state.EffSurfaceSubscribeStop); !ok {
		t.Fatalf("effs[0] = %T, want EffSurfaceSubscribeStop", effs[0])
	}
	sync, ok := effs[1].(state.EffSendResponseSync)
	if !ok {
		t.Fatalf("effs[1] = %T, want EffSendResponseSync", effs[1])
	}
	if sync.ReqID != "" {
		t.Fatalf("ReqID = %q, want empty for push", sync.ReqID)
	}
	body, ok := sync.Body.(state.SurfaceUnsubscribedReply)
	if !ok || body.SessionID != "sess-1" {
		t.Fatalf("Body = %#v, want SurfaceUnsubscribedReply{sess-1}", sync.Body)
	}
}

func TestEncodePushResponseSurfaceUnsubscribed(t *testing.T) {
	wire, err := proto.EncodePushResponse(proto.CmdNameSurfaceUnsubscribe, proto.RespSurfaceUnsubscribed{
		SessionID:    "sess-1",
		SubscriberID: "web-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err := proto.DecodeEnvelope(wire)
	if err != nil {
		t.Fatal(err)
	}
	if env.Cmd != proto.CmdNameSurfaceUnsubscribe {
		t.Fatalf("cmd = %q", env.Cmd)
	}
	if env.ReqID != "" {
		t.Fatalf("req_id = %q, want empty", env.ReqID)
	}
	resp, err := proto.DecodeResponseByCommand(env)
	if err != nil {
		t.Fatal(err)
	}
	body, ok := resp.(proto.RespSurfaceUnsubscribed)
	if !ok || body.SessionID != "sess-1" || body.SubscriberID != "web-1" {
		t.Fatalf("resp = %#v", resp)
	}
}
