// Package wslhost owns the Windows WSL host-server supervisor configuration
// surface. This file holds the versioned registration snapshot schema, pure
// validation, registration-time default binding, and ACL policy model.
package wslhost

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// SchemaVersionV1 is the only supported registration snapshot schema.
const SchemaVersionV1 = 1

// Canonical relative paths under the machine-wide ProgramData root.
const (
	RelativeConfigPath = "AgentGrid/wsl-host-supervisor.json"
	RelativeLogPath    = "AgentGrid/logs/wsl-host.log"
)

// TLS mode values persisted in the snapshot.
const (
	TLSModeTLS      = "tls"
	TLSModeInsecure = "insecure"
)

// Fixed retention policy for the machine-wide log. RecordSink enforces these
// bounds (10 MiB active size, five files total, thirty-day age prune).
const (
	LogRotateBytes        int64 = 10 * 1024 * 1024
	LogRetainFiles              = 5
	LogRetainDays               = 30
	DefaultGracefulStopMS       = 15_000
)

// Snapshot is the sole runtime authority for a registered generation.
// Wire and persistence remain stdlib JSON only.
type Snapshot struct {
	SchemaVersion    int        `json:"schema_version"`
	Generation       string     `json:"generation"`
	WindowsIdentity  string     `json:"windows_identity"`
	Distribution     string     `json:"distribution"`
	LinuxUser        string     `json:"linux_user"`
	ServerExecutable string     `json:"server_executable"`
	DataDir          string     `json:"data_dir"`
	Addr             string     `json:"addr"`
	TLSMode          string     `json:"tls_mode"`
	HealthURL        string     `json:"health_url"`
	LogPath          string     `json:"log_path"`
	Thresholds       Thresholds `json:"thresholds"`
	RegisteredAt     string     `json:"registered_at"`
}

// Thresholds are fixed numerical policy values projected into the snapshot at
// registration. Runtime must not re-derive them from interactive defaults.
type Thresholds struct {
	HealthProbeTimeoutMS       int   `json:"health_probe_timeout_ms"`
	HealthCadenceMS            int   `json:"health_cadence_ms"`
	StartupBudgetMS            int   `json:"startup_budget_ms"`
	ResumeRecheckMS            int   `json:"resume_recheck_ms"`
	HealthRestartThreshold     int   `json:"health_restart_threshold"`
	ConfigTerminalThreshold    int   `json:"config_terminal_threshold"`
	TransientTerminalThreshold int   `json:"transient_terminal_threshold"`
	GracefulStopBudgetMS       int   `json:"graceful_stop_budget_ms"`
	BackoffSeconds             []int `json:"backoff_seconds"`
}

// RegistrationRequest is operator input. Empty identity/distribution are
// resolved once from Defaults at registration; runtime never re-resolves them.
type RegistrationRequest struct {
	WindowsIdentity  string
	Distribution     string
	LinuxUser        string
	ServerExecutable string
	DataDir          string
	Addr             string
	TLSMode          string
	HealthURL        string
	LogPath          string
}

// Defaults are registration-time only sources (current login / wsl --set-default).
type Defaults struct {
	WindowsIdentity string
	Distribution    string
}

// DefaultThresholds returns the ADR-fixed health/retry/stop budgets.
func DefaultThresholds() Thresholds {
	return Thresholds{
		HealthProbeTimeoutMS:       2_000,
		HealthCadenceMS:            5_000,
		StartupBudgetMS:            60_000,
		ResumeRecheckMS:            10_000,
		HealthRestartThreshold:     3,
		ConfigTerminalThreshold:    5,
		TransientTerminalThreshold: 10,
		GracefulStopBudgetMS:       DefaultGracefulStopMS,
		BackoffSeconds:             []int{1, 2, 5, 15, 60},
	}
}

// ConfigPath joins programDataRoot with the canonical config relative path.
func ConfigPath(programDataRoot string) string {
	return filepath.Join(programDataRoot, filepath.FromSlash(RelativeConfigPath))
}

// DefaultLogPath joins programDataRoot with the canonical log relative path.
func DefaultLogPath(programDataRoot string) string {
	return filepath.Join(programDataRoot, filepath.FromSlash(RelativeLogPath))
}

// BuildSnapshot resolves omitted identity/distribution once, stamps thresholds,
// mints a generation fingerprint, and validates the resulting snapshot.
func BuildSnapshot(req RegistrationRequest, defaults Defaults, programDataRoot string, now time.Time) (Snapshot, error) {
	identity := strings.TrimSpace(req.WindowsIdentity)
	if identity == "" {
		identity = strings.TrimSpace(defaults.WindowsIdentity)
	}
	distro := strings.TrimSpace(req.Distribution)
	if distro == "" {
		distro = strings.TrimSpace(defaults.Distribution)
	}
	addr := strings.TrimSpace(req.Addr)
	if addr == "" {
		addr = ":8443"
	}
	tlsMode := strings.TrimSpace(req.TLSMode)
	if tlsMode == "" {
		tlsMode = TLSModeTLS
	}
	logPath := strings.TrimSpace(req.LogPath)
	if logPath == "" {
		logPath = DefaultLogPath(programDataRoot)
	}
	healthURL := strings.TrimSpace(req.HealthURL)
	if healthURL == "" {
		var err error
		healthURL, err = DeriveHealthURL(addr, tlsMode)
		if err != nil {
			return Snapshot{}, err
		}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	snap := Snapshot{
		SchemaVersion:    SchemaVersionV1,
		WindowsIdentity:  identity,
		Distribution:     distro,
		LinuxUser:        strings.TrimSpace(req.LinuxUser),
		ServerExecutable: strings.TrimSpace(req.ServerExecutable),
		DataDir:          strings.TrimSpace(req.DataDir),
		Addr:             addr,
		TLSMode:          tlsMode,
		HealthURL:        healthURL,
		LogPath:          logPath,
		Thresholds:       DefaultThresholds(),
		RegisteredAt:     now.UTC().Format(time.RFC3339Nano),
	}
	snap.Generation = GenerationFingerprint(snap)
	if err := ValidateSnapshot(snap); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// DeriveHealthURL projects the health target from the bound listen addr/TLS mode.
func DeriveHealthURL(addr, tlsMode string) (string, error) {
	hostport := strings.TrimSpace(addr)
	if hostport == "" {
		return "", errors.New("wslhost: addr is required to derive health_url")
	}
	if strings.HasPrefix(hostport, ":") {
		hostport = "127.0.0.1" + hostport
	}
	scheme := "https"
	switch tlsMode {
	case TLSModeTLS:
		scheme = "https"
	case TLSModeInsecure:
		scheme = "http"
	default:
		return "", fmt.Errorf("wslhost: unknown tls_mode %q", tlsMode)
	}
	u := url.URL{Scheme: scheme, Host: hostport, Path: "/healthz"}
	return u.String(), nil
}

// GenerationFingerprint is a stable content hash over the bound target fields.
// RegisteredAt is included so re-apply of identical targets still yields a new generation.
func GenerationFingerprint(s Snapshot) string {
	payload := strings.Join([]string{
		fmt.Sprintf("%d", s.SchemaVersion),
		s.WindowsIdentity,
		s.Distribution,
		s.LinuxUser,
		s.ServerExecutable,
		s.DataDir,
		s.Addr,
		s.TLSMode,
		s.HealthURL,
		s.LogPath,
		s.RegisteredAt,
	}, "\x1f")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16])
}

// ValidateSnapshot rejects partial, schema-invalid, or unbound snapshots so they
// can never become runtime authority.
func ValidateSnapshot(s Snapshot) error {
	if s.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("wslhost: unsupported schema_version %d", s.SchemaVersion)
	}
	required := []struct {
		name, value string
	}{
		{"generation", s.Generation},
		{"windows_identity", s.WindowsIdentity},
		{"distribution", s.Distribution},
		{"linux_user", s.LinuxUser},
		{"server_executable", s.ServerExecutable},
		{"data_dir", s.DataDir},
		{"addr", s.Addr},
		{"tls_mode", s.TLSMode},
		{"health_url", s.HealthURL},
		{"log_path", s.LogPath},
		{"registered_at", s.RegisteredAt},
	}
	for _, f := range required {
		if strings.TrimSpace(f.value) == "" {
			return fmt.Errorf("wslhost: %s is required", f.name)
		}
	}
	switch s.TLSMode {
	case TLSModeTLS, TLSModeInsecure:
	default:
		return fmt.Errorf("wslhost: invalid tls_mode %q", s.TLSMode)
	}
	if _, err := url.ParseRequestURI(s.HealthURL); err != nil {
		return fmt.Errorf("wslhost: invalid health_url: %w", err)
	}
	t := s.Thresholds
	if t.HealthProbeTimeoutMS <= 0 || t.HealthCadenceMS <= 0 || t.StartupBudgetMS <= 0 ||
		t.ResumeRecheckMS <= 0 || t.HealthRestartThreshold <= 0 || t.ConfigTerminalThreshold <= 0 ||
		t.TransientTerminalThreshold <= 0 || t.GracefulStopBudgetMS <= 0 || len(t.BackoffSeconds) == 0 {
		return errors.New("wslhost: thresholds incomplete")
	}
	if want := GenerationFingerprint(s); s.Generation != want {
		return fmt.Errorf("wslhost: generation fingerprint mismatch")
	}
	return nil
}

// ParseSnapshot decodes and validates JSON snapshot bytes.
func ParseSnapshot(data []byte) (Snapshot, error) {
	if len(bytesTrimSpace(data)) == 0 {
		return Snapshot{}, errors.New("wslhost: empty snapshot")
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, fmt.Errorf("wslhost: invalid snapshot json: %w", err)
	}
	if err := ValidateSnapshot(s); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

// MarshalSnapshot encodes a validated snapshot as canonical JSON.
func MarshalSnapshot(s Snapshot) ([]byte, error) {
	if err := ValidateSnapshot(s); err != nil {
		return nil, err
	}
	return json.Marshal(s)
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// ACLPrincipal identifies a security principal in the portable policy model.
type ACLPrincipal string

// Fixed principal classes used by the ACL policy (Windows SIDs mapped later).
const (
	PrincipalSYSTEM         ACLPrincipal = "SYSTEM"
	PrincipalAdministrators ACLPrincipal = "Administrators"
	PrincipalServiceIdentity ACLPrincipal = "ServiceIdentity"
	PrincipalBuiltinUsers   ACLPrincipal = "BUILTIN\\Users"
)

// ACLRight is a portable permission class.
type ACLRight string

const (
	ACLFullControl ACLRight = "full_control"
	ACLModify      ACLRight = "modify"
	ACLRead        ACLRight = "read"
)

// ACLEntry is one allow ACE in the policy model.
type ACLEntry struct {
	Principal ACLPrincipal `json:"principal"`
	Right     ACLRight     `json:"right"`
}

// ACLPolicyForConfig is the installer ACL for the registration snapshot file.
// Users do not get write access to config.
func ACLPolicyForConfig(serviceIdentity string) []ACLEntry {
	_ = serviceIdentity // identity is recorded by the applier; policy class is fixed
	return []ACLEntry{
		{Principal: PrincipalSYSTEM, Right: ACLFullControl},
		{Principal: PrincipalAdministrators, Right: ACLFullControl},
		{Principal: PrincipalServiceIdentity, Right: ACLModify},
	}
}

// ACLPolicyForLog is the installer ACL for the machine-wide log path.
// BUILTIN\Users may read so signed-in operators can open ProgramData logs.
func ACLPolicyForLog(serviceIdentity string) []ACLEntry {
	_ = serviceIdentity
	return []ACLEntry{
		{Principal: PrincipalSYSTEM, Right: ACLFullControl},
		{Principal: PrincipalAdministrators, Right: ACLFullControl},
		{Principal: PrincipalServiceIdentity, Right: ACLModify},
		{Principal: PrincipalBuiltinUsers, Right: ACLRead},
	}
}

// LogRetentionPolicy documents the fixed rotation bounds (enforced by RecordSink).
type LogRetentionPolicy struct {
	MaxBytes    int64 `json:"max_bytes"`
	RetainFiles int   `json:"retain_files"`
	RetainDays  int   `json:"retain_days"`
}

// DefaultLogRetention returns the ADR-fixed 10 MiB × 5 files / 30 days policy.
func DefaultLogRetention() LogRetentionPolicy {
	return LogRetentionPolicy{
		MaxBytes:    LogRotateBytes,
		RetainFiles: LogRetainFiles,
		RetainDays:  LogRetainDays,
	}
}
