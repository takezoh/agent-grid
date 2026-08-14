package gateway

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// Contract: contract-wire-shape-conformance, contract-output-passthrough,
// contract-input-passthrough, contract-identity-single-origin
// (unit-wire-codec).

// TestDecodesCheckedInSessionInfoGoldenVector is
// ver-cli-decodes-the-same-golden-vector.
//
// The pair matters more than either half: the server test reproduces this file
// from its own serializer, and this test reads the same file into the struct
// the CLI actually uses. A rename on the server side fails there; a convenience
// shape on the client side fails here.
func TestDecodesCheckedInSessionInfoGoldenVector(t *testing.T) {
	t.Parallel()

	path := filepath.Join(moduleRoot(t), "..", "..", "protocol", "fixtures", "session-info-golden.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden vector: %v", err)
	}
	var session SessionInfo
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatalf("decode golden vector: %v", err)
	}

	for _, expected := range []struct {
		field string
		got   string
		want  string
	}{
		{"id", session.ID, "s_0a1b2c3d4e5f"},
		{"project", session.Project, "/home/operator/repos/agent-grid"},
		{"workspace", session.Workspace, "default"},
		{"command", session.Command, "claude --permission-mode acceptEdits"},
		{"root_driver", session.RootDriver, "claude"},
		{"created_at", session.CreatedAt, "2026-08-14T09:00:00Z"},
		{"status.lifecycle", session.Status.Lifecycle, "active"},
		{"status.failure", session.Status.Failure, "none"},
		{"status.changed_at", session.Status.ChangedAt, "2026-08-14T09:30:00Z"},
		{"view.card.title", session.View.Card.Title, "Wire up the terminal client"},
		{"view.display_name", session.View.DisplayName, "claude"},
		{"view.model", session.View.Model, "claude-opus-5"},
		{"view.effort", session.View.Effort, "high"},
		{"workspace_root", session.WorkspaceRoot, "/home/operator/repos/agent-grid"},
		{"head_frame_id", session.HeadFrameID, "f_root"},
	} {
		if expected.got != expected.want {
			t.Errorf("%s decoded as %q, want %q", expected.field, expected.got, expected.want)
		}
	}
	if session.Status.Signal == nil || session.Status.Signal.Kind != "bell" {
		t.Errorf("status.signal did not decode: %+v", session.Status.Signal)
	}
	// The card title is the display name; the driver name is not. Pinning both
	// here is what stops a later edit quietly swapping them.
	if session.View.Card.Title == session.View.DisplayName {
		t.Error("the golden vector no longer distinguishes the card title from the driver name")
	}
}

// TestSurfaceOutputDecodesToExactBytes is ver-output-bytes-are-byte-faithful.
//
// The counterexample is specific and has bitten this repository before: decode
// the payload into a Go string and re-marshal it, and encoding/json replaces
// every non-UTF-8 byte with U+FFFD — garbling 256-colour sequences and any raw
// binary a session prints.
func TestSurfaceOutputDecodesToExactBytes(t *testing.T) {
	t.Parallel()

	payload := []byte{0x1b, '[', '3', '8', ';', '5', ';', 0xff, 0xfe, 'm', 0x00, 0x80, 'A'}
	encoded, err := json.Marshal([]any{
		1.5, "o", base64.StdEncoding.EncodeToString(payload), "s1",
	})
	if err != nil {
		t.Fatalf("build frame: %v", err)
	}

	frame, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}
	output, ok := frame.(OutputFrame)
	if !ok {
		t.Fatalf("frame type = %T, want OutputFrame", frame)
	}
	if string(output.Data) != string(payload) {
		t.Errorf("output bytes changed in transit:\n got %v\nwant %v", output.Data, payload)
	}
	if output.SessionID != "s1" {
		t.Errorf("session id = %q, want s1", output.SessionID)
	}
}

// TestInputFramesNeverSplitUTF8 is ver-utf8-sequence-never-split (the table
// half; FuzzInputFraming is the other).
func TestInputFramesNeverSplitUTF8(t *testing.T) {
	t.Parallel()

	// "あ" is three bytes; splitting it anywhere must hold the tail back.
	full := []byte("aあb")
	for split := 1; split < len(full); split++ {
		first, pending := SplitCompleteUTF8(full[:split])
		if !utf8.Valid(first) {
			t.Errorf("split at %d emitted an invalid sequence: %v", split, first)
		}
		rest, stillPending := SplitCompleteUTF8(append(pending, full[split:]...))
		if stillPending != nil {
			t.Errorf("split at %d never released the held bytes: %v", split, stillPending)
		}
		if got := string(first) + string(rest); got != string(full) {
			t.Errorf("split at %d reassembled to %q, want %q", split, got, full)
		}
	}
}

// TestInvalidLeadByteIsNotHeldForever is the other side of the same rule. A
// byte that can never start a sequence must be passed through, because waiting
// for a continuation that is not coming would stall every subsequent keystroke.
func TestInvalidLeadByteIsNotHeldForever(t *testing.T) {
	t.Parallel()

	for _, input := range [][]byte{{0xff}, {'a', 0xff}, {0x80}, {0xc0, 0x41}} {
		complete, pending := SplitCompleteUTF8(input)
		if pending != nil {
			t.Errorf("SplitCompleteUTF8(%v) held %v; an unfinishable sequence must pass through",
				input, pending)
		}
		if len(complete) != len(input) {
			t.Errorf("SplitCompleteUTF8(%v) dropped bytes: %v", input, complete)
		}
	}
}

// FuzzInputFraming is ver-input-fuzz-order-and-bytes.
//
// Streaming the input through arbitrary split points must reproduce the
// original byte sequence exactly and must never place a partial multi-byte
// sequence in a frame. A table test only proves the split points someone
// thought of.
func FuzzInputFraming(f *testing.F) {
	f.Add([]byte("hello"), 1)
	f.Add([]byte("あいうえお"), 2)
	f.Add([]byte("🎛️ mixed アスキー"), 3)
	f.Add([]byte{0x1b, '[', 'A', 0xe3, 0x81, 0x82}, 4)
	// Two lead bytes in a row: the first can never be completed, so it must be
	// released rather than held, and releasing it is a legal boundary. Found by
	// this fuzzer while the invariant was still stated over the emitted frame
	// instead of over the original payload's rune boundaries.
	f.Add([]byte{0xe3, 0xe3}, 3)

	f.Fuzz(func(t *testing.T, payload []byte, chunk int) {
		if chunk <= 0 {
			chunk = 1
		}
		if chunk > 64 {
			chunk = 64
		}
		boundaries := runeBoundaries(payload)
		var sent []byte
		var pending []byte
		for offset := 0; offset < len(payload); offset += chunk {
			end := min(offset+chunk, len(payload))
			complete, held := SplitCompleteUTF8(append(pending, payload[offset:end]...))
			pending = held
			if len(complete) == 0 {
				continue
			}
			sent = append(sent, complete...)
			// The framing rule, stated over the original payload: a frame
			// boundary may only fall where a rune ends. An implementation that
			// emitted the front half of a sequence whose tail is still buffered
			// would land between two of these positions.
			if _, ok := boundaries[len(sent)]; !ok {
				t.Fatalf("a frame boundary at %d splits a multi-byte sequence in %v",
					len(sent), payload)
			}
		}
		sent = append(sent, pending...)
		if string(sent) != string(payload) {
			t.Fatalf("streaming changed the bytes:\n got %v\nwant %v", sent, payload)
		}
	})
}

// runeBoundaries is the set of byte offsets at which a rune ends.
//
// An invalid byte decodes as one-byte RuneError, so it is its own boundary:
// splitting there is allowed, because that byte can never be completed by a
// byte arriving later. Splitting inside a *valid* sequence is what the rule
// forbids, and this set is exactly the complement of those positions.
func runeBoundaries(payload []byte) map[int]struct{} {
	boundaries := map[int]struct{}{0: {}}
	for offset := 0; offset < len(payload); {
		_, size := utf8.DecodeRune(payload[offset:])
		offset += size
		boundaries[offset] = struct{}{}
	}
	return boundaries
}

// TestOutboundFramesCarryTheBareUpstreamIdentifier pins the wire half of
// contract-identity-single-origin.
func TestOutboundFramesCarryTheBareUpstreamIdentifier(t *testing.T) {
	t.Parallel()

	const bare = "s_0a1b2c3d4e5f"
	frames := map[string][]byte{
		"ld": EncodeLifecycleDesired(
			Correlation{ClientInstanceID: "ci", ConnectionGeneration: 1, ClientRevision: 7},
			bare, 120, 40, true, "req-1"),
		"i": EncodeInput(bare, []byte("hi")),
		"r": EncodeResize(bare, 120, 40),
	}
	for kind, encoded := range frames {
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("%s frame is not JSON: %v", kind, err)
		}
		if decoded["k"] != kind {
			t.Errorf("%s frame has k = %v", kind, decoded["k"])
		}
		if decoded["sessionId"] != bare {
			t.Errorf("%s frame carries session id %v, want the bare upstream identifier %q",
				kind, decoded["sessionId"], bare)
		}
		if strings.Contains(string(encoded), "srv_") {
			t.Errorf("%s frame carries a server-qualified identity: %s", kind, encoded)
		}
	}
}

// TestServerQualifiedIdentityHasExactlyOneConstructor is
// ver-identity-construction-has-one-owner.
//
// The compile-time half is stronger than any scan: SessionRef's fields are
// unexported, so no other package can assemble one from parts it happens to
// hold, and calling NewSessionRef from elsewhere is not a violation — it is
// what a single constructor is for.
//
// What the scan adds is the concept rather than the type: another package
// minting a server identifier of its own, or gluing a server id and a session
// id into one string that then looks like something a server would accept.
func TestServerQualifiedIdentityHasExactlyOneConstructor(t *testing.T) {
	t.Parallel()

	for _, offending := range moduleSourcesOutside(t, "internal/gateway",
		`"srv_"`, "ServerID()+", "+ref.ServerID()", `ServerID() + "`) {
		t.Errorf("%s builds a server-qualified identity outside the codec", offending)
	}
}

// TestOutboundFrameSerializationHasExactlyOneSite is
// ver-outbound-serialization-has-one-owner.
//
// The marker is the `k` discriminator: every frame this protocol defines
// carries one, so a frame built anywhere else has to write it.
func TestOutboundFrameSerializationHasExactlyOneSite(t *testing.T) {
	t.Parallel()

	for _, offending := range moduleSourcesOutside(t, "internal/gateway",
		`json:"k"`, `"k":"`, `\"k\":`) {
		t.Errorf("%s serialises an outbound frame outside the codec", offending)
	}
}

// TestIdentityOwnershipGateFailsOnInjectedSecondOwner is
// ver-ownership-gate-fails-on-synthesized-violation.
//
// A structural gate that has never fired is indistinguishable from one that
// cannot. This synthesises the violation in a temporary tree and requires the
// scanner to find it.
func TestIdentityOwnershipGateFailsOnInjectedSecondOwner(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSource(t, filepath.Join(root, "go.mod"), "module example.test\n\ngo 1.26.1\n")
	writeSource(t, filepath.Join(root, "internal", "gateway", "identity.go"),
		"package gateway\n\nfunc NewSessionRef() {}\n")
	writeSource(t, filepath.Join(root, "internal", "rogue", "rogue.go"),
		"package rogue\n\nfunc build() string { return \"srv_\" + \"x\" }\n")
	writeSource(t, filepath.Join(root, "internal", "rogue", "rogue_test.go"),
		"package rogue\n\n// \"srv_\" here must be ignored: a gate's own test names what it forbids.\n")

	offending := sourcesOutsideIn(t, root, "internal/gateway", `"srv_"`)
	if len(offending) != 1 || offending[0] != filepath.Join("internal", "rogue", "rogue.go") {
		t.Fatalf("the ownership gate did not fire on an injected second owner: %v", offending)
	}
}

func writeSource(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
