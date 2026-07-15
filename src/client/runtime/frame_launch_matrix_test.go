package runtime

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	rsubsystem "github.com/takezoh/agent-grid/client/runtime/subsystem"
	"github.com/takezoh/agent-grid/client/state"
	"github.com/takezoh/agent-grid/platform/agentlaunch"
	platformconfig "github.com/takezoh/agent-grid/platform/config"
	"github.com/takezoh/agent-grid/platform/sandbox"
	sandboxdc "github.com/takezoh/agent-grid/platform/sandbox/devcontainer"
)

// Note: managedHomeDirs uses os.ReadDir; the os import above already covers it.

// Frame-launch matrix driven through the REAL launcher stack
// (NewDispatcherAdapter → SandboxDispatcher → DirectDispatcher /
// DevcontainerLauncher), with only the docker-backed sandbox.Manager faked.
// This pins the launch wiring touched by e41ab1c across:
//
//	env       : host / per-project container / shared container
//	lifecycle : new-session / cold-start / warm-start
//
// What each cell proves:
//   - host    → DirectDispatcher injects AG_SOCKET, strips the container
//     token, registers no frame token / mounts / endpoint.
//   - container → wrapLaunchForSpawn generates a bearer token, the launcher
//     hands it to the Manager's BuildLaunchCommand (it rides the docker-exec
//     command, not the backend spawn env), and the runtime registers token +
//     mounts + endpoint. per-project vs shared differ in the run-dir key
//     (projectPath hash vs SharedContainerKey hash).
//   - the command-execution ORDER is subsystem.Ensure → subsystem.BindFrame →
//     mgr.EnsureInstance → mgr.AcquireFrame → mgr.BuildLaunchCommand →
//     backend.SpawnFrame.
//
// The subsystem backends are faked (recSubsysFactory) so no real codex
// app-server starts here; the codex command/socket rewrite is covered at the
// stream-package altitude. The bridge binary copy and the container
// workspace bind-mount are covered by platform/agentlaunch tests — here the
// faked Manager returns a nil ContainerState (all its methods are nil-safe).

type envKind int

const (
	envHost envKind = iota
	envProject
	envShared
)

// orderRecorder collects an ordered trace of cross-layer launch calls.
type orderRecorder struct {
	mu sync.Mutex
	ev []string
}

func (o *orderRecorder) add(s string) {
	o.mu.Lock()
	o.ev = append(o.ev, s)
	o.mu.Unlock()
}

func (o *orderRecorder) snapshot() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.ev...)
}

// kindCounter records which subsystem kind was Ensure'd, so a test can assert
// codex→stream / generic→cli selection without running a backend.
type kindCounter struct {
	mu sync.Mutex
	m  map[state.LaunchSubsystem]int
}

func (k *kindCounter) inc(kind state.LaunchSubsystem) {
	k.mu.Lock()
	k.m[kind]++
	k.mu.Unlock()
}

func (k *kindCounter) count(kind state.LaunchSubsystem) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.m[kind]
}

// fakeSandboxManager satisfies sandbox.Manager[*sandboxdc.ContainerState] with
// no docker. mockMgr (platform/agentlaunch) is package-local and cannot be
// imported here, so this is a runtime-scope equivalent with a call recorder.
type fakeSandboxManager struct {
	rec          *orderRecorder
	mu           sync.Mutex
	lastBuildEnv map[string]string
	ensureN      int
	acquireN     int
	releaseN     int
	destroyN     int
	inst         *sandbox.Instance[*sandboxdc.ContainerState]
}

func (m *fakeSandboxManager) EnsureInstance(_ context.Context, project, _ string, _ sandbox.StartOptions) (*sandbox.Instance[*sandboxdc.ContainerState], error) {
	m.rec.add("mgr.EnsureInstance")
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureN++
	if m.inst == nil {
		m.inst = &sandbox.Instance[*sandboxdc.ContainerState]{ProjectPath: project, Internal: nil}
	}
	return m.inst, nil
}

func (m *fakeSandboxManager) BuildLaunchCommand(_ *sandbox.Instance[*sandboxdc.ContainerState], spec sandbox.LaunchSpec, _ sandbox.FrameContext, env map[string]string) (string, map[string]string, error) {
	m.rec.add("mgr.BuildLaunchCommand")
	m.mu.Lock()
	m.lastBuildEnv = cloneEnvMap(env, 0)
	m.mu.Unlock()
	// The real Manager bakes env into a `docker exec -e ...` command and returns
	// a separate container-exec env; mirror that shape with a marker env.
	return "docker exec fake " + spec.Command, map[string]string{"CONTAINER": "1"}, nil
}

func (m *fakeSandboxManager) AcquireFrame(_ *sandbox.Instance[*sandboxdc.ContainerState]) {
	m.rec.add("mgr.AcquireFrame")
	m.mu.Lock()
	m.acquireN++
	m.mu.Unlock()
}

func (m *fakeSandboxManager) ReleaseFrame(_ *sandbox.Instance[*sandboxdc.ContainerState]) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releaseN++
	return true
}

func (m *fakeSandboxManager) DestroyInstance(_ context.Context, _ *sandbox.Instance[*sandboxdc.ContainerState]) error {
	m.mu.Lock()
	m.destroyN++
	m.mu.Unlock()
	return nil
}

func (m *fakeSandboxManager) buildEnv() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneEnvMap(m.lastBuildEnv, 0)
}

var _ sandbox.Manager[*sandboxdc.ContainerState] = (*fakeSandboxManager)(nil)

// recSubsystem echoes the bind plan and records lifecycle calls.
type recSubsystem struct {
	id   state.SubsystemID
	kind state.LaunchSubsystem
	rec  *orderRecorder
}

func (s *recSubsystem) Kind() state.LaunchSubsystem { return s.kind }
func (s *recSubsystem) Start(context.Context) error { return nil }
func (s *recSubsystem) BindFrame(_ context.Context, req rsubsystem.BindRequest) (rsubsystem.BindResult, error) {
	s.rec.add("subsystem.BindFrame")
	return rsubsystem.BindResult{Plan: req.Plan}, nil
}
func (s *recSubsystem) ReleaseFrame(state.FrameID) {}
func (s *recSubsystem) Stop(context.Context)       {}

type recSubsysFactory struct {
	kind  state.LaunchSubsystem
	rec   *orderRecorder
	kinds *kindCounter
}

func (f *recSubsysFactory) Ensure(_ context.Context, sid state.SessionID, _ string, _ state.LaunchPlan) (rsubsystem.Subsystem, state.SubsystemID, error) {
	f.rec.add("subsystem.Ensure")
	f.kinds.inc(f.kind)
	id := state.SubsystemID(string(f.kind) + ":" + string(sid))
	return &recSubsystem{id: id, kind: f.kind, rec: f.rec}, id, nil
}

// recordingBackend wraps fakeBackend, appending "backend.SpawnFrame" to the
// shared order trace before delegating. All other methods are promoted.
type recordingBackend struct {
	*fakeBackend
	rec *orderRecorder
}

func (t *recordingBackend) SpawnFrame(frameID, name, command, startDir string, env map[string]string, cols, rows uint16) error {
	t.rec.add("backend.SpawnFrame")
	return t.fakeBackend.SpawnFrame(frameID, name, command, startDir, env, cols, rows)
}

// launchHarness wires a Runtime through the real launcher stack for one env.
type launchHarness struct {
	r        *Runtime
	backend  *fakeBackend
	mgr      *fakeSandboxManager
	rec      *orderRecorder
	kinds    *kindCounter
	dataDir  string
	sockPath string
}

func newLaunchHarness(t *testing.T, env envKind) *launchHarness {
	return buildLaunchHarness(t, env, false)
}

// buildLaunchHarness wires a Runtime through the real launcher stack. When
// persistWarm is true the runtime's warm-frame store is enabled (Config.DataDir
// set) — only the warm-start tests need it. Cold/new-session tests leave it
// off because they don't exercise the warm path; registerContainerFrame now
// Saves synchronously (issues/029 F4) so there's no async write left dangling
// past t.Cleanup either way.
func buildLaunchHarness(t *testing.T, env envKind, persistWarm bool) *launchHarness {
	t.Helper()
	rec := &orderRecorder{}
	kinds := &kindCounter{m: map[state.LaunchSubsystem]int{}}
	mgr := &fakeSandboxManager{rec: rec}
	dataDir := t.TempDir()
	sockPath := filepath.Join(dataDir, "server.sock")

	var user platformconfig.SandboxConfig
	switch env {
	case envHost:
		user = platformconfig.SandboxConfig{Mode: "direct"}
	case envProject:
		user = platformconfig.SandboxConfig{Mode: "devcontainer", Isolation: "project"}
	case envShared:
		user = platformconfig.SandboxConfig{Mode: "devcontainer", Isolation: "shared"}
	}
	resolver := platformconfig.NewSandboxResolver(user)

	// SelfBin is a placeholder path; managed_claude_home / DirectDispatcher only
	// require it to be non-empty to enter the overlay-generation branch (they do
	// not exec it). Tests that observe the managed overlay depend on this being
	// set, so keep it filled unconditionally.
	selfBin := filepath.Join(dataDir, "fake-server")
	disp := &agentlaunch.SandboxDispatcher{
		Resolver: resolver,
		Direct:   agentlaunch.DirectDispatcher{SockPath: sockPath, SelfBin: selfBin, DataDir: dataDir},
	}
	if env != envHost {
		disp.Devcontainer = agentlaunch.NewDevcontainerLauncher(
			mgr, resolver.Resolve, resolver.ResolveProjectScope, nil, dataDir, true)
	}

	base := newFakeBackend()
	cfg := Config{
		Backend:  &recordingBackend{fakeBackend: base, rec: rec},
		Launcher: NewDispatcherAdapter(disp),
		Persist:  &recordingPersist{},
	}
	if persistWarm {
		cfg.DataDir = dataDir
	}
	r := New(cfg)
	t.Cleanup(r.shutdownContainerEndpoints)
	r.SetSandboxedProjectResolver(func(string) bool { return env != envHost })
	r.subsystemFactories = map[state.LaunchSubsystem]rsubsystem.Factory{
		state.LaunchSubsystemCLI:    &recSubsysFactory{kind: state.LaunchSubsystemCLI, rec: rec, kinds: kinds},
		state.LaunchSubsystemStream: &recSubsysFactory{kind: state.LaunchSubsystemStream, rec: rec, kinds: kinds},
	}
	return &launchHarness{r: r, backend: base, mgr: mgr, rec: rec, kinds: kinds, dataDir: dataDir, sockPath: sockPath}
}

func (h *launchHarness) spawnEnv(t *testing.T) map[string]string {
	t.Helper()
	h.backend.mu.Lock()
	defer h.backend.mu.Unlock()
	if len(h.backend.spawnEnvs) != 1 {
		t.Fatalf("SpawnFrame env captures = %d, want 1", len(h.backend.spawnEnvs))
	}
	return h.backend.spawnEnvs[0]
}

func (h *launchHarness) runDir(project string) string {
	return ProjectRunDir(filepath.Join(h.dataDir, "run"), project)
}

// matrixFrame builds a generic (cli-subsystem) frame for the given project.
// The command is always the zero-behaviour minimal-test driver — codex frames
// are constructed inline where the stream subsystem matters.
func matrixFrame(project string) state.SessionFrame {
	return state.SessionFrame{ID: "f1", Project: project, Command: "minimal-test", Driver: state.DriverStateBase{}}
}

// === cold start (spawnFrameWindow) ===

func TestFrameLaunch_ColdStart_Host(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envHost)

	frame := matrixFrame("/proj/host")
	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, frame); err != nil {
		t.Fatalf("spawnFrameWindow: %v", err)
	}

	env := h.spawnEnv(t)
	if env["AG_SOCKET"] != h.sockPath {
		t.Errorf("AG_SOCKET = %q, want %q", env["AG_SOCKET"], h.sockPath)
	}
	if env["AG_SESSION_ID"] != "s1" || env["AG_FRAME_ID"] != "f1" {
		t.Errorf("identity env not baked into host spawn: %v", env)
	}
	if env["AG_SOCKET_TOKEN"] != "" {
		t.Errorf("host launch AG_SOCKET_TOKEN = %q, want empty", env["AG_SOCKET_TOKEN"])
	}
	if _, ok := h.r.frameReg.GetMounts("f1"); ok {
		t.Error("host launch must not register mounts")
	}
	if len(h.r.containerEndpoints) != 0 {
		t.Errorf("host launch must not start a container endpoint, got %d", len(h.r.containerEndpoints))
	}
	if h.mgr.ensureN != 0 {
		t.Errorf("host launch must not touch the sandbox Manager, ensureN=%d", h.mgr.ensureN)
	}
}

func TestFrameLaunch_ColdStart_PerProject(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envProject)

	const project = "/proj/box"
	frame := matrixFrame(project)
	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, frame); err != nil {
		t.Fatalf("spawnFrameWindow: %v", err)
	}

	// The bearer token rides the docker-exec command (Manager env), not the
	// backend spawn env. It must reach BuildLaunchCommand and be registered.
	buildEnv := h.mgr.buildEnv()
	tok := buildEnv["AG_SOCKET_TOKEN"]
	if tok == "" {
		t.Fatal("container launch must hand a bearer token to BuildLaunchCommand")
	}
	if buildEnv["AG_SESSION_ID"] != "s1" || buildEnv["AG_FRAME_ID"] != "f1" {
		t.Errorf("identity env not handed to the container: %v", buildEnv)
	}
	if id, ok := h.r.frameReg.Lookup(tok); !ok || id != "f1" {
		t.Errorf("frameReg.Lookup(token) = (%q, %v), want (f1, true)", id, ok)
	}
	if ms, ok := h.r.frameReg.GetMounts("f1"); !ok || len(ms) == 0 {
		t.Errorf("container run-dir mount not registered: GetMounts = (%v, %v)", ms, ok)
	}
	// per-project run dir is keyed by the project path.
	if _, err := os.Stat(h.runDir(project)); err != nil {
		t.Errorf("per-project run dir not created at %s: %v", h.runDir(project), err)
	}
	if _, ok := h.r.containerEndpoints[project]; !ok {
		t.Error("container launch must start the project endpoint")
	}
}

func TestFrameLaunch_ColdStart_Shared(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envShared)

	const project = "/proj/shared-a"
	frame := matrixFrame(project)
	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, frame); err != nil {
		t.Fatalf("spawnFrameWindow: %v", err)
	}

	// Shared mode keys the run dir by SharedContainerKey, not the project path.
	sharedDir := h.runDir(sandboxdc.SharedContainerKey)
	if _, err := os.Stat(sharedDir); err != nil {
		t.Errorf("shared run dir not created at %s: %v", sharedDir, err)
	}
	if _, err := os.Stat(h.runDir(project)); err == nil {
		t.Errorf("shared mode must not create a per-project run dir at %s", h.runDir(project))
	}
	if h.mgr.buildEnv()["AG_SOCKET_TOKEN"] == "" {
		t.Error("shared container launch must still inject a bearer token")
	}
}

// TestFrameLaunch_ColdStart_RecoverableCodexSpawnsResume guards the headline
// regression: a stopped codex frame with a usable rollout path is relaunched via
// the stream subsystem for resume; a stopped generic frame (no durable state) is skipped.
func TestFrameLaunch_ColdStart_RecoverableCodexSpawnsResume(t *testing.T) {
	h := newLaunchHarness(t, envHost)
	now := time.Now()
	rolloutPath := filepath.Join(t.TempDir(), "rollout-"+codexThreadID+".jsonl")
	if err := os.WriteFile(rolloutPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	codex := state.GetDriver("codex")
	generic := state.GetDriver("generic")
	sess := state.Session{
		ID: "s1", Project: "/proj",
		Frames: []state.SessionFrame{
			{ID: "f-codex", Project: "/proj", Command: "codex",
				Driver: codex.Restore(map[string]string{
					"status":       "stopped",
					"thread_id":    codexThreadID,
					"rollout_path": rolloutPath,
				}, now)},
			{ID: "f-gen", Project: "/proj", Command: "generic",
				Driver: generic.Restore(map[string]string{"status": "stopped"}, now)},
		},
	}

	if err := h.r.recreateSessionFrames("s1", sess); err != nil {
		t.Fatalf("recreateSessionFrames: %v", err)
	}

	h.backend.mu.Lock()
	calls := h.backend.spawnCalls
	spawnedIDs := append([]string(nil), h.backend.spawnFrameIDs...)
	h.backend.mu.Unlock()
	if calls != 1 {
		t.Fatalf("SpawnFrame calls = %d, want 1 (codex resumed, stopped generic skipped)", calls)
	}
	spawnedSet := make(map[string]struct{}, len(spawnedIDs))
	for _, id := range spawnedIDs {
		spawnedSet[id] = struct{}{}
	}
	if _, ok := spawnedSet["f-codex"]; !ok {
		t.Error("recoverable stopped codex frame must be relaunched on cold start")
	}
	if _, ok := spawnedSet["f-gen"]; ok {
		t.Error("stopped generic frame must be skipped on cold start")
	}
	if h.kinds.count(state.LaunchSubsystemStream) != 1 {
		t.Errorf("codex must select the stream subsystem, stream ensures = %d", h.kinds.count(state.LaunchSubsystemStream))
	}
}

// TestFrameLaunch_ColdStart_CommandOrder pins the cross-layer launch order.
func TestFrameLaunch_ColdStart_CommandOrder(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envProject)

	frame := matrixFrame("/proj/box")
	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, frame); err != nil {
		t.Fatalf("spawnFrameWindow: %v", err)
	}

	want := []string{
		"subsystem.Ensure",
		"subsystem.BindFrame",
		"mgr.EnsureInstance",
		"mgr.AcquireFrame",
		"mgr.BuildLaunchCommand",
		"backend.SpawnFrame",
	}
	got := h.rec.snapshot()
	if len(got) != len(want) {
		t.Fatalf("launch order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("launch order = %v, want %v (diverge at %d)", got, want, i)
		}
	}
}

func TestFrameLaunch_ColdStart_SubsystemKindSelection(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envHost)

	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, matrixFrame("/proj")); err != nil {
		t.Fatalf("spawnFrameWindow(minimal): %v", err)
	}
	if h.kinds.count(state.LaunchSubsystemCLI) != 1 || h.kinds.count(state.LaunchSubsystemStream) != 0 {
		t.Errorf("generic/minimal must select cli: cli=%d stream=%d",
			h.kinds.count(state.LaunchSubsystemCLI), h.kinds.count(state.LaunchSubsystemStream))
	}
}

// === new session (spawnFrameWindow goroutine + handleSpawnComplete loop) ===

func (h *launchHarness) newSessionSpawn(t *testing.T, e state.EffSpawnFrame) internalSpawnComplete {
	t.Helper()
	// In production the reducer adds the session+frame before EffSpawnFrame
	// is emitted; handleSpawnComplete's 027 frame-alive check assumes that
	// invariant. Seed the same shape here so the loop completion path doesn't
	// discard our spawn as an orphan.
	if _, ok := h.r.state.Sessions[e.SessionID]; !ok {
		h.r.state.Sessions[e.SessionID] = state.Session{
			ID: e.SessionID, Project: e.Plan.Project,
			Frames: []state.SessionFrame{{ID: e.FrameID, Project: e.Plan.Project, Command: e.Plan.Command}},
		}
	}
	internalCh := make(chan internalEvent, 1)
	eventCh := make(chan state.Event, 1)
	deps := spawnDeps{
		backend:      h.r.cfg.Backend,
		launcher:     launcher(h.r.cfg),
		factories:    h.r.subsystemFactories,
		sendInternal: func(ev internalEvent) { internalCh <- ev },
		sendEvent:    func(ev state.Event) { eventCh <- ev },
	}
	spawnFrameWindow(deps, e)
	select {
	case ev := <-internalCh:
		sc, ok := ev.(internalSpawnComplete)
		if !ok {
			t.Fatalf("expected internalSpawnComplete, got %T", ev)
		}
		h.r.handleSpawnComplete(sc)
		return sc
	case ev := <-eventCh:
		t.Fatalf("spawn reported failure: %T %+v", ev, ev)
		return internalSpawnComplete{}
	}
}

func TestFrameLaunch_NewSession_Host(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envHost)

	sc := h.newSessionSpawn(t, state.EffSpawnFrame{
		SessionID: "s1", FrameID: "f1",
		Plan: state.LaunchPlan{Project: "/proj/host", Command: "minimal-test"},
		Env:  map[string]string{"AG_SESSION_ID": "s1", "AG_FRAME_ID": "f1"},
	})

	if sc.token != "" {
		t.Errorf("host spawn must produce an empty token, got %q", sc.token)
	}
	env := h.spawnEnv(t)
	if env["AG_SOCKET"] != h.sockPath {
		t.Errorf("AG_SOCKET = %q, want %q", env["AG_SOCKET"], h.sockPath)
	}
	if env["AG_SOCKET_TOKEN"] != "" {
		t.Errorf("host spawn AG_SOCKET_TOKEN = %q, want empty", env["AG_SOCKET_TOKEN"])
	}
	if len(h.r.containerEndpoints) != 0 {
		t.Error("host new-session must not start a container endpoint")
	}
}

func TestFrameLaunch_NewSession_PerProject(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envProject)

	const project = "/proj/box"
	sc := h.newSessionSpawn(t, state.EffSpawnFrame{
		SessionID: "s1", FrameID: "f1",
		Plan: state.LaunchPlan{Project: project, Command: "minimal-test"},
		Env:  map[string]string{"AG_SESSION_ID": "s1", "AG_FRAME_ID": "f1"},
	})

	if sc.token == "" {
		t.Fatal("container spawn must produce a bearer token")
	}
	if h.mgr.buildEnv()["AG_SOCKET_TOKEN"] != sc.token {
		t.Errorf("token handed to BuildLaunchCommand = %q, want %q", h.mgr.buildEnv()["AG_SOCKET_TOKEN"], sc.token)
	}
	if id, ok := h.r.frameReg.Lookup(sc.token); !ok || id != "f1" {
		t.Errorf("frameReg.Lookup(token) = (%q, %v), want (f1, true)", id, ok)
	}
	if _, ok := h.r.frameReg.GetMounts("f1"); !ok {
		t.Error("container new-session must register run-dir mounts")
	}
	if _, ok := h.r.containerEndpoints[project]; !ok {
		t.Error("container new-session must start the project endpoint")
	}
}

// === warm start (RecoverSandboxFrames) ===

func TestFrameLaunch_WarmStart_Host(t *testing.T) {
	h := newLaunchHarness(t, envHost)
	h.r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: "/proj/host",
		Frames: []state.SessionFrame{matrixFrame("/proj/host")},
	}

	h.r.RecoverSandboxFrames(context.Background())

	// DirectDispatcher.AdoptFrame returns nil cleanup / nil mounts for host.
	if _, ok := h.r.sandboxCleanups["f1"]; ok {
		t.Error("host adopt returns nil cleanup; nothing should be stored")
	}
	if _, ok := h.r.frameReg.GetMounts("f1"); ok {
		t.Error("host adopt has no mounts to register")
	}
	if len(h.r.containerEndpoints) != 0 {
		t.Error("host warm start must not start a container endpoint")
	}
}

func TestFrameLaunch_WarmStart_PerProject(t *testing.T) {
	h := buildLaunchHarness(t, envProject, true) // warm-frame store needed for token recovery
	const project = "/proj/box"

	// Warm start adopts a pre-running frame; unlike cold start's Wrap it does not
	// create the run dir, so the endpoint socket's parent must already exist.
	if _, err := EnsureProjectRunDir(filepath.Join(h.dataDir, "run"), project); err != nil {
		t.Fatalf("EnsureProjectRunDir: %v", err)
	}
	// A bearer token persisted by the prior daemon run must be re-registered.
	if err := h.r.warmFrames.Save(WarmFrameState{FrameID: "f1", ContainerToken: "warm-tok"}); err != nil {
		t.Fatalf("warm save: %v", err)
	}
	h.r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: project,
		Frames: []state.SessionFrame{matrixFrame(project)},
	}

	h.r.RecoverSandboxFrames(context.Background())

	if id, ok := h.r.frameReg.Lookup("warm-tok"); !ok || id != "f1" {
		t.Errorf("warm token not recovered: Lookup = (%q, %v), want (f1, true)", id, ok)
	}
	if h.mgr.acquireN != 1 {
		t.Errorf("AdoptFrame must AcquireFrame once, acquireN = %d", h.mgr.acquireN)
	}
	if _, ok := h.r.sandboxCleanups["f1"]; !ok {
		t.Error("container adopt cleanup must be stored")
	}
	if ms, ok := h.r.frameReg.GetMounts("f1"); !ok || len(ms) == 0 {
		t.Errorf("adopt mounts not stored: GetMounts = (%v, %v)", ms, ok)
	}
	if _, ok := h.r.containerEndpoints[project]; !ok {
		t.Error("container warm start must restart the project endpoint")
	}
}

// TestWarmStart_AtomicMultiFrame guards the 029 F6 fix. With multiple frames
// in the same project the per-frame loop now uses RegisterWithMounts so every
// frame's token and mounts land behind one lock. Before the fix,
// recoverWarmTokens would Register the second frame's token immediately
// while StoreMounts came later in the same loop — leaving a window where
// the same-project endpoint was already live (from frame 1's iteration) and
// an incoming container hook would see Lookup(token)=ok but
// GetMounts(frame)=miss, leaking container-relative paths.
//
// Test name is intentionally short — Unix-domain socket paths under
// t.TempDir() have a 108-byte limit and long test names push the per-project
// run dir over the edge.
func TestWarmStart_AtomicMultiFrame(t *testing.T) {
	h := buildLaunchHarness(t, envProject, true)
	const project = "/proj/multi"

	if _, err := EnsureProjectRunDir(filepath.Join(h.dataDir, "run"), project); err != nil {
		t.Fatalf("EnsureProjectRunDir: %v", err)
	}
	for _, e := range []struct{ id, tok string }{
		{"f-a", "warm-tok-a"},
		{"f-b", "warm-tok-b"},
	} {
		if err := h.r.warmFrames.Save(WarmFrameState{FrameID: e.id, ContainerToken: e.tok}); err != nil {
			t.Fatalf("warm save %s: %v", e.id, err)
		}
	}
	h.r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: project,
		Frames: []state.SessionFrame{
			{ID: "f-a", Project: project, Command: "minimal-test", Driver: state.DriverStateBase{}},
			{ID: "f-b", Project: project, Command: "minimal-test", Driver: state.DriverStateBase{}},
		},
	}

	h.r.RecoverSandboxFrames(context.Background())

	// Every recovered frame must have BOTH a token lookup and a mounts entry —
	// the atomicity contract of RegisterWithMounts.
	for _, e := range []struct{ id, tok string }{
		{"f-a", "warm-tok-a"},
		{"f-b", "warm-tok-b"},
	} {
		if id, ok := h.r.frameReg.Lookup(e.tok); !ok || string(id) != e.id {
			t.Errorf("Lookup(%q) = (%q, %v), want (%q, true)", e.tok, id, ok, e.id)
		}
		if ms, ok := h.r.frameReg.GetMounts(state.FrameID(e.id)); !ok || len(ms) == 0 {
			t.Errorf("GetMounts(%q) = (%v, %v), want mounts present alongside token", e.id, ms, ok)
		}
	}
	if _, ok := h.r.containerEndpoints[project]; !ok {
		t.Error("shared-project endpoint must be running for both frames")
	}
}

// TestWarmStart_OrphanPruned guards the orphan-pruning behaviour of
// recoverWarmTokens: a warm file for a frame that no longer exists in
// r.state.Sessions must be deleted at startup so warm/ doesn't accumulate
// stale tokens that the framereg would happily rebind.
func TestWarmStart_OrphanPruned(t *testing.T) {
	h := buildLaunchHarness(t, envProject, true)
	if err := h.r.warmFrames.Save(WarmFrameState{FrameID: "ghost", ContainerToken: "orphan-tok"}); err != nil {
		t.Fatalf("warm save: %v", err)
	}
	// No session for "ghost" — recoverWarmTokens must prune it.

	tokens := h.r.recoverWarmTokens()

	if _, ok := tokens[state.FrameID("ghost")]; ok {
		t.Error("recoverWarmTokens returned a staged token for a frame absent from state")
	}
	states, err := h.r.warmFrames.LoadAll()
	if err != nil {
		t.Fatalf("warmFrames.LoadAll: %v", err)
	}
	for _, st := range states {
		if st.FrameID == "ghost" {
			t.Error("orphan warm file was not deleted on recovery")
		}
	}
}

// managedMessagingDriver mirrors minimalDriver but PrepareLaunch returns
// LaunchPlan with ManagedFrameMessaging=true. Used by the cold-start
// ManagedFrameMessaging matrix cell to prove the parallel spawnFrameWindow
// implementation (bootstrap_coldstart.go:113) also threads the field.
type managedMessagingDriver struct{}

func (managedMessagingDriver) Name() string        { return "managed-messaging-test" }
func (managedMessagingDriver) DisplayName() string { return "managed-messaging-test" }
func (managedMessagingDriver) Status(_ state.DriverState) state.Status {
	return state.StatusIdle
}
func (managedMessagingDriver) NewState(_ time.Time) state.DriverState        { return state.DriverStateBase{} }
func (managedMessagingDriver) Persist(_ state.DriverState) map[string]string { return nil }
func (managedMessagingDriver) Restore(_ map[string]string, _ time.Time) state.DriverState {
	return state.DriverStateBase{}
}
func (managedMessagingDriver) View(_ state.DriverState) state.View { return state.View{} }
func (managedMessagingDriver) Step(prev state.DriverState, _ state.FrameContext, _ state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	return prev, nil, state.View{}
}
func (managedMessagingDriver) PrepareLaunch(_ state.DriverState, _ state.LaunchMode, project, command string, _ state.LaunchOptions, _ bool) (state.LaunchPlan, error) {
	return state.LaunchPlan{
		Command:               command,
		StartDir:              project,
		ManagedFrameMessaging: true,
	}, nil
}
func (managedMessagingDriver) StartDir(_ state.DriverState) string { return "" }
func (managedMessagingDriver) WithStartDir(s state.DriverState, _ string) state.DriverState {
	return s
}

func registerManagedMessagingDriver(t *testing.T) {
	t.Helper()
	saved := state.GetRegistry()
	t.Cleanup(func() {
		state.ClearRegistry()
		for _, d := range saved {
			state.Register(d)
		}
	})
	if _, ok := saved[managedMessagingDriver{}.Name()]; !ok {
		state.Register(managedMessagingDriver{})
	}
}

// managedHomeDirs enumerates the `<frame-id>-XXX` overlay directories that
// PrepareManagedClaudeHome creates under `<dataDir>/managed-claude-home/`.
// Returns an empty slice if the base dir does not exist so callers can
// distinguish "overlay never created" from "overlay created and cleaned up".
func managedHomeDirs(t *testing.T, dataDir string) []string {
	t.Helper()
	base := filepath.Join(dataDir, "managed-claude-home")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read managed-claude-home dir: %v", err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, filepath.Join(base, e.Name()))
		}
	}
	return out
}

// TestFrameLaunch_NewSession_Host_ManagedMessaging is the positive matrix cell
// that would have caught the live symptom of the field-drop defect: a
// host-launched claude session with LaunchPlan.ManagedFrameMessaging=true.
//
// Positive assertions (spec-20260714 FR-002):
//   - AG_SOCKET_TOKEN is non-empty (wrapLaunchForSpawn's needsToken branch)
//   - a managed-claude-home overlay directory was created under DataDir
//   - the spawned process's HOME env is redirected to that overlay dir
//
// Anchored at h.newSessionSpawn (which routes through interpret_spawn.go's
// free spawnFrameWindow) — NOT h.r.spawnFrameWindow, which uses the
// bootstrap_coldstart.go path and would trivially pass because the cold-start
// path never constructed a flat EffSpawnFrame. Pairing the positive case with
// the cold-start entrypoint would let the bug re-land unnoticed.
func TestFrameLaunch_NewSession_Host_ManagedMessaging(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envHost)

	sc := h.newSessionSpawn(t, state.EffSpawnFrame{
		SessionID: "s-managed", FrameID: "f-managed",
		Plan: state.LaunchPlan{
			Project:               "/proj/host",
			Command:               "minimal-test",
			ManagedFrameMessaging: true,
		},
		Env: map[string]string{"AG_SESSION_ID": "s-managed", "AG_FRAME_ID": "f-managed"},
	})

	if sc.token == "" {
		t.Fatal("host+ManagedFrameMessaging must produce a bearer token (needsToken branch)")
	}
	env := h.spawnEnv(t)
	if env["AG_SOCKET_TOKEN"] == "" {
		t.Errorf("host+ManagedFrameMessaging spawn env has empty AG_SOCKET_TOKEN — token dropped between wrapLaunchForSpawn and the pty child")
	}
	if env["AG_SOCKET"] != h.sockPath {
		t.Errorf("AG_SOCKET = %q, want %q", env["AG_SOCKET"], h.sockPath)
	}
	overlays := managedHomeDirs(t, h.dataDir)
	if len(overlays) == 0 {
		t.Fatal("managed-claude-home overlay was not created — PrepareManagedClaudeHome was skipped despite ManagedFrameMessaging=true")
	}
	if env["HOME"] == "" {
		t.Errorf("spawn env HOME is empty — overlay HOME was not injected")
	} else {
		matched := false
		for _, o := range overlays {
			if env["HOME"] == o {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("spawn env HOME = %q does not match any created overlay dir %v", env["HOME"], overlays)
		}
	}
	if env[agentlaunch.ManagedClaudeOverlayHomeEnv] == "" {
		t.Errorf("%s env is empty — overlay marker missing", agentlaunch.ManagedClaudeOverlayHomeEnv)
	}
	if env[agentlaunch.ManagedClaudeRealHomeEnv] == "" {
		t.Errorf("%s env is empty — real HOME marker missing", agentlaunch.ManagedClaudeRealHomeEnv)
	}
}

// TestFrameLaunch_NewSession_Host asserts the negative case (FR-003): a host
// launch WITHOUT ManagedFrameMessaging must NOT produce a token or an
// overlay. This test already exists above; the pairing here documents the
// symmetry.
//
// TestFrameLaunch_ColdStart_Host_ManagedMessaging guards the parallel
// implementation of spawnFrameWindow in bootstrap_coldstart.go
// (adr-20260714-coldstart-spawn-parallel-implementation). Even though the
// cold-start path is not defective today (it threads LaunchPlan through as a
// value), a future refactor could regress it independently of interpret_spawn.
// Machine-check the same observables on both entry points.
func TestFrameLaunch_ColdStart_Host_ManagedMessaging(t *testing.T) {
	registerManagedMessagingDriver(t)
	h := newLaunchHarness(t, envHost)

	frame := state.SessionFrame{
		ID:      "f-cold-managed",
		Project: "/proj/host",
		Command: "managed-messaging-test",
		Driver:  state.DriverStateBase{},
	}
	if err := h.r.spawnFrameWindow("s-cold-managed", state.SandboxOverrideAuto, frame); err != nil {
		t.Fatalf("spawnFrameWindow: %v", err)
	}

	env := h.spawnEnv(t)
	if env["AG_SOCKET_TOKEN"] == "" {
		t.Errorf("cold-start host+ManagedFrameMessaging spawn env has empty AG_SOCKET_TOKEN — Runtime.spawnFrameWindow dropped the flag")
	}
	overlays := managedHomeDirs(t, h.dataDir)
	if len(overlays) == 0 {
		t.Fatal("cold-start managed-claude-home overlay was not created — bootstrap_coldstart.go regressed independently of interpret_spawn.go")
	}
	if env[agentlaunch.ManagedClaudeOverlayHomeEnv] == "" {
		t.Errorf("cold-start spawn env missing %s marker", agentlaunch.ManagedClaudeOverlayHomeEnv)
	}
}
