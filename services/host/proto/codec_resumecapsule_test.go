package proto

import "testing"

// TestDecodeResponseResumeCapsules pins the structural probe: the
// "resume_capsules" top-level key must decode into RespResumeCapsules and
// must not be shadowed by other probe arms.
func TestDecodeResponseResumeCapsules(t *testing.T) {
	body := RespResumeCapsules{ResumeCapsules: []ResumeCapsuleInfo{{
		SessionID:    "s1",
		Project:      "/repo",
		Objective:    "ship it",
		CurrentState: "tests green",
		NextAction:   "commit",
		Unresolved:   []string{"naming"},
		GeneratedAt:  "2026-08-04T00:00:00Z",
		SourceTurns:  4,
		FlaggedWrong: true,
	}}}
	wire, err := EncodeResponse("req-1", body)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	env, err := DecodeEnvelope(wire)
	if err != nil {
		t.Fatalf("envelope: %v", err)
	}
	resp, err := DecodeResponseByCommand(env)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := resp.(RespResumeCapsules)
	if !ok {
		t.Fatalf("decoded type = %T, want RespResumeCapsules", resp)
	}
	if len(got.ResumeCapsules) != 1 {
		t.Fatalf("capsules = %d, want 1", len(got.ResumeCapsules))
	}
	c := got.ResumeCapsules[0]
	if c.SessionID != "s1" || c.Objective != "ship it" || !c.FlaggedWrong || c.SourceTurns != 4 {
		t.Errorf("round-trip mismatch: %+v", c)
	}
}

// TestDecodeResponseResumeCapsulesEmpty ensures an empty list still
// carries the discriminator key (no omitempty) so decode stays typed.
func TestDecodeResponseResumeCapsulesEmpty(t *testing.T) {
	wire, err := EncodeResponse("req-1", RespResumeCapsules{ResumeCapsules: []ResumeCapsuleInfo{}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	env, err := DecodeEnvelope(wire)
	if err != nil {
		t.Fatalf("envelope: %v", err)
	}
	resp, err := DecodeResponseByCommand(env)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp.(RespResumeCapsules); !ok {
		t.Fatalf("decoded type = %T, want RespResumeCapsules", resp)
	}
}
