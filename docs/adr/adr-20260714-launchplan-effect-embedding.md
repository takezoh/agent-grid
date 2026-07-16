---
id: adr-20260714-launchplan-effect-embedding
kind: adr
title: ADR — Embed state.LaunchPlan into state.EffSpawnFrame as a single named value
  field
status: accepted
created: '2026-07-14'
updated: '2026-07-14'
tags:
- adr
- launch
- effect
- refactor
- regression-pin
owners: []
relations:
- {type: partOf, target: change-20260714-launchplan-effect-continuity}
- {type: references, target: adr-20260706-frame-messaging-managed-tool-exposure}
- {type: references, target: adr-20260711-0082-frame-exec-launcher}
- {type: references, target: adr-20260711-0083-launchplan-argv-primary}
- {type: references, target: change-20260714-launchplan-effect-continuity}
source_paths:
- src/client/state/effect.go
- src/client/state/reduce_helpers.go
- src/client/runtime/interpret_spawn.go
decision_makers:
- take.gn
summary: Replace duplicated EffSpawnFrame launch fields with one named LaunchPlan
  value carried intact into runtime binding.
---

## Context

`state.LaunchPlan` is resolved once per spawn by a driver's `PrepareLaunch` (`src/client/driver/claude.go` for the live-symptom case; `state.LaunchPreparer` interface). The current effect-boundary transport copies it field-by-field twice more:

1. **Copy 1 — `state.spawnEffect` (`src/client/state/reduce_helpers.go`)** flattens the resolved `launch LaunchPlan` into 8 duplicated top-level fields on `state.EffSpawnFrame` (`Command`, `StartDir`, `Sandbox`, `Options`, `Subsystem`, `Stream`, `Stdin`, `Project`) plus a hardcoded `Mode: LaunchModeCreate`.
2. **Copy 2 — `runtime.spawnFrameWindow` (`src/client/runtime/interpret_spawn.go:64-73`)** reconstructs a fresh `state.LaunchPlan{...}` literal from those flat fields before handing it to `wrapLaunchForSpawn`.

Both copy sites enumerate `LaunchPlan`'s fields by hand. When `ManagedFrameMessaging` was added to `LaunchPlan` by the adr-20260711 family, neither copy was updated, so `needsToken := l.IsContainer(project) || plan.ManagedFrameMessaging` silently evaluated false at `runtime/launcher.go` for host+claude launches — the live symptom (`AG_SOCKET_TOKEN` empty; no `managed-claude-home` overlay). The blast radius extends beyond `ManagedFrameMessaging`: `Argv`, `PreCommands`, and `PreCommandTimeout` are also currently dropped, masked in production only because `stream/backend.go:303,306`'s `BindFrame` unconditionally rewrites them.

adr-20260712-launch-size-pass-through's Alternatives B section independently rejected a parallel `SpawnHint` field alongside `EffSpawnFrame.Options` for the identical SoT-drift reason this bug exemplifies ("同じ意味が2 slotに存在する drift 源"). subsystem.BindRequest/BindResult already thread `state.LaunchPlan` as a single typed field (`Plan LaunchPlan`), not flattened — the local precedent for threading the plan whole exists in the codebase.

Direct code reading of the three `spawnEffect` callers (`reduceCreateSession`, `pushDriverInternal`, `spawnForkSession` in `reduce_session.go`) confirms all three already hold a fully-resolved `launch LaunchPlan` value with `.Project` and `.Sandbox` set on the statement immediately preceding the `spawnEffect` call — so replacing the 8-field copy with `Plan: launch` requires no upstream change.

Go's struct value copy is shallow: `EffSpawnFrame.Plan = plan` copies slice/map headers but shares underlying arrays for `Argv`, `PreCommands`, `Stdin`, and `Options.InitialInput`. This is safe today because the 3 callers do not mutate `launch` after the spawnEffect call (grep-verified), but the invariant must be pinned to prevent a future edit from silently breaking it.

## Decision

Bind this ADR to `contract-effspawnframe-plan-embedding` and `contract-runtime-spawn-goroutine-plan-passthrough`. The design is case-B full replacement:

1. **`src/client/state/effect.go`** — replace the 9 duplicated flat launch fields (`Command`, `StartDir`, `Sandbox`, `Options`, `Subsystem`, `Stream`, `Stdin`, `Project`) plus the dead `Mode` field with a single **named value field** `Plan state.LaunchPlan`. Effect-routing fields (`SessionID`, `FrameID`, `Env`, `ReplyConn`, `ReplyReqID`) unchanged.

2. **`src/client/state/reduce_helpers.go`** — `state.spawnEffect` constructs `EffSpawnFrame{..., Plan: plan, ...}` directly. The `Mode: LaunchModeCreate` hardcode is dropped (Mode has no production reader per grep of `src/client/{state,runtime}/`; only `reduce_session_test.go:332-333`'s tautological self-assertion referenced it). The doc comment is extended to pin the caller-terminal aliasing invariant: **callers must treat `plan` (and its slice/map fields `Argv`, `PreCommands`, `Stdin`, `Options.InitialInput`) as terminal at the spawnEffect call site because the shallow value copy shares backing arrays with the caller.**

3. **`src/client/runtime/interpret_spawn.go`** — delete the `state.LaunchPlan{...}` reconstruction literal at lines 64-73. Use `plan := e.Plan` directly. The existing `plan = bindResult.Plan` reassignment after `subsystem.BindFrame` is unchanged and remains the only legitimate mutation point for the plan within this goroutine. Every `e.Project` reference at lines 75, 85, 101, 110 migrates to `e.Plan.Project` in the same change.

4. **Holding shape** — named value field, NOT:
   - a pointer field (`Plan *state.LaunchPlan`) which would let code outside the effect-dispatch boundary mutate the reducer-emitted value, conflicting with the pure-reducer / effects-as-immutable-values discipline the single-writer architecture depends on (ARCHITECTURE.md);
   - an anonymous embedding (`state.LaunchPlan` with field promotion) which would silently expand EffSpawnFrame's exported surface with every LaunchPlan field under its own promoted name and blur the distinction between launch data and effect-routing data (SessionID / FrameID / Env / ReplyConn / ReplyReqID).

5. **Test-file migration** — every existing `EffSpawnFrame{...}` literal in the 6 test files (`spawn_complete_test.go`, `spawn_panic_test.go`, `frame_launch_matrix_test.go`, `reduce_session_test.go`, `reduce_event_test.go`, `reduce_fuzz_test.go`) migrates to the `.Plan.*` shape in the same change per this repo's single-task signature-change convention (AGENTS.md). The tautological `reduce_session_test.go:332-333` `if spawn.Mode != LaunchModeCreate` assertion is deleted.

6. **Aliasing hazard** — the shallow-copy aliasing invariant is pinned by the `spawnEffect` doc-comment rather than a defensive deep-copy. Defensive deep-copy is deferred to a follow-up if a future spawnEffect caller lands outside `client/state`.

## Consequences

- The two-copies-of-the-same-fact shape that carried the ManagedFrameMessaging drop is structurally eliminated for the launch-plan surface — any future LaunchPlan field is conveyed automatically through `EffSpawnFrame.Plan` without a corresponding manual edit at either copy site.
- One atomic PR touches 6 test files plus 3 production files (`effect.go`, `reduce_helpers.go`, `interpret_spawn.go`), consistent with the repo's convention against splitting a signature change across tasks. `Project` is included in the removal set to close the SSOT gap for that field; leaving Project retained alongside `Plan.Project` would reproduce the exact two-copies-of-the-same-fact shape the fix eliminates.
- The dead `Mode` field is removed as a side effect (no production reader per grep; only a tautological test self-assertion referenced it), simplifying EffSpawnFrame per shared/design-quality.md's simplicity invariant on unused configuration points.
- The shallow-copy aliasing hazard on `Argv` / `PreCommands` / `Stdin` / `Options.InitialInput` is pinned by a `spawnEffect` doc-comment invariant. Sufficient because all 3 callers are in the same package and none currently mutate the plan post-call; defensive copy is deferred.
- Downstream consumers (`runtime.wrapLaunchForSpawn`, `agentlaunch.PrepareManagedClaudeHome`) are unchanged — their behavior is restored once `EffSpawnFrame.Plan` carries `ManagedFrameMessaging` intact.
- adr-20260711-0083-launchplan-argv-primary and adr-20260712-launch-size-pass-through (both status=proposed) remain proposed; this fix does not require them to be accepted, so their status is not coupled to this change's landing.

## Alternatives

### Case A — keep the 3 separate copies and hand-sync each of the 8+1 fields at all 3 sites whenever LaunchPlan changes

**Rejected** (却下): reproduces exactly the two-copies-of-the-same-fact shape that caused this bug; hand-sync discipline already failed once (ManagedFrameMessaging) and will fail again on the next field addition. `adr-20260712-launch-size-pass-through`'s Alternatives B section rejected the same shape for the same reason.

### Gradual migration — add `Plan` alongside the 8 flat fields, keep both populated and readable during a transition period, delete the duplicates in a later change

**Rejected**: reintroduces the exact two-copies-of-the-same-fact shape during the transition period; a reader could keep consuming the stale duplicated field instead of `Plan`, silently reopening the same regression class. The single-PR blast radius (6 test files) is manageable and preferred over structural duplication.

### Named pointer field `Plan *state.LaunchPlan`

**Rejected**: lets code outside the effect-dispatch boundary mutate the LaunchPlan the reducer already emitted, conflicting with the pure-reducer / effects-as-immutable-values discipline the single-writer architecture depends on (ARCHITECTURE.md).

### Anonymous embedding — `state.LaunchPlan` with field promotion (e.Command still works via promotion)

**Rejected**: silently expands EffSpawnFrame's exported surface with every LaunchPlan field under its own promoted name, blurs the distinction between launch data and effect-routing data (SessionID / FrameID / Env / ReplyConn / ReplyReqID), and complicates the field-continuity contract test's introspection.

### Retain a defensive `state.LaunchPlan{...}` self-copy inside runtime.spawnFrameWindow as a safety net

**Rejected**: adds no correctness value (would be byte-for-byte identical to `e.Plan` — tautological) while re-adding exactly the hand-written field-enumeration code that failed to include `ManagedFrameMessaging` in the first place.

### Defensive deep-copy of `plan.Argv` / `PreCommands` / `Stdin` / `Options.InitialInput` inside spawnEffect

**Rejected for this fix; deferred** to a follow-up if a future spawnEffect caller lands outside `client/state`. Doc-comment invariant is sufficient given the grep-verified single-writer discipline in `reduce_session.go`, all 3 callers being in the same package as spawnEffect, and the T0 continuity test asserting slice-content equality (any silent mutation between construction and assertion would be caught).

### Retain Project as a top-level EffSpawnFrame field alongside Plan.Project

**Rejected**: leaving Project retained alongside `Plan.Project` reproduces the exact SSOT-violation shape (two copies of the same fact for one field) that this fix targets for the other 7. FR-005 explicitly names Project in the sole-source-from-Plan list. This alternative was surfaced by critique issue-project-field-duplication-retained and closed by including Project in the removed set.

### Retain the dead Mode field "in case a future feature needs it"

**Rejected**: no production reader exists (grep of `src/client/{state,runtime}/`); the only reference is `reduce_session_test.go:332-333`'s tautological self-assertion. Speculative-generality per shared/design-quality.md simplicity-critic; a future feature that genuinely needs Mode can reintroduce a used field cheaply.

## References

- 2026-07-14 debug root-cause analysis (live symptom: host claude sessions launching without AG_SOCKET_TOKEN and without managed-claude-home overlay)
- `contract-effspawnframe-plan-embedding` / `contract-runtime-spawn-goroutine-plan-passthrough` (plan-20260714-launchplan-effect-continuity)
- Critique issues resolved: issue-project-field-duplication-retained, issue-test-migration-underscoped-matrix-file, issue-mode-field-role-undefined, issue-value-field-aliasing-claim-incorrect, issue-dormant-field-drops-not-declared
- Decision inputs closed: decision-input-launchplan-embed-full-replacement, decision-input-full-removal-vs-gradual-migration, decision-input-runtime-reconstruction-removal, decision-input-plan-field-holding-strategy, decision-input-no-parallel-transport-field-precedent

{% transition from="proposed" to="accepted" date="2026-07-14" %}
User approved after design skill 3-role review; accepted to unblock implementation phase (m1→m2→m3).
{% /transition %}
