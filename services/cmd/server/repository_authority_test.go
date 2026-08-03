package main

import (
	"context"
	"testing"

	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/lib/gitclone"
)

func TestRepositoryAuthorityStoreIsRepositoryBoundAndOneUse(t *testing.T) {
	store := newRepositoryAuthorityStore()
	if err := store.Offer("srv", "42", "octo", "repo", "x-access-token", "secret"); err != nil {
		t.Fatal(err)
	}
	wrong, code, err := store.Acquire(context.Background(), "srv", state.GitHubRepositoryIdentity{
		ID: "43", Owner: "octo", Name: "repo",
	})
	if err != nil || code != gitclone.CodeAuthorityUnavailable || wrong.Token != "" {
		t.Fatalf("wrong identity acquired authority: cred=%#v code=%q err=%v", wrong, code, err)
	}
	credential, code, err := store.Acquire(context.Background(), "srv", state.GitHubRepositoryIdentity{
		ID: "42", Owner: "octo", Name: "repo",
	})
	if err != nil || code != gitclone.CodeOK || credential.Token != "secret" {
		t.Fatalf("first acquire: cred=%#v code=%q err=%v", credential, code, err)
	}
	replayed, code, err := store.Acquire(context.Background(), "srv", state.GitHubRepositoryIdentity{
		ID: "42", Owner: "octo", Name: "repo",
	})
	if err != nil || code != gitclone.CodeAuthorityUnavailable || replayed.Token != "" {
		t.Fatalf("replay acquired authority: cred=%#v code=%q err=%v", replayed, code, err)
	}
}
