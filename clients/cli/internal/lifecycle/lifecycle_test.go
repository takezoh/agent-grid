package lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/takezoh/agent-grid/clients/cli/internal/fleet"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway"
	"github.com/takezoh/agent-grid/clients/cli/internal/gateway/fakegateway"
	"github.com/takezoh/agent-grid/clients/goclient/clientconfig"
)

// Contract: contract-create-and-attach, contract-explicit-terminate,
// contract-config-and-fleet-identity (unit-lifecycle-commands).

const projectPath = "/repo"

func testServer() clientconfig.Server {
	return clientconfig.Server{
		Index: 0, ID: "srv_test", DisplayName: "one", Enabled: true,
		URL: "https://gateway.test", Auth: clientconfig.Auth{Mode: "none"},
	}
}

// admittedClient builds a real transport over the fake gateway.
func admittedClient(t *testing.T, fake *fakegateway.Fake) *gateway.Client {
	t.Helper()
	client, err := gateway.New(testServer(), fake.Doer(), fake.Dialer())
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if err := client.Admit(context.Background()); err != nil {
		t.Fatalf("admit: %v", err)
	}
	return client
}

// serverWithProfiles is a fake publishing one launch profile.
func serverWithProfiles(profiles ...map[string]any) *fakegateway.Fake {
	fake := fakegateway.New()
	entries := make([]any, 0, len(profiles))
	for _, profile := range profiles {
		entries = append(entries, profile)
	}
	fake.SessionConfig = map[string]any{
		"push_commands":         []string{},
		"base_command_profiles": entries,
		"project_roots":         []string{},
		"project_paths":         []string{projectPath},
		"projects": []any{map[string]any{
			"path": projectPath, "isGit": true, "command_profiles": entries,
		}},
	}
	return fake
}

func claudeProfile() map[string]any {
	return map[string]any{"id": "claude", "label": "Claude Code", "revision": "sha256:abc"}
}

// TestCreateOutcomesAreDistinctObservations is
// ver-create-outcomes-are-distinct-observations.
//
// Four states have to stay apart. Reporting a created-then-attach-failed run as
// a creation failure hides a session that is now running and that nobody is
// watching; collapsing a --no-attach creation with an attach failure either
// exits non-zero for a run that did exactly what was asked, or withholds the
// identifier that was the whole point of asking.
func TestCreateOutcomesAreDistinctObservations(t *testing.T) {
	t.Parallel()

	attachFailure := errors.New("the attach failed")

	t.Run("creation fails", testFailedCreation)

	t.Run("creation succeeds and the attach fails", func(t *testing.T) {
		t.Parallel()
		fake := serverWithProfiles(claudeProfile())
		result := Run(context.Background(), admittedClient(t, fake), Params{
			Project: projectPath, Agent: "claude",
			Attach: func(Created) error { return attachFailure },
		})
		if result.Created == nil {
			t.Fatal("the created session was not reported")
		}
		if result.Created.ID == "" {
			t.Error("the created identifier was withheld")
		}
		if !errors.Is(result.AttachErr, attachFailure) {
			t.Errorf("attach error = %v", result.AttachErr)
		}
		if result.CreateErr != nil {
			t.Error("an attach failure was reported as a creation failure")
		}
	})

	t.Run("no attach was asked for", func(t *testing.T) {
		t.Parallel()
		fake := serverWithProfiles(claudeProfile())
		attachCalled := false
		result := Run(context.Background(), admittedClient(t, fake), Params{
			Project: projectPath, Agent: "claude", NoAttach: true,
			Attach: func(Created) error { attachCalled = true; return nil },
		})
		if attachCalled {
			t.Error("--no-attach attempted an attach")
		}
		if result.Created == nil || result.Created.ID == "" {
			t.Fatal("--no-attach withheld the identifier")
		}
		if result.AttachErr != nil || result.CreateErr != nil {
			t.Errorf("--no-attach reported a failure: %+v", result)
		}
		if result.Attached {
			t.Error("--no-attach reported that an attach happened")
		}
	})

	t.Run("full success", func(t *testing.T) {
		t.Parallel()
		fake := serverWithProfiles(claudeProfile())
		result := Run(context.Background(), admittedClient(t, fake), Params{
			Project: projectPath, Agent: "claude",
			Attach: func(Created) error { return nil },
		})
		if result.Created == nil || result.CreateErr != nil || result.AttachErr != nil {
			t.Fatalf("a complete success reported %+v", result)
		}
		if !result.Attached {
			t.Error("a complete success did not report that it attached")
		}
	})
}

// TestCreateAttachesToTheSessionItJustCreated is ver-attach-follows-created-identifier.
//
// Re-resolving a selector after creation could land on a different session that
// happens to match — which is the one thing a create-then-attach must never do.
func TestCreateAttachesToTheSessionItJustCreated(t *testing.T) {
	t.Parallel()

	fake := serverWithProfiles(claudeProfile())
	fake.CreatedSession = map[string]any{"id": "s_brand_new", "project": projectPath}

	var attachedTo string
	result := Run(context.Background(), admittedClient(t, fake), Params{
		Project: projectPath, Agent: "claude",
		Attach: func(created Created) error { attachedTo = created.ID; return nil },
	})
	if result.Created == nil {
		t.Fatalf("creation failed: %v", result.CreateErr)
	}
	if attachedTo != "s_brand_new" {
		t.Errorf("attached to %q, want the session that was just created", attachedTo)
	}
}

// TestUnresolvableAgentFailsBeforeAnyCreatingRequest is
// ver-unresolvable-agent-creates-no-row.
//
// An implementation that posted first and validated afterwards leaves a session
// behind every time the profile name is wrong, and the operator has to notice
// and clean up something they never meant to start.
func TestUnresolvableAgentFailsBeforeAnyCreatingRequest(t *testing.T) {
	t.Parallel()

	fake := serverWithProfiles(claudeProfile())
	fake.CreatedSession = map[string]any{"id": "s_should_not_exist"}

	result := Run(context.Background(), admittedClient(t, fake), Params{
		Project: projectPath, Agent: "not-a-profile",
		Attach: func(Created) error { return nil },
	})
	if !errors.Is(result.CreateErr, ErrUnknownAgent) {
		t.Fatalf("error = %v, want ErrUnknownAgent", result.CreateErr)
	}
	if result.Created != nil {
		t.Errorf("a session was created for an unresolvable agent: %+v", result.Created)
	}
	// The available names are listed so the operator can correct the typo
	// without going to read a configuration file on the server.
	if !strings.Contains(result.CreateErr.Error(), "claude") {
		t.Errorf("the failure does not say what is available: %v", result.CreateErr)
	}
}

// TestProfilesAreReadFromTheTargetServerWithItsRevision pins the freshness
// rule: the profile set and its server-computed revision are read at invocation
// time, and the revision travels with the creation.
func TestProfilesAreReadFromTheTargetServerWithItsRevision(t *testing.T) {
	t.Parallel()

	fake := serverWithProfiles(claudeProfile())
	client := admittedClient(t, fake)

	profiles, err := LoadProfiles(context.Background(), client, projectPath)
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Revision != "sha256:abc" {
		t.Fatalf("profiles = %+v, want the server's revision carried through", profiles)
	}
	resolved, err := ResolveProfile(profiles, "claude")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if resolved.Revision != "sha256:abc" {
		t.Errorf("resolved revision = %q", resolved.Revision)
	}
}

// TestCreateWithoutQualifierIsUsageErrorAndCreatesNothing is
// ver-create-requires-explicit-server, and
// TestDefaultExecutionIsNeverReadAsACreateTarget is
// ver-default-execution-never-infers-a-create-target. They are the same
// function's two failure modes and are checked together.
func TestCreateWithoutQualifierIsUsageErrorAndCreatesNothing(t *testing.T) {
	t.Parallel()

	single := clientconfig.Snapshot{Servers: []clientconfig.Server{testServer()}}
	snapshot := fleet.Observe(context.Background(), single,
		func(context.Context, clientconfig.Server) (fleet.Observation, []gateway.SessionInfo) {
			return fleet.Observation{Kind: fleet.KindConnected}, nil
		})

	if _, err := SelectCreateTarget(snapshot, ""); !errors.Is(err, ErrTargetRequired) {
		t.Errorf("with one server configured, an empty qualifier = %v, want ErrTargetRequired", err)
	}
	if _, err := SelectCreateTarget(snapshot, "   "); !errors.Is(err, ErrTargetRequired) {
		t.Errorf("a blank qualifier = %v, want ErrTargetRequired", err)
	}
	if member, err := SelectCreateTarget(snapshot, "one"); err != nil {
		t.Errorf("an explicit qualifier failed: %v", err)
	} else if member.ServerID != "srv_test" {
		t.Errorf("resolved to %q", member.ServerID)
	}
}

func TestDefaultExecutionIsNeverReadAsACreateTarget(t *testing.T) {
	t.Parallel()

	// The single configured entry is marked for default execution, which is
	// exactly the shape that would silently satisfy an inference.
	marked := testServer()
	marked.DefaultExecution = true
	configured := clientconfig.Snapshot{Servers: []clientconfig.Server{marked}}
	snapshot := fleet.Observe(context.Background(), configured,
		func(context.Context, clientconfig.Server) (fleet.Observation, []gateway.SessionInfo) {
			return fleet.Observation{Kind: fleet.KindConnected}, nil
		})

	if _, err := SelectCreateTarget(snapshot, ""); !errors.Is(err, ErrTargetRequired) {
		t.Fatalf("a default-execution marker satisfied an empty qualifier: %v", err)
	}

	// And the marker is not read anywhere in this package, so it cannot become
	// a fallback later. The markers are the two ways a read is spelled — the
	// struct field and the JSON key — rather than the bare word, which appears
	// in the comment explaining why this rule exists.
	for _, offending := range packageSourcesContaining(t, ".DefaultExecution", `"default_execution"`) {
		t.Errorf("%s reads the default-execution marker", offending)
	}
}

// TestTerminateAffectsOnlyTheTargetedSession is ver-terminate-affects-only-target.
func TestTerminateAffectsOnlyTheTargetedSession(t *testing.T) {
	t.Parallel()

	fake := fakegateway.New()
	client := admittedClient(t, fake)

	if err := Terminate(context.Background(), client, "s_target"); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	if len(fake.Deletes) != 1 || fake.Deletes[0] != "s_target" {
		t.Errorf("terminations sent: %v, want exactly the target", fake.Deletes)
	}
}

// TestRejectedTerminateExitsNonZeroAndLeavesStateUnchanged is
// ver-rejected-terminate-exits-nonzero.
//
// Reporting success on a rejected request is the failure that matters: the
// operator believes the session is gone, stops looking at it, and it keeps
// running.
func TestRejectedTerminateExitsNonZeroAndLeavesStateUnchanged(t *testing.T) {
	t.Parallel()

	fake := fakegateway.New()
	fake.DeleteStatus = 409
	fake.DeleteCode = gateway.ReasonInvalidRequest
	client := admittedClient(t, fake)

	err := Terminate(context.Background(), client, "s_target")
	if err == nil {
		t.Fatal("a rejected termination reported success")
	}
	var transport *gateway.TransportError
	if !errors.As(err, &transport) {
		t.Fatalf("rejection surfaced as %T", err)
	}
	if transport.Reason != gateway.ReasonInvalidRequest {
		t.Errorf("reason = %q, want the published rejection", transport.Reason)
	}
}

// TestTerminateReportAssertsNoUnreadLifecycleState is
// ver-terminate-report-claims-only-acknowledgement.
//
// Success here means an acknowledgement and nothing else. A report that said
// the session "is now stopped" would be asserting a lifecycle state nobody read
// back — and the retention contract makes that state observable, so claiming it
// without looking is a claim that can be wrong.
func TestTerminateReportAssertsNoUnreadLifecycleState(t *testing.T) {
	t.Parallel()

	// The function returns only an error. There is no lifecycle value to
	// return, so no caller can print one that came from here.
	fake := fakegateway.New()
	client := admittedClient(t, fake)
	if err := Terminate(context.Background(), client, "s_target"); err != nil {
		t.Fatalf("Terminate: %v", err)
	}

	// And nothing in this package reads a session back after terminating.
	for _, offending := range packageSourcesContaining(t, "/api/sessions?", "GET /api/sessions") {
		t.Errorf("%s reads state back to describe a termination's result", offending)
	}
}

// TestTerminateIsTheOnlyRequestThisPackageCanRefuseToRepeat pins that a
// termination is a single deliberate emission rather than something with a
// retry loop that could double-apply.
func TestTerminateIsTheOnlyRequestThisPackageCanRefuseToRepeat(t *testing.T) {
	t.Parallel()

	fake := fakegateway.New()
	fake.DeleteStatus = 503
	fake.DeleteCode = gateway.ReasonBackendUnavailable
	client := admittedClient(t, fake)

	if err := Terminate(context.Background(), client, "s_target"); err == nil {
		t.Fatal("a failing termination reported success")
	}
	if len(fake.Deletes) != 1 {
		t.Errorf("%d termination requests were sent for one call", len(fake.Deletes))
	}
}

// testFailedCreation is the first of the four outcomes: nothing was created and
// nothing was attached to.
func testFailedCreation(t *testing.T) {
	t.Parallel()
	fake := serverWithProfiles(claudeProfile())
	fake.CreateStatus = 500
	fake.CreatedSession = map[string]any{"code": "internal_error", "message": "boom"}
	attachCalled := false
	result := Run(context.Background(), admittedClient(t, fake), Params{
		Project: projectPath, Agent: "claude",
		Attach: func(Created) error { attachCalled = true; return nil },
	})
	if result.Created != nil {
		t.Errorf("a failed creation reported a session: %+v", result.Created)
	}
	if result.CreateErr == nil {
		t.Error("a failed creation reported no error")
	}
	if attachCalled {
		t.Error("an attach was attempted after a failed creation")
	}
}
