---
id: adr-20260714-coldstart-spawn-parallel-implementation
kind: adr
title: ADR — The cold-start Runtime.spawnFrameWindow parallel implementation is guarded,
  not consolidated, in this fix
status: accepted
created: '2026-07-14'
updated: '2026-07-14'
tags:
- adr
- launch
- effect
- boundary-duplication
- regression-pin
owners: []
relations:
- {type: partOf, target: change-20260714-launchplan-effect-continuity}
- {type: references, target: adr-20260714-launchplan-effect-embedding}
- {type: references, target: change-20260714-launchplan-effect-continuity}
source_paths:
- src/client/runtime/bootstrap_coldstart.go
- src/client/runtime/interpret_spawn.go
- src/client/runtime/frame_launch_matrix_test.go
decision_makers:
- take.gn
summary: Keep the parallel cold-start launch path, guarded by a matching Tier T1 continuity
  test, until a separately designed consolidation.
---

## Context

Two independent implementations of plan-resolution → `wrapLaunchForSpawn` exist in the runtime:

1. **Event-loop path — `interpret_spawn.go`** — the free function `spawnFrameWindow(deps, e state.EffSpawnFrame)`. This is the path this fix (`adr-20260714-launchplan-effect-embedding`) modifies: it consumes `EffSpawnFrame.Plan` via `plan := e.Plan`, threads it through `subsystem.BindFrame`, and hands the (possibly rewritten) plan to `wrapLaunchForSpawn`.

2. **Cold-start path — `bootstrap_coldstart.go:113`** — the method `(r *Runtime) spawnFrameWindow(id state.SessionID, sandbox state.SandboxOverride, frame state.SessionFrame) error`. This resolves `LaunchPlan` via `drv.PrepareLaunch(frame.Driver, state.LaunchModeColdStart, frame.Project, frame.Command, frame.LaunchOptions, sandboxed)` at line 121, sets `launch.Sandbox` / `launch.Project` at lines 126-127, threads `launch` through `sub.BindFrame` at line 135, and hands it directly to `wrapLaunchForSpawn(launcher(r.cfg), frame.ID, frame.Project, launch, baseEnv)` at line 154 — **never constructing an `EffSpawnFrame` at all**.

The cold-start path is **not defective today** because it threads the whole `launch LaunchPlan` value between `PrepareLaunch` and `wrapLaunchForSpawn` without a flat-field intermediate. But it is **structurally a parallel implementation of the same responsibility** — the SSOT violation this fix targets at the `LaunchPlan`-field granularity exists at the pipeline granularity for cold-start. `FR-006` as originally drafted named only `interpret_spawn.go`; a future refactor could regress cold-start independently with no existing test signal.

Consolidating into a shared `PrepareLaunch → BindFrame → wrapLaunchForSpawn` helper would expand blast radius beyond the `LaunchPlan`-continuity concern (touches cold-start recovery, which has its own subsystem-init / snapshot-restore concerns) and is a broader refactor than this fix warrants. It also does not carry its own justification — the two call sites do have slightly different shapes (cold-start resolves `launch` from the persisted `SessionFrame`; event-loop consumes an already-resolved `EffSpawnFrame.Plan`), so the shape of the shared helper needs its own design.

## Decision

Bind this ADR to `contract-coldstart-plan-passthrough`. **Do not consolidate** the two `spawnFrameWindow` implementations in this fix. Instead:

1. **Rephrase FR-006** to name both entry points explicitly:

   > "The system shall not require any hand-edit to state.EffSpawnFrame, state.spawnEffect, runtime.spawnFrameWindow (interpret_spawn.go, event-loop path), or Runtime.spawnFrameWindow (bootstrap_coldstart.go, cold-start path) for a newly added state.LaunchPlan field ... to reach AgentLauncher.WrapLaunch."

2. **Add a paired Tier T1 case** to `src/client/runtime/frame_launch_matrix_test.go`: `TestFrameLaunch_ColdStart_Host_ManagedMessaging` drives `h.r.spawnFrameWindow` (the `Runtime` method, cold-start path) with a fake driver whose `PrepareLaunch` returns `ManagedFrameMessaging=true`, asserting the same `AG_SOCKET_TOKEN` non-empty and `managed-claude-home` overlay observables as the new-session case (`TestFrameLaunch_NewSession_Host_ManagedMessaging`). Closes the parallel-drift gap for the current `LaunchPlan` surface with matching observability.

3. **Defer consolidation** — consolidation into a shared helper is deferred to a follow-up ADR (critique pass2 open question `oq-consolidate-two-spawn-entrypoints`; critic recommendation: defer). The T1 cold-start case is a sufficient interim guard.

## Consequences

- **FR-006 becomes machine-checked for both entry points**: a new `LaunchPlan` field added later must reach `wrapLaunchForSpawn` from either the event-loop path or the cold-start path with no hand-edit — a violation surfaces as a T1 matrix test failure without any test-code change.
- **Two independent implementations remain in the codebase**: this is an accepted risk mitigated by the matching T1 test and by this ADR making the parallelism explicit in the design record. A future edit that regresses one path without the other is caught immediately by the T1 case whose observations differ from the other path's observations.
- **Consolidation, if pursued later**, has a documented starting point (this ADR) and does not need to reprove the `LaunchPlan`-continuity invariant — that invariant is already pinned by the T1 tests on both paths.
- The cold-start test uses the existing `sandbox.Manager` / `recSubsystem` / `backend` fixtures unchanged (NFR-002).
- The cold-start `Runtime.spawnFrameWindow` production code is **not modified** by this fix — matches the responsibility boundary the fix's other units maintain (event-loop path only).

## Alternatives

### Consolidate the two implementations into a shared PrepareLaunch -> BindFrame -> wrapLaunchForSpawn helper in this fix

**Rejected** (却下): expands blast radius beyond the `LaunchPlan`-continuity concern (touches cold-start recovery + snapshot-restore + subsystem-init sequencing); the parallel-drift risk is adequately mitigated for the current `LaunchPlan` surface by the T1 matrix guard, at a fraction of the cost. Consolidation deserves its own ADR when the shape of the shared helper can be justified against the two current callers plus any future third caller.

### Leave cold-start unguarded and rely on FR-006 being interpreted broadly across future refactors

**Rejected**: reproduces exactly the discipline that failed for `ManagedFrameMessaging` — an invariant that no test enforces will silently drift. The T1 cold-start case is a small addition (~120 LoC) that makes the invariant machine-checked for both entry points.

### Extract a shared helper only for the plan-carrying wire (not the full pipeline) and use it from both spawnFrameWindow implementations in this fix

**Rejected**: the plan-carrying wire in the event-loop path is already trivial (`plan := e.Plan`); no meaningful abstraction is possible without also touching `BindFrame` invocation and post-bind reassignment, which is the broader consolidation this ADR defers.

### Mark bootstrap_coldstart.go's Runtime.spawnFrameWindow as deprecated and reroute cold-start through interpret_spawn.go's free spawnFrameWindow

**Rejected**: the two paths have genuinely different inputs (cold-start starts from a persisted `SessionFrame` before any reducer runs; event-loop starts from an in-memory `EffSpawnFrame` emitted by the reducer). Rerouting would require synthesizing an EffSpawnFrame outside the reducer, which is exactly the SSOT violation adr-20260714-launchplan-effect-embedding fixes.

## References

- 2026-07-14 debug root-cause analysis
- `contract-coldstart-plan-passthrough` (plan-20260714-launchplan-effect-continuity)
- Critique issues resolved: issue-cold-start-parallel-spawn-not-analyzed, issue-matrix-test-wrong-path-pairing
- Critique pass2 deferred open questions: oq-consolidate-two-spawn-entrypoints
- Related ADR: adr-20260714-launchplan-effect-embedding

{% transition from="proposed" to="accepted" date="2026-07-14" %}
User approved after design skill 3-role review; accepted to unblock implementation phase (m1→m2→m3).
{% /transition %}
