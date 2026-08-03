---
change: change-20260727-resilient-server-reconnect-health
role: implementation
contracts:
- contract-active-recovery-lifecycle
- contract-daemon-hello-proof
- contract-active-resource-fence
- contract-statusbar-recovery
- contract-fleet-health-authority
- contract-fleet-health-outcomes
- contract-fleet-resource-lifecycle
- contract-host-health-wire
- contract-independent-status-axes
- contract-connections-diagnosis
- contract-health-configuration-boundary
contract_projections:
- id: contract-active-recovery-lifecycle
  verifications:
  - verify-active-retry-exact
  - verify-initial-recovery-stop
  - verify-manual-retry-repeat
  - verify-stale-generation-noop
  discretion:
  - discretion-active-generation-token
- id: contract-daemon-hello-proof
  verifications:
  - verify-hello-only-open
  - verify-open-without-hello
  discretion: []
- id: contract-active-resource-fence
  verifications:
  - verify-active-resource-closure
  - verify-single-flight-retry
  discretion:
  - discretion-cancelable-scheduler
- id: contract-statusbar-recovery
  verifications:
  - verify-statusbar-retry-access
  - verify-status-announcement-focus
  - verify-single-recovery-owner
  discretion:
  - discretion-status-focus-implementation
- id: contract-fleet-health-authority
  verifications:
  - verify-single-host-health-writer
  - verify-cross-host-health-conformance
  - verify-generation-fence
  discretion:
  - discretion-health-storage
- id: contract-fleet-health-outcomes
  verifications:
  - verify-health-outcome-partition
  - verify-stale-never-current
  - verify-total-outage
  - verify-fleet-recovery
  discretion: []
- id: contract-fleet-resource-lifecycle
  verifications:
  - verify-health-singleflight
  - verify-health-three-second-bound
  - verify-health-resource-closure
  discretion: []
- id: contract-host-health-wire
  verifications:
  - verify-health-wire-parity
  - verify-session-wire-compat
  - verify-health-delivery-loss
  discretion: []
- id: contract-independent-status-axes
  verifications:
  - verify-independent-status-axes
  - verify-enabled-denominator
  - verify-fleet-summary-exits
  discretion: []
- id: contract-connections-diagnosis
  verifications:
  - verify-connections-parity
  - verify-browser-connections-reachability
  - verify-workspace-connections-editable
  - verify-connections-no-retry
  discretion:
  - discretion-connections-layout
- id: contract-health-configuration-boundary
  verifications:
  - verify-health-secret-redaction
  - verify-health-not-persisted
  discretion: []
adrs:
- adr-20260624-0011-two-step-ws-close-on-daemon-disconnect
- adr-20260624-0012-daemon-client-eager-dial-supervisor
- adr-20260624-0022-subscribe-retry-in-socket-layer
- adr-20260727-host-neutral-fleet-provider
- adr-20260727-application-scoped-fleet-session-authority
- adr-20260727-server-qualified-identity-partial-failure
- adr-20260727-single-user-server-auth
- adr-20260727-exact-match-client-server-protocol
- adr-20260803-resilient-fleet-health-single-authority
decision_dispositions:
- decision_input_ref: decision-input-recovery-policy
  disposition: Adopt exact one initiating attempt plus sixteen automatic retries,
    then reusable manual Retry.
  contract_refs:
  - contract-active-recovery-lifecycle
- decision_input_ref: decision-input-recovery-proof
  disposition: Adopt current-generation daemon hello as the only recovery proof.
  contract_refs:
  - contract-daemon-hello-proof
- decision_input_ref: decision-input-status-owner
  disposition: Adopt persistent StatusBar as sole active-recovery and Retry owner.
  contract_refs:
  - contract-statusbar-recovery
- decision_input_ref: decision-input-fleet-denominator
  disposition: Aggregate enabled configured entries only and retain disabled entries
    only in diagnosis.
  contract_refs:
  - contract-fleet-health-outcomes
  - contract-independent-status-axes
- decision_input_ref: decision-input-health-boundary
  disposition: Keep health runtime-only and separate from immutable configuration
    persistence.
  contract_refs:
  - contract-health-configuration-boundary
- decision_input_ref: decision-input-diagnosis-entry
  disposition: Provide a consistently named Connections action and server-qualified
    diagnosis in both hosts.
  contract_refs:
  - contract-connections-diagnosis
- decision_input_ref: decision-input-retry-reachability
  disposition: Close through persistent no-session StatusBar, native Retry, generation
    single-flight, and focus verification.
  contract_refs:
  - contract-statusbar-recovery
  - contract-active-recovery-lifecycle
- decision_input_ref: decision-input-status-announcement
  disposition: Use dedicated polite status text with Retry as a sibling action and
    focus the fixed persistent StatusBar landmark only when focused Retry disappears.
  contract_refs:
  - contract-statusbar-recovery
- decision_input_ref: decision-input-runtime-health-boundary
  disposition: Extend the existing one host FleetRuntime as sole host health writer
    and propose a complementary projection/delivery-overlay ADR that cannot create
    a second owner.
  adr_refs:
  - adr-20260803-resilient-fleet-health-single-authority
  contract_refs:
  - contract-fleet-health-authority
  - contract-health-configuration-boundary
- decision_input_ref: decision-input-shared-connections
  disposition: Share one pure row/label/reason projection and permit only host navigation/rendering
    differences.
  adr_refs:
  - adr-20260803-resilient-fleet-health-single-authority
  contract_refs:
  - contract-host-health-wire
  - contract-connections-diagnosis
- decision_input_ref: decision-input-adr-0011
  disposition: Preserve two-step daemon-disconnected control plus typed close and
    create a new client socket.
  adr_refs:
  - adr-20260624-0011-two-step-ws-close-on-daemon-disconnect
  contract_refs:
  - contract-active-recovery-lifecycle
  - contract-daemon-hello-proof
- decision_input_ref: decision-input-adr-0012
  disposition: Preserve eager unbounded gateway daemon supervision outside the finite
    shared-UI lifecycle.
  adr_refs:
  - adr-20260624-0012-daemon-client-eager-dial-supervisor
  contract_refs:
  - contract-active-recovery-lifecycle
- decision_input_ref: decision-input-adr-0022
  disposition: Preserve only frame-not-ready subscribe retry and reusable backoff
    mechanics; this change owns active lifecycle count, proof, and fencing.
  adr_refs:
  - adr-20260624-0022-subscribe-retry-in-socket-layer
  contract_refs:
  - contract-active-recovery-lifecycle
- decision_input_ref: decision-input-adr-host-neutral
  disposition: Add equal credential-free health meaning to both FleetProvider host
    adapters.
  adr_refs:
  - adr-20260727-host-neutral-fleet-provider
  - adr-20260803-resilient-fleet-health-single-authority
  contract_refs:
  - contract-fleet-health-authority
  - contract-host-health-wire
  - contract-connections-diagnosis
- decision_input_ref: decision-input-adr-app-fleet
  disposition: Reuse the existing one-per-host FleetRuntime rather than adding view
    or Settings health authorities.
  adr_refs:
  - adr-20260727-application-scoped-fleet-session-authority
  - adr-20260803-resilient-fleet-health-single-authority
  contract_refs:
  - contract-fleet-health-authority
- decision_input_ref: decision-input-adr-partial-failure
  disposition: Preserve compound server identity, latest-generation wins, stale retention,
    and healthy-member usability.
  adr_refs:
  - adr-20260727-server-qualified-identity-partial-failure
  contract_refs:
  - contract-fleet-health-authority
  - contract-fleet-health-outcomes
- decision_input_ref: decision-input-adr-auth
  disposition: Keep credentials host-owned and exclude bearer values and raw secret-bearing
    errors from health projection.
  adr_refs:
  - adr-20260727-single-user-server-auth
  contract_refs:
  - contract-health-configuration-boundary
- decision_input_ref: decision-input-adr-exact-protocol
  disposition: Preserve exact-match admission; mismatch is Incompatible protocol and
    terminal for member operational/reconnect work until config or process generation
    changes.
  adr_refs:
  - adr-20260727-exact-match-client-server-protocol
  contract_refs:
  - contract-fleet-health-authority
  - contract-fleet-health-outcomes
- decision_input_ref: decision-input-testing-policy
  disposition: Require feature tests plus fake, invariant contract, and FakeVsReal
    backstop for modified external gateway seams.
  contract_refs:
  - contract-daemon-hello-proof
  - contract-fleet-health-authority
  - contract-host-health-wire
- decision_input_ref: issue-exact-match-protocol-adr-omitted
  disposition: Resolved by tracing the exact-match ADR and making protocol mismatch
    terminal Incompatible protocol with no retry in the member generation.
  adr_refs:
  - adr-20260727-exact-match-client-server-protocol
  contract_refs:
  - contract-fleet-health-authority
  - contract-fleet-health-outcomes
- decision_input_ref: issue-fleet-authority-double-owner
  disposition: Resolved by retaining FleetRuntime as the sole writer; the complementary
    ADR fixes projection and delivery-overlay semantics without creating an authority.
  adr_refs:
  - adr-20260727-application-scoped-fleet-session-authority
  - adr-20260803-resilient-fleet-health-single-authority
  contract_refs:
  - contract-fleet-health-authority
  - contract-host-health-wire
- decision_input_ref: issue-delivery-failure-owner-gap
  disposition: Resolved with a provider-owned liveness overlay that preserves and
    never advances the last host revision.
  contract_refs:
  - contract-host-health-wire
- decision_input_ref: issue-health-refresh-lifecycle-undefined
  disposition: Resolved with five-second Connected freshness, bounded refresh, stale
    prior retention on failure, and later current recovery.
  contract_refs:
  - contract-fleet-health-outcomes
  - contract-fleet-resource-lifecycle
- decision_input_ref: issue-active-attempt-generation-race
  disposition: Resolved with separate lifecycle generation and per-attempt sequence
    checked by every asynchronous continuation.
  contract_refs:
  - contract-active-recovery-lifecycle
  - contract-active-resource-fence
- decision_input_ref: issue-health-wire-schema-owner-missing
  disposition: Resolved by making session-protocol.ts the schema/decoder owner and
    canonical JSON fixtures the cross-codec conformance input.
  contract_refs:
  - contract-host-health-wire
- decision_input_ref: issue-health-state-invalid-combinations
  disposition: Resolved with an exclusive qualified observation union and codec rejection
    of invalid combinations.
  contract_refs:
  - contract-fleet-health-outcomes
  - contract-host-health-wire
- decision_input_ref: issue-status-live-region-interaction-conflict
  disposition: Resolved with sibling live text and Retry plus one fixed persistent
    StatusBar focus landmark.
  contract_refs:
  - contract-statusbar-recovery
- decision_input_ref: issue-test-closure-nonexecuting
  disposition: Resolved with owning-package T0/T1 commands and the executable FakeVsRealFleetHealth
    T3 gate.
  contract_refs:
  - contract-fleet-health-authority
  - contract-host-health-wire
  - contract-connections-diagnosis
- decision_input_ref: issue-connections-semantics-split-owner
  disposition: Resolved with one pure connections-presentation projection consumed
    by both host renderers.
  contract_refs:
  - contract-connections-diagnosis
- decision_input_ref: issue-adr-0022-overclaimed
  disposition: Resolved by narrowing ADR 0022 to frame-not-ready subscribe retry and
    reusable backoff mechanics.
  adr_refs:
  - adr-20260624-0022-subscribe-retry-in-socket-layer
  contract_refs:
  - contract-active-recovery-lifecycle
- decision_input_ref: issue-security-redaction-boundary-unverifiable
  disposition: Resolved with producer-side closed classifiers before snapshot/log
    boundaries and sentinel tests for wire, DOM, config, and logs.
  contract_refs:
  - contract-health-configuration-boundary
- decision_input_ref: issue-resource-progress-bounds-incomplete
  disposition: Resolved by bounding capabilities, ticket, and hello phases to three
    seconds and making shutdown handle convergence deterministic.
  contract_refs:
  - contract-fleet-resource-lifecycle
- decision_input_ref: issue-browser-editability-outcome-weakened
  disposition: Resolved with keyboard-operable browser configuration-path copy and
    restart instruction that remain available during health failure.
  contract_refs:
  - contract-connections-diagnosis
milestones:
- id: '1'
- id: '2'
- id: '3'
- id: '4'
- id: '5'
reference_algorithms: []
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Contract map

- `contract-active-recovery-lifecycle` / `contract-daemon-hello-proof` / `contract-active-resource-fence`: `clients/ui/src/socket/connection.ts`がmanual lifecycle generationとattempt sequenceを別々に採番し、timer、ticket、socket、close、helloの全continuationで両方を照合する。initiating attemptの後に自動16回、helloだけが会計をreset/openにし、Retryはfresh lifecycleを1つだけ開始する。
- `contract-statusbar-recovery`: `StatusBar`をsession未選択時も常設し、dedicated polite status textとsibling Retryを分離する。focused Retry消滅時のfocus先はaccessible labelと`tabIndex=-1`を持つpersistent StatusBar landmarkに固定する。
- `contract-fleet-health-authority` / `contract-fleet-health-outcomes` / `contract-fleet-resource-lifecycle`: Web/Workspaceの既存application-scoped FleetRuntimeを唯一のhost health writerにする。独立renderer/settings probeを削除し、exclusive qualified observation、5秒freshness、各3秒のcapability/ticket/hello deadline、exact mismatch terminal、generation fencing、shutdown cleanupを実装する。
- `contract-host-health-wire`: `clients/ui/src/fleet/session-protocol.ts`を`FleetHealthSnapshotV1` schema/decoder ownerにし、Workspace preload/IPCとWeb Go codecをcanonical JSON fixtureへ適合させる。provider delivery closeはlast `(hostInstanceId, revision)`を維持したHealth unavailable overlayだけを生成し、host revisionを進めない。
- `contract-connections-diagnosis`: `clients/ui/src/fleet/connections-presentation.ts`をrows/labels/reasonsの唯一のpure projectionにし、React panelとWorkspace settingsが同じtyped outputをrenderする。browserはhost-reported pathのCopy configuration pathとrestart instructionを提供する。
- `contract-health-configuration-boundary`: 各host transport adapterでraw errorをclosed `HealthReason`へsnapshot/log生成前に分類する。config save/load型へhealthを混ぜず、raw URL、token、ticket、error stringを境界外へ出さない。

## Dependency-ordered units

1. `unit-1-active-lifecycle`: Connection/storeを二重ID fence、hello-only recovery、exact retry accountingへ変更し、hostile orderingを含むT0/T1 testを先に追加する。
2. `unit-2-host-health`: 両host runtimeへqualified union、exact protocol admission、freshness/phase deadlines、typed redaction、resource closureを追加する。1 enabled serverにつきworker最大1、Disabledは0とする。
3. `unit-3-provider-migration`: schema owner、canonical fixtures、両codec、delivery overlay、App-level subscriptionをflag-dayで追加する。renderer polling fallbackは禁止する。
4. `unit-4-shell-connections`: independent status reducer、StatusBar accessibility、shared Connections projection、Workspace navigation、browser config-path actionを追加する。
5. `unit-5-integration-verification`: browser/Workspace/Web host/gatewayとfake-invariant-FakeVsReal tripleを実行し、ADR/persistent design promotion evidenceを閉じる。

## Test seams and discretion

Clock/scheduler、fetch、WebSocket/channel、host member evidenceをinjectする。private lifecycle token object、cancelable schedulerの内部表現、health private map、Connections layout、fixed landmarkをfocusするref/effect plumbingは、design-planのescalation条件を超えない範囲だけimplementation discretionとする。公開state、owner、revision、deadline、focus destination、wire union、sanitized reasonは変更してはならない。
