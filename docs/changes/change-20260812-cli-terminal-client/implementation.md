---
change: change-20260812-cli-terminal-client
role: implementation
contracts:
- contract-attach-transport-topology
- contract-attach-lease-progress
- contract-terminal-mode-lifecycle
- contract-detach-boundary
- contract-input-passthrough
- contract-output-passthrough
- contract-wire-shape-conformance
- contract-identity-single-origin
- contract-geometry-propagation
- contract-exit-classification
- contract-transport-close-taxonomy
- contract-protocol-admission
- contract-config-and-fleet-identity
- contract-config-entry-admission
- contract-server-naming-prerequisite
- contract-discovery-projection
- contract-status-vocabulary
- contract-selector-resolution
- contract-create-and-attach
- contract-explicit-terminate
- contract-module-boundary
- contract-external-dependency-registration
contract_projections:
- id: contract-attach-transport-topology
  verifications:
  - ver-attach-topology-snapshot-two-clients
  - ver-attach-capacity-refusal-is-transport-with-the-published-reason
  - ver-attach-topology-no-per-session-route
  - ver-attach-never-reports-accepted-as-established
  discretion: []
- id: contract-attach-lease-progress
  verifications:
  - ver-lease-renews-under-stdout-backpressure
  - ver-lease-revision-monotonic
  discretion:
  - discretion-pump-goroutine-topology
- id: contract-terminal-mode-lifecycle
  verifications:
  - ver-restore-on-every-exit-path
  - ver-pre-subscribe-failure-leaves-terminal-untouched
  - ver-restore-is-idempotent
  - ver-non-tty-stdin-fails-without-mode-change
  discretion: []
- id: contract-detach-boundary
  verifications:
  - ver-post-detach-bytes-surface-nowhere
  - ver-input-queue-flushed-before-restore
  - ver-configured-detach-byte-only
  - ver-detach-sends-no-terminate
  discretion: []
- id: contract-input-passthrough
  verifications:
  - ver-utf8-sequence-never-split
  - ver-input-fuzz-order-and-bytes
  - ver-ctrl-c-forwarded-as-0x03
  discretion:
  - discretion-input-read-buffering
- id: contract-output-passthrough
  verifications:
  - ver-output-bytes-are-byte-faithful
  - ver-foreign-session-output-dropped
  - ver-first-payload-written-first
  discretion: []
- id: contract-wire-shape-conformance
  verifications:
  - ver-server-serialization-matches-golden-vector
  - ver-cli-decodes-the-same-golden-vector
  - ver-golden-vector-renaming-fails-a-gate
  discretion: []
- id: contract-identity-single-origin
  verifications:
  - ver-identity-construction-has-one-owner
  - ver-outbound-serialization-has-one-owner
  - ver-ownership-gate-fails-on-synthesized-violation
  discretion: []
- id: contract-geometry-propagation
  verifications:
  - ver-initial-geometry-is-first-frame
  - ver-geometry-clamped-into-range
  - ver-resize-burst-coalesces-to-latest
  discretion: []
- id: contract-exit-classification
  verifications:
  - ver-departure-classes-follow-the-published-reason
  - ver-exit-codes-match-the-fixed-assignment
  - ver-tty-and-non-tty-identical
  - ver-credential-never-reaches-diagnostics
  discretion: []
- id: contract-transport-close-taxonomy
  verifications:
  - ver-cli-maps-each-close-reason-to-its-exit-class
  discretion: []
- id: contract-protocol-admission
  verifications:
  - ver-mismatch-blocks-operational-work
  - ver-mismatch-does-not-disable-peers
  - ver-version-projection-registered
  discretion: []
- id: contract-config-and-fleet-identity
  verifications:
  - ver-default-execution-never-infers-a-create-target
  discretion: []
- id: contract-config-entry-admission
  verifications:
  - ver-invalid-entry-is-a-third-vocabulary-in-discovery
  - ver-invalid-entry-is-never-connected-to
  discretion: []
- id: contract-server-naming-prerequisite
  verifications:
  - ver-qualifier-precedence-unusable-outranks-ambiguous
  discretion: []
- id: contract-discovery-projection
  verifications:
  - ver-rows-carry-owning-server
  - ver-json-separates-unreachable-from-empty
  - ver-display-name-is-verbatim-card-title
  - ver-cli-never-drops-a-published-session
  discretion:
  - discretion-table-rendering
- id: contract-status-vocabulary
  verifications:
  - ver-matrix-copy-matches-canonical
  - ver-precedence-collapses-like-existing-clients
  discretion: []
- id: contract-selector-resolution
  verifications:
  - ver-declared-kind-decides-resolution
  - ver-qualifier-narrows-cross-server-collision
  - ver-unreachable-only-match-is-transport
  - ver-failure-paths-have-no-side-effect
  discretion: []
- id: contract-create-and-attach
  verifications:
  - ver-create-outcomes-are-distinct-observations
  - ver-create-requires-explicit-server
  - ver-unresolvable-agent-creates-no-row
  - ver-attach-follows-created-identifier
  discretion: []
- id: contract-explicit-terminate
  verifications:
  - ver-terminate-affects-only-target
  - ver-rejected-terminate-exits-nonzero
  - ver-terminate-report-claims-only-acknowledgement
  - ver-terminate-is-the-only-emitter
  discretion: []
- id: contract-module-boundary
  verifications:
  - ver-no-services-module-edge
  - ver-cli-registered-in-gates
  discretion:
  - discretion-flag-parsing-implementation
- id: contract-external-dependency-registration
  verifications:
  - ver-dependency-triple-declared
  - ver-tty-seam-drives-all-exit-paths
  discretion:
  - discretion-fake-gateway-internals
adrs:
- adr-cli-independent-go-module
- adr-cli-attach-via-lifecycle-multiplexed-ws
- adr-cli-raw-mode-critical-section
- adr-cli-status-display-matrix-cli-pinning
- adr-cli-selector-kinds-reduced-to-id-and-prefix
- adr-cli-failure-class-exit-codes
- adr-cli-sessioninfo-golden-vector-conformance
decision_dispositions:
- decision_input_ref: candidate-cross-platform-terminal-runtime
  disposition: adopted_as_module_and_platform_adapter_shape
  adr_refs:
  - adr-cli-independent-go-module
  - adr-cli-raw-mode-critical-section
  contract_refs:
  - contract-module-boundary
  - contract-terminal-mode-lifecycle
- decision_input_ref: candidate-existing-public-wire-contract-reuse
  disposition: adopted_and_narrowed_to_specific_public_routes
  adr_refs:
  - adr-cli-attach-via-lifecycle-multiplexed-ws
  - adr-cli-sessioninfo-golden-vector-conformance
  contract_refs:
  - contract-attach-transport-topology
  - contract-create-and-attach
  - contract-explicit-terminate
  - contract-wire-shape-conformance
- decision_input_ref: dh-ctrl-bracket-no-collision
  disposition: kept_provisional_routed_to_spike
  contract_refs:
  - contract-detach-boundary
- decision_input_ref: dh-stable-name-field-unknown
  disposition: resolved_by_measurement_no_stable_name_field_exists
  adr_refs:
  - adr-cli-selector-kinds-reduced-to-id-and-prefix
  contract_refs:
  - contract-selector-resolution
- decision_input_ref: dh-geometry-divergence
  disposition: left_with_execution_plane_with_cli_overwrite_risk_recorded
  contract_refs:
  - contract-geometry-propagation
- decision_input_ref: esc-no-presence-field
  disposition: confirmed_by_measurement_and_adopted_as_display_prohibition
  contract_refs:
  - contract-attach-transport-topology
- decision_input_ref: esc-status-vocabulary
  disposition: adopted_and_pinned_for_the_cli_consumer_only
  adr_refs:
  - adr-cli-status-display-matrix-cli-pinning
  contract_refs:
  - contract-status-vocabulary
- decision_input_ref: esc-sessioninfo-fields
  disposition: adopted_as_column_bound_with_display_name_pinned_to_card_title_and_shape_gated_by_golden_vector
  adr_refs:
  - adr-cli-sessioninfo-golden-vector-conformance
  contract_refs:
  - contract-discovery-projection
  - contract-wire-shape-conformance
- decision_input_ref: handoff-cli-binary-canonical-name
  disposition: carried_unresolved_packaging_decision_with_recorded_name_collision_boundary_question
- decision_input_ref: handoff-shared-pty-geometry-resolution
  disposition: carried_unresolved_owned_by_execution_plane
  contract_refs:
  - contract-geometry-propagation
- decision_input_ref: handoff-exit-code-numeric-assignment
  disposition: closed_by_design_with_a_fixed_numeric_assignment
  adr_refs:
  - adr-cli-failure-class-exit-codes
  contract_refs:
  - contract-exit-classification
- decision_input_ref: handoff-input-transport-batching
  disposition: partially_taken_boundary_rule_contracted_granularity_delegated
  contract_refs:
  - contract-input-passthrough
- decision_input_ref: handoff-ambiguous-candidate-breadth
  disposition: carried_unresolved_escalated_to_user
  contract_refs:
  - contract-selector-resolution
- decision_input_ref: evidence-windows-console-behavior
  disposition: evidence_seeking_routed_to_spike
  contract_refs:
  - contract-terminal-mode-lifecycle
- decision_input_ref: evidence-windows-resize-source
  disposition: evidence_seeking_routed_to_spike
  contract_refs:
  - contract-geometry-propagation
- decision_input_ref: evidence-detach-key-chord-collision
  disposition: evidence_seeking_routed_to_spike
  contract_refs:
  - contract-detach-boundary
- decision_input_ref: evidence-session-end-frame-shape
  disposition: evidence_seeking_with_a_recorded_negative_branch_that_falls_to_the_retention_prerequisite
  adr_refs:
  - adr-cli-session-terminal-state-retention-prerequisite
  contract_refs:
  - contract-exit-classification
  - contract-session-terminal-state-retention
- decision_input_ref: evidence-create-request-vocabulary
  disposition: narrowed_to_the_parameter_map_profile_source_located_at_session_config
  contract_refs:
  - contract-create-and-attach
- decision_input_ref: evidence-cwd-worktree-field
  disposition: evidence_seeking_routed_to_phase0_column_choice_deferred_to_the_observation
  contract_refs:
  - contract-discovery-projection
- decision_input_ref: open-resource-exhausted-failure-class
  disposition: interim_reason_string_replaced_by_the_published_close_reason_dedicated_code_declined_budget_ownership_left_open
  adr_refs:
  - adr-cli-failure-class-exit-codes
  - adr-transport-close-reason-taxonomy
  contract_refs:
  - contract-attach-transport-topology
  - contract-exit-classification
  - contract-transport-close-taxonomy
- decision_input_ref: open-ambiguous-candidate-breadth
  disposition: unresolved_escalated_to_user
  contract_refs:
  - contract-selector-resolution
- decision_input_ref: open-model-column-premise-falsified
  disposition: returned_to_requirements_design_takes_no_column_decision
  contract_refs:
  - contract-discovery-projection
- decision_input_ref: open-status-matrix-cross-language-drift
  disposition: recorded_out_of_scope_ts_and_csharp_copies_remain_unpinned
  adr_refs:
  - adr-cli-status-display-matrix-cli-pinning
  contract_refs:
  - contract-status-vocabulary
milestones:
- id: Phase 0 public-contract pin-down
- id: CLI module skeleton, verb dispatch, and gate registration
- id: Auth bootstrap and protocol admission
- id: Fleet reachability observation
- id: Wire codec, identity ownership, and shape conformance
- id: Shared status vocabulary
- id: Discovery rendering and machine-readable output
- id: Selector resolution
- id: Platform terminal adapter
- id: Attach runtime
- id: Lifecycle commands
- id: Fake gateway and dependency registration
- id: Platform hardening and spikes
reference_algorithms: []
---

<!-- lifecycle is owned by change.md -->
# Implementation

## Content

Canonical plan: `design-plan/design.md` + `design-plan/spine.yaml`. That plan is **shared** with `change-20260814-cli-prerequisite-contracts`; it lives here, under this change, and is neither split nor copied. Both changes reference the same anchors. This member carries the CLI half: the implementation contracts, the ADRs, the test seams, the ordering, and the promotion manifest proposal. Evidence grades: **[measured]** at a named path in this repository, **[inferred]** falsifiable as stated, **[unmeasured]** routed to a spike.

### Boundary

`clients/cli` is a client of the **public wire contract**. It does not import `services/*`, does not allocate a PTY, does not spawn Claude Code or Codex, does not manage process groups, does not persist sessions, does not fan out output, and does not build a reconnect snapshot. It introduces no controller/observer/takeover concept, no input-ownership lock, no exclusive attach, no CLI-only authentication, no CLI-only port, and no CLI-only REST endpoint.

Three contract groups the CLI depends on are **not** CLI work and are no longer part of this change. They are `change-20260814-cli-prerequisite-contracts`, which this change `dependsOn`:

- execution-plane terminal-state retention (`unit-terminal-state-retention-prerequisite`);
- the published transport close reason vocabulary (`unit-transport-close-taxonomy-prerequisite`);
- the canonical client configuration contract, its single Go loader, and per-entry admission with a server naming rule (`unit-shared-goclient-config` → `unit-client-config-parity` → `unit-server-naming-contract`).

Their measured gaps, their units, their ADRs and their verifications are in that change's `implementation.md` and `verification.md`. This member does not restate them.

### Units and dependency order

Thirteen of the shared plan's eighteen units are CLI work. Phase 0 first, per the source requirement's contract-inventory rule; then the CLI in order of increasing machinery, with the terminal adapter before the attach runtime because the fake terminal is a precondition for testing every attach exit path.

| # | Unit | Depends on |
|---|---|---|
| 1 | unit-phase0-contract-pin | — |
| 2 | unit-module-skeleton | 1 |
| 3 | unit-transport-admission | 2 — **and** `unit-shared-goclient-config` in `change-20260814-cli-prerequisite-contracts` |
| 4 | unit-fleet-reachability | 3 |
| 5 | unit-wire-codec | 3 |
| 6 | unit-status-vocabulary | 2 |
| 7 | unit-discovery-render | 4, 6 |
| 8 | unit-selector | 7 |
| 9 | unit-tty-adapter | 2 |
| 10 | unit-fake-gateway | 5 |
| 11 | unit-attach-runtime | 5, 8, 9, 10 |
| 12 | unit-lifecycle-commands | 11 |
| 13 | unit-platform-hardening-spikes | 11 |

**The cross-change edge is exactly one: unit 3 needs the prerequisite change's `unit-shared-goclient-config`**, because the CLI reads the client configuration through that loader rather than through a parser of its own. Nothing else in the CLI chain is gated on the prerequisite change landing; the other prerequisite contracts change what the CLI *observes*, not whether it builds. Where a prerequisite has not landed, the CLI-side verification that reads it is the thing that stays red — see `verification.md`.

Per-unit objectives, acceptance items, output format, tool guidance and boundaries are in `design-plan/spine.yaml` under `units`; they are not restated here.

### Paths this change touches

| Unit | Paths |
|---|---|
| unit-phase0-contract-pin | `docs/changes/change-20260812-cli-terminal-client/implementation.md`, `clients/cli/internal/gateway/testdata/`, `protocol/fixtures/` |
| unit-module-skeleton | `clients/cli/go.mod`, `clients/cli/cmd/agent-grid/main.go`, `clients/cli/internal/app/root.go`, `clients/cli/internal/app/flags.go`, `clients/cli/.golangci.yml`, `Makefile`, `test-harness/profiles.json` |
| unit-transport-admission | `clients/cli/internal/gateway/client.go`, `clients/cli/internal/gateway/auth.go` |
| unit-fleet-reachability | `clients/cli/internal/fleet/fleet.go` |
| unit-wire-codec | `clients/cli/internal/gateway/wire.go`, `clients/cli/internal/gateway/identity.go`, `clients/cli/internal/gateway/wire_test.go`, `services/host/state/view/view_golden_test.go` |
| unit-status-vocabulary | `clients/cli/internal/view/status.go`, `clients/cli/internal/view/status_test.go` |
| unit-discovery-render | `clients/cli/internal/view/table.go`, `clients/cli/internal/view/json.go` |
| unit-selector | `clients/cli/internal/selector/selector.go` |
| unit-tty-adapter | `clients/cli/internal/tty/tty.go`, `tty_unix.go`, `tty_windows.go`, `tty_fake.go` |
| unit-fake-gateway | `clients/cli/internal/gateway/fakegateway/fakegateway.go`, `test-harness/dependencies.json` |
| unit-attach-runtime | `clients/cli/internal/attach/runtime.go`, `input.go`, `output.go`, `pump.go` |
| unit-lifecycle-commands | `clients/cli/internal/lifecycle/create.go`, `clients/cli/internal/lifecycle/terminate.go` |
| unit-platform-hardening-spikes | `clients/cli/internal/tty/tty_windows.go`, `docs/changes/change-20260812-cli-terminal-client/verification.md` |

`services/gorules/static_enforcement_test.go` is touched by both changes — here to register the CLI module and the SessionInfo golden vector, there to register the close-reason vocabulary. Neither change owns the file.

### Components

`comp-entry`, `comp-fleet`, `comp-transport`, `comp-wire`, `comp-selector`, `comp-view`, `comp-attach`, `comp-tty`, `comp-lifecycle`, `comp-fakegateway` (all `planned`, under `clients/cli/`), and `comp-enforcement` (`existing`: `Makefile`, `test-harness/`, `services/gorules/`, `protocol/session-status-display.json`). Their responsibilities and paths are in `design-plan/design.md` §C under the matching anchor. The four prerequisite components — `comp-config-contract`, `comp-goclient-config`, `comp-gateway-transport`, `comp-execution-plane-lifecycle` — belong to `change-20260814-cli-prerequisite-contracts`.

### Decisions recorded as ADRs

All eleven ADRs of the shared plan are `accepted`, via `consultation-20260814-cli-terminal-client-design` and its follow-up. Seven are CLI decisions and are listed below. Four — session terminal-state retention, the transport close taxonomy, the shared Go client configuration module, and the server display-name naming contract — are implemented by `change-20260814-cli-prerequisite-contracts`. Every one of the eleven carries `originatedFrom` this change, because this is where the decision was discovered; the four prerequisite ones additionally carry a `references` relation to the change that implements them.

| Plan id (design.md §E) | Document id | Decision |
|---|---|---|
| adr-cli-independent-go-module | `adr-20260814-cli-independent-go-module` | `clients/cli` is an independent Go module with no `require`/`replace` on the services module; no root `go.mod`/`go.work`. |
| adr-cli-attach-via-lifecycle-multiplexed-ws | `adr-20260814-cli-attach-via-lifecycle-multiplexed-ws` | Attach uses `GET /ws?ticket=` plus `k:"ld"`, not `GET /ws?session=`. Two independent measured counterexamples disqualify the per-session route. |
| adr-cli-raw-mode-critical-section | `adr-20260814-cli-raw-mode-critical-section` | Raw entry only after a terminal accepted subscription outcome; a single restore path; the terminal input queue is discarded before restore on detach. |
| adr-cli-status-display-matrix-cli-pinning | `adr-20260814-cli-status-display-matrix-pinning` | `protocol/session-status-display.json` becomes canonical **for the CLI consumer only**, pinned by test. The TypeScript and C# copies stay unpinned and out of scope. |
| adr-cli-selector-kinds-reduced-to-id-and-prefix | `adr-20260814-cli-selector-kinds-id-and-prefix` | Selector kinds are the full identifier and an identifier prefix — the upstream `assumption-stable-name-field-unknown` invalidation behaviour, fired by measurement. |
| adr-cli-failure-class-exit-codes | `adr-20260814-cli-failure-class-exit-codes` | `0` success, `1` internal fault, `2` usage, `3` not found, `4` ambiguous, `5` auth/compat, `6` transport. The capacity refusal is separated by the published close reason rather than by a client-authored string, and the dedicated-code question is answered no. |
| adr-cli-sessioninfo-golden-vector-conformance | `adr-20260814-sessioninfo-golden-vector-conformance` | The session payload shape the CLI reads is pinned by a golden vector that the **server's** serializer is tested against. |

### Implementation contracts

Eighteen contracts are CLI-owned. Four more bind both sides of the change boundary: they are realized by `change-20260814-cli-prerequisite-contracts` and consumed here, so this member carries only their CLI-side units and CLI-side verifications.

| Contract | Owner component | Requirements | CLI carrier units | Realized in |
|---|---|---|---|---|
| contract-attach-transport-topology | comp-attach | FR-007, FR-009, FR-014, NFR-006 | phase0-contract-pin, transport-admission, wire-codec, attach-runtime | this change |
| contract-attach-lease-progress | comp-attach | FR-013, FR-001 | attach-runtime | this change |
| contract-terminal-mode-lifecycle | comp-tty | FR-002, FR-004, FR-001 | tty-adapter, attach-runtime, platform-hardening-spikes | this change |
| contract-detach-boundary | comp-attach | FR-010, FR-011, FR-001 | attach-runtime, tty-adapter, platform-hardening-spikes | this change |
| contract-input-passthrough | comp-attach | FR-005, FR-006 | attach-runtime, wire-codec | this change |
| contract-output-passthrough | comp-attach | FR-007, FR-008, FR-009, NFR-004 | attach-runtime, wire-codec | this change |
| contract-wire-shape-conformance | comp-wire | NFR-008, FR-018 | wire-codec, phase0-contract-pin | this change |
| contract-identity-single-origin | comp-wire | NFR-007, FR-019 | wire-codec, lifecycle-commands | this change |
| contract-geometry-propagation | comp-attach | FR-012 | attach-runtime, tty-adapter, platform-hardening-spikes | this change |
| contract-exit-classification | comp-entry | FR-014, FR-024, FR-025, FR-027, FR-029, FR-032† | module-skeleton, transport-admission, selector, attach-runtime, phase0-contract-pin | this change |
| contract-protocol-admission | comp-transport | NFR-001, FR-016, NFR-005 | transport-admission, fleet-reachability, module-skeleton | this change |
| contract-discovery-projection | comp-view | FR-015, FR-016, FR-018, FR-019 | discovery-render, fleet-reachability | this change |
| contract-status-vocabulary | comp-view | FR-017 | status-vocabulary | this change |
| contract-selector-resolution | comp-selector | FR-020, FR-021, FR-024 | selector | this change |
| contract-create-and-attach | comp-lifecycle | FR-022, FR-024 | lifecycle-commands, phase0-contract-pin | this change |
| contract-explicit-terminate | comp-lifecycle | FR-001, FR-023 | lifecycle-commands | this change |
| contract-module-boundary | comp-enforcement | NFR-002, NFR-005 | module-skeleton | this change |
| contract-external-dependency-registration | comp-enforcement | NFR-005, FR-002, NFR-001 | fake-gateway | this change |
| contract-transport-close-taxonomy | comp-gateway-transport | FR-032†, FR-014, FR-024 | transport-admission, attach-runtime | prerequisite change |
| contract-config-and-fleet-identity | comp-config-contract | FR-003, FR-019, FR-021, FR-022, NFR-003 | lifecycle-commands | prerequisite change |
| contract-config-entry-admission | comp-config-contract | FR-030†, FR-028†, FR-031 | fleet-reachability, discovery-render | prerequisite change |
| contract-server-naming-prerequisite | comp-config-contract | FR-028†, FR-029, FR-021 | selector | prerequisite change |

† requirement owned by `change-20260814-cli-prerequisite-contracts`.

The decision rules, outcome partitions, operational inputs, invariants, forbidden clauses and risk-tagged witnesses for each contract are in `design-plan/design.md` §D under the matching anchor.

### What the CLI owes the four shared contracts

These are the only obligations this change takes on from the prerequisite contracts. Each is a CLI-side behaviour with its own verification here; none of them re-implements the prerequisite.

| Shared contract | CLI-side obligation | CLI verification |
|---|---|---|
| contract-transport-close-taxonomy | derive the exit class and the printed reason from the published close reason; present a departure that carries nothing as an unattributed loss rather than guessing it into a named cause | ver-cli-maps-each-close-reason-to-its-exit-class |
| contract-config-and-fleet-identity | never read `default_execution` — or the first-enabled fallback the loaders apply — as a creation target | ver-default-execution-never-infers-a-create-target |
| contract-config-entry-admission | render a configuration-invalid entry in a vocabulary disjoint from reachability and from session lifecycle state, and never probe it | ver-invalid-entry-is-a-third-vocabulary-in-discovery, ver-invalid-entry-is-never-connected-to |
| contract-server-naming-prerequisite | resolve a qualifier over every configured entry, with unusable outranking ambiguous | ver-qualifier-precedence-unusable-outranks-ambiguous |

### Test seams

- **Gateway (REST + WebSocket)** — `comp-fakegateway` drives ticket minting, admission, hello, subscription, output, capacity refusal and every disconnect shape in process. Registered in `test-harness/dependencies.json` with the full triple (fake, invariant-naming contract test, `FakeVsReal*` T3 backstop).
- **Local terminal** — `comp-tty` is the only place terminal state changes, behind one interface with a fake. [measured] The registry's existing `pty` entry is `strategy: exception` / `kind: trusted-runtime` and covers the **server-side** pty, so the client terminal needs its own entry and its own judgement.
- **Pure core** — selector resolution, status precedence, view projection, wire codec and exit-code mapping are pure functions over decoded values; they need neither a socket nor a terminal.
- **State construction** — session state is constructed only through the fake gateway's public frame sequence, never by poking internal structures.
- **Client configuration** — read through the prerequisite change's single Go loader package, imported from `clients/goclient`; the CLI adds no second parser of `servers.json`, and `ver-every-go-host-loads-through-one-owner` in the prerequisite change is what fails if it does.
- **Residue at T3** — what only a real terminal can show (echo actually disappearing and returning, Windows console behaviour, the Ctrl-] chord collision).

### Delegated to the implementer

All five `implementation_discretion` items of the shared plan sit on CLI contracts and CLI units; none moved to the prerequisite change.

| Discretion | Carrier unit | Question | Escalate when |
|---|---|---|---|
| discretion-input-read-buffering | unit-attach-runtime | stdin read buffer size and frame-coalescing granularity | the UTF-8 boundary rule would have to be relaxed; the server bounds bytes per input frame; coalescing changes observable ordering |
| discretion-pump-goroutine-topology | unit-attach-runtime | goroutine/channel arrangement of both pumps and lease renewal | renewal can only work by depending on output progress; a second restore path appears; cancellation needs a frame-vocabulary change |
| discretion-flag-parsing-implementation | unit-module-skeleton | flag and subcommand parsing mechanism | the mechanism decides the usage exit code itself; it treats a non-verb first argument as a target; it adds an unaccepted runtime dependency |
| discretion-table-rendering | unit-discovery-render | column widths, alignment, colouring (**not** truncation — FR-018 forbids it; and **not** the column set — that is a requirements-side decision) | a display name cannot be laid out without truncation; colouring would change non-TTY content; a column would have to be dropped |
| discretion-fake-gateway-internals | unit-fake-gateway | real listener versus in-process handlers, and how frames are driven | the fake would need a frame the real gateway does not produce; a fidelity test tempts an assertion change |

### Evidence to acquire before the dependent choice

All six evidence items are acquired in this change.

| Item | Blocks | Resolved by |
|---|---|---|
| evidence-create-request-vocabulary | the `parameters` mapping in creation (profile source itself is measured: `GET /api/session-config` with `ID` + `Revision`) | unit-phase0-contract-pin |
| evidence-session-end-frame-shape | the shape of departure class (b); a negative result no longer degrades the contract, since the prerequisite change's retention unit publishes the transition either way | unit-phase0-contract-pin |
| evidence-cwd-worktree-field | the working-directory column choice among `project` / `workspace` / `workspace_root` | unit-phase0-contract-pin |
| evidence-windows-console-behavior | the Windows terminal adapter | unit-platform-hardening-spikes |
| evidence-windows-resize-source | the Windows size-change event source | unit-platform-hardening-spikes |
| evidence-detach-key-chord-collision | the default detach key | unit-platform-hardening-spikes |

### Phase 0 observations (unit-phase0-contract-pin, acquired)

Every fact below is a capture from a production handler, encoder or serializer
in this repository, not a paraphrase of one. The captures are checked in and are
re-derived on every `go test ./...`, so a server-side change to any of these
shapes fails here rather than in a client at runtime.

| Fixture | Produced by | Answers |
|---|---|---|
| `protocol/fixtures/session-info-golden.json` | `proto.SessionInfo` serializer, byte-exact | the session payload shape the CLI decodes |
| `clients/cli/internal/gateway/testdata/session-config.json` | `handleSessionConfig` | evidence-create-request-vocabulary |
| `clients/cli/internal/gateway/testdata/create-session-request.json` | admitted by `decodeCreateRequest` | the creation body |
| `clients/cli/internal/gateway/testdata/create-session-response.json` | `apiSessionInfo` serializer | what a creation returns |
| `clients/cli/internal/gateway/testdata/lifecycle-session-end.json` | `encodeFromSessionsChanged` | evidence-session-end-frame-shape |
| `clients/cli/internal/gateway/testdata/departure-capacity.json` | `departureControlFrame` ∘ `departureReason` | the capacity refusal the CLI must classify |
| `clients/cli/internal/gateway/testdata/rejection-unauthenticated.json` | `writeTransportRejection` | the pre-upgrade rejection body |

The golden vector is generated with `go test ./host/state/view/ -run TestSessionInfoGoldenVectorIsStable -update-session-info-golden`; the six CLI fixtures with `go test ./server/api/ -run TestCLIPhase0Fixtures -update-cli-phase0`. Both then assert equality without the flag.

**evidence-create-request-vocabulary — resolved.** The agent kind is a
`command_profiles[]` entry addressed by `id` plus its server-computed
`revision`; the working directory is `projects[].path`, an absolute directory
the server resolved and existence-checked. A creation therefore sends
`{version: 2, project, profile_id, profile_revision, parameters}` — `parameters`
being the validated map the profile's `parameters[]` declares — and the server
materialises the argv. The revision is server-validated at request time and a
stale one is a 409 `stale_profile_revision`, which is why unit-lifecycle-commands
reads the profile set from the target server per invocation instead of caching it.

**evidence-session-end-frame-shape — resolved, positively.** A session that ends
server-side arrives as an ordinary view update, `{"k":"v","sessions":[…]}`, in
which the row is still present and carries `status.lifecycle == "stopped"`. The
recorded negative branch does not fire: the prerequisite change's retention unit
makes the row survive, so the CLI attributes a departure to the session ending
by reading a published transition rather than by watching a row vanish or by
keeping private memory of what it once saw.

**evidence-cwd-worktree-field — resolved.** The shipping SPA presents
`project` as the working directory: `groupSessionsByProject`
(`clients/ui/src/store/daemon.ts:181`) keys on `s.project`, and the group header
shows its basename with the full path in the `title` attribute
(`clients/ui/src/components/SessionList.tsx:228`). `workspace_root` is the
worktree root and is a different fact; `workspace` is the workspace-switcher
bucket. The cwd column is therefore `project`, and a session created with
`--worktree` additionally carries `workspace_root`.

**Command-to-operation mapping.** Every CLI command is a client of an operation
that already exists. No row needs a server change.

| Command | Public operations | Gap |
|---|---|---|
| `ls` | `GET /api/capabilities` (version), `POST /api/ws-ticket`, `GET /ws?ticket=` → `k:"h"` then `k:"v"` | none — client-only |
| `attach` | the same admission, then inbound `k:"ld"` with `correlation`+`sessionId`+`cols`/`rows`+`desired`, outbound `k:"lo"` outcome and asciicast output arrays, inbound `k:"i"`/`k:"r"` | none — client-only |
| `new` | `GET /api/session-config`, `POST /api/sessions`, then the attach path | none — client-only |
| `kill` | `DELETE /api/sessions/{id}` | none — client-only |

The lifecycle socket is the only attach transport: `GET /ws` **without**
`?session=` (`serveAttach`, `services/server/api/mux.go:621`). The per-session
route shares one empty `SubscriberID` across clients, which is the measured
disqualification recorded in `adr-20260814-cli-attach-via-lifecycle-multiplexed-ws`.

**Lease shape.** `k:"ld"` carries `correlation.{clientInstanceID,connectionGeneration,clientRevision}`
(`services/host/proto/lifecycle.go:26`). `clientRevision` is the lease revision the
CLI must advance monotonically; the daemon answers with `k:"lo"` whose `status`
is one of `accepted`, `waiting`, `applied`, `rejected`, `superseded`, `released`,
`degraded`, `no_output` — of which `accepted` and `waiting` are explicitly
non-terminal, which is what `ver-attach-never-reports-accepted-as-established`
pins.

**Reachability vocabulary.** The observation kinds are `disabled`, `checking`,
`connected`, `unavailable`, `incompatible_protocol`, `health_unavailable`, and
the reasons are `connection_timeout`, `connection_failed`, `http_error`,
`websocket_failed`, `empty_ticket`, `protocol_mismatch`, `refresh_failed`,
`delivery_lost` (`clients/web-host/internal/webhost/fleet.go:1046`). unit-fleet-reachability
reuses them verbatim and adds `configuration_invalid` as a further non-reachability
admission kind alongside `disabled`.

### Decision dispositions

The complete disposition ledger for the shared plan is `design-plan/spine.yaml` `dispositions`; it is the single source and is not duplicated. The rows below are the ones whose ADR or contract lands here. Ten further dispositions — the retention, close-taxonomy, configuration-contract and naming ones — are carried by `change-20260814-cli-prerequisite-contracts/implementation.md`.

| Input | Disposition |
|---|---|
| candidate-cross-platform-terminal-runtime | adopted as the module shape and the platform adapter shape |
| candidate-existing-public-wire-contract-reuse | adopted and narrowed to specific public routes (lifecycle socket, REST create, REST delete, session-config) |
| dh-ctrl-bracket-no-collision | kept provisional, routed to spike; the escape hatch is already contracted |
| dh-stable-name-field-unknown | resolved by measurement — no stable name field exists |
| dh-geometry-divergence | left with the execution plane; the CLI's overwrite capability is recorded |
| esc-no-presence-field | confirmed by measurement; adopted as a display prohibition |
| esc-status-vocabulary | adopted, pinned for the CLI consumer only |
| esc-sessioninfo-fields | adopted as the column bound; display name pinned to `view.card.title`; shape gated by a golden vector |
| handoff-cli-binary-canonical-name | **resolved at packaging**: `agent-grid`, no installed alias (`adr-20260814-cli-canonical-binary-name`) |
| handoff-shared-pty-geometry-resolution | carried unresolved; owned by the execution plane |
| handoff-exit-code-numeric-assignment | **closed by design** with a fixed numeric assignment |
| handoff-input-transport-batching | partially taken — the boundary rule is contracted, the granularity delegated |
| handoff-ambiguous-candidate-breadth | carried unresolved; escalated to the user |
| evidence-windows-console-behavior / -resize-source / -detach-key-chord-collision | evidence-seeking, routed to spike |
| evidence-session-end-frame-shape | evidence-seeking with a recorded negative branch that falls to the prerequisite change's retention unit |
| evidence-create-request-vocabulary | narrowed to the parameter map; the profile source is located |
| evidence-cwd-worktree-field | evidence-seeking; the column choice is deferred to the observation |
| open-resource-exhausted-failure-class | interim client-authored reason string replaced by the published close reason from the prerequisite change; dedicated exit code declined; budget ownership left open |
| open-ambiguous-candidate-breadth | unresolved; escalated to the user |
| open-model-column-premise-falsified | returned to requirements; design takes no column decision |
| open-status-matrix-cross-language-drift | recorded out of scope; TypeScript and C# copies remain unpinned |

`candidate-fleet-session-identity-reuse` is split across the two changes: the configuration half (canonical URL, derived server id) is carried by the prerequisite change, and the wire and single-construction-owner half is `contract-identity-single-origin` here.

### Promotion manifest (applied 2026-08-14)

Applied to `change.md`. `design-clients-cli` now exists at `docs/design/design-clients-cli.md`. The configuration-contract, execution-plane and gateway entries moved to `change-20260814-cli-prerequisite-contracts/implementation.md`.

| Action | Design | Reason |
|---|---|---|
| upsert | `design-clients-cli` (new, component) | New client host. Responsibility: a terminal client of the public wire contract. Boundary: `clients/cli/`. Forbidden: any `services/*` dependency, PTY ownership, ownership/takeover control, a replay store. Capability: terminal session attach presentation. |
| none | `design-clients-ui`, `design-workspace-desktop` | Untouched by this change. The Workspace design's reference to "the CLI" as a paired consumer of its control endpoint is a recorded boundary question on the binary name (`handoff-cli-binary-canonical-name`), not a change to that design. |

Governing relation for the change: `conformsTo` `design-host` (id `design-client`) is **not** correct for the CLI — its `source_paths` are `services/host/`, `services/server/api/`, `services/cmd/server/`, `clients/web-host/` and do not govern clients in general [measured]. `design-clients-cli` was created for that reason, and `change.md` now carries `conformsTo design-clients-cli`.
