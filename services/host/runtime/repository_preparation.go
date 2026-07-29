package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/takezoh/agent-grid/services/host/state"
	"github.com/takezoh/agent-grid/services/platform/lib/gitclone"
)

const (
	PrepCapActivePerServer     = 4
	PrepCapQueuedPerServer     = 32
	PrepCapActivePerRepository = 2
	PrepCapNeverAvailableBytes = int64(20) << 30 // 20 GiB
	PrepCapCleanupAge          = 24 * time.Hour
)

type PreparationRecord struct {
	Preparation state.RepositoryPreparation `json:"preparation"`
	Intent      state.CreationIntent        `json:"intent"`
	Marker      string                      `json:"marker"`
	StagingDir  string                      `json:"staging_dir,omitempty"`
	FinalDir    string                      `json:"final_dir,omitempty"`
	UpdatedAt   time.Time                   `json:"updated_at"`
}

type PreparationStore interface {
	Save(rec PreparationRecord) error
	Load(id state.PreparationID) (PreparationRecord, error)
	List() ([]PreparationRecord, error)
	Delete(id state.PreparationID) error
}

type FilePreparationStore struct {
	Root string
	FS   gitclone.FilesystemProcess
}

func NewFilePreparationStore(root string, fs gitclone.FilesystemProcess) *FilePreparationStore {
	if fs == nil {
		fs = gitclone.OSFilesystem{}
	}
	return &FilePreparationStore{Root: root, FS: fs}
}

func (s *FilePreparationStore) path(id state.PreparationID) string {
	return filepath.Join(s.Root, "preparations", string(id)+".json")
}

func (s *FilePreparationStore) Save(rec PreparationRecord) error {
	if rec.Preparation.ID == "" {
		return fmt.Errorf("preparation id required")
	}
	rec.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(s.Root, "preparations")
	if err := s.FS.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return s.FS.WriteFileAtomic(s.path(rec.Preparation.ID), data, 0o644)
}

func (s *FilePreparationStore) Load(id state.PreparationID) (PreparationRecord, error) {
	b, err := s.FS.ReadFile(s.path(id))
	if err != nil {
		return PreparationRecord{}, err
	}
	var rec PreparationRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return PreparationRecord{}, err
	}
	return rec, nil
}

func (s *FilePreparationStore) List() ([]PreparationRecord, error) {
	dir := filepath.Join(s.Root, "preparations")
	entries, err := listJSONNames(s.FS, dir)
	if err != nil {
		return nil, err
	}
	out := make([]PreparationRecord, 0, len(entries))
	for _, name := range entries {
		id := state.PreparationID(name[:len(name)-len(".json")])
		rec, err := s.Load(id)
		if err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (s *FilePreparationStore) Delete(id state.PreparationID) error {
	return s.FS.RemoveAll(s.path(id))
}

type MemoryPreparationStore struct {
	mu   sync.Mutex
	recs map[state.PreparationID]PreparationRecord
}

func NewMemoryPreparationStore() *MemoryPreparationStore {
	return &MemoryPreparationStore{recs: map[state.PreparationID]PreparationRecord{}}
}

func (s *MemoryPreparationStore) Save(rec PreparationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.UpdatedAt = time.Now().UTC()
	s.recs[rec.Preparation.ID] = rec
	return nil
}

func (s *MemoryPreparationStore) Load(id state.PreparationID) (PreparationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.recs[id]
	if !ok {
		return PreparationRecord{}, fmt.Errorf("not found")
	}
	return rec, nil
}

func (s *MemoryPreparationStore) List() ([]PreparationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PreparationRecord, 0, len(s.recs))
	for _, r := range s.recs {
		out = append(out, r)
	}
	return out, nil
}

func (s *MemoryPreparationStore) Delete(id state.PreparationID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.recs, id)
	return nil
}

type CloneAuthority interface {
	Acquire(ctx context.Context, server state.ExecutionServerID, repo state.GitHubRepositoryIdentity) (gitclone.Credential, string, error)
}

type PreparationCapacity struct {
	ActiveGlobal    int
	QueuedGlobal    int
	ActiveByRepo    map[string]int
	NeverAvailBytes int64
	ProcessByIntent map[state.CreationIntentID]int
	StagingByIntent map[state.CreationIntentID]int
}

func NewPreparationCapacity() *PreparationCapacity {
	return &PreparationCapacity{
		ActiveByRepo:    map[string]int{},
		ProcessByIntent: map[state.CreationIntentID]int{},
		StagingByIntent: map[state.CreationIntentID]int{},
	}
}

func repoKey(server state.ExecutionServerID, repo state.GitHubRepositoryIdentity) string {
	return string(server) + "/" + string(repo.ID)
}

func (c *PreparationCapacity) AdmitClone(server state.ExecutionServerID, repo state.GitHubRepositoryIdentity, intent state.CreationIntentID, cloneBytes int64) string {
	if c.ProcessByIntent[intent] > 0 || c.StagingByIntent[intent] > 0 {
		return "intent_worker_busy"
	}
	if c.ActiveGlobal >= PrepCapActivePerServer {
		if c.QueuedGlobal >= PrepCapQueuedPerServer {
			return "capacity_queued_full"
		}
		return "capacity_active_full"
	}
	if c.ActiveByRepo[repoKey(server, repo)] >= PrepCapActivePerRepository {
		return "capacity_repository_active_full"
	}
	if c.NeverAvailBytes+cloneBytes > PrepCapNeverAvailableBytes {
		return "capacity_disk_full"
	}
	return ""
}

func (c *PreparationCapacity) BeginActive(server state.ExecutionServerID, repo state.GitHubRepositoryIdentity, intent state.CreationIntentID) {
	c.ActiveGlobal++
	c.ActiveByRepo[repoKey(server, repo)]++
	c.ProcessByIntent[intent] = 1
	c.StagingByIntent[intent] = 1
}

func (c *PreparationCapacity) EndActive(server state.ExecutionServerID, repo state.GitHubRepositoryIdentity, intent state.CreationIntentID) {
	if c.ActiveGlobal > 0 {
		c.ActiveGlobal--
	}
	k := repoKey(server, repo)
	if c.ActiveByRepo[k] > 0 {
		c.ActiveByRepo[k]--
	}
	delete(c.ProcessByIntent, intent)
	delete(c.StagingByIntent, intent)
}

func (c *PreparationCapacity) Enqueue() {
	c.QueuedGlobal++
}

func (c *PreparationCapacity) Dequeue() {
	if c.QueuedGlobal > 0 {
		c.QueuedGlobal--
	}
}

func (c *PreparationCapacity) AddNeverAvailable(n int64) {
	if n > 0 {
		c.NeverAvailBytes += n
	}
}

func (c *PreparationCapacity) RemoveNeverAvailable(n int64) {
	if n <= 0 {
		return
	}
	c.NeverAvailBytes -= n
	if c.NeverAvailBytes < 0 {
		c.NeverAvailBytes = 0
	}
}

// DirBytes sums file sizes under path (best-effort; missing path → 0).
func DirBytes(root string) int64 {
	if root == "" {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func listJSONNames(fs gitclone.FilesystemProcess, dir string) ([]string, error) {
	if _, ok := fs.(gitclone.OSFilesystem); !ok {
		return nil, nil
	}
	entries, err := readDirNames(dir)
	if err != nil {
		return nil, err
	}
	return entries, nil
}
