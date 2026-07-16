---
change: change-20260714-launchplan-effect-continuity
role: requirements
---

# Requirements

## Legacy Source (verbatim)

````markdown
---
id: spec-20260714-launchplan-effect-continuity
kind: spec
title: LaunchPlan field-continuity across the state->runtime spawn effect boundary
status: approved
created: '2026-07-14'
tags:
- launch
- effect
- refactor
- regression-pin
owners: []
functional_requirements:
- id: FR-001
  type: ubiquitous
  priority: must
  statement: The system shall convey every field of the LaunchPlan a driver's PrepareLaunch
    resolves through state.EffSpawnFrame to the value runtime.spawnFrameWindow passes
    into AgentLauncher.WrapLaunch, with no field dropped, defaulted, or independently
    reconstructed along the way. Concretely for the current LaunchPlan shape this
    means Command, Argv, PreCommands, PreCommandTimeout, StartDir, Project, Sandbox,
    Options (all subfields), Subsystem, Stream (all subfields), Stdin, and ManagedFrameMessaging
    each survive intact.
  rationale: Invariant framing (ubiquitous EARS) — captures the observable-preservation
    contract the field-drop defect class violates and gives the T0 continuity test
    a mechanical falsifier.
- id: FR-002
  type: event_driven
  priority: must
  statement: WHEN a driver's PrepareLaunch returns a LaunchPlan with ManagedFrameMessaging=true
    THEN the system shall generate a bearer token, inject a non-empty AG_SOCKET_TOKEN
    into the spawned process's environment, and apply the managed Claude HOME overlay
    via PrepareManagedClaudeHome before the process is spawned, regardless of whether
    the launch target is a container or the host.
  rationale: Restores host+claude to parity with the container path's already-correct
    behavior; the observable positive case of the fix.
- id: FR-003
  type: unwanted
  priority: must
  statement: IF a LaunchPlan has ManagedFrameMessaging=false and the launch is not
    container-bound THEN the system shall not generate a bearer token, shall not set
    a non-empty AG_SOCKET_TOKEN, and shall not apply the managed Claude HOME overlay.
  rationale: Negative case pinning NFR-003; guards against the fix accidentally broadening
    token issuance to all host launches.
- id: FR-004
  type: event_driven
  priority: must
  statement: WHEN subsystem.BindFrame returns a BindResult carrying an updated LaunchPlan
    (e.g. a worktree-substituted StartDir or codex resume command) THEN the system
    shall use that updated LaunchPlan, not the pre-bind plan, as the input to wrapLaunchForSpawn.
  rationale: Preserves the existing, correct behavior of the plan = bindResult.Plan
    reassignment; the reconstruction-removal must not accidentally revert to the pre-bind
    e.Plan.
- id: FR-005
  type: state_driven
  priority: must
  statement: WHILE runtime.spawnFrameWindow is resolving a launch for a given EffSpawnFrame
    the system shall treat state.EffSpawnFrame.Plan as the sole source of Command,
    Argv, PreCommands, PreCommandTimeout, StartDir, Project, Sandbox, Options, Subsystem,
    Stream, Stdin, and ManagedFrameMessaging for that spawn, with no second independently
    constructed state.LaunchPlan value in play.
  rationale: Pins the SSOT for the launch plan inside the spawn goroutine; forbids
    any future reintroduction of a parallel flat-field carrier on EffSpawnFrame.
- id: FR-006
  type: ubiquitous
  priority: must
  statement: The system shall not require any hand-edit to state.EffSpawnFrame, state.spawnEffect,
    runtime.spawnFrameWindow (interpret_spawn.go, event-loop path), or Runtime.spawnFrameWindow
    (bootstrap_coldstart.go, cold-start path) for a newly added state.LaunchPlan field
    (including a field newly added to a transitively reachable exported nested struct
    such as LaunchOptions, StreamLaunchOptions, or WorktreeOption) to reach AgentLauncher.WrapLaunch.
  rationale: Mechanical falsifiability against a future field addition; names both
    entry points so the cold-start parallel path is not silently outside the invariant.
non_functional_requirements:
- id: NFR-001a
  type: maintainability
  criteria: The Tier T0 field-continuity contract test in the state package (spawnEffect
    -> EffSpawnFrame.Plan structural equivalence) shall run under `go test ./client/state/...`
    with no additional setup, no I/O, and no external dependency (pure in-memory struct
    comparison over a reflection-populated sentinel LaunchPlan).
  measurement: Assert by running `go test -count=1 -run TestSpawnEffect_PlanFieldContinuity
    ./client/state/...` and confirming the test binary imports no I/O packages beyond
    reflect/testing/state.
- id: NFR-001b
  type: maintainability
  criteria: The Tier T1 runtime-side continuity assertion in frame_launch_matrix_test.go
    (EffSpawnFrame.Plan -> plan handed to wrapLaunchForSpawn deep-equals bindResult.Plan)
    shall exercise the real spawnFrameWindow implementation with only the sandbox
    / backend / subsystem fixtures already present in that file — it is not pure T0.
- id: NFR-002
  type: maintainability
  criteria: This fix shall not introduce any new external dependency, so the fake
    / FakeVsReal / contract-test triple required for external-dependency tests (AGENTS.md)
    does not apply; existing sandbox.Manager / recSubsystem fakes already present
    in frame_launch_matrix_test.go are reused unchanged.
- id: NFR-003
  type: security
  criteria: AG_SOCKET_TOKEN generation and injection shall remain scoped exactly to
    `l.IsContainer(project) || plan.ManagedFrameMessaging` in runtime/launcher.go
    wrapLaunchForSpawn; the fix shall not broaden token issuance to host launches
    whose LaunchPlan has ManagedFrameMessaging=false. A regression test (the paired
    negative host + minimal-test case in frame_launch_matrix_test.go) shall assert
    this negative case explicitly.
- id: NFR-004
  type: compatibility
  criteria: EffSpawnFrame shape changes shall not require any snapshot / event-log
    / WS wire-protocol migration; enforcement TestEffSpawnFrameIsNotSerialized in
    the state package reflect-walks EffSpawnFrame's fields and asserts no serialization
    struct tags nor json.Marshaler/Unmarshaler implementations.
acceptance:
- id: AC-001
  given: the reflection-driven T0 continuity fixture constructs a state.LaunchPlan
    with a distinct sentinel per exported leaf (Argv, PreCommands, PreCommandTimeout,
    ManagedFrameMessaging, and every recursively reachable field of LaunchOptions
    / StreamLaunchOptions / WorktreeOption included)
  when: the test calls spawnEffect(sessID, frameID, plan, connID, reqID)
  then: reflect.DeepEqual(effect.Plan, plan) holds, and the walker self-test confirms
    every reachable exported field was populated non-zero — a hand-maintained fixture
    list is not required
  requirement_refs:
  - FR-001
  - FR-005
  - FR-006
- id: AC-002
  given: frame_launch_matrix_test.go with a host + ManagedFrameMessaging=true case
    driven via h.newSessionSpawn (interpret_spawn.go pipeline)
  when: the harness runs the full real-launcher-stack spawn
  then: the captured spawn env contains a non-empty AG_SOCKET_TOKEN and a managed-claude-home
    directory exists under the harness DataDir; the paired ManagedFrameMessaging=false
    case in the same suite continues to observe AG_SOCKET_TOKEN empty and no managed-claude-home
    directory
  requirement_refs:
  - FR-002
  - FR-003
  - NFR-003
- id: AC-003
  given: frame_launch_matrix_test.go with a cold-start host + ManagedFrameMessaging=true
    case driven via h.r.spawnFrameWindow (bootstrap_coldstart.go pipeline)
  when: the harness runs the cold-start spawn against a fake driver whose PrepareLaunch
    returns ManagedFrameMessaging=true
  then: the captured spawn env contains a non-empty AG_SOCKET_TOKEN and a managed-claude-home
    directory exists under the harness DataDir — same observables as the new-session
    case, guarding the parallel bootstrap_coldstart.go pipeline against future field-drop
    regressions
  requirement_refs:
  - FR-002
  - FR-006
- id: AC-004
  given: a codex frame whose subsystem.BindFrame returns a BindResult with a worktree-substituted
    StartDir or a resume-flagged Command
  when: runtime.spawnFrameWindow reassigns plan = bindResult.Plan and invokes wrapLaunchForSpawn
  then: the plan handed to wrapLaunchForSpawn is bindResult.Plan, not e.Plan; the
    T1 continuity assertion in frame_launch_matrix_test.go proves this by capturing
    the plan at wrapLaunchForSpawn entry via a spy hook
  requirement_refs:
  - FR-004
  - FR-005
- id: AC-005
  given: TestEffSpawnFrameIsNotSerialized in the state package
  when: a future edit adds a `json:` / `msgpack:` / `proto:` struct tag to any EffSpawnFrame
    field, or makes EffSpawnFrame satisfy json.Marshaler / json.Unmarshaler
  then: the test fails at run time, blocking the change — guarding the assumption
    that EffSpawnFrame is a pure in-process reducer-to-runtime effect value with no
    snapshot / event-log / wire-protocol migration surface
  requirement_refs:
  - NFR-004
relations:
- {type: references, target: adr-20260706-frame-messaging-managed-tool-exposure}
- {type: references, target: adr-20260711-0082-frame-exec-launcher}
- {type: references, target: adr-20260714-launchplan-effect-embedding}
- {type: references, target: adr-20260714-launch-plan-field-continuity-invariant}
- {type: references, target: adr-20260714-coldstart-spawn-parallel-implementation}
- {type: implementedBy, target: plan-20260714-launchplan-effect-continuity}
derived_from: |
  2026-07-14 debug root-cause analysis of a live symptom — sandbox=host claude sessions
  launching without the frame-messaging bearer token (AG_SOCKET_TOKEN empty) and without
  the managed-claude-home HOME overlay. RCA traced the drop to state.EffSpawnFrame flattening
  the resolved LaunchPlan into duplicated top-level fields (Command / StartDir / Sandbox
  / Options / Subsystem / Stream / Stdin / Project / Mode) and runtime.spawnFrameWindow
  (interpret_spawn.go) reconstructing a state.LaunchPlan literal from those fields, both
  of which enumerate LaunchPlan's fields by hand. ManagedFrameMessaging (added by the
  adr-20260711 family) was added to LaunchPlan without updating either copy site, so it
  silently evaluated false downstream. Argv, PreCommands, and PreCommandTimeout are also
  currently dropped but masked in production by stream/backend.go's BindFrame unconditionally
  rewriting them.
summary: LaunchPlan field-continuity across state.EffSpawnFrame -> runtime.spawnFrameWindow
  is enforced by embedding the resolved LaunchPlan as a single named value field on
  EffSpawnFrame, backed by a Tier T0 reflection-driven continuity test, a T1 matrix
  host+managed-messaging case, and a paired T1 cold-start guard on the parallel bootstrap
  implementation.
---

## Background

A live production symptom surfaced during the 2026-07-14 debug session: `sandbox: "host"` claude sessions were being launched without the frame-messaging bearer token (`AG_SOCKET_TOKEN` empty in `/proc/<pid>/environ`) and without the managed-claude-home HOME overlay directory (`find` returned nothing). The container path was unaffected because `IsContainer(project)` in `runtime/launcher.go` independently forces `needsToken=true`.

Root-cause analysis identified a structural SSOT violation: `state.LaunchPlan` is resolved once per spawn by a driver's `PrepareLaunch`, then copied field-by-field twice more — once into `state.EffSpawnFrame`'s duplicated top-level fields inside `state.spawnEffect` (`src/client/state/reduce_helpers.go`), and again out of those top-level fields into a freshly reconstructed `state.LaunchPlan` literal inside `runtime.spawnFrameWindow` (`src/client/runtime/interpret_spawn.go:64-73`). Each copy site enumerates `LaunchPlan`'s fields by hand. When `ManagedFrameMessaging` was added to `LaunchPlan` (adr-20260711 family) neither copy was updated, so it silently evaluated false at the `needsToken := l.IsContainer(project) || plan.ManagedFrameMessaging` predicate. The blast radius is larger than the live symptom: `Argv`, `PreCommands`, and `PreCommandTimeout` are also silently dropped, masked only because `stream/backend.go:303,306` unconditionally rewrites them inside `BindFrame`.

A second, parallel implementation of the same pipeline exists on the cold-start path (`bootstrap_coldstart.go:113`) — `Runtime.spawnFrameWindow(id, sandbox, frame)` — which is correct today only because it threads the whole `launch LaunchPlan` value between `PrepareLaunch` and `wrapLaunchForSpawn` without a flat-field intermediate. Consolidating the two implementations expands blast radius beyond this fix; a matching T1 cold-start test guards the parallel path against future drift instead.

## Counterexample (spec that this design prevents)

- **誤実装 1**: EffSpawnFrame retains the 8 duplicated flat launch fields plus Mode alongside a new `Plan` field for a "gradual migration" period. Reproduces the two-copies-of-the-same-fact shape the bug exemplifies; readers can consume the stale flat field instead of `Plan`. **Forbidden by FR-005** (Plan is the sole source) and by adr-20260714-launchplan-effect-embedding.
- **誤実装 2**: runtime.spawnFrameWindow retains a defensive `state.LaunchPlan{...}` self-copy for "safety" after `plan := e.Plan`. Adds no correctness value and re-adds the manual field-enumeration code that failed to include ManagedFrameMessaging. **Forbidden by FR-005** and by adr-20260714-launchplan-effect-embedding Alternatives.
- **誤実装 3**: The T0 continuity fixture is a hand-written per-field assertion table (`if plan.Command != want.Command { ... }` per field). Reintroduces the discipline that failed for ManagedFrameMessaging. **Forbidden by NFR-001a and FR-006** — the fixture MUST be reflection-driven with recursion into every reachable exported nested struct.
- **誤実装 4**: The new host + ManagedFrameMessaging=true matrix case is driven via `h.r.spawnFrameWindow` (bootstrap_coldstart.go) instead of `h.newSessionSpawn` (interpret_spawn.go). Would pass trivially even against the pre-fix code because the cold-start path never flattened the plan; the test is vacuous. **Forbidden by AC-002** — the positive case MUST route through the interpret_spawn.go pipeline.
- **誤実装 5**: `EffSpawnFrame.Plan` is held as a pointer field (`Plan *state.LaunchPlan`) allowing downstream mutation. Conflicts with the pure-reducer / effects-as-immutable-values discipline in ARCHITECTURE.md's single-writer architecture. **Forbidden by adr-20260714-launchplan-effect-embedding** — MUST be a named value field.
- **誤実装 6**: `EffSpawnFrame` uses anonymous embedding of `state.LaunchPlan` (field promotion). Silently expands EffSpawnFrame's exported surface, blurs the distinction between launch data and effect-routing data. **Forbidden by adr-20260714-launchplan-effect-embedding**.
- **誤実装 7**: The Mode field is retained "in case a future feature needs it" (speculative). No production reader exists (verified by grep); the only reference is a tautological self-assertion in reduce_session_test.go. **Forbidden by AC-005 and simplicity-critic** — dead surface is removed.
- **誤実装 8**: A `json:` struct tag or a `json.Marshaler` implementation is added to `state.EffSpawnFrame` for a "future snapshot" use case. **Forbidden by NFR-004** — enforced by TestEffSpawnFrameIsNotSerialized in the state package.
- **誤実装 9**: The cold-start `Runtime.spawnFrameWindow` in bootstrap_coldstart.go is refactored to introduce a flat-field intermediate mirroring the pre-fix interpret_spawn.go shape, silently dropping ManagedFrameMessaging. **Forbidden by FR-006 and caught by AC-003** — the paired cold-start T1 matrix case fails immediately.

## Legacy context

- Live-symptom trigger: `sandbox: "host"` claude sessions since the adr-20260711 family landed ManagedFrameMessaging on `state.LaunchPlan` without updating either copy site.
- Dormant drops masked by `stream/backend.go`: Argv, PreCommands, PreCommandTimeout are overwritten unconditionally inside `BindFrame`, so no production symptom surfaces for them today. A future driver that pre-populates these in `PrepareLaunch` would regress without a test signal — mitigated by the T0 continuity test's currently-failing sentinel coverage of these fields.
- Not modified by this fix: `src/client/driver/claude.go` (sole producer of ManagedFrameMessaging=true), `platform/agentlaunch.LaunchPlan` / `agentlaunch.Dispatcher.Wrap` (translation boundary), `platform/agentlaunch/managed_claude_home.go` (already correct), `bridge frame-exec` / AG_FRAME_SPEC transport (adr-20260711-0082/0083/0084).

## Open Questions

- Consolidation of interpret_spawn.go's free `spawnFrameWindow` and bootstrap_coldstart.go's `Runtime.spawnFrameWindow` into a single shared `PrepareLaunch -> BindFrame -> wrapLaunchForSpawn` helper is deferred to a follow-up ADR. The T1 cold-start matrix case is a sufficient interim guard for the LaunchPlan-continuity concern.
- Extraction of a pure exported helper `resolveSpawnPlan(e EffSpawnFrame, bindResult BindResult) LaunchPlan` is deferred; the split T0 (state package) + T1 (matrix hook) coverage closes the observability gap without touching production API surface.

````
