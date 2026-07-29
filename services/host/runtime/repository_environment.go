package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/lib/gitclone"
)

type RepositoryEnvironmentRecord struct {
	ID           state.RepositoryEnvironmentID `json:"id"`
	ServerID     state.ExecutionServerID       `json:"server_id"`
	RepositoryID state.GitHubRepositoryID      `json:"repository_id"`
	ContainerID  string                        `json:"container_id"`
	ConfigGen    string                        `json:"config_generation"`
	SessionsRoot string                        `json:"sessions_root"`
	Ready        bool                          `json:"ready"`
	Disposition  state.EnvironmentDisposition  `json:"disposition"`
}

func EnvironmentKey(server state.ExecutionServerID, repoID state.GitHubRepositoryID, configGen string) string {
	return string(server) + "|" + string(repoID) + "|" + configGen
}

type RepositoryEnvironmentStore interface {
	Save(rec RepositoryEnvironmentRecord) error
	Load(id state.RepositoryEnvironmentID) (RepositoryEnvironmentRecord, error)
	FindReady(server state.ExecutionServerID, repoID state.GitHubRepositoryID, configGen string) (RepositoryEnvironmentRecord, bool, error)
	Delete(id state.RepositoryEnvironmentID) error
}

type MemoryRepositoryEnvironmentStore struct {
	mu    sync.Mutex
	byID  map[state.RepositoryEnvironmentID]RepositoryEnvironmentRecord
	byKey map[string]state.RepositoryEnvironmentID
}

func NewMemoryRepositoryEnvironmentStore() *MemoryRepositoryEnvironmentStore {
	return &MemoryRepositoryEnvironmentStore{
		byID:  map[state.RepositoryEnvironmentID]RepositoryEnvironmentRecord{},
		byKey: map[string]state.RepositoryEnvironmentID{},
	}
}

func (s *MemoryRepositoryEnvironmentStore) Save(rec RepositoryEnvironmentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[rec.ID] = rec
	s.byKey[EnvironmentKey(rec.ServerID, rec.RepositoryID, rec.ConfigGen)] = rec.ID
	return nil
}

func (s *MemoryRepositoryEnvironmentStore) Load(id state.RepositoryEnvironmentID) (RepositoryEnvironmentRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.byID[id]
	if !ok {
		return RepositoryEnvironmentRecord{}, fmt.Errorf("not found")
	}
	return rec, nil
}

func (s *MemoryRepositoryEnvironmentStore) FindReady(server state.ExecutionServerID, repoID state.GitHubRepositoryID, configGen string) (RepositoryEnvironmentRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byKey[EnvironmentKey(server, repoID, configGen)]
	if !ok {
		return RepositoryEnvironmentRecord{}, false, nil
	}
	rec := s.byID[id]
	if !rec.Ready {
		return RepositoryEnvironmentRecord{}, false, nil
	}
	return rec, true, nil
}

func (s *MemoryRepositoryEnvironmentStore) Delete(id state.RepositoryEnvironmentID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.byID[id]; ok {
		delete(s.byKey, EnvironmentKey(rec.ServerID, rec.RepositoryID, rec.ConfigGen))
		delete(s.byID, id)
	}
	return nil
}

type FileRepositoryEnvironmentStore struct {
	Root string
	FS   gitclone.FilesystemProcess
}

func NewFileRepositoryEnvironmentStore(root string, fs gitclone.FilesystemProcess) *FileRepositoryEnvironmentStore {
	if fs == nil {
		fs = gitclone.OSFilesystem{}
	}
	return &FileRepositoryEnvironmentStore{Root: root, FS: fs}
}

func (s *FileRepositoryEnvironmentStore) path(id state.RepositoryEnvironmentID) string {
	return filepath.Join(s.Root, "repository-environments", string(id)+".json")
}

func (s *FileRepositoryEnvironmentStore) Save(rec RepositoryEnvironmentRecord) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := s.FS.MkdirAll(filepath.Dir(s.path(rec.ID)), 0o755); err != nil {
		return err
	}
	return s.FS.WriteFileAtomic(s.path(rec.ID), data, 0o644)
}

func (s *FileRepositoryEnvironmentStore) Load(id state.RepositoryEnvironmentID) (RepositoryEnvironmentRecord, error) {
	b, err := s.FS.ReadFile(s.path(id))
	if err != nil {
		return RepositoryEnvironmentRecord{}, err
	}
	var rec RepositoryEnvironmentRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return RepositoryEnvironmentRecord{}, err
	}
	return rec, nil
}

func (s *FileRepositoryEnvironmentStore) FindReady(server state.ExecutionServerID, repoID state.GitHubRepositoryID, configGen string) (RepositoryEnvironmentRecord, bool, error) {
	return RepositoryEnvironmentRecord{}, false, nil
}

func (s *FileRepositoryEnvironmentStore) Delete(id state.RepositoryEnvironmentID) error {
	return s.FS.RemoveAll(s.path(id))
}

type RepositoryDevcontainer interface {
	Ensure(ctx context.Context, req RepositoryEnvironmentRequest) (RepositoryEnvironmentResult, error)
}

type RepositoryEnvironmentRequest struct {
	ServerID     state.ExecutionServerID
	Repository   state.GitHubRepositoryIdentity
	ConfigGen    string
	SessionsRoot string
	ClonePath    string
}

type RepositoryEnvironmentResult struct {
	Code          string
	EnvironmentID state.RepositoryEnvironmentID
	ContainerID   string
	Reused        bool
	MountScope    string // must be sessions root
	FrameCWD      string // must be clone path
	Diagnostic    string
}

const (
	EnvCodeOK              = "ok"
	EnvCodeStale           = "stale_environment"
	EnvCodeFailed          = "environment_failed"
	EnvCodeUnavailable     = "environment_authority_unavailable"
	EnvCodeIsolationDenied = "isolation_forbidden"
)

type RepositoryEnvironmentManager struct {
	Store        RepositoryEnvironmentStore
	Devcontainer RepositoryDevcontainer
	ConfigGen    string // pinned generation for this daemon config
	SessionsRoot string
}

func (m *RepositoryEnvironmentManager) Ensure(ctx context.Context, server state.ExecutionServerID, repo state.GitHubRepositoryIdentity, clonePath string) (RepositoryEnvironmentResult, error) {
	if m == nil || m.Devcontainer == nil {
		return RepositoryEnvironmentResult{Code: EnvCodeUnavailable}, nil
	}
	gen := m.ConfigGen
	if gen == "" {
		gen = "default"
	}
	if m.Store != nil {
		if rec, ok, err := m.Store.FindReady(server, repo.ID, gen); err == nil && ok {
			return RepositoryEnvironmentResult{
				Code: EnvCodeOK, EnvironmentID: rec.ID, ContainerID: rec.ContainerID,
				Reused: true, MountScope: rec.SessionsRoot, FrameCWD: clonePath,
			}, nil
		}
	}
	res, err := m.Devcontainer.Ensure(ctx, RepositoryEnvironmentRequest{
		ServerID: server, Repository: repo, ConfigGen: gen,
		SessionsRoot: m.SessionsRoot, ClonePath: clonePath,
	})
	if err != nil {
		return RepositoryEnvironmentResult{Code: EnvCodeFailed, Diagnostic: err.Error()}, err
	}
	if res.Code == EnvCodeOK && m.Store != nil {
		_ = m.Store.Save(RepositoryEnvironmentRecord{
			ID: res.EnvironmentID, ServerID: server, RepositoryID: repo.ID,
			ContainerID: res.ContainerID, ConfigGen: gen, SessionsRoot: m.SessionsRoot,
			Ready: true, Disposition: state.EnvDispositionReady,
		})
	}
	if res.MountScope == "" {
		res.MountScope = m.SessionsRoot
	}
	if res.FrameCWD == "" {
		res.FrameCWD = clonePath
	}
	return res, nil
}

type FakeRepositoryDevcontainer struct {
	mu            sync.Mutex
	Calls         []RepositoryEnvironmentRequest
	Ready         map[string]RepositoryEnvironmentRecord
	FailCode      string
	NextContainer int
}

func NewFakeRepositoryDevcontainer() *FakeRepositoryDevcontainer {
	return &FakeRepositoryDevcontainer{Ready: map[string]RepositoryEnvironmentRecord{}}
}

func (f *FakeRepositoryDevcontainer) Ensure(_ context.Context, req RepositoryEnvironmentRequest) (RepositoryEnvironmentResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, req)
	if f.FailCode != "" {
		return RepositoryEnvironmentResult{Code: f.FailCode, Diagnostic: f.FailCode}, nil
	}
	if req.SessionsRoot == "" {
		return RepositoryEnvironmentResult{Code: EnvCodeFailed, Diagnostic: "sessions root required"}, nil
	}
	key := EnvironmentKey(req.ServerID, req.Repository.ID, req.ConfigGen)
	if rec, ok := f.Ready[key]; ok && rec.Ready {
		return RepositoryEnvironmentResult{
			Code: EnvCodeOK, EnvironmentID: rec.ID, ContainerID: rec.ContainerID,
			Reused: true, MountScope: req.SessionsRoot, FrameCWD: req.ClonePath,
		}, nil
	}
	f.NextContainer++
	id := state.RepositoryEnvironmentID(fmt.Sprintf("env-%s-%d", req.Repository.ID, f.NextContainer))
	cid := fmt.Sprintf("ctr-%d", f.NextContainer)
	f.Ready[key] = RepositoryEnvironmentRecord{
		ID: id, ServerID: req.ServerID, RepositoryID: req.Repository.ID,
		ContainerID: cid, ConfigGen: req.ConfigGen, SessionsRoot: req.SessionsRoot, Ready: true,
	}
	return RepositoryEnvironmentResult{
		Code: EnvCodeOK, EnvironmentID: id, ContainerID: cid,
		Reused: false, MountScope: req.SessionsRoot, FrameCWD: req.ClonePath,
	}, nil
}
