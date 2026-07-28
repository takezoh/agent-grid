---
change: change-20260727-multi-server-session-palette
role: verification
success_conditions:
- All implementation contract normal/adversarial witnesses pass.
- All six critique issues are traced exactly once and no blocker/design choice remains.
- Web and Workspace normalized traces are deep-equal.
- Legacy currentServerId/raw URL/raw sessionId/server-specific window signatures are zero.
contract_verifications:
- id: verify-profile-data-model
  tier: T1
  requirement_refs:
  - FR-002
  - FR-003
  - FR-005
  criterion: Observe `profile IDs, parameter IDs, presence state, ordered descriptors and immutable revisions exclude
    duplicate or ambiguous states` and prove malformed/stale inputs create nothing.
  command: cd src && go test ./host/config/...
- id: verify-profile-model
  tier: T1
  requirement_refs:
  - FR-002
  - FR-003
  - FR-007
  - FR-010
  criterion: Observe `typed profile config produces one ordered immutable profile snapshot; semantic changes always
    change revision` and prove malformed/stale inputs create nothing.
  command: cd src && go test ./host/config/...
- id: verify-project-profile-override
  tier: T1
  requirement_refs:
  - FR-002
  - FR-005
  criterion: Observe `project profiles inherit unspecified IDs, fully replace matching IDs, remove explicit disabled
    IDs, and reject duplicates` and prove malformed/stale inputs create nothing.
  command: cd src && go test ./host/config/...
- id: verify-descriptor-rest
  tier: T1
  requirement_refs:
  - FR-001
  - FR-002
  - FR-003
  criterion: Observe `one GET /api/session-config returns a server-local immutable versioned project/profile snapshot`
    and prove malformed/stale inputs create nothing.
  command: cd src && go test ./server/api/...
- id: verify-server-side-materialization
  tier: T1
  requirement_refs:
  - FR-004
  - FR-005
  - FR-006
  criterion: Observe `the host/state service alone validates the exact revision and emits shell-free executable/argv
    tokens` and prove malformed/stale inputs create nothing.
  command: cd src && go test ./host/state/... ./server/api/...
- id: verify-create-wire-evolution
  tier: T1
  requirement_refs:
  - FR-006
  - FR-010
  criterion: Observe `only version=2 profile payloads are accepted; absent, mixed, or unknown shapes create no session`
    and prove malformed/stale inputs create nothing.
  command: cd src && go test ./server/api/...
- id: verify-typed-parameter-form
  tier: T1
  requirement_refs:
  - FR-003
  - FR-004
  - FR-007
  criterion: Observe `one ordered accessible form preserves absent/default/explicit state for arbitrary N enum/boolean
    parameters` and prove malformed/stale inputs create nothing.
  command: cd clients/ui && npm run test:unit
- id: verify-schema-drift-reconciliation
  tier: T1
  requirement_refs:
  - FR-005
  - FR-009
  criterion: Observe `stale create never runs and refreshed forms retain only same-ID compatible-kind values plus
    valid focus` and prove malformed/stale inputs create nothing.
  command: cd clients/ui && npm run test:unit
- id: verify-confirm-create-control-flow
  tier: T1
  requirement_refs:
  - FR-004
  - FR-005
  - FR-006
  criterion: Observe `only explicit confirmation advances one validated immutable snapshot through materialization
    to exactly one target create` and prove malformed/stale inputs create nothing.
  command: cd src && go test ./host/state/... ./server/api/...
- id: verify-server-qualified-identity
  tier: T1
  requirement_refs:
  - FR-001
  - FR-008
  - FR-009
  criterion: Observe `compound server/local identity and visible server label survive discovery, selection, create
    and active context` and prove malformed/stale inputs create nothing.
  command: cd clients/ui && npm run test:unit
- id: verify-fleet-snapshot-protocol
  tier: T2
  requirement_refs:
  - FR-FLEET-001
  - FR-FLEET-002
  - FR-FLEET-008
  - FR-FLEET-010
  criterion: Observe Each accepted envelope is a credential-free full replacement with monotonically increasing
    fleet revision, per-member generation/freshness, and compound sessions; all host adapters project identical
    values. and every forbidden legacy/fabricated outcome remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-application-fleet-authority
  tier: T1
  requirement_refs:
  - FR-FLEET-001
  - FR-FLEET-002
  - FR-FLEET-008
  criterion: Observe Within one host process there is exactly one configured fleet state machine; all live views
    see the same ordered membership revisions while retaining independent activeSessionRef. and every forbidden
    legacy/fabricated outcome remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-compound-session-routing
  tier: T1
  requirement_refs:
  - FR-FLEET-004
  - FR-FLEET-005
  - FR-FLEET-012
  criterion: Observe Every session list key, active selection, create result, operation, and event correlation preserves
    the exact same compound identity; no current-server fallback exists. and every forbidden legacy/fabricated outcome
    remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-create-observation-convergence
  tier: T1
  requirement_refs:
  - FR-FLEET-003
  - FR-FLEET-009
  criterion: Observe Exactly one create side effect occurs; confirmed means the compound session is already in the
    shared fleet snapshot, while created_unconfirmed communicates side-effect success without inventing membership.
    and every forbidden legacy/fabricated outcome remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-partial-failure-stale-projection
  tier: T1
  requirement_refs:
  - FR-FLEET-006
  - FR-FLEET-008
  criterion: Observe Member state distinguishes current, reconnecting/stale, unavailable/stale, incompatible/no-current-snapshot,
    and never-observed; healthy scopes remain available. and every forbidden legacy/fabricated outcome remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-credential-free-host-boundary
  tier: T2
  requirement_refs:
  - FR-FLEET-005
  - FR-FLEET-007
  - FR-FLEET-010
  criterion: Observe All renderer-visible values are credential-free and server-neutral; authorization is attached
    only by the target host boundary after enabled serverId validation. and every forbidden legacy/fabricated outcome
    remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-view-local-selection
  tier: T1
  requirement_refs:
  - FR-FLEET-002
  - FR-FLEET-004
  - FR-FLEET-012
  criterion: Clean resets then commits immediately in the same window; dirty selection leaves old active unchanged
    and only sets pending; Cancel clears pending; Confirm revalidates and performs discard-before-commit; stale/unavailable
    Confirm preserves old active and commits nothing. Any immediate dirty switch fails.
  command: cd clients/ui && npm run test:unit
- id: verify-fleet-resource-lifecycle
  tier: T1
  requirement_refs:
  - FR-FLEET-002
  - FR-FLEET-006
  - FR-FLEET-008
  - FR-FLEET-010
  criterion: Observe Resource counts converge to configured servers plus live subscribers, REST and timeout bounds
    hold, and teardown leaves zero owned timers, pending operations, and channels. and every forbidden legacy/fabricated
    outcome remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-host-parity
  tier: T2
  requirement_refs:
  - FR-FLEET-001
  - FR-FLEET-006
  - FR-FLEET-007
  - FR-FLEET-010
  criterion: Observe A shared semantic trace is host-independent after removing transport metadata. and every forbidden
    legacy/fabricated outcome remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-fleet-migration
  tier: T2
  requirement_refs:
  - FR-FLEET-004
  - FR-FLEET-005
  - FR-FLEET-007
  - FR-FLEET-010
  - FR-FLEET-012
  criterion: Observe Only the host-owned fleet path exists; single-server configurations still appear as a one-member
    fleet without a separate adapter or implicit default server. and every forbidden legacy/fabricated outcome remains
    absent.
  command: cd clients/ui && npm run test:unit
- id: verify-fleet-publication-backpressure
  tier: T1
  requirement_refs:
  - FR-FLEET-001
  - FR-FLEET-002
  - NFR-FLEET-003
  criterion: Observe Fast subscribers continue while each slow subscriber retains at most one in-flight and one
    latest pending snapshot reference; an unwriteable transport is severed as slow_subscriber and resubscribe replays
    current state. and every forbidden legacy/fabricated outcome remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-fleet-event-ordering
  tier: T1
  requirement_refs:
  - FR-FLEET-002
  - FR-FLEET-008
  - NFR-FLEET-004
  criterion: Observe Concurrent member, reconnect, create-observation and subscriber events are serialized by one
    host writer; stale generations have no effect and accepted revisions are monotonic. and every forbidden legacy/fabricated
    outcome remains absent.
  command: cd clients/ui && npm run test:unit
- id: verify-scoped-discovery-recovery
  tier: T1
  requirement_refs:
  - FR-005
  - FR-009
  criterion: Each failed scope shows reason and retry; healthy scopes remain; retry returns to the same flow; valid
    selection/value/focus are retained; invalid targets are explained and require reselection; silent reset, global
    stop, stale execution and hidden failure all fail.
  command: cd clients/ui && npm run test:unit
---

# Verification

## Tier gates

- T0: pure reducer/property tests cover compound collisions, member generation, replacement, fixed 10s create deadline, settle-once and bounded latest-wins mailboxes.
- T1: `cd clients/ui && npm run test:unit`, `cd clients/workspace && npm test`, and Web host Go tests cover projection, routed APIs, same-window dirty switching, teardown and one-member regression.
- T2: shared two-server fixtures drive Web HTTP/WS and Workspace preload/IPC and assert deep-equal credential-free snapshots and typed failures; compile/lint scans reject every legacy consumer signature.
- T3: each upstream lifecycle dependency ships the repository-required fake, `FakeVsReal*` backstop and invariant-naming contract test; real-stack Playwright proves New Session on B becomes visible from a view initially active on A.

## Required witnesses

The executable matrix includes dirty selection immediate-switch rejection, Confirm-current/Confirm-invalid ordering, scoped catalog/profile/parameter failure with retained selection/value/focus and retry, A/A control, A/B create, two live views, local-ID and display-label collision, first-load and last-known partial failure, stale generation, dirty Cancel/Confirm, unsafe redirect, shutdown/reconnect and slow subscriber replay. `created_unconfirmed` is observed exactly at the fixed deadline without duplicate POST.

## Executed evidence

- `cd clients/ui && npm run lint && npm run typecheck && npm run test:unit` — 121 files / 1770 tests passed.
- `cd clients/workspace && npm run typecheck && npm run test:unit` — 13 files / 84 tests passed.
- `cd clients/web-host && go test -race ./internal/webhost` — passed, including two-view fleet publication and create-after-observation.
- `make lint` — passed with zero Go lint issues.
- `cd clients/workspace && npm run build` — passed, including the production SPA build and Electron TypeScript emit.
- `cd clients/ui && ./scripts/e2e-smoke.sh e2e/app.smoke.spec.ts` and
  `... e2e/session-context-menu.spec.ts` — 4 browser tests passed against the
  fleet-aware deterministic fake.
