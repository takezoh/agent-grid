package gateway

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contract: contract-transport-close-taxonomy
// (adr-20260814-transport-close-reason-taxonomy).

// TestCloseReasonVocabularyMatchesTheSchema pins this module's projection to
// protocol/transport-close.schema.json in both directions.
//
// One direction alone is not enough. Missing a reason the plane sends leaves
// the CLI classifying a real departure as unattributed; carrying one the plane
// cannot send leaves dead code that looks like coverage.
func TestCloseReasonVocabularyMatchesTheSchema(t *testing.T) {
	t.Parallel()

	path := filepath.Join(moduleRoot(t), "..", "..", "protocol", "transport-close.schema.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the published vocabulary: %v", err)
	}
	var schema struct {
		Defs struct {
			CloseReason struct {
				Enum []string `json:"enum"`
			} `json:"closeReason"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode the published vocabulary: %v", err)
	}
	published := schema.Defs.CloseReason.Enum
	if len(published) == 0 {
		t.Fatal("the schema declares no close reasons")
	}

	if len(published) != len(PublishedCloseReasons) {
		t.Errorf("the schema publishes %d reasons, this module carries %d",
			len(published), len(PublishedCloseReasons))
	}
	for index, name := range published {
		if index >= len(PublishedCloseReasons) {
			t.Errorf("this module does not carry the published reason %q", name)
			continue
		}
		if got := string(PublishedCloseReasons[index]); got != name {
			t.Errorf("position %d: schema says %q, this module says %q", index, name, got)
		}
	}

	// The local sentinel must not have leaked into the published list, and must
	// not be a value the schema knows about either.
	if ReasonUnattributed.IsPublished() {
		t.Error("the unattributed sentinel is in the published vocabulary; it is a client-side value")
	}
	for _, name := range published {
		if name == string(ReasonUnattributed) {
			t.Error("the schema publishes the unattributed sentinel; a server must never send it")
		}
	}
}

// TestEveryPublishedReasonHasAFailureKind makes the reason-to-kind mapping
// total. A reason with no branch would fall through to the default and be
// reported as an ordinary unavailability, which is precisely the guessing the
// taxonomy exists to remove.
func TestEveryPublishedReasonHasAFailureKind(t *testing.T) {
	t.Parallel()

	expected := map[CloseReason]error{
		ReasonGoingAway:            ErrUnavailable,
		ReasonBackendUnavailable:   ErrUnavailable,
		ReasonUnauthenticated:      ErrUnauthenticated,
		ReasonIncompatibleProtocol: ErrIncompatibleProtocol,
		ReasonCapacity:             ErrCapacity,
		ReasonInvalidRequest:       ErrInvalidRequest,
		ReasonInternalError:        ErrInternal,
	}
	for _, reason := range PublishedCloseReasons {
		want, declared := expected[reason]
		if !declared {
			t.Errorf("published reason %q has no expected failure kind in this test", reason)
			continue
		}
		got := FromCloseReason(reason, "")
		if !errors.Is(got, want) {
			t.Errorf("%q maps to %v, want %v", reason, got.Kind, want)
		}
		if got.Reason != reason {
			t.Errorf("%q lost its reason on the way to a failure: %q", reason, got.Reason)
		}
	}
}

// TestRedactionRemovesCredentialsTicketsAndURLs is the gateway half of FR-027.
// The application-level half — that none of this reaches the user's terminal —
// is TestAuthFailureDiagnosticsCarryNoCredentialTicketOrURL in internal/app.
//
// The realistic leak is not a deliberate print: it is net/http embedding the
// request URL in its error string, and that URL carrying a freshly minted
// single-use ticket. So a URL is removed wholesale rather than having its query
// trimmed.
func TestRedactionRemovesCredentialsTicketsAndURLs(t *testing.T) {
	t.Parallel()

	const token = "s3cr3t-bearer-token"
	const ticket = "tkt-abc123"
	raw := `Get "https://gateway.test/ws?ticket=` + ticket + `&protocolVersion=protocol-3": ` +
		`dial tcp: token=` + token + ` refused`

	cleaned := redact(raw, token, ticket)

	for _, secret := range []string{token, ticket, "gateway.test", "?ticket="} {
		if strings.Contains(cleaned, secret) {
			t.Errorf("redaction left %q in: %s", secret, cleaned)
		}
	}
	if !strings.Contains(cleaned, "dial tcp") {
		t.Errorf("redaction removed the part that explains the failure: %s", cleaned)
	}
}

// TestVersionTokenIsSanitizedBeforeItIsPresented stops an untrusted server from
// choosing what the CLI prints.
func TestVersionTokenIsSanitizedBeforeItIsPresented(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		"protocol-4":                    "protocol-4",
		"":                              "<missing>",
		"protocol-abc":                  "<invalid>",
		"protocol-3\nWARNING: run this": "<invalid>",
		"anything else":                 "<invalid>",
	} {
		if got := sanitizeVersionToken(raw); got != want {
			t.Errorf("sanitizeVersionToken(%q) = %q, want %q", raw, got, want)
		}
	}
}
