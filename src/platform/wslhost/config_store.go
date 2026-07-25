package wslhost

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrServiceNotStopped is returned when apply is attempted while the service runs.
var ErrServiceNotStopped = errors.New("wslhost: service must be stopped before apply")

// ErrNoSnapshot means the store has no validated generation installed.
var ErrNoSnapshot = errors.New("wslhost: no registration snapshot")

// ConfigStore is the injected filesystem seam for the atomic registration snapshot.
type ConfigStore interface {
	// Load returns the currently installed validated generation.
	Load() (Snapshot, error)
	// AtomicReplace validates and installs snap as the sole runtime config source.
	AtomicReplace(snap Snapshot) error
}

// ACLApplier is the injected ACL seam for config and log paths.
type ACLApplier interface {
	ApplyConfigACL(path, serviceIdentity string) error
	ApplyLogACL(path, serviceIdentity string) error
	ValidateConfigACL(path, serviceIdentity string) error
	ValidateLogACL(path, serviceIdentity string) error
	// Applied returns the last applied ACL entries keyed by path (fakes/tests).
	Applied(path string) []ACLEntry
}

// ServiceGate reports whether SCM has stopped the supervisor unit.
// Apply refuses to replace the snapshot while the service is running.
type ServiceGate interface {
	Stopped() (bool, error)
}

// FileConfigStore persists the snapshot with temp-file, fsync, rename, then ACL.
type FileConfigStore struct {
	Path string
	ACL  ACLApplier
	// SyncFile optionally fsyncs; nil uses the real *os.File Sync.
	SyncFile func(f *os.File) error
}

func (*FileConfigStore) appliesConfigACLAtomically() {}

// Load reads and validates the on-disk snapshot. Partial/invalid files fail closed.
func (s *FileConfigStore) Load() (Snapshot, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, ErrNoSnapshot
		}
		return Snapshot{}, fmt.Errorf("wslhost: read snapshot: %w", err)
	}
	return ParseSnapshot(data)
}

// AtomicReplace writes a temp file in the same directory, fsyncs, renames, and
// applies config ACL. A failed validation never touches the installed generation.
func (s *FileConfigStore) AtomicReplace(snap Snapshot) error {
	data, err := MarshalSnapshot(snap)
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("wslhost: mkdir config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".wsl-host-supervisor-*.tmp")
	if err != nil {
		return fmt.Errorf("wslhost: create temp snapshot: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("wslhost: write temp snapshot: %w", err)
	}
	syncFn := s.SyncFile
	if syncFn == nil {
		syncFn = (*os.File).Sync
	}
	if err := syncFn(tmp); err != nil {
		return fmt.Errorf("wslhost: fsync temp snapshot: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("wslhost: close temp snapshot: %w", err)
	}
	if s.ACL != nil {
		if err := s.ACL.ApplyConfigACL(tmpName, snap.WindowsIdentity); err != nil {
			return fmt.Errorf("wslhost: apply config ACL: %w", err)
		}
		if err := s.ACL.ValidateConfigACL(tmpName, snap.WindowsIdentity); err != nil {
			return fmt.Errorf("wslhost: validate config ACL: %w", err)
		}
	}
	cleanup = false
	if err := replaceFile(tmpName, s.Path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("wslhost: rename snapshot: %w", err)
	}
	if recorder, ok := s.ACL.(interface{ recordRename(string, string) }); ok {
		recorder.recordRename(tmpName, s.Path)
	}
	return nil
}

// MemoryConfigStore is the in-process fake ConfigStore (partial-write / fail inject).
type MemoryConfigStore struct {
	mu        sync.Mutex
	current   *Snapshot
	FailLoad  error
	FailWrite error
	// CorruptRaw, when set, is returned by Load instead of the structured snapshot
	// so adversarial partial JSON can be exercised.
	CorruptRaw []byte
	Writes     int
}

// Load implements ConfigStore.
func (m *MemoryConfigStore) Load() (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailLoad != nil {
		return Snapshot{}, m.FailLoad
	}
	if len(m.CorruptRaw) > 0 {
		return ParseSnapshot(m.CorruptRaw)
	}
	if m.current == nil {
		return Snapshot{}, ErrNoSnapshot
	}
	return *m.current, nil
}

// AtomicReplace implements ConfigStore.
func (m *MemoryConfigStore) AtomicReplace(snap Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailWrite != nil {
		return m.FailWrite
	}
	if err := ValidateSnapshot(snap); err != nil {
		return err
	}
	cp := snap
	m.current = &cp
	m.CorruptRaw = nil
	m.Writes++
	return nil
}

// InstallCorrupt forces a non-authoritative raw payload for Load tests.
func (m *MemoryConfigStore) InstallCorrupt(raw []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CorruptRaw = append([]byte(nil), raw...)
	m.current = nil
}

// RecordingACLApplier is the fake ACLApplier that records intended ACEs.
type RecordingACLApplier struct {
	mu      sync.Mutex
	byPath  map[string][]ACLEntry
	Fail    error
	FailVal error
}

// NewRecordingACLApplier constructs an empty recording applier.
func NewRecordingACLApplier() *RecordingACLApplier {
	return &RecordingACLApplier{byPath: make(map[string][]ACLEntry)}
}

func (a *RecordingACLApplier) ApplyConfigACL(path, serviceIdentity string) error {
	return a.apply(path, ACLPolicyForConfig(serviceIdentity))
}

func (a *RecordingACLApplier) ApplyLogACL(path, serviceIdentity string) error {
	return a.apply(path, ACLPolicyForLog(serviceIdentity))
}

func (a *RecordingACLApplier) apply(path string, policy []ACLEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Fail != nil {
		return a.Fail
	}
	cp := append([]ACLEntry(nil), policy...)
	a.byPath[path] = cp
	return nil
}

func (a *RecordingACLApplier) ValidateConfigACL(path, serviceIdentity string) error {
	return a.validate(path, ACLPolicyForConfig(serviceIdentity))
}

func (a *RecordingACLApplier) ValidateLogACL(path, serviceIdentity string) error {
	return a.validate(path, ACLPolicyForLog(serviceIdentity))
}

func (a *RecordingACLApplier) validate(path string, want []ACLEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.FailVal != nil {
		return a.FailVal
	}
	got := a.byPath[path]
	if !aclEqual(got, want) {
		return fmt.Errorf("wslhost: ACL mismatch for %s", path)
	}
	return nil
}

// Applied implements ACLApplier.
func (a *RecordingACLApplier) Applied(path string) []ACLEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]ACLEntry(nil), a.byPath[path]...)
}

func (a *RecordingACLApplier) recordRename(from, to string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.byPath[to] = append([]ACLEntry(nil), a.byPath[from]...)
	delete(a.byPath, from)
}

func aclEqual(a, b []ACLEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// StaticServiceGate is a test/production stub ServiceGate.
type StaticServiceGate struct {
	IsStopped bool
	Err       error
}

// Stopped implements ServiceGate.
func (g StaticServiceGate) Stopped() (bool, error) {
	if g.Err != nil {
		return false, g.Err
	}
	return g.IsStopped, nil
}

// Registrar owns registration binding and stopped-apply semantics.
type Registrar struct {
	Store           ConfigStore
	ACL             ACLApplier
	Gate            ServiceGate
	ProgramDataRoot string
	// ConfigFile is the absolute snapshot path; empty uses ConfigPath(ProgramDataRoot).
	ConfigFile string
	Now        func() time.Time
}

func (r *Registrar) configPath() string {
	if r.ConfigFile != "" {
		return r.ConfigFile
	}
	return ConfigPath(r.ProgramDataRoot)
}

// Register resolves defaults once, validates, requires a stopped service, and
// atomically installs the generation plus config/log ACLs.
func (r *Registrar) Register(req RegistrationRequest, defaults Defaults) (Snapshot, error) {
	now := time.Now().UTC()
	if r.Now != nil {
		now = r.Now()
	}
	snap, err := BuildSnapshot(req, defaults, r.ProgramDataRoot, now)
	if err != nil {
		return Snapshot{}, err
	}
	if err := r.Apply(snap); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// Apply installs an already-built snapshot only when the service is stopped.
// Runtime must call Load afterwards; it never re-resolves interactive defaults.
func (r *Registrar) Apply(snap Snapshot) error {
	if err := ValidateSnapshot(snap); err != nil {
		return err
	}
	if r.Gate != nil {
		stopped, err := r.Gate.Stopped()
		if err != nil {
			return err
		}
		if !stopped {
			return ErrServiceNotStopped
		}
	}
	if r.ACL != nil {
		if err := os.MkdirAll(filepath.Dir(snap.LogPath), 0o755); err != nil {
			return fmt.Errorf("wslhost: create log directory: %w", err)
		}
		f, err := os.OpenFile(snap.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return fmt.Errorf("wslhost: create log: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("wslhost: close log: %w", err)
		}
		if err := r.ACL.ApplyLogACL(snap.LogPath, snap.WindowsIdentity); err != nil {
			return fmt.Errorf("wslhost: apply log ACL: %w", err)
		}
		if err := r.ACL.ValidateLogACL(snap.LogPath, snap.WindowsIdentity); err != nil {
			return fmt.Errorf("wslhost: validate log ACL: %w", err)
		}
		if _, ok := r.Store.(interface{ appliesConfigACLAtomically() }); !ok {
			cfgPath := r.configPath()
			if err := r.ACL.ApplyConfigACL(cfgPath, snap.WindowsIdentity); err != nil {
				return fmt.Errorf("wslhost: apply config ACL: %w", err)
			}
			if err := r.ACL.ValidateConfigACL(cfgPath, snap.WindowsIdentity); err != nil {
				return fmt.Errorf("wslhost: validate config ACL: %w", err)
			}
		}
	}
	return r.Store.AtomicReplace(snap)
}

// LoadRuntime returns the sole authoritative generation. Callers must not
// consult interactive defaults or wsl --set-default after this returns.
func (r *Registrar) LoadRuntime() (Snapshot, error) {
	return r.Store.Load()
}
