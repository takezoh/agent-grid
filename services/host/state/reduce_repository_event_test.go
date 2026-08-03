package state

import (
	"encoding/json"
	"testing"
)

func TestCreateRepositorySessionEventDispatchesDurablePreparation(t *testing.T) {
	payload, err := json.Marshal(RepositoryCreateParams{
		IntentID: "intent-1", PreparationID: "prep-1", ServerID: "server-1",
		Repository:  GitHubRepositoryIdentity{ID: "42", Owner: "octo", Name: "repo"},
		Fingerprint: "42:server-1",
		Command:     "codex",
	})
	if err != nil {
		t.Fatal(err)
	}
	next, effects := Reduce(New(), EvEvent{
		ConnID: 1, ReqID: "request-1", Event: EventCreateRepositorySession, Payload: payload,
	})
	prep, ok := next.RepositoryPreparations["prep-1"]
	if !ok || prep.Stage != PrepStageConfirmed || prep.ServerID != "server-1" {
		t.Fatalf("preparation = %#v, exists=%v", prep, ok)
	}
	if len(effects) != 3 {
		t.Fatalf("effects = %d, want persist + clone + response", len(effects))
	}
}

func TestGetRepositoryPreparationEventReturnsCurrentStage(t *testing.T) {
	s := New()
	s.RepositoryPreparations["prep-1"] = RepositoryPreparation{
		ID: "prep-1", IntentID: "intent-1", ServerID: "server-1",
		Repository: GitHubRepositoryIdentity{ID: "42", Owner: "octo", Name: "repo"},
		Stage:      PrepStageFailed, FailureCode: PrepFailureCloneFailed, RetryOrigin: RetryOriginClone,
		Revision: 3,
	}
	payload, err := json.Marshal(RepositoryPreparationParams{PreparationID: "prep-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, effects := Reduce(s, EvEvent{
		ConnID: 1, ReqID: "request-2", Event: EventGetRepositoryPreparation, Payload: payload,
	})
	if len(effects) != 1 {
		t.Fatalf("effects = %d, want response", len(effects))
	}
	response, ok := effects[0].(EffSendResponse)
	if !ok {
		t.Fatalf("effect = %T", effects[0])
	}
	reply, ok := response.Body.(RepositoryPreparationReply)
	if !ok || reply.Stage != string(PrepStageFailed) || reply.FailureCode != string(PrepFailureCloneFailed) || reply.Revision != 3 {
		t.Fatalf("reply = %#v", response.Body)
	}
}
