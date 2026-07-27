package proto

import "testing"

func TestLifecycleEventsRoundTripWithoutPrivateOwner(t *testing.T) {
	correlation := PublicCorrelation{ClientInstanceID: "c1", ConnectionGeneration: 2, ClientRevision: 7}
	events := []ServerEvent{
		EvtLifecycleOutcome{RevisionOutcome: RevisionOutcome{Correlation: correlation, Status: RevisionApplied, OutputSeq: 3, FinalSeq: 3}},
		EvtLifecycleOutput{LifecycleOutput: LifecycleOutput{Correlation: correlation, Sequence: 3, Final: true, Digest: "sha256:x"}},
		EvtLifecycleDiagnostic{LifecycleDiagnostic: LifecycleDiagnostic{Correlation: correlation, Watermark: 3, DropCount: 1, Unknown: true}},
	}
	for _, want := range events {
		wire, err := EncodeEvent(want)
		if err != nil {
			t.Fatalf("encode %T: %v", want, err)
		}
		env, err := DecodeEnvelope(wire)
		if err != nil {
			t.Fatalf("decode envelope %T: %v", want, err)
		}
		got, err := DecodeEvent(env)
		if err != nil {
			t.Fatalf("decode event %T: %v", want, err)
		}
		if got.EventName() != want.EventName() {
			t.Fatalf("event name = %q, want %q", got.EventName(), want.EventName())
		}
		if string(wire) == "" || string(wire) == "null" {
			t.Fatalf("empty wire for %T", want)
		}
	}
}

func TestLifecycleDesiredCommandRoundTrip(t *testing.T) {
	want := CmdLifecycleDesired{
		Correlation: PublicCorrelation{ClientInstanceID: "c1", ConnectionGeneration: 2, ClientRevision: 9},
		SessionID:   "s1",
		Cols:        120,
		Rows:        40,
		Desired:     true,
	}
	wire, err := EncodeCommand("req-1", want)
	if err != nil {
		t.Fatal(err)
	}
	env, err := DecodeEnvelope(wire)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeCommand(env)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("command = %#v, want %#v", got, want)
	}
}
