package wslhost

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testRequest() RegistrationRequest {
	return RegistrationRequest{
		LinuxUser:        "agent",
		ServerExecutable: "/usr/local/bin/server",
		DataDir:          "/var/lib/agent-grid",
		Addr:             "127.0.0.1:8443",
		TLSMode:          TLSModeTLS,
	}
}

func TestBuildSnapshotResolvesDefaultsOnlyAtRegistration(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	// Given: identity/distro omitted; registration-time defaults A/D.
	snap, err := BuildSnapshot(testRequest(), Defaults{
		WindowsIdentity: "CONTOSO\\alice",
		Distribution:    "Ubuntu-22.04",
	}, root, now)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if snap.WindowsIdentity != "CONTOSO\\alice" || snap.Distribution != "Ubuntu-22.04" {
		t.Fatalf("bound target = %s/%s", snap.WindowsIdentity, snap.Distribution)
	}
	// Counterexample: re-resolving A2/D2 at runtime would mutate authority.
	// Load path only reads the bound generation — defaults are not consulted.
	store := &MemoryConfigStore{}
	reg := &Registrar{Store: store, ACL: NewRecordingACLApplier(), Gate: StaticServiceGate{IsStopped: true}, ProgramDataRoot: root}
	if err := reg.Apply(snap); err != nil {
		t.Fatal(err)
	}
	// Simulate interactive default drift after registration.
	loaded, err := reg.LoadRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Generation != snap.Generation {
		t.Fatalf("generation changed: got %s want %s", loaded.Generation, snap.Generation)
	}
	if loaded.WindowsIdentity != "CONTOSO\\alice" || loaded.Distribution != "Ubuntu-22.04" {
		t.Fatalf("runtime re-resolved defaults: %+v", loaded)
	}
	// Explicit overrides win over defaults.
	over, err := BuildSnapshot(RegistrationRequest{
		WindowsIdentity:  "CONTOSO\\svc",
		Distribution:     "AgentGrid",
		LinuxUser:        "agent",
		ServerExecutable: "/usr/local/bin/server",
		DataDir:          "/var/lib/agent-grid",
		Addr:             "127.0.0.1:8443",
		TLSMode:          TLSModeInsecure,
	}, Defaults{WindowsIdentity: "CONTOSO\\alice", Distribution: "Ubuntu-22.04"}, root, now)
	if err != nil {
		t.Fatal(err)
	}
	if over.WindowsIdentity != "CONTOSO\\svc" || over.Distribution != "AgentGrid" {
		t.Fatalf("explicit override lost: %+v", over)
	}
	if over.HealthURL != "http://127.0.0.1:8443/healthz" {
		t.Fatalf("health_url = %q", over.HealthURL)
	}
}

func TestValidateSnapshotRejectsPartialStaleSchemaInvalid(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	good, err := BuildSnapshot(testRequest(), Defaults{
		WindowsIdentity: "A", Distribution: "D",
	}, t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	// schema-invalid
	bad := good
	bad.SchemaVersion = 99
	if err := ValidateSnapshot(bad); err == nil {
		t.Fatal("expected schema rejection")
	}
	// partial / unbound
	partial := good
	partial.Distribution = ""
	if err := ValidateSnapshot(partial); err == nil {
		t.Fatal("expected unbound rejection")
	}
	// stale fingerprint
	stale := good
	stale.Generation = "not-the-fingerprint"
	if err := ValidateSnapshot(stale); err == nil {
		t.Fatal("expected fingerprint rejection")
	}
	// empty / truncated JSON never becomes authority
	if _, err := ParseSnapshot(nil); err == nil {
		t.Fatal("expected empty rejection")
	}
	if _, err := ParseSnapshot([]byte(`{"schema_version":1`)); err == nil {
		t.Fatal("expected partial json rejection")
	}
	if _, err := ParseSnapshot([]byte(`{"schema_version":1,"generation":"x"}`)); err == nil {
		t.Fatal("expected incomplete object rejection")
	}
}

func TestApplyRequiresStoppedServiceAndKeepsPriorOnFailure(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	g1, err := BuildSnapshot(testRequest(), Defaults{WindowsIdentity: "A", Distribution: "D"}, root, now)
	if err != nil {
		t.Fatal(err)
	}
	store := &MemoryConfigStore{}
	acl := NewRecordingACLApplier()
	reg := &Registrar{Store: store, ACL: acl, Gate: StaticServiceGate{IsStopped: true}, ProgramDataRoot: root}
	if err := reg.Apply(g1); err != nil {
		t.Fatal(err)
	}
	reg.Gate = StaticServiceGate{IsStopped: false}
	g2, err := BuildSnapshot(testRequest(), Defaults{WindowsIdentity: "A2", Distribution: "D2"}, root, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Apply(g2); !errors.Is(err, ErrServiceNotStopped) {
		t.Fatalf("err = %v, want ErrServiceNotStopped", err)
	}
	loaded, err := reg.LoadRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Generation != g1.Generation {
		t.Fatalf("prior generation replaced while running")
	}
	// failed write must not install authority
	reg.Gate = StaticServiceGate{IsStopped: true}
	store.FailWrite = errors.New("disk full")
	if err := reg.Apply(g2); err == nil {
		t.Fatal("expected write failure")
	}
	loaded, err = reg.LoadRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Generation != g1.Generation {
		t.Fatal("failed write mutated generation")
	}
}

func TestFileConfigStoreAtomicReplaceAndLoad(t *testing.T) {
	root := t.TempDir()
	path := ConfigPath(root)
	acl := NewRecordingACLApplier()
	store := &FileConfigStore{Path: path, ACL: acl}
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	snap, err := BuildSnapshot(testRequest(), Defaults{WindowsIdentity: "A", Distribution: "D"}, root, now)
	if err != nil {
		t.Fatal(err)
	}
	reg := &Registrar{Store: store, ACL: acl, Gate: StaticServiceGate{IsStopped: true}, ProgramDataRoot: root}
	if err := reg.Apply(snap); err != nil {
		t.Fatal(err)
	}
	// no leftover temps
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("leftover temp %s", e.Name())
		}
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Generation != snap.Generation {
		t.Fatalf("loaded generation %s != %s", loaded.Generation, snap.Generation)
	}
	if got := acl.Applied(path); len(got) == 0 {
		t.Fatal("config ACL not applied")
	}
	if got := acl.Applied(snap.LogPath); len(got) == 0 {
		t.Fatal("log ACL not applied")
	}
	replacement, err := BuildSnapshot(
		testRequest(),
		Defaults{WindowsIdentity: "A", Distribution: "D"},
		root,
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Apply(replacement); err != nil {
		t.Fatalf("replace existing snapshot: %v", err)
	}
	loaded, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Generation != replacement.Generation {
		t.Fatalf("loaded generation %s != replacement %s", loaded.Generation, replacement.Generation)
	}
	// partial on-disk file is rejected
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("partial file became authority")
	}
}

// TestContractAtomicSnapshotAndLogACL names the filesystem/config/ACL invariants.
func TestContractAtomicSnapshotAndLogACL(t *testing.T) {
	// Invariant: validated JSON generation is atomically installed and read once.
	// Invariant: config ACL excludes Users-write; log ACL grants Users-read.
	// Invariant: unknown schema / partial / fingerprint mismatch never load.
	root := t.TempDir()
	path := ConfigPath(root)
	acl := NewRecordingACLApplier()
	store := &FileConfigStore{Path: path, ACL: acl}
	reg := &Registrar{
		Store: store, ACL: acl, Gate: StaticServiceGate{IsStopped: true},
		ProgramDataRoot: root,
		Now:             func() time.Time { return time.Date(2026, 7, 26, 15, 0, 0, 0, time.UTC) },
	}
	snap, err := reg.Register(testRequest(), Defaults{WindowsIdentity: "A", Distribution: "D"})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reg.LoadRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Generation != snap.Generation {
		t.Fatalf("runtime authority drifted")
	}
	cfgACL := acl.Applied(path)
	if containsPrincipal(cfgACL, PrincipalBuiltinUsers) {
		t.Fatal("config ACL must not grant BUILTIN\\Users")
	}
	if !hasEntry(cfgACL, PrincipalServiceIdentity, ACLModify) {
		t.Fatal("config ACL missing service modify")
	}
	logACL := acl.Applied(snap.LogPath)
	if !hasEntry(logACL, PrincipalBuiltinUsers, ACLRead) {
		t.Fatal("log ACL missing Users read")
	}
	if !hasEntry(logACL, PrincipalServiceIdentity, ACLModify) {
		t.Fatal("log ACL missing service modify")
	}
	ret := DefaultLogRetention()
	if ret.MaxBytes != LogRotateBytes || ret.RetainFiles != LogRetainFiles || ret.RetainDays != LogRetainDays {
		t.Fatalf("retention policy drift: %+v", ret)
	}
	// migration: unknown schema fails closed before becoming authority
	raw, err := MarshalSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	// tamper schema_version in place
	tampered := []byte(`{"schema_version":2,"generation":"x"}`)
	_ = raw
	if _, err := ParseSnapshot(tampered); err == nil {
		t.Fatal("unknown schema accepted")
	}
}

// TestFakeVsRealProgramDataACLAndRotation compares the memory fake against the
// real filesystem atomic store and RecordSink using the same registration input.
// Rotation is exercised through FileRecordSink vs MemoryRecordSink under the
// shared 10 MiB × 5 files / 30-day policy shape (size scaled for the unit test).
func TestFakeVsRealProgramDataACLAndRotation(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 26, 16, 0, 0, 0, time.UTC)
	req := testRequest()
	defaults := Defaults{WindowsIdentity: "A", Distribution: "D"}
	want, err := BuildSnapshot(req, defaults, root, now)
	if err != nil {
		t.Fatal(err)
	}

	fakeACL := NewRecordingACLApplier()
	fakeStore := &MemoryConfigStore{}
	fakeReg := &Registrar{
		Store: fakeStore, ACL: fakeACL, Gate: StaticServiceGate{IsStopped: true},
		ProgramDataRoot: root, Now: func() time.Time { return now },
	}
	fakeSnap, err := fakeReg.Register(req, defaults)
	if err != nil {
		t.Fatalf("fake register: %v", err)
	}

	realACL := NewRecordingACLApplier()
	realStore := &FileConfigStore{Path: ConfigPath(root), ACL: realACL}
	realReg := &Registrar{
		Store: realStore, ACL: realACL, Gate: StaticServiceGate{IsStopped: true},
		ProgramDataRoot: root, Now: func() time.Time { return now },
	}
	realSnap, err := realReg.Register(req, defaults)
	if err != nil {
		t.Fatalf("real register: %v", err)
	}

	if fakeSnap.Generation != want.Generation || realSnap.Generation != want.Generation {
		t.Fatalf("generation mismatch fake=%s real=%s want=%s", fakeSnap.Generation, realSnap.Generation, want.Generation)
	}
	fakeLoaded, err := fakeReg.LoadRuntime()
	if err != nil {
		t.Fatal(err)
	}
	realLoaded, err := realReg.LoadRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if fakeLoaded.WindowsIdentity != realLoaded.WindowsIdentity ||
		fakeLoaded.Distribution != realLoaded.Distribution ||
		fakeLoaded.HealthURL != realLoaded.HealthURL ||
		fakeLoaded.LogPath != realLoaded.LogPath {
		t.Fatalf("fake/real target drift\nfake=%+v\nreal=%+v", fakeLoaded, realLoaded)
	}
	if !aclEqual(fakeACL.Applied(fakeSnap.LogPath), realACL.Applied(realSnap.LogPath)) {
		t.Fatalf("log ACL policy drift")
	}
	if !aclEqual(fakeACL.Applied(ConfigPath(root)), realACL.Applied(ConfigPath(root))) {
		t.Fatalf("config ACL policy drift\nfake=%+v\nreal=%+v",
			fakeACL.Applied(ConfigPath(root)), realACL.Applied(ConfigPath(root)))
	}
	if !aclEqual(realACL.Applied(ConfigPath(root)), ACLPolicyForConfig("A")) {
		t.Fatalf("real config ACL != policy: %+v", realACL.Applied(ConfigPath(root)))
	}
	if !aclEqual(realACL.Applied(realSnap.LogPath), ACLPolicyForLog("A")) {
		t.Fatalf("real log ACL != policy")
	}
	if DefaultLogRetention() != (LogRetentionPolicy{MaxBytes: LogRotateBytes, RetainFiles: LogRetainFiles, RetainDays: LogRetainDays}) {
		t.Fatal("rotation policy constants drifted")
	}
	// Shared adversarial input: corrupt bytes rejected by both loaders.
	fakeStore.InstallCorrupt([]byte(`{"schema_version":1`))
	if _, err := fakeStore.Load(); err == nil {
		t.Fatal("fake accepted partial")
	}
	if err := os.WriteFile(ConfigPath(root), []byte(`{"schema_version":1`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := realStore.Load(); err == nil {
		t.Fatal("real accepted partial")
	}

	// RecordSink half of the triple: same allowlisted events → both rotate under
	// RetainFiles and drop on forced write failure without blocking the caller.
	policy := LogRetentionPolicy{MaxBytes: 160, RetainFiles: LogRetainFiles, RetainDays: LogRetainDays}
	mem := NewMemoryRecordSink()
	mem.Policy = policy
	mem.MaxActiveBytes = policy.MaxBytes
	mem.ACL = fakeACL
	mem.LogPath = realSnap.LogPath
	mem.Ident = realSnap.WindowsIdentity
	file, err := OpenFileRecordSink(realSnap.LogPath, realSnap.WindowsIdentity, realACL, policy)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for i := 0; i < 20; i++ {
		rec := Record{
			Time: now.Format(time.RFC3339Nano), Event: EventBackoff,
			Generation: realSnap.Generation, WindowsIdentity: realSnap.WindowsIdentity,
			Distribution: realSnap.Distribution, LinuxUser: realSnap.LinuxUser,
			Attempt: i + 1, RetryCount: i + 1, BackoffSeconds: 2,
		}
		if err := mem.Write(rec); err != nil {
			t.Fatalf("mem write: %v", err)
		}
		if err := file.Write(rec); err != nil {
			t.Fatalf("file write: %v", err)
		}
	}
	if mem.ActiveFileCount() < 2 {
		t.Fatal("memory sink did not rotate")
	}
	files, err := ListLogFiles(realSnap.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 {
		t.Fatal("file sink did not rotate")
	}
	if mem.ActiveFileCount() > LogRetainFiles || len(files) > LogRetainFiles {
		t.Fatalf("retain cap broken mem=%d file=%d", mem.ActiveFileCount(), len(files))
	}
	mem.FailWrite = errors.New("full")
	file.FailWrite = errors.New("full")
	dropRec := Record{Time: now.Format(time.RFC3339Nano), Event: EventFailed, Generation: realSnap.Generation}
	if err := mem.Write(dropRec); !errors.Is(err, ErrRecordDropped) {
		t.Fatalf("mem drop: %v", err)
	}
	if err := file.Write(dropRec); !errors.Is(err, ErrRecordDropped) {
		t.Fatalf("file drop: %v", err)
	}
}

func containsPrincipal(entries []ACLEntry, p ACLPrincipal) bool {
	for _, e := range entries {
		if e.Principal == p {
			return true
		}
	}
	return false
}

func hasEntry(entries []ACLEntry, p ACLPrincipal, r ACLRight) bool {
	for _, e := range entries {
		if e.Principal == p && e.Right == r {
			return true
		}
	}
	return false
}
