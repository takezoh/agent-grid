package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/takezoh/agent-grid/services/platform/appid"
	"golang.org/x/sys/unix"
)

// Probe / stop classification strings are the machine-readable contract
// consumed by the Windows WSL supervisor (HostControl). Keep names stable.
const (
	probeClassAbsent          = "absent"
	probeClassMatchedRunning  = "matched_running"
	probeClassMismatch        = "mismatch"
	probeClassUnknown         = "unknown"
	stopClassSignaled         = "signaled"
	stopClassRejected         = "rejected"
	stopClassAbsent           = "absent"
	stopClassUnknown          = "unknown"
	healthStatusOK            = "ok"
	healthStatusUnavailable   = "unavailable"
	healthStatusUnhealthy     = "unhealthy"
	defaultHealthProbeTimeout = 2 * time.Second
)

// Injectable seams so T0/T2 tests classify without racing real processes.
var (
	inspectDaemonLockFn   = inspectDaemonLock
	readProcessIdentityFn = readProcessIdentity
	signalProcessFn       = signalProcess
	probeHealthFn         = probeHealthHTTP
)

// processIdentity is the Linux fingerprint used for adoption and stop authority.
type processIdentity struct {
	PID        int
	Executable string
	Argv       []string
	StartToken string
}

// ownershipToken is the stop-authority proof returned by a matched probe.
// Stop revalidates pid + start token against the live lock holder before SIGTERM.
type ownershipToken struct {
	PID        int    `json:"pid"`
	StartToken string `json:"start_token"`
	DataDir    string `json:"data_dir"`
}

// healthEvidence is optional configured-health observation attached to a probe.
type healthEvidence struct {
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// probeResult is the typed JSON stdout contract for supervisor-probe.
type probeResult struct {
	Class           string          `json:"class"`
	DataDir         string          `json:"data_dir"`
	PID             int             `json:"pid,omitempty"`
	StartToken      string          `json:"start_token,omitempty"`
	Executable      string          `json:"executable,omitempty"`
	Argv            []string        `json:"argv,omitempty"`
	ArgvFingerprint string          `json:"argv_fingerprint,omitempty"`
	OwnershipToken  string          `json:"ownership_token,omitempty"`
	Health          *healthEvidence `json:"health,omitempty"`
	Reason          string          `json:"reason,omitempty"`
}

// stopResult is the typed JSON stdout contract for supervisor-stop.
type stopResult struct {
	Class   string `json:"class"`
	DataDir string `json:"data_dir,omitempty"`
	PID     int    `json:"pid,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// probeRequest is the validated input for classification.
type probeRequest struct {
	DataDir         string
	ExpectedExe     string
	ExpectedArgv    []string
	ArgvFingerprint string
	HealthURL       string
	HealthTimeout   time.Duration
}

func runSupervisorProbe(args []string, stdout io.Writer) error {
	req, err := parseProbeArgs(args)
	if err != nil {
		return err
	}
	result := classifyProbe(req)
	return writeJSON(stdout, result)
}

func runSupervisorStop(args []string, stdout io.Writer) error {
	dataDir, token, err := parseStopArgs(args)
	if err != nil {
		return err
	}
	result := executeStop(dataDir, token)
	return writeJSON(stdout, result)
}

func parseProbeArgs(args []string) (probeRequest, error) {
	fs := flag.NewFlagSet("supervisor-probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dataDir := fs.String("data-dir", "", "daemon data directory (lock path root)")
	exe := fs.String("exe", "", "expected executable path (resolved via /proc/<pid>/exe)")
	argvFP := fs.String("argv-fingerprint", "", "expected sha256 hex of NUL-joined argv")
	healthURL := fs.String("health-url", "", "optional GET health target (e.g. http://127.0.0.1:8443/healthz)")
	healthTimeout := fs.Duration("health-timeout", defaultHealthProbeTimeout, "health request timeout")
	var expectedArgv multiFlag
	fs.Var(&expectedArgv, "arg", "expected argv element (repeatable; full argv including argv0)")
	if err := fs.Parse(args); err != nil {
		return probeRequest{}, fmt.Errorf("supervisor-probe: %w", err)
	}
	if strings.TrimSpace(*dataDir) == "" {
		return probeRequest{}, fmt.Errorf("supervisor-probe: -data-dir is required")
	}
	return probeRequest{
		DataDir:         *dataDir,
		ExpectedExe:     strings.TrimSpace(*exe),
		ExpectedArgv:    append([]string(nil), expectedArgv...),
		ArgvFingerprint: strings.TrimSpace(*argvFP),
		HealthURL:       strings.TrimSpace(*healthURL),
		HealthTimeout:   *healthTimeout,
	}, nil
}

func parseStopArgs(args []string) (dataDir, token string, err error) {
	fs := flag.NewFlagSet("supervisor-stop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dd := fs.String("data-dir", "", "daemon data directory (lock path root)")
	tok := fs.String("ownership-token", "", "token from a matched supervisor-probe")
	if err := fs.Parse(args); err != nil {
		return "", "", fmt.Errorf("supervisor-stop: %w", err)
	}
	if strings.TrimSpace(*dd) == "" {
		return "", "", fmt.Errorf("supervisor-stop: -data-dir is required")
	}
	if strings.TrimSpace(*tok) == "" {
		return "", "", fmt.Errorf("supervisor-stop: -ownership-token is required")
	}
	return *dd, *tok, nil
}

// classifyProbe maps lock + process fingerprint evidence onto the four
// exclusive classes: absent, matched_running, mismatch, unknown.
func classifyProbe(req probeRequest) probeResult {
	lockPath := filepath.Join(req.DataDir, appid.PidFileName)
	out := probeResult{Class: probeClassUnknown, DataDir: req.DataDir}

	holder, err := inspectDaemonLockFn(lockPath)
	if err != nil {
		out.Reason = "lock_inspect_failed: " + err.Error()
		return out
	}
	if !holder.Held {
		out.Class = probeClassAbsent
		out.Reason = "no_exclusive_lock_holder"
		out.Health = maybeHealth(req)
		return out
	}
	if holder.PID <= 0 {
		out.Class = probeClassUnknown
		out.Reason = "lock_held_unreadable_pid"
		return out
	}

	ident, err := readProcessIdentityFn(holder.PID)
	if err != nil {
		out.Class = probeClassUnknown
		out.PID = holder.PID
		out.Reason = "process_identity_unavailable: " + err.Error()
		return out
	}

	out.PID = ident.PID
	out.StartToken = ident.StartToken
	out.Executable = ident.Executable
	out.Argv = ident.Argv
	out.ArgvFingerprint = argvFingerprint(ident.Argv)
	out.Health = maybeHealth(req)

	if !identityMatches(req, ident) {
		out.Class = probeClassMismatch
		out.Reason = "executable_or_argv_mismatch"
		return out
	}

	tok, err := encodeOwnershipToken(ownershipToken{
		PID:        ident.PID,
		StartToken: ident.StartToken,
		DataDir:    req.DataDir,
	})
	if err != nil {
		out.Class = probeClassUnknown
		out.Reason = "ownership_token_encode_failed: " + err.Error()
		return out
	}
	out.Class = probeClassMatchedRunning
	out.OwnershipToken = tok
	out.Reason = "fingerprint_matched_lock_holder"
	return out
}

func executeStop(dataDir, rawToken string) stopResult {
	out := stopResult{Class: stopClassUnknown, DataDir: dataDir}
	tok, err := decodeOwnershipToken(rawToken)
	if err != nil {
		out.Class = stopClassRejected
		out.Reason = "malformed_ownership_token"
		return out
	}
	if tok.DataDir != "" && filepath.Clean(tok.DataDir) != filepath.Clean(dataDir) {
		out.Class = stopClassRejected
		out.Reason = "token_data_dir_mismatch"
		return out
	}

	lockPath := filepath.Join(dataDir, appid.PidFileName)
	holder, err := inspectDaemonLockFn(lockPath)
	if err != nil {
		out.Reason = "lock_inspect_failed: " + err.Error()
		return out
	}
	if !holder.Held {
		out.Class = stopClassAbsent
		out.Reason = "no_exclusive_lock_holder"
		return out
	}
	if holder.PID <= 0 || holder.PID != tok.PID {
		out.Class = stopClassRejected
		out.PID = holder.PID
		out.Reason = "lock_holder_pid_mismatch"
		return out
	}

	ident, err := readProcessIdentityFn(holder.PID)
	if err != nil {
		out.PID = holder.PID
		out.Reason = "process_identity_unavailable: " + err.Error()
		return out
	}
	if ident.StartToken != tok.StartToken {
		out.Class = stopClassRejected
		out.PID = holder.PID
		out.Reason = "start_token_mismatch"
		return out
	}

	if err := signalProcessFn(holder.PID, unix.SIGTERM); err != nil {
		out.PID = holder.PID
		out.Reason = "signal_failed: " + err.Error()
		return out
	}
	out.Class = stopClassSignaled
	out.PID = holder.PID
	out.Reason = "sigterm_delivered"
	return out
}

func identityMatches(req probeRequest, ident processIdentity) bool {
	if req.ExpectedExe != "" {
		expectedExe := filepath.Clean(req.ExpectedExe)
		if resolved, err := filepath.EvalSymlinks(expectedExe); err == nil {
			expectedExe = filepath.Clean(resolved)
		}
		if expectedExe != filepath.Clean(ident.Executable) {
			// Also accept basename match when expected is not absolute.
			if filepath.IsAbs(req.ExpectedExe) || filepath.Base(ident.Executable) != req.ExpectedExe {
				return false
			}
		}
	}
	if len(req.ExpectedArgv) > 0 {
		if !argvEqual(req.ExpectedArgv, ident.Argv) {
			return false
		}
	}
	if req.ArgvFingerprint != "" {
		if !strings.EqualFold(req.ArgvFingerprint, argvFingerprint(ident.Argv)) {
			return false
		}
	}
	// With no expected fingerprint material, a live lock holder is matched so
	// the caller can adopt and bind stop authority to the returned token.
	return true
}

func maybeHealth(req probeRequest) *healthEvidence {
	if req.HealthURL == "" {
		return nil
	}
	timeout := req.HealthTimeout
	if timeout <= 0 {
		timeout = defaultHealthProbeTimeout
	}
	return probeHealthFn(req.HealthURL, timeout)
}

func argvFingerprint(argv []string) string {
	sum := sha256.Sum256([]byte(strings.Join(argv, "\x00")))
	return hex.EncodeToString(sum[:])
}

func argvEqual(a, b []string) bool {
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

func encodeOwnershipToken(tok ownershipToken) (string, error) {
	raw, err := json.Marshal(tok)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeOwnershipToken(s string) (ownershipToken, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return ownershipToken{}, err
	}
	var tok ownershipToken
	if err := json.Unmarshal(raw, &tok); err != nil {
		return ownershipToken{}, err
	}
	if tok.PID <= 0 || tok.StartToken == "" {
		return ownershipToken{}, fmt.Errorf("incomplete token")
	}
	return tok, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// multiFlag accumulates repeated -arg values.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func probeHealthHTTP(url string, timeout time.Duration) *healthEvidence {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return &healthEvidence{Status: healthStatusUnavailable, Reason: err.Error()}
	}
	defer resp.Body.Close()
	var body struct {
		Status string `json:"status"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	ev := &healthEvidence{HTTPStatus: resp.StatusCode}
	switch {
	case resp.StatusCode == http.StatusOK && body.Status == healthStatusOK:
		ev.Status = healthStatusOK
	case resp.StatusCode == http.StatusOK:
		ev.Status = healthStatusUnhealthy
		ev.Reason = "status_not_ok"
	default:
		ev.Status = healthStatusUnhealthy
		ev.Reason = fmt.Sprintf("http_%d", resp.StatusCode)
	}
	return ev
}
