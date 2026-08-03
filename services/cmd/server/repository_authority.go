package main

import (
	"context"
	"errors"
	"sync"

	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/lib/gitclone"
)

type repositoryAuthorityStore struct {
	mu      sync.Mutex
	pending map[string][]gitclone.Credential
}

func newRepositoryAuthorityStore() *repositoryAuthorityStore {
	return &repositoryAuthorityStore{pending: map[string][]gitclone.Credential{}}
}

func repositoryAuthorityKey(serverID, repositoryID, owner, name string) string {
	return serverID + "\x00" + repositoryID + "\x00" + owner + "\x00" + name
}

func (s *repositoryAuthorityStore) Offer(serverID, repositoryID, owner, name, username, token string) error {
	if s == nil || serverID == "" || repositoryID == "" || owner == "" || name == "" || token == "" {
		return errors.New(gitclone.CodeAuthorityUnavailable)
	}
	key := repositoryAuthorityKey(serverID, repositoryID, owner, name)
	s.mu.Lock()
	s.pending[key] = append(s.pending[key], gitclone.Credential{Username: username, Token: token})
	s.mu.Unlock()
	return nil
}

func (s *repositoryAuthorityStore) Acquire(
	_ context.Context,
	server state.ExecutionServerID,
	repository state.GitHubRepositoryIdentity,
) (gitclone.Credential, string, error) {
	key := repositoryAuthorityKey(string(server), string(repository.ID), string(repository.Owner), string(repository.Name))
	s.mu.Lock()
	defer s.mu.Unlock()
	queue := s.pending[key]
	if len(queue) == 0 {
		return gitclone.Credential{}, gitclone.CodeAuthorityUnavailable, nil
	}
	credential := queue[0]
	queue[0] = gitclone.Credential{}
	if len(queue) == 1 {
		delete(s.pending, key)
	} else {
		s.pending[key] = queue[1:]
	}
	return credential, gitclone.CodeOK, nil
}
