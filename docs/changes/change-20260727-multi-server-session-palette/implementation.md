---
change: change-20260727-multi-server-session-palette
role: implementation
contracts:
- contract-profile-data-model
- contract-profile-model
- contract-project-profile-override
- contract-descriptor-rest
- contract-server-side-materialization
- contract-create-wire-evolution
- contract-typed-parameter-form
- contract-schema-drift-reconciliation
- contract-confirm-create-control-flow
- contract-server-qualified-identity
- contract-fleet-snapshot-protocol
- contract-application-fleet-authority
- contract-compound-session-routing
- contract-create-observation-convergence
- contract-partial-failure-stale-projection
- contract-credential-free-host-boundary
- contract-view-local-selection
- contract-fleet-resource-lifecycle
- contract-host-parity
- contract-fleet-migration
- contract-fleet-publication-backpressure
- contract-fleet-event-ordering
- contract-scoped-discovery-recovery
contract_projections:
- id: contract-profile-data-model
  decision_rules:
  - decision-profile-data-model
  observable_effects:
  - observable-profile-data-model
  operational_inputs:
  - input-profile-data-model
  semantic_profiles: []
  failures:
  - failure-profile-data-model
  verifications:
  - verify-profile-data-model
  witnesses:
  - witness-profile-data-model-normal
  - witness-profile-data-model-adversarial
- id: contract-profile-model
  decision_rules:
  - decision-profile-model
  observable_effects:
  - observable-profile-model
  operational_inputs:
  - input-profile-model
  semantic_profiles:
  - profile-profile-model-evolution
  failures:
  - failure-profile-model
  verifications:
  - verify-profile-model
  witnesses:
  - witness-profile-model-normal
  - witness-profile-model-adversarial
- id: contract-project-profile-override
  decision_rules:
  - decision-project-profile-override
  observable_effects:
  - observable-project-profile-override
  operational_inputs:
  - input-project-profile-override
  semantic_profiles: []
  failures:
  - failure-project-profile-override
  verifications:
  - verify-project-profile-override
  witnesses:
  - witness-project-profile-override-normal
  - witness-project-profile-override-adversarial
- id: contract-descriptor-rest
  decision_rules:
  - decision-descriptor-rest
  observable_effects:
  - observable-descriptor-rest
  operational_inputs:
  - input-descriptor-rest
  semantic_profiles: []
  failures:
  - failure-descriptor-rest
  verifications:
  - verify-descriptor-rest
  witnesses:
  - witness-descriptor-rest-normal
  - witness-descriptor-rest-adversarial
- id: contract-server-side-materialization
  decision_rules:
  - decision-server-side-materialization
  observable_effects:
  - observable-server-side-materialization
  operational_inputs:
  - input-server-side-materialization
  semantic_profiles: []
  failures:
  - failure-server-side-materialization
  verifications:
  - verify-server-side-materialization
  witnesses:
  - witness-server-side-materialization-normal
  - witness-server-side-materialization-adversarial
- id: contract-create-wire-evolution
  decision_rules:
  - decision-create-wire-evolution
  observable_effects:
  - observable-create-wire-evolution
  operational_inputs:
  - input-create-wire-evolution
  semantic_profiles:
  - profile-create-wire-evolution-evolution
  failures:
  - failure-create-wire-evolution
  verifications:
  - verify-create-wire-evolution
  witnesses:
  - witness-create-wire-evolution-normal
  - witness-create-wire-evolution-adversarial
- id: contract-typed-parameter-form
  decision_rules:
  - decision-typed-parameter-form
  observable_effects:
  - observable-typed-parameter-form
  operational_inputs:
  - input-typed-parameter-form
  semantic_profiles:
  - profile-typed-parameter-form-cost
  failures:
  - failure-typed-parameter-form
  verifications:
  - verify-typed-parameter-form
  witnesses:
  - witness-typed-parameter-form-normal
  - witness-typed-parameter-form-adversarial
- id: contract-schema-drift-reconciliation
  decision_rules:
  - decision-schema-drift-reconciliation
  observable_effects:
  - observable-schema-drift-reconciliation
  operational_inputs:
  - input-schema-drift-reconciliation
  semantic_profiles: []
  failures:
  - failure-schema-drift-reconciliation
  verifications:
  - verify-schema-drift-reconciliation
  witnesses:
  - witness-schema-drift-reconciliation-normal
  - witness-schema-drift-reconciliation-adversarial
- id: contract-confirm-create-control-flow
  decision_rules:
  - decision-confirm-create-control-flow
  observable_effects:
  - observable-confirm-create-control-flow
  operational_inputs:
  - input-confirm-create-control-flow
  semantic_profiles: []
  failures:
  - failure-confirm-create-control-flow
  verifications:
  - verify-confirm-create-control-flow
  witnesses:
  - witness-confirm-create-control-flow-normal
  - witness-confirm-create-control-flow-adversarial
- id: contract-server-qualified-identity
  decision_rules:
  - decision-server-qualified-identity
  observable_effects:
  - observable-server-qualified-identity
  operational_inputs:
  - input-server-qualified-identity
  semantic_profiles: []
  failures:
  - failure-server-qualified-identity
  verifications:
  - verify-server-qualified-identity
  witnesses:
  - witness-server-qualified-identity-normal
  - witness-server-qualified-identity-adversarial
- id: contract-fleet-snapshot-protocol
  decision_rules:
  - decision-fleet-snapshot-protocol-determinate
  - decision-fleet-snapshot-protocol-unknown
  - decision-fleet-snapshot-protocol-inconclusive
  - decision-fleet-snapshot-protocol-conflicting
  observable_effects:
  - observable-fleet-snapshot-protocol
  operational_inputs:
  - input-fleet-snapshot-protocol-workspace-lifecycle
  - input-fleet-snapshot-protocol-web-lifecycle
  semantic_profiles:
  - profile-fleet-snapshot-protocol-outcome-partition
  - profile-fleet-snapshot-protocol-scope-consistency
  failures:
  - failure-fleet-snapshot-protocol
  verifications:
  - verify-fleet-snapshot-protocol
  witnesses:
  - witness-fleet-snapshot-protocol-normal
  - witness-fleet-snapshot-protocol-adversarial
- id: contract-application-fleet-authority
  decision_rules:
  - decision-application-fleet-authority-determinate
  - decision-application-fleet-authority-unknown
  - decision-application-fleet-authority-inconclusive
  - decision-application-fleet-authority-conflicting
  observable_effects:
  - observable-application-fleet-authority
  operational_inputs:
  - input-application-fleet-authority-workspace-subscribers
  - input-application-fleet-authority-web-subscribers
  semantic_profiles:
  - profile-application-fleet-authority-outcome-partition
  - profile-application-fleet-authority-scope-consistency
  failures:
  - failure-application-fleet-authority
  verifications:
  - verify-application-fleet-authority
  witnesses:
  - witness-application-fleet-authority-normal
  - witness-application-fleet-authority-adversarial
- id: contract-compound-session-routing
  decision_rules:
  - decision-compound-session-routing-determinate
  observable_effects:
  - observable-compound-session-routing
  operational_inputs:
  - input-compound-session-routing-current-fleet-revision
  - input-compound-session-routing-host-routing-registry
  semantic_profiles: []
  failures:
  - failure-compound-session-routing
  verifications:
  - verify-compound-session-routing
  witnesses:
  - witness-compound-session-routing-normal
  - witness-compound-session-routing-adversarial
- id: contract-create-observation-convergence
  decision_rules:
  - decision-create-observation-convergence-determinate
  - decision-create-observation-convergence-unknown
  - decision-create-observation-convergence-inconclusive
  - decision-create-observation-convergence-conflicting
  observable_effects:
  - observable-create-observation-convergence
  operational_inputs:
  - input-create-observation-convergence-workspace-create-observation
  - input-create-observation-convergence-web-create-observation
  - input-create-observation-convergence-create-observation-deadline
  semantic_profiles:
  - profile-create-observation-convergence-outcome-partition
  - profile-create-observation-convergence-scope-consistency
  failures:
  - failure-create-observation-convergence
  verifications:
  - verify-create-observation-convergence
  witnesses:
  - witness-create-observation-convergence-normal
  - witness-create-observation-convergence-adversarial
- id: contract-partial-failure-stale-projection
  decision_rules:
  - decision-partial-failure-stale-projection-determinate
  - decision-partial-failure-stale-projection-unknown
  - decision-partial-failure-stale-projection-inconclusive
  observable_effects:
  - observable-partial-failure-stale-projection
  operational_inputs:
  - input-partial-failure-stale-projection-last-accepted-member-snapshot
  semantic_profiles:
  - profile-partial-failure-stale-projection-outcome-partition
  - profile-partial-failure-stale-projection-scope-consistency
  failures:
  - failure-partial-failure-stale-projection
  verifications:
  - verify-partial-failure-stale-projection
  witnesses:
  - witness-partial-failure-stale-projection-normal
  - witness-partial-failure-stale-projection-adversarial
- id: contract-credential-free-host-boundary
  decision_rules:
  - decision-credential-free-host-boundary-determinate
  - decision-credential-free-host-boundary-unknown
  - decision-credential-free-host-boundary-inconclusive
  observable_effects:
  - observable-credential-free-host-boundary
  operational_inputs:
  - input-credential-free-host-boundary-workspace-routing-authority
  - input-credential-free-host-boundary-web-routing-authority
  semantic_profiles:
  - profile-credential-free-host-boundary-outcome-partition
  failures:
  - failure-credential-free-host-boundary
  verifications:
  - verify-credential-free-host-boundary
  witnesses:
  - witness-credential-free-host-boundary-normal
  - witness-credential-free-host-boundary-adversarial
- id: contract-view-local-selection
  decision_rules:
  - decision-view-local-selection-clean
  - decision-view-local-selection-dirty
  - decision-view-local-selection-cancel
  - decision-view-local-selection-confirm-current
  - decision-view-local-selection-confirm-invalid
  observable_effects:
  - observable-view-local-selection
  operational_inputs:
  - input-view-local-selection-selection-fleet-revision
  - input-view-local-selection-workspace-prepare-switch
  semantic_profiles: []
  failures:
  - failure-view-local-selection
  verifications:
  - verify-view-local-selection
  witnesses:
  - witness-view-local-selection-clean
  - witness-view-local-selection-dirty-immediate-switch-forbidden
  - witness-view-local-selection-confirm-stale
- id: contract-fleet-resource-lifecycle
  decision_rules:
  - decision-fleet-resource-lifecycle-determinate
  - decision-fleet-resource-lifecycle-unknown
  - decision-fleet-resource-lifecycle-inconclusive
  observable_effects:
  - observable-fleet-resource-lifecycle
  operational_inputs:
  - input-fleet-resource-lifecycle-workspace-cancellation-scope
  - input-fleet-resource-lifecycle-web-cancellation-scope
  semantic_profiles:
  - profile-fleet-resource-lifecycle-cost-convergence
  - profile-fleet-resource-lifecycle-outcome-partition
  failures:
  - failure-fleet-resource-lifecycle
  verifications:
  - verify-fleet-resource-lifecycle
  witnesses:
  - witness-fleet-resource-lifecycle-normal
  - witness-fleet-resource-lifecycle-adversarial
- id: contract-host-parity
  decision_rules:
  - decision-host-parity-determinate
  - decision-host-parity-unknown
  - decision-host-parity-inconclusive
  - decision-host-parity-conflicting
  observable_effects:
  - observable-host-parity
  operational_inputs:
  - input-host-parity-shared-host-fixture
  semantic_profiles:
  - profile-host-parity-outcome-partition
  - profile-host-parity-scope-consistency
  failures:
  - failure-host-parity
  verifications:
  - verify-host-parity
  witnesses:
  - witness-host-parity-normal
  - witness-host-parity-adversarial
- id: contract-fleet-migration
  decision_rules:
  - decision-fleet-migration-determinate
  observable_effects:
  - observable-fleet-migration
  operational_inputs: []
  semantic_profiles:
  - profile-fleet-migration-contract-evolution
  failures:
  - failure-fleet-migration
  verifications:
  - verify-fleet-migration
  witnesses:
  - witness-fleet-migration-normal
  - witness-fleet-migration-adversarial
- id: contract-fleet-publication-backpressure
  decision_rules:
  - decision-fleet-publication-backpressure-determinate
  - decision-fleet-publication-backpressure-unknown
  - decision-fleet-publication-backpressure-inconclusive
  observable_effects:
  - observable-fleet-publication-backpressure
  operational_inputs:
  - input-fleet-publication-backpressure-immutable-publication-value
  - input-fleet-publication-backpressure-subscriber-mailboxes
  semantic_profiles:
  - profile-fleet-publication-backpressure-cost-convergence
  - profile-fleet-publication-backpressure-outcome-partition
  failures:
  - failure-fleet-publication-backpressure
  verifications:
  - verify-fleet-publication-backpressure
  witnesses:
  - witness-fleet-publication-backpressure-normal
  - witness-fleet-publication-backpressure-adversarial
- id: contract-fleet-event-ordering
  decision_rules:
  - decision-fleet-event-ordering-determinate
  - decision-fleet-event-ordering-unknown
  - decision-fleet-event-ordering-inconclusive
  - decision-fleet-event-ordering-conflicting
  observable_effects:
  - observable-fleet-event-ordering
  operational_inputs:
  - input-fleet-event-ordering-workspace-event-sequence
  - input-fleet-event-ordering-web-event-sequence
  semantic_profiles:
  - profile-fleet-event-ordering-outcome-partition
  - profile-fleet-event-ordering-scope-consistency
  failures:
  - failure-fleet-event-ordering
  verifications:
  - verify-fleet-event-ordering
  witnesses:
  - witness-fleet-event-ordering-normal
  - witness-fleet-event-ordering-adversarial
- id: contract-scoped-discovery-recovery
  decision_rules:
  - decision-scoped-discovery-recovery-determinate
  - decision-scoped-discovery-recovery-unknown
  - decision-scoped-discovery-recovery-inconclusive
  observable_effects:
  - observable-scoped-discovery-failure
  - observable-scoped-discovery-retry
  operational_inputs:
  - input-scoped-discovery-generation
  - input-retained-palette-context
  semantic_profiles:
  - profile-scoped-discovery-recovery-outcome-partition
  failures:
  - failure-scoped-discovery-recovery
  verifications:
  - verify-scoped-discovery-recovery
  witnesses:
  - witness-scoped-discovery-normal
  - witness-scoped-discovery-partial-retry
  - witness-scoped-discovery-invalidated-context
adrs:
- adr-20260727-fleet-provider-boundary
- adr-20260727-typed-launch-profile-schema
- adr-20260727-server-argv-materialization
- adr-20260727-arbitrary-parameter-palette-state
- adr-20260727-server-qualified-identity-partial-failure
- adr-20260727-host-neutral-fleet-provider
- adr-20260714-workspace-session-switch-lifecycle
- adr-20260727-application-scoped-fleet-session-authority
decision_dispositions:
- decision_input_ref: decision-input-unified-workspace-fleet
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs:
  - adr-20260727-application-scoped-fleet-session-authority
  contract_refs:
  - contract-application-fleet-authority
  - contract-fleet-snapshot-protocol
- decision_input_ref: decision-input-server-routing-not-boundary
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs:
  - adr-20260727-application-scoped-fleet-session-authority
  - adr-20260714-workspace-session-switch-lifecycle
  contract_refs:
  - contract-compound-session-routing
  - contract-view-local-selection
- decision_input_ref: decision-input-fleet-config-aggregation
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs:
  - adr-20260727-host-neutral-fleet-provider
  - adr-20260727-server-qualified-identity-partial-failure
  contract_refs:
  - contract-fleet-snapshot-protocol
  - contract-partial-failure-stale-projection
- decision_input_ref: decision-input-declarative-typed-parameters
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs:
  - adr-20260727-typed-launch-profile-schema
  - adr-20260727-arbitrary-parameter-palette-state
  contract_refs:
  - contract-profile-model
  - contract-typed-parameter-form
- decision_input_ref: decision-input-parameter-contract-handoff
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs:
  - adr-20260727-typed-launch-profile-schema
  - adr-20260727-server-argv-materialization
  contract_refs:
  - contract-server-side-materialization
  - contract-confirm-create-control-flow
- decision_input_ref: decision-input-host-neutral-provider
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs:
  - adr-20260727-host-neutral-fleet-provider
  contract_refs:
  - contract-host-parity
  - contract-credential-free-host-boundary
- decision_input_ref: decision-input-server-qualified-partial-failure
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs:
  - adr-20260727-server-qualified-identity-partial-failure
  contract_refs:
  - contract-compound-session-routing
  - contract-partial-failure-stale-projection
- decision_input_ref: decision-input-fleet-provider-security
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs:
  - adr-20260727-host-neutral-fleet-provider
  contract_refs:
  - contract-credential-free-host-boundary
- decision_input_ref: decision-input-architecture-host-parity
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs:
  - adr-20260727-host-neutral-fleet-provider
  - adr-20260727-application-scoped-fleet-session-authority
  contract_refs:
  - contract-host-parity
  - contract-application-fleet-authority
- decision_input_ref: decision-input-external-dependency-triple
  disposition: adopted
  rationale: Fixed by the listed accepted ADR and/or executable implementation contract; no design choice remains.
  adr_refs: []
  contract_refs:
  - contract-host-parity
  - contract-fleet-snapshot-protocol
milestones:
- id: milestone-1
- id: milestone-2
- id: milestone-3
- id: milestone-4
- id: milestone-5
- id: milestone-6
- id: milestone-7
- id: milestone-8
- id: milestone-9
reference_algorithms:
- id: alg-schema-revision
- id: alg-fleet-member-publication
- id: alg-create-observation-deadline
---

# Implementation

## Ownership and seams

`component-fleet-provider-protocol` is the single semantic owner. Workspace Electron main and Web host each realize that protocol once per host process; neither renderer nor window owns fleet membership. Each realization injects configured routing, credential attachment, lifecycle transport, clock/timer and cancellation. The pure reducer owns generation acceptance, per-member replacement, fleet revision, create correlation, subscriber mailboxes and typed effects.

The SPA receives credential-free immutable full snapshots, stores compound keys, and routes every sessions/workspace/transcript/socket/subscription operation through `SessionRef`. Scoped project/catalog/profile/parameter discovery failure retains healthy scopes and still-valid selection/value/focus through explicit retry; refreshed invalid context is explained and cannot create. High-frequency terminal/output traffic stays on a separately acquired active-session channel. `selectSession` remains the accepted dirty-aware transaction.

## Migration and delivery order

The nine milestones in `design-plan.json` retain profile/config/materialization and palette units, then add protocol, both host runtimes, SPA migration, server-neutral Workspace lifecycle and the cross-host verification triple. Migration is flag-day: `currentServerId`, raw URL helpers, raw sessionId operations and server-specific window routing have no compatibility fallback. Single-server is the same configured fleet with one member.

## Implemented fleet runtime

- `clients/web-host/internal/webhost/fleet.go` owns one lazy-started runtime for every enabled server, consumes authoritative hello/view-update lifecycle, retains stale last-known members, rejects old generations and publishes full latest-wins snapshots at `/host/fleet`.
- `clients/workspace/src/main/fleet-session-runtime.ts` owns the equivalent Electron-main runtime. Preload exposes only credential-free snapshots, compound routed requests and an active-channel relay; upstream URL, token and authorization remain in main.
- Create now goes through the host authority. Both hosts wait up to 10 seconds for the returned compound identity to appear in lifecycle state and return `confirmed` or `created_unconfirmed` without repeating POST.
- The SPA stores a compound JSON key for each session, routes session/workspace/transcript/terminal operations by that key and recreates only the active channel when server selection changes.
- `WindowRegistry` owns one generic fleet window. A subsequent Shell `openSession` selects the supplied `SessionRef` in that window instead of constructing a server/session-specific renderer.
