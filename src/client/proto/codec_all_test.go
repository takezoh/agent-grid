package proto

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestEncodeDecodeAllCommands(t *testing.T) {
	cmds := []Command{
		CmdSubscribe{Filters: []string{"a"}},
		CmdUnsubscribe{},
		CmdEvent{Event: "x"},
		CmdSubsystemEvent{Source: "s", Kind: "k"},
		CmdSurfaceReadText{SessionID: "s1", Lines: 10},
		CmdSurfaceSendText{SessionID: "s1", Text: "hi"},
		CmdSurfaceSendKey{SessionID: "s1", Key: "Escape"},
		CmdDriverList{},
		CmdHookEvent{Token: "t", Hook: "PreToolUse"},
	}
	for _, c := range cmds {
		raw, err := EncodeCommand("rid", c)
		if err != nil {
			t.Errorf("encode %T: %v", c, err)
			continue
		}
		env, err := DecodeEnvelope(raw)
		if err != nil {
			t.Errorf("decode env %T: %v", c, err)
			continue
		}
		if env.ReqID != "rid" {
			t.Errorf("ReqID = %q", env.ReqID)
		}
		got, err := DecodeCommand(env)
		if err != nil {
			t.Errorf("decode cmd %T: %v", c, err)
			continue
		}
		if got.CommandName() != c.CommandName() {
			t.Errorf("name mismatch: got %q, want %q", got.CommandName(), c.CommandName())
		}
	}
}

// TestEncodeDecodeAllFrameCommands covers the frame-messaging command
// variants (direct token + by-thread), which TestEncodeDecodeAllCommands
// above does not exercise.
func TestEncodeDecodeAllFrameCommands(t *testing.T) {
	cmds := []Command{
		CmdFrameList{Token: "tok"},
		CmdFrameRead{Token: "tok", PeerFrameID: "peer"},
		CmdFrameSend{Token: "tok", TargetFrameID: "target", Body: "hi"},
		CmdFrameReply{Token: "tok", MessageID: "m1", Body: "reply"},
		CmdFrameListByThread{SessionID: "s1", ThreadID: "t1"},
		CmdFrameReadByThread{SessionID: "s1", ThreadID: "t1", PeerFrameID: "peer"},
		CmdFrameSendByThread{SessionID: "s1", ThreadID: "t1", TargetFrameID: "target", Body: "hi"},
		CmdFrameReplyByThread{SessionID: "s1", ThreadID: "t1", MessageID: "m1", Body: "reply"},
	}
	for _, c := range cmds {
		t.Run(c.CommandName(), func(t *testing.T) {
			raw, err := EncodeCommand("rid", c)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			env, err := DecodeEnvelope(raw)
			if err != nil {
				t.Fatalf("decode env: %v", err)
			}
			got, err := DecodeCommand(env)
			if err != nil {
				t.Fatalf("decode cmd: %v", err)
			}
			if !reflect.DeepEqual(got, c) {
				t.Errorf("round-trip mismatch: got %#v, want %#v", got, c)
			}
		})
	}
}

func TestDecodeCommandBadJSON(t *testing.T) {
	env := Envelope{Type: TypeCommand, Cmd: CmdNameSubscribe, Data: json.RawMessage(`bad`)}
	if _, err := DecodeCommand(env); err == nil {
		t.Error("expected unmarshal error")
	}
}

func TestEncodeDecodeAllEvents(t *testing.T) {
	evts := []ServerEvent{
		EvtSessionsChanged{},
		EvtProjectSelected{Project: "p"},
		EvtLogLine{Path: "/a", Line: "x"},
		EvtSessionFileLine{SessionID: "s", Kind: "k", Line: "x"},
		EvtAgentNotification{SessionID: "s", Cmd: 9, Title: "t"},
	}
	for _, e := range evts {
		raw, err := EncodeEvent(e)
		if err != nil {
			t.Errorf("encode %T: %v", e, err)
			continue
		}
		env, err := DecodeEnvelope(raw)
		if err != nil {
			t.Errorf("decode env %T: %v", e, err)
			continue
		}
		got, err := DecodeEvent(env)
		if err != nil {
			t.Errorf("decode evt %T: %v", e, err)
			continue
		}
		if got.EventName() != e.EventName() {
			t.Errorf("name mismatch: got %q, want %q", got.EventName(), e.EventName())
		}
	}
}

func TestDecodeEventWrongType(t *testing.T) {
	if _, err := DecodeEvent(Envelope{Type: TypeCommand, Name: EvtNameSessionsChanged}); err == nil {
		t.Error("expected error")
	}
}

func TestEncodeResponse(t *testing.T) {
	raw, err := EncodeResponse("r1", RespCreateSession{SessionID: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	env, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	if env.Status != StatusOK {
		t.Errorf("status = %q", env.Status)
	}
	var r RespCreateSession
	if err := DecodeResponse(env, &r); err != nil {
		t.Fatal(err)
	}
	if r.SessionID != "abc" {
		t.Errorf("got %q", r.SessionID)
	}
}

func TestDecodeResponseWrongType(t *testing.T) {
	var r RespOK
	if err := DecodeResponse(Envelope{Type: TypeCommand}, &r); err == nil {
		t.Error("expected error")
	}
}

func TestDecodeResponseError(t *testing.T) {
	var r RespOK
	if err := DecodeResponse(Envelope{Type: TypeResponse, Status: StatusError}, &r); err == nil {
		t.Error("expected error")
	}
}

func TestDecodeResponseEmpty(t *testing.T) {
	var r RespOK
	if err := DecodeResponse(Envelope{Type: TypeResponse, Status: StatusOK}, &r); err != nil {
		t.Errorf("empty Data should not error: %v", err)
	}
}

func TestDecodeResponseByCommand(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"session_id", `{"session_id":"x"}`},
		{"sessions", `{"sessions":[]}`},
		{"active", `{"active_session_id":"x"}`},
		{"text", `{"text":"x"}`},
		{"drivers", `{"drivers":[]}`},
	}
	for _, c := range cases {
		env := Envelope{Type: TypeResponse, Data: json.RawMessage(c.raw)}
		if _, err := DecodeResponseByCommand(env); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	// empty -> RespOK
	if r, err := DecodeResponseByCommand(Envelope{}); err != nil || r == nil {
		t.Errorf("empty: r=%v err=%v", r, err)
	}
	// invalid json -> RespOK fallback
	if r, err := DecodeResponseByCommand(Envelope{Data: json.RawMessage(`bad`)}); err != nil || r == nil {
		t.Errorf("invalid json: %v %v", r, err)
	}
	// unknown shape -> RespOK
	if r, err := DecodeResponseByCommand(Envelope{Data: json.RawMessage(`{"x":1}`)}); err != nil || r == nil {
		t.Errorf("unknown shape: %v %v", r, err)
	}
}

// TestDecodeResponseByCommandFrameShapes covers the frame-messaging and
// surface-unsubscribed heuristic branches that TestDecodeResponseByCommand
// above does not reach (it only exercises the shapes routed via env.Cmd).
func TestDecodeResponseByCommandFrameShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want any
	}{
		{"reply", `{"session_id":"s1","message_id":"m1","reply":{"id":"r1"}}`, RespFrameReply{}},
		{"message", `{"session_id":"s1","message":{"id":"m1"}}`, RespFrameSend{}},
		{"frames", `{"frames":[]}`, RespFrameList{}},
		{"messages-no-summary", `{"session_id":"s1","messages":[]}`, RespFrameRead{}},
		{"messages-with-summary", `{"session_id":"s1","summary":{},"messages":[]}`, RespSessionMessages{}},
		{"session-and-subscriber", `{"session_id":"s1","subscriber_id":"w1"}`, RespSurfaceUnsubscribed{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := Envelope{Type: TypeResponse, Data: json.RawMessage(c.raw)}
			r, err := DecodeResponseByCommand(env)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			gotType := reflect.TypeOf(r)
			wantType := reflect.TypeOf(c.want)
			if gotType != wantType {
				t.Errorf("type = %s, want %s", gotType, wantType)
			}
		})
	}
}

func TestDecodeEnvelopeInvalid(t *testing.T) {
	if _, err := DecodeEnvelope([]byte(`not json`)); err == nil {
		t.Error("expected error")
	}
}

func TestDecodeIntoEmpty(t *testing.T) {
	// Empty data should not error.
	var c CmdSubscribe
	got, err := decodeInto(nil, &c)
	if err != nil || got == nil {
		t.Errorf("decodeInto empty: got=%v err=%v", got, err)
	}
	var e EvtLogLine
	gotE, err := decodeIntoEvent(nil, &e)
	if err != nil || gotE == nil {
		t.Errorf("decodeIntoEvent empty: got=%v err=%v", gotE, err)
	}
}

func TestDecodeIntoEventBad(t *testing.T) {
	var e EvtLogLine
	if _, err := decodeIntoEvent([]byte(`bad`), &e); err == nil {
		t.Error("expected error")
	}
}
