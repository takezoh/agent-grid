---
change: change-20260727-web-host-client-boundary
role: implementation
contracts:
- contract-client-config-model
- contract-config-snapshot
- contract-config-outcomes
- contract-host-neutral-provider
- contract-host-credential-boundary
- contract-web-transport
- contract-web-module
- contract-services-flag-day
contract_projections:
- id: contract-client-config-model
  decision_rules: [decision-config-model]
  observable_effects: [observable-config-projection]
  operational_inputs: [input-servers-v2]
  semantic_profiles: []
  failures: [failure-config-model-internal]
  verifications: [verify-config-model]
  witnesses: [witness-config-model-normal, witness-config-model-adversarial]
- id: contract-config-snapshot
  decision_rules: [decision-config-snapshot]
  observable_effects: [observable-config-snapshot]
  operational_inputs: [input-config-snapshot-source]
  semantic_profiles: []
  failures: [failure-config-snapshot-order]
  verifications: [verify-config-snapshot]
  witnesses: [witness-config-snapshot-normal, witness-config-snapshot-adversarial]
- id: contract-config-outcomes
  decision_rules: [decision-config-determinate, decision-config-unknown, decision-config-inconclusive]
  observable_effects: [observable-config-outcome]
  operational_inputs: [input-config-evidence]
  semantic_profiles: [profile-config-outcomes]
  failures: [failure-config-unavailable, failure-config-invalid]
  verifications: [verify-config-outcomes]
  witnesses: [witness-config-outcomes-normal, witness-config-outcomes-adversarial]
- id: contract-host-neutral-provider
  decision_rules: [decision-provider-contract]
  observable_effects: [observable-provider-parity]
  operational_inputs: [input-provider-snapshot]
  semantic_profiles: []
  failures: [failure-provider-server]
  verifications: [verify-provider-parity]
  witnesses: [witness-provider-normal, witness-provider-adversarial]
- id: contract-host-credential-boundary
  decision_rules: [decision-host-credential]
  observable_effects: [observable-credential-boundary]
  operational_inputs: [input-host-credential]
  semantic_profiles: []
  failures: [failure-credential-target]
  verifications: [verify-credential-boundary]
  witnesses: [witness-credential-normal, witness-credential-adversarial]
- id: contract-web-transport
  decision_rules: [decision-web-transport]
  observable_effects: [observable-web-transport]
  operational_inputs: [input-web-request]
  semantic_profiles: []
  failures: [failure-web-upstream, failure-web-origin]
  verifications: [verify-web-transport]
  witnesses: [witness-web-transport-normal, witness-web-transport-adversarial]
- id: contract-web-module
  decision_rules: [decision-web-module]
  observable_effects: [observable-web-module]
  operational_inputs: []
  semantic_profiles: [profile-web-module-evolution]
  failures: [failure-web-module-edge]
  verifications: [verify-web-module]
  witnesses: [witness-web-module-normal, witness-web-module-adversarial]
- id: contract-services-flag-day
  decision_rules: [decision-services-flag-day]
  observable_effects: [observable-services-layout]
  operational_inputs: []
  semantic_profiles: [profile-services-evolution]
  failures: [failure-services-stale-reference]
  verifications: [verify-services-baseline, verify-services-target]
  witnesses: [witness-services-normal, witness-services-adversarial]
adrs:
- adr-20260727-host-neutral-fleet-provider
- adr-20260727-independent-web-host-module
- adr-20260727-web-host-transport-continuity
- adr-20260727-services-go-module-flag-day
decision_dispositions:
- decision_input_ref: decision-input-web-host-module
  disposition: adopted
  rationale: Independent module and owned TLS boundary are fixed.
  adr_refs: [adr-20260727-independent-web-host-module]
  contract_refs: [contract-web-module]
- decision_input_ref: decision-input-host-neutral-fleet
  disposition: adopted
  rationale: Accepted credential isolation is generalized while browser one-server is superseded.
  adr_refs: [adr-20260727-host-neutral-fleet-provider]
  contract_refs: [contract-host-neutral-provider, contract-host-credential-boundary]
- decision_input_ref: decision-input-root-flag-day
  disposition: adopted
  rationale: The flag day is retained but its target is the explicit independent services module; repository-root Go module is rejected.
  adr_refs: [adr-20260727-services-go-module-flag-day]
  contract_refs: [contract-services-flag-day]
- decision_input_ref: decision-input-web-observables
  disposition: adopted
  rationale: Qualified routes preserve the required observable semantics without a one-server default.
  adr_refs: [adr-20260727-web-host-transport-continuity]
  contract_refs: [contract-web-transport]
milestones:
- id: chunk-1-provider-boundary
- id: chunk-2-web-host
- id: chunk-3-services-flag-day
reference_algorithms: []
---

# Implementation

## Fixed Boundaries

- `servers.json` schema version 2 is loaded once before any listener/window exists. The in-memory snapshot is immutable until process exit.
- Each host retains credential-bearing entries and exposes only `serverId`, display metadata, auth capability and typed per-server outcomes.
- clients/ui consumes one FleetProvider. Web uses same-origin HTTP/WS transport; Workspace uses preload/IPC transport. No host-kind or cardinality branch is permitted.
- Web routes are `/host/config`, `/host/servers/{serverId}/api/...`, and `/host/servers/{serverId}/ws`. Legacy unqualified `/api` and `/ws` do not survive the flag day.
- clients/web-host owns embed, config load, proxy, CSP/header/origin checks and its small TLS serving implementation. It imports nothing from the services module.
- `services/` is module `github.com/takezoh/agent-grid/services` and contains nonclient deployable services plus dedicated shared infrastructure. Go client hosts remain under `clients/`.
- `clients/config` owns language-neutral versioned schema/fixtures for every client host.
- Allowed top-level ownership is `services/`, `clients/`, `protocol/`, `contracts/`, `docs/`, `deploy/`, `scripts/`, and `test-harness/`; there is no root Go module or `src` wrapper.

## External Seams

- Config filesystem: injected config-dir/temp directory and shared schema fixtures.
- Upstream server fleet: fake HTTP/WS servers capturing target, Authorization, redirect and cancellation.
- Workspace transport: preload/IPC fake implementing the same FleetProvider conformance suite.
- Browser transport: httptest plus Playwright real-stack path through qualified same-origin routes.
- Go module resolution: `GOWORK=off` gates for services and clients/web-host independently.

## Sequence

1. Fix the language-neutral config/projection fixtures and conformance suite.
2. Remove UI host/cardinality branches and adapt Workspace to the common contract.
3. Move/generalize Web host and prove nested-module independence.
4. Apply the services/config/top-level ownership flag day; update every active consumer.
5. Reconcile the active multi-server change and promote the permanent no-special-case design.

Rollback is whole-change revert. Dual routes, modules, symlinks and temporary `src/` forwarders are forbidden because they recreate ambiguous ownership.
