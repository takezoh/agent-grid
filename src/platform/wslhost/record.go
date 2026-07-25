package wslhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrRecordDropped means the sink could not persist an event; lifecycle must
// continue (blocked disk / full volume cannot stall supervision).
var ErrRecordDropped = errors.New("wslhost: record dropped")

// ErrForbiddenRecordField is returned when an adversarial free-form payload
// attempts to persist raw streams, env, tokens, or secrets.
var ErrForbiddenRecordField = errors.New("wslhost: forbidden record field")

// ErrUnknownRecordEvent rejects unallowlisted event names.
var ErrUnknownRecordEvent = errors.New("wslhost: unknown record event")

// Operator-readable event names (contract-user-observability).
const (
	EventServiceStart   = "service_start"
	EventServiceStop    = "service_stop"
	EventReady          = "ready"
	EventBackoff        = "backoff"
	EventFailed         = "failed"
	EventAlreadyRunning = "already_running"
	EventChildExit      = "child_exit"
	EventEscalation     = "escalation"
	EventBudgetExpiry   = "budget_expiry"
	EventResume         = "resume"
	EventProbe          = "probe"
	EventSpawn          = "spawn"
)

// CanonicalProgramDataLogHint is the operator-facing path template.
const CanonicalProgramDataLogHint = `%ProgramData%\AgentGrid\logs\wsl-host.log`

// Record is the sole structured payload written to the machine-wide sink.
// Fields are intentionally closed: there is no stdout/stderr/env slot.
type Record struct {
	Time            string   `json:"time"`
	Event           string   `json:"event"`
	Generation      string   `json:"generation,omitempty"`
	WindowsIdentity string   `json:"windows_identity,omitempty"`
	Distribution    string   `json:"distribution,omitempty"`
	LinuxUser       string   `json:"linux_user,omitempty"`
	WSLArgs         []string `json:"wsl_args,omitempty"` // non-secret argv only
	ChildPID        int      `json:"child_pid,omitempty"`
	Readiness       string   `json:"readiness,omitempty"` // e.g. ok / not_ready
	ExitCode        *int     `json:"exit_code,omitempty"`
	RetryCount      int      `json:"retry_count,omitempty"`
	Attempt         int      `json:"attempt,omitempty"`
	BackoffSeconds  int      `json:"backoff_seconds,omitempty"`
	BudgetMS        int      `json:"budget_ms,omitempty"`
	Escalation      string   `json:"escalation,omitempty"` // child_kill | terminate_distro
	FailureClass    string   `json:"failure_class,omitempty"`
	Reason          string   `json:"reason,omitempty"` // short code, never raw stream
	DroppedCount    uint64   `json:"dropped_count,omitempty"`
}

// RecordSink is the injected filesystem/record seam.
type RecordSink interface {
	// Write persists one allowlisted event. Disk failure returns ErrRecordDropped
	// (or nil after counting) and must not block the lifecycle caller beyond
	// a bounded attempt.
	Write(rec Record) error
	// Close flushes and releases the active file handle.
	Close() error
	// Dropped reports how many events were discarded due to sink failure.
	Dropped() uint64
}

// SinkRecorder projects lifecycle events into the persistent operator record
// schema. Sink failures are deliberately non-blocking for supervision.
type SinkRecorder struct {
	snap Snapshot
	sink RecordSink
}

func NewSinkRecorder(snap Snapshot, sink RecordSink) Recorder {
	return &SinkRecorder{snap: snap, sink: sink}
}

func (r *SinkRecorder) Record(event LifecycleEvent) {
	if r == nil || r.sink == nil {
		return
	}
	rec := RecordFromSnapshot(r.snap, operatorEvent(event.Kind))
	rec.Time = event.At.UTC().Format(time.RFC3339Nano)
	rec.Attempt = event.Attempt
	rec.ExitCode = event.ExitCode
	rec.FailureClass = event.Class
	rec.Reason = event.Reason
	if event.DelayMS > 0 {
		if event.Kind == KindBudgetExpiry {
			rec.BudgetMS = event.DelayMS
		} else {
			rec.BackoffSeconds = event.DelayMS / 1000
		}
	}
	if event.Kind == KindEscalation {
		rec.Escalation = event.Reason
		rec.Reason = event.Setting
	}
	_ = r.sink.Write(rec)
}

func operatorEvent(kind EventKind) string {
	switch kind {
	case KindStart:
		return EventServiceStart
	case KindReady:
		return EventReady
	case KindBackoff:
		return EventBackoff
	case KindFailed:
		return EventFailed
	case KindAlreadyRunning, KindAdopt:
		return EventAlreadyRunning
	case KindChildExit:
		return EventChildExit
	case KindResume:
		return EventResume
	case KindSpawn:
		return EventSpawn
	case KindServiceStop:
		return EventServiceStop
	case KindBudgetExpiry:
		return EventBudgetExpiry
	case KindEscalation:
		return EventEscalation
	default:
		return EventProbe
	}
}

// AllowedEvents is the closed set of event names the sink will persist.
func AllowedEvents() map[string]struct{} {
	return map[string]struct{}{
		EventServiceStart:   {},
		EventServiceStop:    {},
		EventReady:          {},
		EventBackoff:        {},
		EventFailed:         {},
		EventAlreadyRunning: {},
		EventChildExit:      {},
		EventEscalation:     {},
		EventBudgetExpiry:   {},
		EventResume:         {},
		EventProbe:          {},
		EventSpawn:          {},
	}
}

// ForbiddenRecordKeys must never appear in free-form adversarial payloads.
func ForbiddenRecordKeys() map[string]struct{} {
	return map[string]struct{}{
		"stdout": {}, "stderr": {}, "env": {}, "environment": {},
		"environ": {}, "token": {}, "secret": {}, "password": {},
		"api_key": {}, "apikey": {}, "authorization": {}, "raw_output": {},
	}
}

// ValidateRecord enforces the event allowlist and rejects empty events.
func ValidateRecord(rec Record) error {
	if strings.TrimSpace(rec.Event) == "" {
		return fmt.Errorf("%w: empty event", ErrUnknownRecordEvent)
	}
	if _, ok := AllowedEvents()[rec.Event]; !ok {
		return fmt.Errorf("%w: %s", ErrUnknownRecordEvent, rec.Event)
	}
	return nil
}

// EncodeRecord validates and marshals a single JSON line (no trailing issues
// beyond one '\n'). Only Record struct tags are emitted.
func EncodeRecord(rec Record) ([]byte, error) {
	if err := ValidateRecord(rec); err != nil {
		return nil, err
	}
	if strings.TrimSpace(rec.Time) == "" {
		rec.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	// Belt-and-suspenders: refuse if forbidden keys leaked into the object.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	for k := range probe {
		if _, bad := ForbiddenRecordKeys()[strings.ToLower(k)]; bad {
			return nil, fmt.Errorf("%w: %s", ErrForbiddenRecordField, k)
		}
	}
	out := make([]byte, 0, len(raw)+1)
	out = append(out, raw...)
	out = append(out, '\n')
	return out, nil
}

// RejectForbiddenMap fails closed on adversarial free-form field maps.
func RejectForbiddenMap(fields map[string]any) error {
	for k := range fields {
		if _, bad := ForbiddenRecordKeys()[strings.ToLower(k)]; bad {
			return fmt.Errorf("%w: %s", ErrForbiddenRecordField, k)
		}
	}
	return nil
}

// RecordFromSnapshot stamps registration-bound identity fields onto a record.
func RecordFromSnapshot(snap Snapshot, event string) Record {
	return Record{
		Time:            time.Now().UTC().Format(time.RFC3339Nano),
		Event:           event,
		Generation:      snap.Generation,
		WindowsIdentity: snap.WindowsIdentity,
		Distribution:    snap.Distribution,
		LinuxUser:       snap.LinuxUser,
	}
}

// --- File sink (ProgramData JSONL + rotation) ---

// FileRecordSink writes allowlisted JSON lines under the bound log path and
// enforces 10 MiB × 5 files / 30-day retention.
type FileRecordSink struct {
	Path            string
	ACL             ACLApplier
	ServiceIdentity string
	Policy          LogRetentionPolicy
	Now             func() time.Time

	mu      sync.Mutex
	file    *os.File
	size    int64
	dropped uint64
	// FailWrite, when set, forces the next write to drop (tests / injection).
	FailWrite error
}

// OpenFileRecordSink creates the log directory, applies log ACL, and opens the sink.
func OpenFileRecordSink(path, serviceIdentity string, acl ACLApplier, policy LogRetentionPolicy) (*FileRecordSink, error) {
	if policy.MaxBytes <= 0 {
		policy = DefaultLogRetention()
	}
	s := &FileRecordSink{
		Path:            path,
		ACL:             acl,
		ServiceIdentity: serviceIdentity,
		Policy:          policy,
		Now:             func() time.Time { return time.Now().UTC() },
	}
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileRecordSink) Write(rec Record) error {
	line, err := EncodeRecord(rec)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailWrite != nil {
		s.dropped++
		return ErrRecordDropped
	}
	if err := s.ensureOpenLocked(); err != nil {
		s.dropped++
		return ErrRecordDropped
	}
	if s.size+int64(len(line)) > s.Policy.MaxBytes && s.size > 0 {
		if err := s.rotateLocked(); err != nil {
			s.dropped++
			return ErrRecordDropped
		}
	}
	n, err := s.file.Write(line)
	if err != nil {
		s.dropped++
		return ErrRecordDropped
	}
	s.size += int64(n)
	return nil
}

func (s *FileRecordSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

func (s *FileRecordSink) Dropped() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

func (s *FileRecordSink) ensureOpen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureOpenLocked()
}

func (s *FileRecordSink) ensureOpenLocked() error {
	if s.file != nil {
		return nil
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	s.file = f
	s.size = st.Size()
	if s.ACL != nil {
		if err := s.ACL.ApplyLogACL(s.Path, s.ServiceIdentity); err != nil {
			_ = s.file.Close()
			s.file = nil
			return err
		}
		if err := s.ACL.ValidateLogACL(s.Path, s.ServiceIdentity); err != nil {
			_ = s.file.Close()
			s.file = nil
			return err
		}
	}
	_ = s.pruneByAgeLocked()
	return nil
}

func (s *FileRecordSink) rotateLocked() error {
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
	// Keep at most Policy.RetainFiles total (active + backups).
	backups := s.Policy.RetainFiles - 1
	if backups < 0 {
		backups = 0
	}
	if backups > 0 {
		oldest := fmt.Sprintf("%s.%d", s.Path, backups)
		_ = os.Remove(oldest)
		for i := backups - 1; i >= 1; i-- {
			from := fmt.Sprintf("%s.%d", s.Path, i)
			to := fmt.Sprintf("%s.%d", s.Path, i+1)
			if _, err := os.Stat(from); err == nil {
				_ = os.Rename(from, to)
			}
		}
		_ = os.Rename(s.Path, s.Path+".1")
	} else {
		_ = os.Remove(s.Path)
	}
	_ = s.pruneByAgeLocked()
	f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	s.file = f
	s.size = 0
	if s.ACL != nil {
		if err := s.ACL.ApplyLogACL(s.Path, s.ServiceIdentity); err != nil {
			_ = s.file.Close()
			s.file = nil
			return err
		}
		if err := s.ACL.ValidateLogACL(s.Path, s.ServiceIdentity); err != nil {
			_ = s.file.Close()
			s.file = nil
			return err
		}
	}
	return nil
}

func (s *FileRecordSink) pruneByAgeLocked() error {
	days := s.Policy.RetainDays
	if days <= 0 {
		return nil
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now()
	}
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	dir := filepath.Dir(s.Path)
	base := filepath.Base(s.Path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name != base && !strings.HasPrefix(name, base+".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().UTC().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
	return nil
}

// ListLogFiles returns active + rotated log paths for tests/operators.
func ListLogFiles(path string) ([]string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if name == base || strings.HasPrefix(name, base+".") {
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Strings(out)
	return out, nil
}

// --- Memory fake ---

// MemoryRecordSink is the in-process fake with failure injection and rotation
// simulation for the ConfigStore+ACL+RecordSink triple.
type MemoryRecordSink struct {
	mu      sync.Mutex
	Records []Record
	// Files simulates rotated generations: index 0 = active.
	Files     [][]Record
	Policy    LogRetentionPolicy
	FailWrite error
	// MaxActiveBytes when >0 simulates size rotation against encoded length.
	MaxActiveBytes int64
	activeBytes    int64
	dropped        uint64
	Now            func() time.Time
	// AppliedACL records the last log ACL path for FakeVsReal comparison.
	LogPath string
	ACL     ACLApplier
	Ident   string
}

// NewMemoryRecordSink constructs a fake sink with default retention.
func NewMemoryRecordSink() *MemoryRecordSink {
	return &MemoryRecordSink{
		Policy: DefaultLogRetention(),
		Files:  [][]Record{{}},
		Now:    func() time.Time { return time.Now().UTC() },
	}
}

// Write implements RecordSink.
func (m *MemoryRecordSink) Write(rec Record) error {
	line, err := EncodeRecord(rec)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailWrite != nil {
		m.dropped++
		return ErrRecordDropped
	}
	if m.MaxActiveBytes <= 0 {
		m.MaxActiveBytes = m.Policy.MaxBytes
	}
	if m.activeBytes+int64(len(line)) > m.MaxActiveBytes && m.activeBytes > 0 {
		m.rotateLocked()
	}
	if len(m.Files) == 0 {
		m.Files = [][]Record{{}}
	}
	m.Files[0] = append(m.Files[0], rec)
	m.Records = append(m.Records, rec)
	m.activeBytes += int64(len(line))
	if m.ACL != nil && m.LogPath != "" {
		_ = m.ACL.ApplyLogACL(m.LogPath, m.Ident)
	}
	return nil
}

func (m *MemoryRecordSink) rotateLocked() {
	// Prepend new empty active file; drop beyond RetainFiles.
	m.Files = append([][]Record{{}}, m.Files...)
	if len(m.Files) > m.Policy.RetainFiles {
		m.Files = m.Files[:m.Policy.RetainFiles]
	}
	m.activeBytes = 0
}

// Close implements RecordSink.
func (m *MemoryRecordSink) Close() error { return nil }

// Dropped implements RecordSink.
func (m *MemoryRecordSink) Dropped() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dropped
}

// ActiveFileCount returns how many logical log files the fake retains.
func (m *MemoryRecordSink) ActiveFileCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Files)
}

// --- SCM pre-start complement (process never created) ---

// SCMPreStartLookup describes the deterministic operator path when SCM cannot
// create the supervisor process (credential / Log on as a service / ACL).
// The dedicated ProgramData file is not authoritative for pre-process failures.
type SCMPreStartLookup struct {
	ServiceName     string   `json:"service_name"`
	PrimaryCommand  string   `json:"primary_command"`
	EventLogSource  string   `json:"event_log_source"`
	EventLogChannel string   `json:"event_log_channel"`
	Steps           []string `json:"steps"`
	LogPathHint     string   `json:"log_path_hint"`
}

// LookupSCMPreStartFailure returns the fixed operator lookup for FR-WSL-007.
// serviceName is the registered Windows service name.
func LookupSCMPreStartFailure(serviceName string) SCMPreStartLookup {
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "agent-grid-wsl-supervisor"
	}
	return SCMPreStartLookup{
		ServiceName:     serviceName,
		PrimaryCommand:  fmt.Sprintf("sc query %s", serviceName),
		EventLogSource:  "Service Control Manager",
		EventLogChannel: "System",
		LogPathHint:     CanonicalProgramDataLogHint,
		Steps: []string{
			fmt.Sprintf("Run: sc query %s — inspect STATE and WIN32_EXIT_CODE for credential/access failures.", serviceName),
			"Open Windows Event Viewer → Windows Logs → System; filter Source = Service Control Manager.",
			fmt.Sprintf("Match events for service %q (start failure / logon failure / access denied).", serviceName),
			"Classify as credential or access (Log on as a service, profile, executable ACL, config ACL).",
			fmt.Sprintf("Do not claim success from a missing or silent %s; pre-process failures never write a generation-start event there.", CanonicalProgramDataLogHint),
		},
	}
}
