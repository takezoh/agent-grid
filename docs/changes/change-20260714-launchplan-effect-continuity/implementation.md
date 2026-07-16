---
change: change-20260714-launchplan-effect-continuity
role: implementation
---

# Implementation

## Legacy Source (verbatim)

````markdown
---
id: plan-20260714-launchplan-effect-continuity
kind: plan
title: LaunchPlan field-continuity across the state->runtime spawn effect boundary
  — implementation plan
status: draft
created: '2026-07-14'
goal: Eliminate the silent drop of state.LaunchPlan.ManagedFrameMessaging (live symptom)
  and the already-dormant drops of Argv, PreCommands, PreCommandTimeout across the
  spawnEffect -> EffSpawnFrame -> runtime.spawnFrameWindow (interpret_spawn.go) boundary
  by collapsing EffSpawnFrame's duplicated launch fields into a single embedded `Plan
  state.LaunchPlan` field, guarded by a T0 field-continuity contract test, a T1 matrix
  host+managed-messaging case anchored on TestFrameLaunch_NewSession_Host, a matching
  T1 cold-start guard on Runtime.spawnFrameWindow (bootstrap_coldstart.go), and a
  package-local serialization-guard test.
scope_in:
- Add `Plan state.LaunchPlan` as a named value field to state.EffSpawnFrame (src/client/state/effect.go)
  and remove the 9 duplicated flat fields (Command, StartDir, Sandbox, Options, Subsystem,
  Stream, Stdin, Project, Mode)
- Update state.spawnEffect (reduce_helpers.go) to construct EffSpawnFrame{..., Plan
  plan, ...} directly; extend its doc comment with the caller-terminal aliasing invariant
- Delete the state.LaunchPlan{...} reconstruction literal in runtime.spawnFrameWindow
  (interpret_spawn.go:64-73) and migrate e.Project references (lines 75, 85, 101,
  110) to e.Plan.Project
- Add the Tier T0 reflection-driven field-continuity test at src/client/state/spawn_effect_plan_continuity_test.go
- Extend src/client/runtime/frame_launch_matrix_test.go with a host + ManagedFrameMessaging=true
  case driven via h.newSessionSpawn (interpret_spawn.go pipeline), a paired cold-start
  case driven via h.r.spawnFrameWindow (bootstrap_coldstart.go pipeline), and a T1
  continuity spy hook capturing the plan handed to wrapLaunchForSpawn
- Add src/client/state/effspawnframe_serialization_guard_test.go pinning the no-serialization
  invariant
- Mechanically migrate every EffSpawnFrame{...} literal in the 6 test files (spawn_complete_test.go,
  spawn_panic_test.go, reduce_session_test.go, reduce_event_test.go, reduce_fuzz_test.go,
  frame_launch_matrix_test.go) to the .Plan.* shape in the same change
- Manual T3 verification via /proc/<pid>/environ inspection of a host claude session
scope_out:
- Any change to platform/agentlaunch.LaunchPlan, agentlaunch.Dispatcher.Wrap, DirectDispatcher.Wrap,
  or the dispatcherAdapter translation boundary in launcher.go
- The devcontainer / container sandbox launch path (unaffected — IsContainer independently
  forces needsToken=true)
- Changing ManagedFrameMessaging semantics or which drivers set it
- The `bridge frame-exec` / AG_FRAME_SPEC transport mechanism (adr-20260711-0082/0083/0084)
- Any change to frame-messaging broker authority, tool exposure routing, or response-source
  policy (adr-20260706-*)
- FrameLifecycle.SpawnFrame's backend-interface signature (adr-20260712-spawnframe-inline-size-pair)
- Consolidating interpret_spawn.go and bootstrap_coldstart.go spawnFrameWindow implementations
  (deferred to a follow-up ADR; guarded by matching T1 tests in this fix)
- Extracting a pure exported helper resolveSpawnPlan from interpret_spawn.go
- Defensive deep-copy of plan.Argv / PreCommands / Stdin / Options.InitialInput inside
  spawnEffect
- Coupling adr-20260711-0083-launchplan-argv-primary's status transition to this fix's
  landing
milestones:
- id: m1
  title: 'm1 — EffSpawnFrame struct shape shift and runtime passthrough (units collapse-effspawnframe-into-plan,
    rewrite-runtime-spawn-passthrough)'
  status: todo
- id: m2
  title: 'm2 — Test migration, T0 continuity, and serialization guard (units migrate-effspawnframe-test-literals,
    add-t0-field-continuity-test, add-effspawnframe-serialization-guard)'
  status: todo
- id: m3
  title: 'm3 — Matrix coverage and cold-start guard (units add-matrix-host-managed-messaging-case,
    add-coldstart-matrix-guard)'
  status: todo
contracts:
- contract-effspawnframe-plan-embedding
- contract-runtime-spawn-goroutine-plan-passthrough
- contract-needstoken-managed-messaging
- contract-launchplan-field-continuity-test
- contract-frame-launch-matrix-host-managed-messaging
- contract-coldstart-plan-passthrough
contract_projections:
- id: contract-effspawnframe-plan-embedding
  decision_rules: [decision-effspawnframe-carries-plan-as-value-field]
  observable_effects: [observable-effspawnframe-plan-equals-input]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-effspawnframe-nonembedded-launch-field-reintroduced]
  verifications: [verify-effspawnframe-plan-continuity-t0]
  witnesses: [witness-effspawnframe-plan-embedding-normal, witness-effspawnframe-hypothetical-future-nested-field]
- id: contract-runtime-spawn-goroutine-plan-passthrough
  decision_rules: [decision-runtime-uses-eplan-then-bindresult]
  observable_effects: [observable-wraplaunch-receives-postbind-plan]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-runtime-uses-prebind-plan]
  verifications: [verify-runtime-postbind-plan-t1]
  witnesses: [witness-runtime-postbind-plan-normal, witness-runtime-postbind-plan-adversarial]
- id: contract-needstoken-managed-messaging
  decision_rules: [decision-needstoken-scope]
  observable_effects: [observable-agsockettoken-and-home-overlay-iff-needstoken]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-needstoken-false-negative-or-positive]
  verifications: [verify-needstoken-matrix-t1]
  witnesses: [witness-needstoken-host-managed-messaging-true, witness-needstoken-host-managed-messaging-false-adversarial]
- id: contract-launchplan-field-continuity-test
  decision_rules: [decision-reflection-walker-recursion-contract]
  observable_effects: [observable-continuity-walker-covers-all-fields]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-continuity-fixture-drift]
  verifications: [verify-continuity-walker-self-test]
  witnesses: [witness-continuity-walker-covers-current-shape, witness-continuity-hypothetical-nested-field-addition]
- id: contract-frame-launch-matrix-host-managed-messaging
  decision_rules: [decision-matrix-anchors-on-newsession-pipeline]
  observable_effects: [observable-matrix-newsession-host-managed-token-and-overlay]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-matrix-wrong-pipeline-anchor]
  verifications: [verify-matrix-newsession-and-spy-t1]
  witnesses: [witness-matrix-newsession-host-managed-normal, witness-matrix-wrong-entry-point-adversarial]
- id: contract-coldstart-plan-passthrough
  decision_rules: [decision-coldstart-guard-not-consolidate]
  observable_effects: [observable-coldstart-host-managed-token-and-overlay]
  operational_inputs: []
  semantic_profiles: []
  failures: [failure-coldstart-plan-drop]
  verifications: [verify-coldstart-matrix-t1]
  witnesses: [witness-coldstart-host-managed-normal, witness-coldstart-parallel-drift-adversarial]
adrs:
- adr-20260714-launchplan-effect-embedding
- adr-20260714-launch-plan-field-continuity-invariant
- adr-20260714-coldstart-spawn-parallel-implementation
decision_dispositions:
- {decision_input_ref: decision-input-launchplan-embed-full-replacement, disposition: adopted,
   rationale: 'Case B (full replacement — embed state.LaunchPlan as a named value field) is adopted, closing the LaunchPlan-field SSOT drift structurally rather than by hand-sync discipline. adr-20260714-launchplan-effect-embedding fixes the shape; contract-effspawnframe-plan-embedding pins the observable.',
   adr_refs: [adr-20260714-launchplan-effect-embedding],
   contract_refs: [contract-effspawnframe-plan-embedding]}
- {decision_input_ref: decision-input-full-removal-vs-gradual-migration, disposition: adopted,
   rationale: 'Full removal in the same change is adopted. Gradual migration would reintroduce the two-copies-of-the-same-fact shape during the transition; single-PR blast radius (6 test files) is manageable per this repo''s single-task signature-change convention.',
   adr_refs: [adr-20260714-launchplan-effect-embedding],
   contract_refs: [contract-effspawnframe-plan-embedding]}
- {decision_input_ref: decision-input-runtime-reconstruction-removal, disposition: adopted,
   rationale: 'The state.LaunchPlan{...} literal in spawnFrameWindow is deleted outright; `plan := e.Plan` replaces it, with `plan = bindResult.Plan` unchanged after BindFrame. A defensive self-copy would be tautological and re-add the manual-copy drift class.',
   adr_refs: [adr-20260714-launchplan-effect-embedding],
   contract_refs: [contract-runtime-spawn-goroutine-plan-passthrough]}
- {decision_input_ref: decision-input-spawneffect-caller-passthrough, disposition: implementation_detail,
   rationale: 'Direct code reading of reduce_session.go (~lines 74-85, 199-208, 283-292) confirms all 3 spawnEffect callers already hold a fully-resolved launch LaunchPlan value with .Project and .Sandbox set immediately before the call. No observable effect / owner / failure semantic changes based on whether an intermediate conversion/validation helper is introduced; either choice satisfies contract-effspawnframe-plan-embedding.',
   adr_refs: [],
   contract_refs: [contract-effspawnframe-plan-embedding],
   implementation_decision_ref: spawn-effect-caller-passthrough-form}
- {decision_input_ref: decision-input-t0-field-continuity-test, disposition: adopted,
   rationale: 'adr-20260714-launch-plan-field-continuity-invariant adopts a reflection-driven T0 fixture with a pinned recursion contract (scalar / enum / string / bool / time.Duration / []byte / []string / [][]string leaves) plus a T1 runtime spy hook for the wrapLaunchForSpawn-input leg; together they close FR-006 mechanically.',
   adr_refs: [adr-20260714-launch-plan-field-continuity-invariant],
   contract_refs: [contract-launchplan-field-continuity-test, contract-frame-launch-matrix-host-managed-messaging]}
- {decision_input_ref: decision-input-plan-field-holding-strategy, disposition: adopted,
   rationale: 'Named value field `Plan state.LaunchPlan` is adopted (not pointer, not anonymous embedding). Rationale in adr-20260714-launchplan-effect-embedding: preserves reducer-emit / effect-consume immutability discipline (no cross-goroutine mutation), keeps effect-routing data distinct from launch data (no promoted-name blur), and does not require a defensive deep-copy (aliasing invariant pinned by spawnEffect doc comment).',
   adr_refs: [adr-20260714-launchplan-effect-embedding],
   contract_refs: [contract-effspawnframe-plan-embedding]}
- {decision_input_ref: decision-input-no-parallel-transport-field-precedent, disposition: adopted,
   rationale: 'The design-culture precedent from adr-20260712-launch-size-pass-through''s Alternatives B (rejecting a parallel SpawnHint field) is honored: EffSpawnFrame carries the launch-plan meaning in exactly one slot (Plan). No parallel field is added for the same meaning. Bound to adr-20260714-launchplan-effect-embedding.',
   adr_refs: [adr-20260714-launchplan-effect-embedding],
   contract_refs: [contract-effspawnframe-plan-embedding]}
reference_algorithms: []
relations:
- {type: implements, target: spec-20260714-launchplan-effect-continuity}
- {type: hasPart, target: adr-20260714-launchplan-effect-embedding}
- {type: hasPart, target: adr-20260714-launch-plan-field-continuity-invariant}
- {type: hasPart, target: adr-20260714-coldstart-spawn-parallel-implementation}
summary: Collapse EffSpawnFrame's duplicated launch fields into a single embedded
  state.LaunchPlan, backed by a T0 reflection-driven continuity test, a T1 matrix
  host+managed-messaging case anchored on TestFrameLaunch_NewSession_Host, and a paired
  T1 cold-start guard on the parallel bootstrap implementation.
---

## Goal

Eliminate the silent drop of state.LaunchPlan.ManagedFrameMessaging (the live symptom) and the already-dormant drops of Argv, PreCommands, and PreCommandTimeout across the spawnEffect -> EffSpawnFrame -> runtime.spawnFrameWindow (interpret_spawn.go, event-loop path) boundary by collapsing EffSpawnFrame's duplicated launch fields into a single embedded `Plan state.LaunchPlan` value field.

## Non-goals

- Any change to `platform/agentlaunch.LaunchPlan` / `agentlaunch.Dispatcher.Wrap` / `DirectDispatcher.Wrap` / `dispatcherAdapter` translation boundary
- Consolidating interpret_spawn.go's free `spawnFrameWindow` with bootstrap_coldstart.go's `Runtime.spawnFrameWindow` — deferred to a follow-up ADR
- Extracting a pure exported helper `resolveSpawnPlan(e, bindResult) LaunchPlan` — deferred; the split T0 + T1 coverage closes the observability gap without new production API surface
- Changing ManagedFrameMessaging semantics or the set of drivers that set it

## Targets (seams touched)

Production edits:

- `src/client/state/effect.go` — EffSpawnFrame struct: replace the 9 duplicated flat launch fields (Command, StartDir, Sandbox, Options, Subsystem, Stream, Stdin, Project, Mode) with a single named value field `Plan state.LaunchPlan`. Effect-routing fields (SessionID, FrameID, Env, ReplyConn, ReplyReqID) unchanged.
- `src/client/state/reduce_helpers.go` — `state.spawnEffect`: construct `EffSpawnFrame{..., Plan: plan, ...}` directly; drop the `Mode: LaunchModeCreate` hardcode and the 8 individual field copies. Extend the doc comment with the caller-terminal aliasing invariant (Argv / PreCommands / Stdin / Options.InitialInput share backing arrays across the shallow value copy).
- `src/client/runtime/interpret_spawn.go` — `spawnFrameWindow` (free function, event-loop path): delete the `state.LaunchPlan{...}` reconstruction literal at lines 64-73; replace with `plan := e.Plan`; migrate every `e.Project` reference (lines 75, 85, 101, 110) to `e.Plan.Project`. Preserve the existing `plan = bindResult.Plan` reassignment after `subsystem.BindFrame`.

Injection / dispatch seams unchanged by this fix (grounded because correctness is defined relative to them):

- `runtime.wrapLaunchForSpawn` (`src/client/runtime/launcher.go`) — `needsToken := l.IsContainer(project) || plan.ManagedFrameMessaging` predicate is already correct; behavior is restored once EffSpawnFrame.Plan carries ManagedFrameMessaging intact.
- `agentlaunch.PrepareManagedClaudeHome` (`src/platform/agentlaunch/managed_claude_home.go`) — invocation restored once `needsToken` evaluates true for host+claude.
- `bootstrap_coldstart.go:113` — `Runtime.spawnFrameWindow(id, sandbox, frame)` — parallel implementation, not modified by this fix but guarded by a paired T1 matrix case.
- `subsystem.BindFrame` (`src/client/runtime/subsystem/subsystem.go`) — `BindRequest{Plan: plan}` already threads the whole plan, unchanged.
- Three `spawnEffect` call sites in `src/client/state/reduce_session.go` (reduceCreateSession, pushDriverInternal, spawnForkSession) — already hold a fully-resolved `launch LaunchPlan` value; not modified.

Test seams:

- New file: `src/client/state/spawn_effect_plan_continuity_test.go` — Tier T0 reflection-driven field-continuity test with a walker self-test.
- New file: `src/client/state/effspawnframe_serialization_guard_test.go` — package-local NFR-004 guard (no json/msgpack/proto struct tags; no json.Marshaler/Unmarshaler).
- Extend: `src/client/runtime/frame_launch_matrix_test.go` — add TestFrameLaunch_NewSession_Host_ManagedMessaging (positive host case anchored on h.newSessionSpawn / interpret_spawn.go pipeline), TestFrameLaunch_ColdStart_Host_ManagedMessaging (paired cold-start guard on h.r.spawnFrameWindow), and the T1 spy hook capturing the plan handed to wrapLaunchForSpawn.
- Migrate literals in 6 test files (state and runtime packages) from top-level EffSpawnFrame fields to `.Plan.*`: `spawn_complete_test.go`, `spawn_panic_test.go`, `frame_launch_matrix_test.go` (lines 510-513 and 535-538), `reduce_session_test.go` (also delete the tautological Mode assertion at lines 332-333), `reduce_event_test.go`, `reduce_fuzz_test.go`.

## Approach

The defect is a structural SSOT violation, not a logic bug (see `spec.md` §Background). The fix embeds `Plan state.LaunchPlan` as EffSpawnFrame's single carrier of launch data (case-B full replacement adopted under adr-20260714-launchplan-effect-embedding); Case A (gradual migration retaining both flat fields and Plan) is rejected because it reintroduces the two-copies-of-the-same-fact shape during the transition period.

The 9 duplicated flat fields on EffSpawnFrame (Command, StartDir, Sandbox, Options, Subsystem, Stream, Stdin, Project, Mode) are removed in the same change — including Project (per critique issue-project-field-duplication-retained: leaving Project retained alongside Plan.Project would reproduce the exact SSOT-violation shape the approach claims to eliminate) and Mode (per critique issue-mode-field-role-undefined: dead hardcoded field with no production reader).

The runtime-side reconstruction literal in `interpret_spawn.go:64-73` is deleted outright; `plan := e.Plan` replaces it, with the existing `plan = bindResult.Plan` reassignment after `subsystem.BindFrame` unchanged (FR-004 preserved). Every `e.Project` reference at lines 75, 85, 101, 110 migrates to `e.Plan.Project` in the same change. All 6 test files that construct `EffSpawnFrame{...}` literals or read the removed top-level fields migrate to the `.Plan.*` shape per the repo's single-task signature-change convention (frame_launch_matrix_test.go is the 6th file previously missed per critique issue-test-migration-underscoped-matrix-file).

Two structural gaps are closed by test additions:

1. **Silent field drop invisibility (FR-001, FR-006)** — closed by a Tier T0 reflection-driven continuity test in the state package under adr-20260714-launch-plan-field-continuity-invariant. The fixture walks LaunchPlan's `reflect.Type` recursively, terminating only at scalar / enum / string / bool / time.Duration / []byte / []string / [][]string leaves and recursing into every reachable exported nested struct (LaunchOptions, StreamLaunchOptions, WorktreeOption, any future nested struct); a walker self-test asserts every reachable exported field is non-zero after the walk. Per-kind sentinels are pinned in the ADR (strings → unique field-path label; ints/enums → prime-number ladder; time.Duration → distinct ns per field-path; bool → true; []byte → non-nil non-empty with field-path bytes; slices → non-nil single-element seeded). Complementary runtime-side continuity (EffSpawnFrame.Plan → plan handed to wrapLaunchForSpawn) is asserted at Tier T1 via a spy hook on the matrix harness — no new exported production surface (extraction of `resolveSpawnPlan` deferred per critique issue-t0-test-seam-unavailable).

2. **Real-launcher-stack coverage gap for host+ManagedFrameMessaging=true (FR-002, FR-003)** — the existing matrix covered host (only via a driver that never sets ManagedFrameMessaging) and container (via IsContainer's independent true branch). The new positive case is anchored on `h.newSessionSpawn` (mirroring TestFrameLaunch_NewSession_Host at line 506) which routes through `interpret_spawn.go`'s free `spawnFrameWindow` — the exact pipeline this fix modifies (per critique issue-matrix-test-wrong-path-pairing: anchoring on `h.r.spawnFrameWindow` cold-start would pass trivially even against the pre-fix code because that path never flattened the plan).

The cold-start parallel implementation (`bootstrap_coldstart.go:113` — `Runtime.spawnFrameWindow`) is a second, independent implementation of the same `PrepareLaunch → BindFrame → wrapLaunchForSpawn` pipeline (per critique issue-cold-start-parallel-spawn-not-analyzed). It is not modified by this fix but is guarded by a matching Tier T1 case (`TestFrameLaunch_ColdStart_Host_ManagedMessaging`) that drives `h.r.spawnFrameWindow` with a fake driver whose `PrepareLaunch` returns `ManagedFrameMessaging=true`, asserting the same AG_SOCKET_TOKEN + managed-claude-home observables — closing the parallel-drift gap under adr-20260714-coldstart-spawn-parallel-implementation. Consolidation into a shared helper is deferred.

Assumption-3 (EffSpawnFrame is a pure in-process reducer-to-runtime effect value, not persisted / not on the wire) is pinned by a new package-local `TestEffSpawnFrameIsNotSerialized` that reflect-walks EffSpawnFrame's fields and asserts no `json` / `msgpack` / `proto` struct tag is present and that EffSpawnFrame does not satisfy `json.Marshaler` / `json.Unmarshaler` (per critique issue-assumption-3-not-grounded, NFR-004).

The shallow-copy aliasing hazard on `Argv` / `PreCommands` / `Stdin` / `Options.InitialInput` (Go's struct value copy is shallow — slice/map fields share backing arrays across `EffSpawnFrame.Plan = plan`) is pinned by a doc-comment invariant on `spawnEffect` (callers must treat plan as terminal at the call site) rather than a defensive deep-copy (per critique issue-value-field-aliasing-claim-incorrect; deferred deep-copy is documented as a follow-up if a future caller lands outside client/state).

## Verification

### Tier T0 (pure Go unit, no I/O)

- `src/client/state/spawn_effect_plan_continuity_test.go` — reflection-driven `TestSpawnEffect_PlanFieldContinuity` asserts `reflect.DeepEqual(spawnEffect(...).Plan, plan)` for a sentinel LaunchPlan built via the recursive walker; `TestSpawnEffect_PlanFieldContinuity_WalkerCoverage` asserts every reachable exported field is non-zero after the walk (proves the fixture is not silently incomplete). Command: `cd src && go test -count=1 -run TestSpawnEffect_PlanFieldContinuity ./client/state/...`. Criterion: pass, with the walker touching every reachable exported field of LaunchPlan including nested LaunchOptions / StreamLaunchOptions / WorktreeOption. Currently-failing sentinels (Argv, PreCommands, PreCommandTimeout) prove the fix eliminates the drop class beyond the visible ManagedFrameMessaging instance.
- `src/client/state/effspawnframe_serialization_guard_test.go` — `TestEffSpawnFrameIsNotSerialized` reflect-walks EffSpawnFrame's exported fields and asserts no `json` / `msgpack` / `proto` struct tag is present; asserts EffSpawnFrame does not satisfy `json.Marshaler` / `json.Unmarshaler`. Command: `cd src && go test -count=1 -run TestEffSpawnFrameIsNotSerialized ./client/state/...`. Criterion: pass; introducing a `json:` tag on any EffSpawnFrame field or making EffSpawnFrame satisfy `json.Marshaler` fails the test.

### Tier T1 (wired, no external process)

- `src/client/runtime/frame_launch_matrix_test.go` — new `TestFrameLaunch_NewSession_Host_ManagedMessaging` (positive host case via `h.newSessionSpawn` → `interpret_spawn.go` pipeline) asserts captured `AG_SOCKET_TOKEN != ""` and managed-claude-home directory exists under harness DataDir. Existing `TestFrameLaunch_NewSession_Host` (negative host case, minimal-test driver, `ManagedFrameMessaging=false`) continues to assert both absent — proving no over-broadening. Command: `cd src && go test -count=1 -run TestFrameLaunch_NewSession_Host ./client/runtime/...`. Criterion: both cases pass; a temporary revert of `rewrite-runtime-spawn-passthrough` makes the positive case fail (proving the assertion is not vacuous).
- Same file — new `TestFrameLaunch_ColdStart_Host_ManagedMessaging` (cold-start via `h.r.spawnFrameWindow` → `bootstrap_coldstart.go` pipeline) asserts identical AG_SOCKET_TOKEN + managed-claude-home observables against a fake driver whose `PrepareLaunch` returns `ManagedFrameMessaging=true`. Command: `cd src && go test -count=1 -run TestFrameLaunch_ColdStart_Host ./client/runtime/...`. Criterion: pass; existing negative case continues to observe token absent.
- Same file — T1 spy hook (launcher spy or `deps.wrapLaunchHook`) captures the plan handed to `wrapLaunchForSpawn` when the harness runs `h.newSessionSpawn` with a sentinel plan and a `BindFrame` fake that rewrites `StartDir` / `Command`; asserts the captured plan equals `bindResult.Plan`, not `e.Plan`. Command: `cd src && go test -count=1 -run TestFrameLaunch_NewSession ./client/runtime/...`. Criterion: assertion passes; a temporary revert to using `e.Plan` post-bind at the wrapLaunchForSpawn call site makes the assertion fail.

### Tier T2 (contract / subprocess)

- Not required for this fix — no new external dependency (NFR-002), so the fake / FakeVsReal / contract-test triple does not apply.

### Tier T3 (manual fidelity verification)

- Create a `sandbox: "host"` claude session via `POST /api/sessions`; inspect `/proc/<claude-pid>/environ` and confirm HOME points at a managed-claude-home overlay directory and `AG_SOCKET_TOKEN` is non-empty. This is manual, not scripted — records the live symptom's resolution.

### Milestone DoD

- **m1 (struct-shape-shift)**: `cd src && go build ./client/state/...` compiles after `collapse-effspawnframe-into-plan`, and `cd src && go build ./client/runtime/...` compiles after `rewrite-runtime-spawn-passthrough` — pending test-file migration in m2 (packages need not compile end-to-end at m1's boundary since m1 does not touch test files).
- **m2 (test-migration-and-continuity)**: `cd src && go build ./client/state/... ./client/runtime/...` and `cd src && go test -count=1 -run TestSpawnEffect_PlanFieldContinuity ./client/state/...` and `cd src && go test -count=1 -run TestEffSpawnFrameIsNotSerialized ./client/state/...` all pass.
- **m3 (matrix-and-coldstart-coverage)**: `cd src && go test -count=1 -run TestFrameLaunch_NewSession_Host ./client/runtime/...` and `cd src && go test -count=1 -run TestFrameLaunch_ColdStart_Host ./client/runtime/...` and the T1 spy-hook assertion all pass. Full local verification: `cd src && go test ./...`.

## Chunks

### m1 — struct-shape-shift

Members: `component:component-effspawnframe`, `component:component-spawn-effect-builder`, `component:component-runtime-spawn-goroutine`, `req:FR-001`, `req:FR-004`, `req:FR-005`, `req:FR-006`, `adr:adr-20260714-launchplan-effect-embedding`.

Units (task-grade):

1. **collapse-effspawnframe-into-plan** (contract-refs: `contract-effspawnframe-plan-embedding`; implementation_decisions_remaining: `spawn-effect-caller-passthrough-form`; depends_on: none)
   - Objective: Replace EffSpawnFrame's 9 duplicated launch fields (Command, StartDir, Sandbox, Options, Subsystem, Stream, Stdin, Project, Mode) with a single named value field `Plan state.LaunchPlan`; update `spawnEffect` to construct `EffSpawnFrame{..., Plan: plan, ...}` directly (dropping the `Mode: LaunchModeCreate` and `Project: plan.Project` copies plus the other 7); extend `spawnEffect`'s doc comment with the caller-terminal aliasing invariant (Argv, PreCommands, Stdin, Options.InitialInput share backing arrays across the shallow value copy).
   - Output format: Edits to `src/client/state/effect.go` and `src/client/state/reduce_helpers.go`; no test-file edits.
   - Tool guidance: Read `src/client/state/effect.go` (EffSpawnFrame struct ~lines 15-30) and `src/client/state/reduce_helpers.go` (spawnEffect func ~lines 39-59). Use Edit. Do NOT touch reduce_session.go's 3 call sites (verify by Read but not Edit).
   - Task boundaries: This unit changes only the struct shape and its sole production constructor. Test file migration is `migrate-effspawnframe-test-literals`. Runtime-side reconstruction removal is `rewrite-runtime-spawn-passthrough`.
   - Files touched: `src/client/state/effect.go`, `src/client/state/reduce_helpers.go`.
   - Acceptance: EffSpawnFrame contains exactly one launch-related field (Plan); spawnEffect has no residual `Mode:` / `Project:` / `Command:` / `StartDir:` / `Sandbox:` / `Options:` / `Subsystem:` / `Stream:` / `Stdin:` assignments in the constructor; the doc comment includes the caller-terminal invariant text.
   - Max diff LoC: 120.

2. **rewrite-runtime-spawn-passthrough** (contract-refs: `contract-runtime-spawn-goroutine-plan-passthrough`; depends_on: `collapse-effspawnframe-into-plan`)
   - Objective: In `src/client/runtime/interpret_spawn.go`, delete the `state.LaunchPlan{...}` reconstruction literal at lines 64-73 and replace with `plan := e.Plan`; migrate every `e.Project` reference at lines 75, 85, 101, 110 to `e.Plan.Project`; preserve the existing `plan = bindResult.Plan` reassignment after `subsystem.BindFrame`.
   - Output format: Edits to `src/client/runtime/interpret_spawn.go` only.
   - Tool guidance: Read `src/client/runtime/interpret_spawn.go` (lines 40-115). Use Edit. Do NOT touch the `BindRequest{Plan: plan, ...}` call — it already threads the plan whole. Do NOT touch wrapLaunchForSpawn / launcher.go. Do NOT touch bootstrap_coldstart.go.
   - Task boundaries: Does not touch test files. Does not touch bootstrap_coldstart.go — its Runtime.spawnFrameWindow is a separate implementation guarded by `add-coldstart-matrix-guard`.
   - Files touched: `src/client/runtime/interpret_spawn.go`.
   - Acceptance: no `state.LaunchPlan{` literal remains in spawnFrameWindow; no `e.Project` reference remains; `plan = bindResult.Plan` after BindFrame is unchanged and remains the sole plan-mutation point.
   - Max diff LoC: 60.

### m2 — test-migration-and-continuity (depends on m1)

Members: `component:component-effspawnframe-test-migration`, `component:component-launchplan-continuity-contract-test`, `req:FR-001`, `req:FR-006`, `req:NFR-004`, `adr:adr-20260714-launch-plan-field-continuity-invariant`.

Units:

1. **migrate-effspawnframe-test-literals** (contract-refs: [] — decision_closure_reason: mechanical rename with zero runtime effect; correctness is proven by the packages compiling and the T0 continuity test asserting the equivalent behavior; depends_on: `collapse-effspawnframe-into-plan`)
   - Objective: Mechanically rewrite every existing `EffSpawnFrame{...}` literal (or top-level field read like `spawn.Project` / `spawn.Command` / `spawn.StartDir` / `spawn.Stdin` / `spawn.Mode`) in the 6 test files to the `.Plan.*` shape; delete the tautological `reduce_session_test.go:332-333` `if spawn.Mode != LaunchModeCreate` assertion (no replacement — the field is removed as dead).
   - Files touched: `src/client/runtime/spawn_complete_test.go`, `src/client/runtime/spawn_panic_test.go`, `src/client/runtime/frame_launch_matrix_test.go`, `src/client/state/reduce_session_test.go`, `src/client/state/reduce_event_test.go`, `src/client/state/reduce_fuzz_test.go`.
   - Acceptance: `cd src && go build ./client/state/... ./client/runtime/...` succeeds; `grep -rn 'EffSpawnFrame{' src/client/` returns only literals whose launch fields appear under `Plan: state.LaunchPlan{...}`; reduce_session_test.go no longer contains the Mode assertion.
   - Max diff LoC: 300.

2. **add-t0-field-continuity-test** (contract-refs: `contract-launchplan-field-continuity-test`; depends_on: `collapse-effspawnframe-into-plan`)
   - Objective: Add `src/client/state/spawn_effect_plan_continuity_test.go` — a Tier T0 pure test that constructs a state.LaunchPlan sentinel via a reflect.Value recursive walker terminating at scalar / enum / string / bool / time.Duration / []byte / []string / [][]string leaves (recursing into LaunchOptions, StreamLaunchOptions, WorktreeOption, and any transitively reachable exported nested struct); calls spawnEffect; asserts `reflect.DeepEqual(effect.Plan, plan)`. Includes a walker self-test asserting the walker touched every reachable exported field (non-zero after walk). Code comment explicitly names Argv, PreCommands, PreCommandTimeout as currently-failing sentinels.
   - Files touched: `src/client/state/spawn_effect_plan_continuity_test.go` (new).
   - Acceptance: `cd src && go test -count=1 -run TestSpawnEffect_PlanFieldContinuity ./client/state/...` passes; the walker self-test fails if any reachable exported field is left zero after the walk; the test imports no I/O packages beyond reflect / testing / state / time.
   - Max diff LoC: 160.

3. **add-effspawnframe-serialization-guard** (contract-refs: [] — decision_closure_reason: test asserts the absence of a capability; it does not add production behavior; the pinned invariant is a categorical one and any two implementation choices yield identical outcomes; depends_on: `collapse-effspawnframe-into-plan`)
   - Objective: Add `src/client/state/effspawnframe_serialization_guard_test.go` — a package-local test that reflect-walks EffSpawnFrame's exported fields and asserts no `json` / `msgpack` / `proto` struct tag is present; asserts EffSpawnFrame does not satisfy `json.Marshaler` / `json.Unmarshaler`. Pins NFR-004.
   - Files touched: `src/client/state/effspawnframe_serialization_guard_test.go` (new).
   - Acceptance: `cd src && go test -count=1 -run TestEffSpawnFrameIsNotSerialized ./client/state/...` passes; introducing a `json:` tag on any EffSpawnFrame field or making EffSpawnFrame satisfy `json.Marshaler` fails the test.
   - Max diff LoC: 60.

### m3 — matrix-and-coldstart-coverage (depends on m2)

Members: `component:component-frame-launch-matrix-host-claude-case`, `component:component-wrap-launch-for-spawn`, `component:component-runtime-coldstart-spawn`, `req:FR-002`, `req:FR-003`, `req:FR-004`, `req:FR-006`, `req:NFR-003`, `adr:adr-20260714-coldstart-spawn-parallel-implementation`.

Units:

1. **add-matrix-host-managed-messaging-case** (contract-refs: `contract-needstoken-managed-messaging`, `contract-frame-launch-matrix-host-managed-messaging`; depends_on: `rewrite-runtime-spawn-passthrough`, `migrate-effspawnframe-test-literals`)
   - Objective: Extend `src/client/runtime/frame_launch_matrix_test.go` with (a) `TestFrameLaunch_NewSession_Host_ManagedMessaging` — constructs its EffSpawnFrame with `Plan.ManagedFrameMessaging=true` and drives it via `h.newSessionSpawn` (NOT `h.r.spawnFrameWindow`), asserting `AG_SOCKET_TOKEN` non-empty and managed-claude-home directory exists under DataDir; (b) a T1 continuity spy hook (launcher spy or `deps.wrapLaunchHook`) that captures the plan reaching `wrapLaunchForSpawn` and asserts equality with `bindResult.Plan` when driven with a sentinel plan.
   - Files touched: `src/client/runtime/frame_launch_matrix_test.go`.
   - Acceptance: `cd src && go test -count=1 -run TestFrameLaunch_NewSession_Host ./client/runtime/...` passes (existing negative case unchanged; new positive case passes); positive case would fail against the pre-fix code (verified by local revert).
   - Max diff LoC: 200.

2. **add-coldstart-matrix-guard** (contract-refs: `contract-coldstart-plan-passthrough`; depends_on: `migrate-effspawnframe-test-literals`)
   - Objective: Add `TestFrameLaunch_ColdStart_Host_ManagedMessaging` to `src/client/runtime/frame_launch_matrix_test.go` that drives `h.r.spawnFrameWindow` (bootstrap_coldstart.go path) with a fake driver whose `PrepareLaunch` returns `ManagedFrameMessaging=true`, asserting the same `AG_SOCKET_TOKEN` non-empty and managed-claude-home overlay observables — guards Runtime.spawnFrameWindow's parallel plan-resolution → wrapLaunchForSpawn implementation.
   - Files touched: `src/client/runtime/frame_launch_matrix_test.go`.
   - Acceptance: `cd src && go test -count=1 -run TestFrameLaunch_ColdStart_Host_ManagedMessaging ./client/runtime/...` passes; existing `TestFrameLaunch_ColdStart_Host` (ManagedFrameMessaging=false) continues to pass.
   - Max diff LoC: 120.

## Open Questions (deferred implementation choices)

- **oq-consolidate-two-spawn-entrypoints** (critique pass2) — Consolidation of interpret_spawn.go's `spawnFrameWindow` and bootstrap_coldstart.go's `Runtime.spawnFrameWindow` into a shared `PrepareLaunch → BindFrame → wrapLaunchForSpawn` helper is **deferred** to a follow-up ADR. This fix's guarding T1 cold-start case is a sufficient interim mitigation.
- **oq-extract-resolvespawnplan-helper** (critique pass2) — Extraction of a pure exported helper `resolveSpawnPlan(e EffSpawnFrame, bindResult BindResult) LaunchPlan` is **deferred**. The split T0 (state package) + T1 (matrix hook) coverage closes the observability gap without changing production API surface.
- **oq-defensive-slice-copy** (critique pass2) — Defensive deep-copy of `plan.Argv` / `PreCommands` / `Stdin` / `Options.InitialInput` inside `spawnEffect` is **deferred**. The spawnEffect doc-comment invariant is sufficient given the 3 callers are all in the same package.
- **oq-adr-status-argv-primary** (critique pass2) — `adr-20260711-0083-launchplan-argv-primary` remains at status=proposed. This fix does not require ADR-0083 to be accepted; the flattened-fields removal is sufficient on its own.

## Risks

- **R-1**: A future editor to `state.EffSpawnFrame` (adding a new top-level launch field alongside `Plan` for local convenience) reintroduces the two-copies-of-the-same-fact shape. Mitigation: contract-effspawnframe-plan-embedding's invariants and the T0 continuity test's structural assertion; code review of any EffSpawnFrame field addition.
- **R-2**: A future refactor inside `bootstrap_coldstart.go`'s `Runtime.spawnFrameWindow` introduces a flat-field intermediate mirroring the pre-fix `interpret_spawn.go` pattern. Mitigation: `TestFrameLaunch_ColdStart_Host_ManagedMessaging` fails immediately if ManagedFrameMessaging is dropped along the cold-start path.
- **R-3**: A future edit adds a `json:` struct tag to EffSpawnFrame for a snapshot use case. Mitigation: `TestEffSpawnFrameIsNotSerialized` fails at run time.
- **R-4**: The 3 spawnEffect callers in `reduce_session.go` gain a caller that mutates `launch.Argv` / `PreCommands` / `Stdin` / `Options.InitialInput` after the spawnEffect call. Mitigation: doc-comment invariant on spawnEffect; defensive deep-copy deferred but documented as a fallback if the invariant is ever violated by a caller outside `client/state`.

## Migration

- No snapshot / event-log / WS wire-protocol migration is required (NFR-004, pinned by `TestEffSpawnFrameIsNotSerialized`). EffSpawnFrame is a pure in-process reducer-to-runtime effect value.
- The 6 test file migrations are mechanical; landed atomically in the same PR as the EffSpawnFrame struct edit (repo single-task signature-change convention, AGENTS.md).
- No daemon restart or wire-protocol coordination is required beyond a normal `agent-grid-server` rebuild + restart to pick up the new binary.

## Resolved decisions

- **decision-input-launchplan-embed-full-replacement**: adopted — case B (full replacement) via adr-20260714-launchplan-effect-embedding.
- **decision-input-full-removal-vs-gradual-migration**: adopted — full removal in the same change.
- **decision-input-runtime-reconstruction-removal**: adopted — delete the state.LaunchPlan{...} literal in spawnFrameWindow.
- **decision-input-spawneffect-caller-passthrough**: implementation_detail — captured by `spawn-effect-caller-passthrough-form` remaining, with invariance witnesses.
- **decision-input-t0-field-continuity-test**: adopted — reflection-driven T0 + T1 matrix spy hook via adr-20260714-launch-plan-field-continuity-invariant.
- **decision-input-plan-field-holding-strategy**: adopted — named value field `Plan state.LaunchPlan`.
- **decision-input-no-parallel-transport-field-precedent**: adopted — no parallel field carrying the launch-plan meaning; Plan is the sole slot.

````
