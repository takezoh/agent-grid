---
change: change-20260814-cli-prerequisite-contracts
role: implementation
contracts:
- contract-session-terminal-state-retention
- contract-transport-close-taxonomy
- contract-config-and-fleet-identity
- contract-client-config-parity
- contract-config-entry-admission
- contract-server-naming-prerequisite
contract_projections:
- id: contract-session-terminal-state-retention
  verifications:
  - ver-explicit-terminate-leaves-a-terminal-row
  - ver-self-exit-paths-leave-a-terminal-row
  - ver-terminal-transition-is-published-on-lifecycle
  - ver-ended-and-never-existed-are-distinguishable
  discretion: []
- id: contract-transport-close-taxonomy
  verifications:
  - ver-every-server-initiated-close-carries-a-published-reason
  - ver-pre-upgrade-rejections-carry-the-same-vocabulary
  - ver-close-reason-vocabulary-is-closed-and-registered
  discretion: []
- id: contract-config-and-fleet-identity
  verifications:
  - ver-every-go-host-loads-through-one-owner
  - ver-server-id-digest-golden-vector
  - ver-config-never-written-and-directory-per-platform
  discretion: []
- id: contract-client-config-parity
  verifications:
  - ver-expected-outcome-corpus-agrees-in-three-languages
  - ver-go-loader-matches-the-expected-outcome-corpus
  - ver-corpus-covers-every-per-entry-and-file-level-rule
  - ver-measured-divergences-are-closed-by-the-corpus
  discretion: []
- id: contract-config-entry-admission
  verifications:
  - ver-host-starts-and-only-the-offending-entries-are-unusable
  - ver-unreadable-entry-list-still-stops-the-host
  discretion: []
- id: contract-server-naming-prerequisite
  verifications:
  - ver-duplicate-display-name-flagged-per-entry-by-every-loader
  - ver-charset-violation-flagged-identically
  - ver-naming-violation-names-the-entry-without-renaming
  discretion: []
adrs:
- adr-cli-session-terminal-state-retention-prerequisite
- adr-transport-close-reason-taxonomy
- adr-shared-go-client-config-module
- adr-server-display-name-naming-contract
decision_dispositions:
- decision_input_ref: dh-lifecycle-contract-sufficient
  disposition: found_insufficient_on_four_termination_paths_prerequisite_contract_change_recorded
  adr_refs:
  - adr-cli-session-terminal-state-retention-prerequisite
  contract_refs:
  - contract-session-terminal-state-retention
- decision_input_ref: open-terminate-observability
  disposition: unresolved_escalated_to_user_with_a_proposed_prerequisite_adr
  adr_refs:
  - adr-cli-session-terminal-state-retention-prerequisite
  contract_refs:
  - contract-session-terminal-state-retention
- decision_input_ref: esc-transport-close-untyped
  disposition: closed_as_a_prerequisite_public_contract_change_with_a_closed_published_reason_vocabulary
  adr_refs:
  - adr-transport-close-reason-taxonomy
  contract_refs:
  - contract-transport-close-taxonomy
- decision_input_ref: esc-fleet-identity
  disposition: adopted_as_row_identity_discipline_grounded_in_the_canonical_client_config_contract
  adr_refs:
  - adr-shared-go-client-config-module
  contract_refs:
  - contract-config-and-fleet-identity
  - contract-client-config-parity
- decision_input_ref: candidate-fleet-session-identity-reuse
  disposition: adopted_with_bare_upstream_id_on_the_wire_and_single_construction_owner
  adr_refs:
  - adr-shared-go-client-config-module
  contract_refs:
  - contract-config-and-fleet-identity
- decision_input_ref: open-shared-go-client-extraction
  disposition: superseded_common_ownership_is_the_client_config_contract_itself_one_loader_per_language_with_a_shared_expected_outcome_corpus
  adr_refs:
  - adr-shared-go-client-config-module
  contract_refs:
  - contract-config-and-fleet-identity
  - contract-client-config-parity
- decision_input_ref: open-web-host-server-side-config
  disposition: recorded_out_of_scope_the_hosts_own_runtime_settings_are_not_the_client_config_contract
  contract_refs:
  - contract-client-config-parity
- decision_input_ref: handoff-server-display-name-naming-contract
  disposition: closed_as_prerequisite_contract_change_with_enforcement_point_charset_migration_and_per_entry_admission
  adr_refs:
  - adr-server-display-name-naming-contract
  contract_refs:
  - contract-server-naming-prerequisite
  - contract-config-entry-admission
  - contract-config-and-fleet-identity
- decision_input_ref: esc-config-entry-admission-granularity
  disposition: closed_as_per_entry_admission_with_a_readability_only_file_level_residue
  adr_refs:
  - adr-server-display-name-naming-contract
  contract_refs:
  - contract-config-entry-admission
  - contract-server-naming-prerequisite
- decision_input_ref: open-config-contract-design-ownership
  disposition: unresolved_promotion_candidate_recorded
  adr_refs:
  - adr-server-display-name-naming-contract
  contract_refs:
  - contract-server-naming-prerequisite
milestones:
- id: Execution-plane terminal-state retention prerequisite
- id: Public transport close reason vocabulary
- id: One Go loader for the canonical client configuration contract
- id: Canonical client configuration contract and its three-language parity corpus
- id: Server naming rule and per-entry configuration admission
reference_algorithms: []
---

<!-- lifecycle is owned by change.md -->

# Implementation

## Content

Canonical plan: `../change-20260812-cli-terminal-client/design-plan/design.md` + `../change-20260812-cli-terminal-client/design-plan/spine.yaml`, **shared** with `change-20260812-cli-terminal-client` and neither split nor copied. Every id below — unit, contract, component, requirement, verification — is that plan's id. Anchors are the `<!-- anchor: … -->` markers in `design.md`.

### Boundary

This change contains the parts of the shared plan that are **not** `clients/cli` work: three contract groups the terminal CLI depends on but does not implement.

- **A — execution-plane terminal-state retention.** An ended session stays observable with a terminal lifecycle state, and the transition is published on the lifecycle socket. `services/host/state/`, `services/server/api/gateway.go`.
- **B — public transport close reason taxonomy.** A closed published vocabulary carried on every server-initiated close and in every pre-upgrade rejection body. `protocol/`, `services/server/api/`.
- **C — client configuration contract, its Go loader, and per-entry admission with a server naming rule.** `clients/config`, a new `clients/goclient` module, and the three existing loaders.

Nothing here builds, imports, or configures the CLI binary. No CLI observable behaviour is implemented in this change; the CLI's use of these contracts is `change-20260812-cli-terminal-client`, which `dependsOn` this change.

### Units and dependency order

Group A and group B are independent of each other and of group C. Group C is a strictly ordered chain of three, and that order is load-bearing: a naming rule landing on a whole-file failure path would stop hosts that work today, and a per-entry rewrite with no expected-outcome corpus would change three loaders with nothing able to tell whether they still agree.

| # | Unit | Group | Depends on | Plan anchor |
|---|---|---|---|---|
| 1 | unit-terminal-state-retention-prerequisite | A | — | `spine.yaml` `units[]` |
| 2 | unit-transport-close-taxonomy-prerequisite | B | — | `spine.yaml` `units[]` |
| 3 | unit-shared-goclient-config | C | — | `spine.yaml` `units[]` |
| 4 | unit-client-config-parity | C | 3 | `spine.yaml` `units[]` |
| 5 | unit-server-naming-contract | C | 4 | `spine.yaml` `units[]` |

Per-unit objectives, acceptance items, output format, tool guidance and task boundaries are in `spine.yaml` under `units`; they are not restated here.

### Paths this change touches

| Unit | Paths |
|---|---|
| unit-terminal-state-retention-prerequisite | `services/host/state/reduce_session.go`, `services/host/state/reduce_frame_evict.go`, `services/host/state/reduce_tick.go`, `services/host/state/view/status.go`, `services/server/api/gateway.go` |
| unit-transport-close-taxonomy-prerequisite | `protocol/transport-close.schema.json`, `protocol/README.md`, `services/server/api/gateway.go`, `services/server/api/mux.go`, `services/server/api/auth.go`, `services/server/api/protocol_version.go`, `services/gorules/static_enforcement_test.go` |
| unit-shared-goclient-config | `clients/goclient/go.mod`, `clients/goclient/clientconfig/`, `clients/web-host/internal/clientconfig/`, `clients/web-host/go.mod` |
| unit-client-config-parity | `clients/config/README.md`, `clients/config/servers.schema.json`, `clients/config/fixtures/`, `clients/goclient/clientconfig/`, `clients/workspace/src/main/desktop-config.ts`, `clients/windows-shell/AgentGrid.Shell.Core/Configuration/DesktopConfig.cs` |
| unit-server-naming-contract | `clients/config/servers.schema.json`, `clients/config/README.md`, `clients/config/fixtures/`, `clients/goclient/clientconfig/`, `clients/web-host/internal/webhost/host.go`, `clients/workspace/src/main/desktop-config.ts`, `clients/windows-shell/AgentGrid.Shell.Core/Configuration/DesktopConfig.cs` |

`services/gorules/static_enforcement_test.go` is also touched by the CLI change (module and golden-vector gate registration). Both changes add entries to that file; neither owns it.

### Components

| Component | Kind | Owns |
|---|---|---|
| comp-execution-plane-lifecycle | existing | session record lifetime and lifecycle publication (`design.md` `comp-execution-plane-lifecycle`) |
| comp-gateway-transport | existing | socket admission and departure (`design.md` `comp-gateway-transport`) |
| comp-config-contract | existing | the canonical client configuration contract (`design.md` `comp-config-contract`) |
| comp-goclient-config | planned | the Go projection of that contract, introduced by unit 3 (`design.md` `comp-goclient-config`) |

### Implementation contracts

| Contract | Owner | Requirements | Carrier units in this change | Plan anchor |
|---|---|---|---|---|
| contract-session-terminal-state-retention | comp-execution-plane-lifecycle | FR-026, FR-015, FR-014 | terminal-state-retention-prerequisite | `design.md` `contract-session-terminal-state-retention` |
| contract-transport-close-taxonomy | comp-gateway-transport | FR-032, FR-014, FR-024 | transport-close-taxonomy-prerequisite | `design.md` `contract-transport-close-taxonomy` |
| contract-config-and-fleet-identity | comp-config-contract | FR-003, FR-019, FR-021, FR-022, NFR-003 | shared-goclient-config, client-config-parity | `design.md` `contract-config-and-fleet-identity` |
| contract-client-config-parity | comp-config-contract | NFR-009, NFR-003, FR-021 | client-config-parity, shared-goclient-config, server-naming-contract | `design.md` `contract-client-config-parity` |
| contract-config-entry-admission | comp-config-contract | FR-030, FR-028, FR-031 | server-naming-contract | `design.md` `contract-config-entry-admission` |
| contract-server-naming-prerequisite | comp-config-contract | FR-028, FR-029, FR-021 | server-naming-contract | `design.md` `contract-server-naming-prerequisite` |

The decision rules, outcome partitions, operational inputs, invariants, forbidden clauses and risk-tagged witnesses for each contract are in `design.md` under the matching anchor. They are not restated here.

Four of these contracts also list CLI units among their `unit_refs`, because the same contract binds both sides of the boundary. The CLI-side carriers stay in the CLI change:

| Contract | CLI-side carrier unit | CLI-side obligation |
|---|---|---|
| contract-transport-close-taxonomy | unit-transport-admission, unit-attach-runtime | map each published reason to its exit class |
| contract-config-and-fleet-identity | unit-lifecycle-commands | never read `default_execution` as a create target |
| contract-config-entry-admission | unit-fleet-reachability, unit-discovery-render | never probe an invalid entry; render it in a third vocabulary |
| contract-server-naming-prerequisite | unit-selector | unusable-outranks-ambiguous qualifier precedence |

### Decisions recorded as ADRs

All eleven ADRs of the shared plan are `accepted`. Four are implemented by this change; the other seven are CLI decisions and stay with `change-20260812-cli-terminal-client`. Each document carries `originatedFrom` the CLI change — that is where the decision was discovered — plus a `references` relation to this change, which implements it.

| Plan id (`design.md` §E) | Document | Decision |
|---|---|---|
| adr-cli-session-terminal-state-retention-prerequisite | `docs/adr/adr-20260814-session-terminal-state-retention.md` | An ended session is retained with a terminal lifecycle state on all four termination paths, and the transition is published on the lifecycle socket. Retention window and reclaimer stay open. |
| adr-transport-close-reason-taxonomy | `docs/adr/adr-20260814-transport-close-reason-taxonomy.md` | A closed published vocabulary of transport close reasons, carried on every server-initiated close and in every pre-upgrade rejection body, in the reason slot rather than in numeric close codes. |
| adr-shared-go-client-config-module | `docs/adr/adr-20260814-shared-go-client-config-module.md` | `clients/config` is the canonical client configuration contract for every host; each client language holds exactly one loader of it; three-language agreement is proven by a corpus whose fixtures carry their expected outcomes. |
| adr-server-display-name-naming-contract | `docs/adr/adr-20260814-server-display-name-naming-contract.md` | Display-name uniqueness by exact code-point equality after trimming, a CLI-safe character set, enforced at load in all three loaders **per entry**. Only an unreadable entry list stops a host. |

### The measured gaps these units close

Evidence grades: **[measured]** at a named path in this repository. The full argument is in `design.md` under each contract anchor; the summary is here so the unit's starting point is not a claim.

**A — session terminal-state retention.** [measured] Four termination paths, three of which delete the session record: explicit terminate (`reduceStopSession` executes `delete(s.Sessions, sid)`, `services/host/state/reduce_session.go:313-329`, durable snapshot removed at `services/host/runtime/runtime.go:789-792`); a clean or signalled self-exit with code in `{0,129,130,137,143}` (`reduce_tick.go:75-115` → `evictRootFrame`, `reduce_frame_evict.go:29-32`); and a vanished backend frame (`reduce_tick.go:49-57`). Only the abnormal-exit-code path leaves a row. Meanwhile `protocol/session-status.schema.json` declares `lifecycle ∈ {active, idle, stopped}`, `protocol/session-status-display.json` gives `stopped` a label, a glyph and a precedence slot, and `services/host/state/view/status.go:112,140-150` maps it — so every shipping client already renders a state the plane can reach on one path in four. Frame and process teardown stay exactly as they are; only the fate of the session record and what is published change.

**B — transport close reason taxonomy.** [measured] The gateway has one close helper always using `StatusGoingAway` (`services/server/api/gateway.go:179-181`) carrying one of four strings none of which appears in any schema; every other departure unwinds into `defer c.CloseNow()` (`mux.go:613`) with no frame at all; `controlMsg.Code` is `int,omitempty` and every call site passes `0` (`wire.go:45-55`), so only the free string `data` discriminates; and a rejected ticket is an HTTP 401 with the plain-text body `unauthorized` before the socket exists (`mux.go:198-204`, `auth.go:20-23`), with empty, unknown, used and expired collapsed into one boolean (`ticket.go:66-81`). Graceful shutdown, ticket rejection and a network drop are mutually indistinguishable to every client today. The shipped clients confirm the third source-requirement condition rather than assert it: the SPA does not bind the close event at all (`clients/ui/src/socket/connection.ts:400`), the Workspace relay publishes `close` with no payload (`electron-entry.ts:488-491`), and its fleet prober classifies failures by regular expressions over `Error.message` (`server-health.ts:126-131`).

**C1 — one Go loader.** [measured] `clients/config/README.md:3-4` already declares `servers.json` a shared contract and three loaders read it, yet `clients/config/servers.schema.json` is read by no code and the three fixture tests assert only shape (Go reads one of four fixtures, `config_test.go:10-35`; TypeScript asserts a bare `.toThrow()`, `desktop-config.test.ts:339-365`; C# asserts `Assert.Throws`, `DesktopConfigTests.cs:215-243`). The Go loader has no Windows configuration-directory branch (`config.go:40-49`).

**C2 — parity corpus.** [measured] Three divergences follow from C1: Go silently drops unknown entry fields; Go rejects URL credentials at `config.go:143` while TypeScript strips them at `desktop-config.ts:328-332`; and the Windows directory branch is missing. Because the server identity is the digest of the *canonical* URL, the second divergence gives one server two identities across hosts — and that identity is the value FR-019 puts in machine-readable output.

**C3 — naming rule and per-entry admission.** [measured] `clients/config/servers.schema.json:22` constrains a display name only with `minLength: 1`, and the uniqueness check in all three loaders (`clients/web-host/internal/clientconfig/config.go:91-93`, `clients/workspace/src/main/desktop-config.ts:302-304`, `clients/windows-shell/.../DesktopConfig.cs:132-133`) is keyed on the URL digest, so the upstream naming constraint has no enforcement point. Two impacts fall out and are part of unit 5 rather than recorded and deferred: `clients/config/README.md:41-43` documents whole-file rejection and must be rewritten, and `clients/web-host/internal/webhost/host.go:30-31` refuses a snapshot with no servers and must be relaxed.

### Test seams

- **Client configuration loading** — after unit 3 there is exactly one Go parser of `servers.json`, importable from another module without an `internal` path. Both Go hosts and the CLI consume it; the fixture corpus is the seam that makes the three languages comparable rather than merely similar.
- **Execution-plane reducers** — `services/host/state/reduce_*.go` are already pure reducers over state and events; the four termination paths are drivable without a real backend.
- **Gateway departure shapes** — every server-initiated close and every pre-upgrade rejection must be reachable in `services/server/api` tests, including the paths that today unwind into `CloseNow()` with nothing sent.
- **No new external dependency** — this change introduces no new external boundary, so it adds no entry to `test-harness/dependencies.json`. The CLI change owns the gateway and local-terminal boundary registrations.

### Delegated to the implementer

None. The shared plan's five `implementation_discretion` items all sit on CLI contracts and CLI units (`discretion-pump-goroutine-topology`, `discretion-input-read-buffering`, `discretion-table-rendering`, `discretion-flag-parsing-implementation`, `discretion-fake-gateway-internals`) and stay with `change-20260812-cli-terminal-client`.

### Decision dispositions bearing on this change

The complete disposition ledger for the shared plan is `spine.yaml` `dispositions`; it is the single source and is not duplicated. The rows below are the ones whose ADR or contract lands here.

| Input | Disposition |
|---|---|
| dh-lifecycle-contract-sufficient | found insufficient on four termination paths; prerequisite contract change recorded |
| open-terminate-observability | prerequisite ADR **accepted**; the retention window and its reclaimer remain the open sub-question, owned by the durable-snapshot owner |
| esc-transport-close-untyped | closed as a prerequisite public contract change with a closed published reason vocabulary |
| esc-fleet-identity | adopted as the row identity discipline, grounded in the canonical client configuration contract |
| candidate-fleet-session-identity-reuse | adopted; the configuration half (canonical URL, derived server id) lands here, the wire and construction-owner half in the CLI change |
| open-shared-go-client-extraction | **superseded** — common ownership is the client configuration contract itself, one loader per language with a shared expected-outcome corpus |
| open-web-host-server-side-config | recorded out of scope — a host's own runtime settings are not the client configuration contract |
| handoff-server-display-name-naming-contract | **closed** as a prerequisite contract change with enforcement point, character set, migration shape and per-entry admission |
| esc-config-entry-admission-granularity | closed as per-entry admission with a readability-only file-level residue |
| open-config-contract-design-ownership | unresolved; promotion candidate recorded below |

Two further dispositions are owned by the CLI change but have a half here, recorded so neither side loses it: `open-resource-exhausted-failure-class` (the interim client-authored reason string is replaced by the published close reason from group B; the dedicated exit code is declined in the CLI change) and `evidence-session-end-frame-shape` (acquired by the CLI change's `unit-phase0-contract-pin`; a negative result no longer degrades the contract because group A publishes the transition either way).

### Promotion manifest (proposed — to be applied to `change.md` by the change owner)

This run's mutation scope does not include `change.md`, so the promotion entries are recorded here.

| Action | Design | Reason |
|---|---|---|
| upsert | `design-clients-goclient` (new, component) | Responsibility: the **Go projection** of the client configuration contract — loading, canonical URL derivation, server identity derivation, per-entry admission, platform directory resolution. Boundary: `clients/goclient/`. It is a projection, not the contract. |
| upsert | `design-windows-shell` | It currently owns `clients/config/` and the server naming contract (RESP-005 / INV-009). This change makes that contract explicitly canonical for every client host and binds three languages to it, which sharpens the name/scope divergence rather than resolving it. Recorded as `open-config-contract-design-ownership`; carving out `design-clients-config` is a candidate, not a decision. |
| upsert | `design-host` | Retained terminal state and its lifecycle publication become an execution-plane responsibility and an invariant. |
| upsert | `design-host` (second axis) | A published closed vocabulary for connection departure becomes a public-contract invariant of the gateway, and the two-step close shape recorded in `adr-20260624-0011` becomes schema-checked rather than conventional. |
| none | `design-clients-ui`, `design-workspace-desktop` | Both gain a capability from group B without being modified by it; adopting the published reason in those clients is their own change. |

Governing designs for this change: `design-host` for groups A and B, `design-windows-shell` for group C until `open-config-contract-design-ownership` is settled.
