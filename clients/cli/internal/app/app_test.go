package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway/fakegateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/tty"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Contract: contract-exit-classification, contract-discovery-projection
// (unit-transport-admission, unit-discovery-render).

const (
	bearerToken = "s3cr3t-bearer-token-do-not-print"
	serverURL   = "https://gateway.test"
)

func configuredFleet() clientconfig.Snapshot {
	return clientconfig.Snapshot{Servers: []clientconfig.Server{{
		Index: 0, ID: "srv_one", DisplayName: "one", Enabled: true, URL: serverURL,
		Auth: clientconfig.Auth{Mode: "bearer", Token: bearerToken},
	}}}
}

func depsWith(doer gateway.Doer, dialer gateway.Dialer) Deps {
	return Deps{
		Doer:   doer,
		Dialer: dialer,
		LoadConfig: func(string) (clientconfig.Snapshot, error) {
			return configuredFleet(), nil
		},
		DefaultConfigDir: func() (string, error) { return "/nonexistent", nil },
		Terminal:         func() (tty.Terminal, error) { return nil, tty.ErrNotATerminal },
	}
}

// leakyDoer fails every request with an error carrying exactly what must never
// be printed: the bearer credential, a minted ticket, and a URL containing
// both. It is modelled on what net/http actually produces — the URL is in the
// error string, not something a call site chose to add.
type leakyDoer struct{}

func (leakyDoer) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New(`Get "` + serverURL + `/ws?ticket=tkt-abc123&token=` +
		bearerToken + `": dial tcp 10.0.0.1:443: connect: connection refused`)
}

// TestAuthFailureDiagnosticsCarryNoCredentialTicketOrURL is
// ver-credential-never-reaches-diagnostics.
//
// The realistic leak is not a deliberate print. It is net/http embedding the
// request URL in its error string, that URL carrying a freshly minted
// single-use ticket, and the CLI passing the error straight through to a
// terminal that is very often being recorded — a CI log, a screen share, a
// support ticket.
func TestAuthFailureDiagnosticsCarryNoCredentialTicketOrURL(t *testing.T) {
	t.Parallel()

	forbidden := []string{bearerToken, "tkt-abc123", "gateway.test", "ticket="}

	for name, argv := range map[string][]string{
		"listing":     {"ls"},
		"json":        {"ls", "--json"},
		"terminating": {"kill", "--id", "s1"},
		"creating":    {"new", "--server", "one", "--agent", "claude", "--cwd", "/repo"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			streams, out, errOut := captureStreams()
			RunWith(depsWith(leakyDoer{}, nil), streams, argv)

			combined := out.String() + errOut.String()
			for _, secret := range forbidden {
				if strings.Contains(combined, secret) {
					t.Errorf("%q reached the user's terminal:\n%s", secret, combined)
				}
			}
			if combined == "" {
				t.Error("the failure produced no diagnostic at all")
			}
		})
	}
}

// TestListingSurvivesAndNamesEveryServer drives `ls` end to end through the
// real transport against the fake gateway.
func TestListingSurvivesAndNamesEveryServer(t *testing.T) {
	t.Parallel()

	fake := fakegateway.New(gateway.SessionInfo{
		ID: "s_live", Project: "/repo", Command: "claude",
		Status: gateway.SessionStatus{Lifecycle: "active", Failure: "none"},
		View: gateway.View{
			Card: gateway.Card{Title: "Wire up the terminal client"}, Model: "claude-opus-5",
		},
	})
	streams, out, errOut := captureStreams()

	class := RunWith(depsWith(fake.Doer(), fake.Dialer()), streams, []string{"ls"})

	if class != ClassSuccess {
		t.Fatalf("ls exited %s: %s", class, errOut.String())
	}
	for _, expected := range []string{"s_live", "Wire up the terminal client", "one", "/repo", "Working"} {
		if !strings.Contains(out.String(), expected) {
			t.Errorf("the listing does not carry %q:\n%s", expected, out.String())
		}
	}
}

// TestJSONListingCarriesSeparateIdentityFields drives the machine-readable
// path end to end.
func TestJSONListingCarriesSeparateIdentityFields(t *testing.T) {
	t.Parallel()

	fake := fakegateway.New(gateway.SessionInfo{
		ID: "s_live", Project: "/repo", Command: "claude",
		Status: gateway.SessionStatus{Lifecycle: "idle", Failure: "none"},
		View:   gateway.View{Card: gateway.Card{Title: "Live"}},
	})
	streams, out, errOut := captureStreams()

	if class := RunWith(depsWith(fake.Doer(), fake.Dialer()), streams,
		[]string{"ls", "--json"}); class != ClassSuccess {
		t.Fatalf("ls --json exited %s: %s", class, errOut.String())
	}

	var document struct {
		SchemaVersion int `json:"schema_version"`
		Sessions      []struct {
			ServerID  string `json:"server_id"`
			SessionID string `json:"session_id"`
			State     string `json:"state"`
		} `json:"sessions"`
		Servers []struct {
			Reachability string `json:"reachability"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("the machine-readable output is not JSON: %v\n%s", err, out.String())
	}
	if document.SchemaVersion != 1 {
		t.Errorf("schema version = %d", document.SchemaVersion)
	}
	if len(document.Sessions) != 1 {
		t.Fatalf("%d sessions", len(document.Sessions))
	}
	if document.Sessions[0].ServerID == "" || document.Sessions[0].SessionID != "s_live" {
		t.Errorf("identity is not carried as two fields: %+v", document.Sessions[0])
	}
	if len(document.Servers) != 1 || document.Servers[0].Reachability != "connected" {
		t.Errorf("server reachability is missing: %+v", document.Servers)
	}
}

// TestConfigurationDirectoryIsNeverWritten is the CLI half of FR-003.
//
// The rule matters because the operator's configuration is theirs: a client
// that rewrote it while reading it could reformat, reorder or drop an entry the
// operator wrote on purpose, and they would find out later.
func TestConfigurationDirectoryIsNeverWritten(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	before := directoryFingerprint(t, directory)

	fake := fakegateway.New()
	deps := depsWith(fake.Doer(), fake.Dialer())
	deps.LoadConfig = clientconfig.Load
	streams, _, _ := captureStreams()

	// The directory is empty, so this fails — which is the point: even the
	// failing path must not create anything.
	RunWith(deps, streams, []string{"ls", "--config-dir", directory})

	if after := directoryFingerprint(t, directory); after != before {
		t.Errorf("the configuration directory changed:\nbefore %v\nafter  %v", before, after)
	}
}

func directoryFingerprint(t *testing.T, directory string) string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(directory, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(directory, path)
		names = append(names, relative)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", directory, err)
	}
	return strings.Join(names, "\n")
}

// TestUnreadableConfigurationIsAUsageFailure pins the one case a fleet is not
// shown at all: the document names no entry, so there is nothing to present and
// the operator has a file to fix.
func TestUnreadableConfigurationIsAUsageFailure(t *testing.T) {
	t.Parallel()

	deps := depsWith(nil, nil)
	deps.LoadConfig = func(string) (clientconfig.Snapshot, error) {
		return clientconfig.Snapshot{}, errors.New("config_invalid: servers.json: servers must be an array")
	}
	streams, _, errOut := captureStreams()

	if class := RunWith(deps, streams, []string{"ls"}); class != ClassUsage {
		t.Errorf("an unreadable configuration exited %s, want %s", class, ClassUsage)
	}
	if !strings.Contains(errOut.String(), "servers.json") {
		t.Errorf("the diagnostic does not name the file:\n%s", errOut.String())
	}
}

// TestAttachWithoutATerminalCreatesNothingAndChangesNothing is the
// application-level half of FR-004.
func TestAttachWithoutATerminalCreatesNothingAndChangesNothing(t *testing.T) {
	t.Parallel()

	fake := fakegateway.New()
	streams, _, errOut := captureStreams()

	class := RunWith(depsWith(fake.Doer(), fake.Dialer()), streams,
		[]string{"attach", "--id", "s_live"})

	if class == ClassSuccess {
		t.Fatal("attach succeeded without a terminal")
	}
	if len(fake.Sockets()) != 0 {
		t.Errorf("%d sockets were opened before the terminal check", len(fake.Sockets()))
	}
	if !strings.Contains(errOut.String(), "terminal") {
		t.Errorf("the diagnostic does not say why:\n%s", errOut.String())
	}
}
