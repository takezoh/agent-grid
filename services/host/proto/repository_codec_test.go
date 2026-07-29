package proto

import (
	"encoding/json"
	"testing"
)

func TestSessionTargetRoundTripAndRejection(t *testing.T) {
	cases := []SessionTargetWire{
		{Source: TargetSourceProject, Project: &ProjectTargetWire{ServerID: "srv-1", Project: "/workspace/demo"}},
		{Source: TargetSourceRepository, Repository: &GitHubRepositoryWire{ID: "gh-99", Owner: "acme", Name: "demo"}},
	}
	for _, want := range cases {
		raw, err := EncodeSessionTarget(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeSessionTarget(raw)
		if err != nil || got.Source != want.Source {
			t.Fatalf("got %+v err %v", got, err)
		}
	}
	// rejects counterexample: incomplete repository / collapsed dual identity
	if _, err := EncodeSessionTarget(SessionTargetWire{
		Source: TargetSourceRepository, Repository: &GitHubRepositoryWire{Owner: "acme", Name: "demo"},
	}); err == nil {
		t.Fatal("missing repository_id")
	}
	if _, err := EncodeSessionTarget(SessionTargetWire{
		Source: TargetSourceProject, Project: &ProjectTargetWire{ServerID: "s", Project: "p"},
		Repository: &GitHubRepositoryWire{ID: "1", Owner: "o", Name: "n"},
	}); err == nil {
		t.Fatal("collapsed dual identity")
	}
}

func TestRepositoryPreparationRoundTripAvailabilityInvariant(t *testing.T) {
	nonAvail := RepositoryPreparationWire{
		PreparationID: "prep-1", IntentID: "intent-1", ServerID: "srv-1",
		Repository: GitHubRepositoryWire{ID: "1", Owner: "a", Name: "b"},
		Stage:      PrepWireCloneReady, Revision: 3, ClonePath: "/clones/1",
	}
	raw, err := EncodeRepositoryPreparation(nonAvail)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeRepositoryPreparation(raw)
	if err != nil || got.SessionID != "" {
		t.Fatalf("%+v %v", got, err)
	}
	// rejects counterexample: Session at clone-ready
	bad := nonAvail
	bad.SessionID = "sess-1"
	if _, err := EncodeRepositoryPreparation(bad); err == nil {
		t.Fatal("session_id on non-available")
	}
	avail := nonAvail
	avail.Stage, avail.SessionID, avail.EnvironmentID = PrepWireAvailable, "sess-1", "env-1"
	raw, err = EncodeRepositoryPreparation(avail)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	repo := m["repository"].(map[string]any)
	if repo["repository_id"] != "1" || repo["owner"] != "a" {
		t.Fatalf("identity collapsed: %#v", repo)
	}
}
