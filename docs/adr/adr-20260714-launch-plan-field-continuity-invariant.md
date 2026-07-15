---
id: adr-20260714-launch-plan-field-continuity-invariant
kind: adr
title: ADR — LaunchPlan field-continuity is enforced by a Tier T0 reflection-driven
  test plus a Tier T1 runtime spy hook
status: accepted
created: '2026-07-14'
updated: '2026-07-14'
tags:
- adr
- launch
- effect
- testing
- regression-pin
owners: []
relations:
- {type: partOf, target: plan-20260714-launchplan-effect-continuity}
- {type: references, target: spec-20260714-launchplan-effect-continuity}
- {type: references, target: adr-20260714-launchplan-effect-embedding}
source_paths:
- src/client/state/spawn_effect_plan_continuity_test.go
- src/client/state/effspawnframe_serialization_guard_test.go
- src/client/runtime/frame_launch_matrix_test.go
decision_makers:
- take.gn
summary: Enforce LaunchPlan field-continuity across the spawnEffect -> EffSpawnFrame
  -> wrapLaunchForSpawn boundary with a split-tier test contract — Tier T0 in the
  state package via a reflection-driven fixture with a pinned recursion contract and
  walker self-test, plus Tier T1 in the runtime package via a spy hook on the matrix
  harness that captures the plan reaching wrapLaunchForSpawn — without exporting new
  production API surface.
---

## Context

The original defect (silent drop of `ManagedFrameMessaging` and dormant drops of `Argv` / `PreCommands` / `PreCommandTimeout`) shipped because no test asserted field-for-field continuity across the `spawnEffect → EffSpawnFrame → runtime.spawnFrameWindow → wrapLaunchForSpawn` boundary. A hand-maintained per-field assertion table would reintroduce exactly the discipline that failed for `ManagedFrameMessaging` — adding a new field to `LaunchPlan` requires remembering to add an assertion line.

`state.LaunchPlan` contains nested exported struct fields:

- `Options LaunchOptions` (`driver_iface.go:330-340`) — itself contains `Worktree WorktreeOption`, `InitialInput []byte`, `Cols`/`Rows uint16`;
- `Stream StreamLaunchOptions` (`driver_iface.go:369+`) — string-typed enums;
- plus slice fields `Argv []string`, `PreCommands [][]string`, `Stdin []byte`, `PreCommandTimeout time.Duration`, `Sandbox SandboxOverride` int enum.

A shallow reflection walker that iterates only `LaunchPlan`'s top-level fields (and hand-writes non-zero sentinels for nested structs) would reproduce the drift class it aims to eliminate — a future field added to `LaunchOptions` would not force a fixture update but also would not be covered; `FR-006`'s mechanical falsifiability collapses.

Additionally, the ideal observation point for the continuity assertion — "the plan value `spawnFrameWindow` hands to `wrapLaunchForSpawn`" — has no direct T0 test seam today: `wrapLaunchForSpawn` is unexported and the plan is a private local inside a goroutine (`interpret_spawn.go:54-101`). Extracting a pure exported helper `resolveSpawnPlan(e, bindResult) LaunchPlan` would add new production API surface for a test-only concern; the T0 constraint (pure, no I/O) is incompatible with an assertion whose observation point requires running the wrapLaunchForSpawn stack.

## Decision

Bind this ADR to `contract-launchplan-field-continuity-test` and `contract-frame-launch-matrix-host-managed-messaging`. Split the continuity assertion into two tiers:

### Tier T0 — state-package structural continuity

**File**: `src/client/state/spawn_effect_plan_continuity_test.go` (new).

**Fixture**: a `reflect.Value` recursive walker that:

- iterates every exported field of `state.LaunchPlan`;
- recurses into every reachable exported struct kind (`LaunchOptions`, `StreamLaunchOptions`, `WorktreeOption`, and any future nested struct);
- terminates only at scalar / enum / string / bool / `time.Duration` / `[]byte` / `[]string` / `[][]string` leaves.

**Per-kind sentinel table** (pinned):

| Leaf kind | Sentinel strategy |
|---|---|
| `string` | unique field-path label (e.g. `"LaunchPlan.Options.Worktree.Path"`) |
| `int` / `int64` / enum / `uint16` | prime-number ladder (each field-path gets a distinct prime) |
| `time.Duration` | distinct nanosecond value per field-path |
| `bool` | `true` |
| `[]byte` | non-nil non-empty with the field-path encoded as bytes |
| `[]string` / `[][]string` / other slices | non-nil single-element seeded with element-type sentinel |

**Assertion**: `reflect.DeepEqual(spawnEffect(sessID, frameID, plan, connID, reqID).Plan, plan)`.

**Walker self-test**: `TestSpawnEffect_PlanFieldContinuity_WalkerCoverage` re-iterates `LaunchPlan`'s `reflect.Value` after population and asserts every reachable exported field is non-zero, failing with a clear message naming any zero field-path. Proves the fixture is not silently incomplete — the walker itself is the only source of drift, and it fails loudly if it is.

**Code comment**: explicitly names `Argv`, `PreCommands`, and `PreCommandTimeout` as currently-failing sentinels (they would fail against the pre-fix code because `interpret_spawn.go`'s reconstruction literal drops them, masked in production only by `stream/backend.go`'s BindFrame overwrite). This gives the fix a mechanically-provable outcome beyond the single visible `ManagedFrameMessaging` instance.

### Tier T1 — runtime-package spy hook

**File**: `src/client/runtime/frame_launch_matrix_test.go` (extend).

**Hook**: a spy — either a launcher spy wrapping the fake dispatcher or a new `deps.wrapLaunchHook` field on the harness — that captures the plan handed to `wrapLaunchForSpawn` when the harness runs `h.newSessionSpawn` with a sentinel plan and a `BindFrame` fake that rewrites `StartDir` / `Command`.

**Assertion**: the captured plan equals `bindResult.Plan`, NOT `e.Plan`, whenever the two differ — proving the reassignment `plan = bindResult.Plan` after `subsystem.BindFrame` is honored (FR-004).

**No new production API surface**: the spy is scoped to the harness; production `wrapLaunchForSpawn` remains unexported.

### Deferred

Extraction of a pure exported helper `resolveSpawnPlan(e EffSpawnFrame, bindResult BindResult) LaunchPlan` is **deferred** to a follow-up if a third caller of the resolution appears (per critique pass2 open question `oq-extract-resolvespawnplan-helper`). The split T0 + T1 coverage closes the observability gap without touching production ABI.

## Consequences

- Any exported field newly added to `state.LaunchPlan` or a transitively reachable exported nested struct (`LaunchOptions` / `StreamLaunchOptions` / `WorktreeOption` / a future addition like a hypothetical `MCPConfig`) is automatically covered by the T0 continuity assertion — no fixture-code edit required.
- `FR-006` (a new `LaunchPlan` field shall not require any hand-edit for that field to reach `WrapLaunch`) is **mechanically falsifiable** — a future violation surfaces as a T0 or T1 test failure without any test-code change.
- The T1 hook does not export new production API surface (spy is scoped to the harness), so production ABI is unchanged.
- The recursion contract is pinned in this ADR body and the T0 test's helper (~30 LoC); future maintainers cannot silently downgrade to a shallow walker without failing the walker self-test.
- The split-tier design honors the design-quality testability principle (logic vs I/O separation): T0 in the state package asserts the pure structural claim; T1 in the runtime package asserts the runtime-side reassignment behavior via an existing fake seam.

## Alternatives

### Explicit per-field assertion table (`if plan.Command != want.Command { ... }` repeated for every field)

**Rejected** (却下): reintroduces a hand-maintained field list — a new `LaunchPlan` field added later requires remembering to add an assertion line, the exact discipline that failed for `ManagedFrameMessaging`.

### reflect.DeepEqual over one sentinel value but with a hand-written per-kind fixture for nested structs (no reflection-driven recursion)

**Rejected**: the hand-written fixture becomes the new drift-prone list — a future field added to `LaunchOptions` would not force a fixture update but would also not be covered; `FR-006`'s mechanical falsifiability collapses.

### testing/quick fuzz generating randomized LaunchPlan values across many runs

**Rejected**: broader input coverage but heavier to write for a pure struct-shape-preservation property; does not obviously beat a reflection-driven single-value assertion for this specific defect class (structural field loss, not value-dependent logic).

### Extract a pure exported helper resolveSpawnPlan from interpret_spawn.go to enable a strictly-T0 assertion of the pre-wrapLaunchForSpawn plan value

**Rejected for this fix; deferred**: adds new production API surface for a test-only concern; the split T0 (state) + T1 (matrix hook) coverage closes the observability gap without touching production ABI. Reconsider when a third caller of the resolution appears.

### Rely only on the existing end-to-end assertions without a dedicated T0 structural test

**Rejected**: leaves the silent-field-drop defect class uncovered until the next LaunchPlan field is added and someone happens to also extend the end-to-end matrix for it — the exact discipline that failed for `ManagedFrameMessaging`. Also collapses `FR-006`'s mechanical falsifiability.

### Sole T1 runtime assertion (no T0 in the state package)

**Rejected**: `NFR-001a` requires a T0-tier pure test for field continuity; a T1-only approach makes every future run of the invariant require the launcher stack fixtures and cannot be exercised as a fast pre-commit check.

## References

- 2026-07-14 debug root-cause analysis
- `contract-launchplan-field-continuity-test` / `contract-frame-launch-matrix-host-managed-messaging` (plan-20260714-launchplan-effect-continuity)
- Critique issues resolved: issue-t0-test-seam-unavailable, issue-reflection-fixture-nested-types-unaddressed, issue-dormant-field-drops-not-declared
- Decision input closed: decision-input-t0-field-continuity-test
- Critique pass2 deferred open questions: oq-extract-resolvespawnplan-helper

{% transition from="proposed" to="accepted" date="2026-07-14" %}
User approved after design skill 3-role review; accepted to unblock implementation phase (m1→m2→m3).
{% /transition %}
