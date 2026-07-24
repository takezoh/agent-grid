---
change: change-20260723-native-clients-phase01
role: implementation
---

# Implementation

## Approach

Ground each component in already-existing repo surface: host/state's FC/IS core (`src/host/state/reduce_*.go`), host/proto's sum types (`src/host/proto/event.go`), server/api's stateless proxy (`src/server/api/wire.go`, `ticket.go`, `auth.go`), the already-scaffolded protocol/README.md + contracts/README.md file lists, and the existing test-harness profile mechanism (`scripts/run-verification-profile.sh` + `test-harness/profiles.json`). Phase 0 replaces the synchronous auto-accept in `src/host/runtime/subsystem/stream/event.go handleRequest` with a durable ApprovalRequest/QuestionRequest domain in host/state, adds a per-WS-connection ephemeral client-instance-id (extended into ticketStore) so `decided_by` has a named producer, closes the lifecycle with pending -> {resolved, expired, cancelled} including frame-teardown drain, pins two-client conflict to first-writer-wins under the existing single-writer Reduce loop, and constrains expiry policy to deny-by-default with no agent-side TTL extension. Phase 1 treats `protocol/*.schema.json` as the single message-type SoT with `openapi.yaml` as the REST-binding declaration (adr-20260724-protocol-message-schema-sot-rest-binding) — REST remains the carrier for bulk reads/bootstrap/commands while WS carries the event stream, and a future DataChannel transport binds the same types; typed C#/Swift/Kotlin/TS models are generated from the schemas via quicktype pinned in the npm lockfile with per-SDK thin transport hand-written (recorded as an ADR superseding ADR-0021 for cross-language only; Go stays stdlib-only), stands up a three-part simulator (fixture + recorded stream + sim server) under `protocol/simulator/` as a governed extension of plan-20260723-repo-structure.md's planned-file table, and adds a new `compatibility` test-harness profile that fails closed on undeclared SDK surface usage. Deep-link URI shape adopts the analysis in plans/remote-control-mobile-session-deep-link.md verbatim; auth trust-boundary keeps same-host bearer+ticket unchanged and defers cross-host answering to multi-host-gateway.md's user-signed-op chain.

## Components

### `component-approval-question-state-domain` — Approval/Question state domain (host/state)

_Responsibility_: Own the durable, single-writer ApprovalRequest and QuestionRequest lifecycle (pending -> resolved | expired | cancelled) as part of host/state's FC/IS core, replacing the current synchronous auto-decide path in the stream subsystem backend and reaping pending state on frame/session teardown.

- **kind**: existing
- **owner**: host layer maintainers (src/host/state + src/host/runtime/subsystem/stream)
- **paths**: `src/host/state/driver_iface.go`, `src/host/state/event.go`, `src/host/state/reduce_event.go`, `src/host/state/reduce_messages.go`, `src/host/runtime/subsystem/stream/event.go`, `src/host/runtime/subsystem/stream/helpers.go`
- **integration points**:
    - SubsystemApprovalRequested/SubsystemApprovalResolved dispatch in driver_iface.go
    - Backend.handleRequest in host/runtime/subsystem/stream/event.go (currently synchronously replies + resolves in one call — must be split into hold + separate CmdApprovalRespond commit)
    - reduce_event.go Reduce dispatch table (new Cmd/DEv cases for approval/question)
    - existing sandbox release path (d1e3a8c4) as the frame-teardown reap seam
- **test seams**: `src/host/state/reduce_event_test.go`, `src/host/state/reduce_fuzz_test.go`, `src/host/state/reduce_messages_test.go`, `driver conformance registry suite (adr-20260705-driver-conformance-registry-suite)`
- **accepted ADR refs**: `adr-20260706-frame-messaging-daemon-broker`, `adr-20260724-approval-question-state-domain-in-host-state`, `adr-20260724-approval-expiry-deny-default-no-extension`, `adr-20260724-approval-lifecycle-teardown-cancel`, `adr-20260724-approval-single-writer-first-commit`
- **implements contracts**: `contract-approval-question-lifecycle`, `contract-approval-resolution-single-writer`

_Grounding rationale_: Verified by reading host/runtime/subsystem/stream/event.go directly: handleRequest for MethodItemCommandExecutionRequestApproval/MethodItemFileChangeRequestApproval calls b.conn.Reply(id, result) and emits SubsystemApprovalResolved in the same function body as SubsystemApprovalRequested — no held pending state exists a human client could observe before resolution. This is the code-level evidence for plan gap #1.

### `component-approval-question-proto-wire` — Approval/Question IPC wire types (host/proto)

_Responsibility_: Define the Go-side ServerEvent/Response sum-type members (EvtApprovalRequested/Resolved, EvtQuestionRequested/Resolved, CmdApprovalRespond, CmdApprovalCancel, CmdQuestionRespond, CmdQuestionCancel, RespApprovalRequests) that carry the state domain across the daemon's Unix-socket IPC boundary and satisfy ADR-0023's pinned discriminated-union broadcast shape.

- **kind**: existing
- **owner**: host/proto maintainers (IPC wire layer)
- **paths**: `src/host/proto/event.go`, `src/host/proto/response.go`
- **integration points**:
    - ServerEvent interface (isEvent/EventName) — new EvtApproval*/EvtQuestion* cases
    - Response interface (isResponse) — new RespApprovalRequests/RespQuestionRequests for the resubscribe payload
    - ADR-0023 pinned viewUpdate frame co-existence: new Evt* cases must reuse the same discriminated-union k=... convention
- **test seams**: `host/proto wire round-trip tests`, `adr-20260705-wire-fixtures-pipeline generated-fixture regression`
- **accepted ADR refs**: `adr-20260705-wire-fixtures-pipeline`, `adr-20260624-0023-view-update-broadcast-shape`, `adr-20260705-view-update-sessions-only`, `adr-20260724-approval-broadcast-coexists-view-update`
- **implements contracts**: `contract-approval-question-envelope`
- **depends on**: `component-approval-question-state-domain`

_Grounding rationale_: event.go's ServerEvent sum type (EvtSessionsChanged, EvtAgentNotification, ...) and response.go's Response sum type are the existing pattern every new wire type must extend verbatim.

### `component-server-api-approval-gateway` — Approval/Question HTTP/WS gateway surface (server/api)

_Responsibility_: Translate host/proto's approval/question Evt*/Resp* types into browser/native-facing WS frames and REST endpoints, staying a stateless proxy per ARCHITECTURE.md, and thread the per-WS-connection client-instance-id (from the extended ticketStore) onto every CmdApprovalRespond/CmdQuestionRespond originating from that connection.

- **kind**: existing
- **owner**: server/api gateway maintainers
- **paths**: `src/server/api/wire.go`, `src/server/api/gateway.go`, `src/server/api/mux.go`, `src/server/api/viewupdate.go`, `src/server/api/ticket.go`, `src/server/api/auth.go`
- **integration points**:
    - encodeServerEvent switch in wire.go — add approval/question cases satisfying ADR-0023 shape
    - mux.go REST route registration pattern — new /api/sessions/{id}/approvals + POST for CmdApprovalRespond
    - gateway.go per-WebSocket surface subscription — thread client-instance-id onto CmdApprovalRespond/CmdQuestionRespond
    - ticket.go ticketStore.mint — extended to also mint client-instance-id and bind it to the ticket for the WS's lifetime
- **test seams**: `src/server/api/mux_scenario_test.go`, `src/server/api/gateway_view_update_test.go`, `src/server/api/wire_fixtures_test.go`, `src/server/api/ticket_test.go`, `src/server/api/auth_test.go`
- **accepted ADR refs**: `adr-20260624-0016-depguard-server-layer-rule`, `adr-20260624-0011-two-step-ws-close-on-daemon-disconnect`, `adr-20260624-0023-view-update-broadcast-shape`, `adr-20260705-view-update-sessions-only`, `adr-20260724-approval-answerer-identity-per-ws-instance`, `adr-20260724-approval-broadcast-coexists-view-update`
- **implements contracts**: `contract-approval-question-envelope`, `contract-reconnect-authoritative-resubscribe`
- **depends on**: `component-approval-question-proto-wire`, `component-auth-trust-boundary`

_Grounding rationale_: wire.go was read directly: encodeServerEvent's exhaustive type switch and controlMsg/viewUpdateFrame/notificationFrame patterns are the exact place new approval/question frame encoders must be added, following the same {k:...} discriminated-union convention already used for tt/et/n/v frames. ticket.go was read directly: ticketStore already mints an opaque 24-byte token — extending it to mint (token, client-instance-id) is additive and does not modify the auth.go bearer scheme.

### `component-reconnect-event-model-extension` — Reconnect / resubscribe event-model extension

_Responsibility_: Extend the existing REST-backfill + WS-tail + two-step-close + subscribe-retry reconnect stack (ADR-0025/0011/0022) with the minimum addition needed for a reconnecting client to receive authoritative pending-approval/question state, without rebuilding it as a general event-sourced replay log.

- **kind**: existing
- **owner**: server/api gateway maintainers + host/proto
- **paths**: `docs/adr/adr-20260624-0025-transcript-rest-backfill-then-ws-tail.md`, `docs/adr/adr-20260624-0011-two-step-ws-close-on-daemon-disconnect.md`, `docs/adr/adr-20260624-0022-subscribe-retry-in-socket-layer.md`, `src/server/api/gateway.go`, `src/server/api/transcript.go`
- **integration points**:
    - gateway.go daemon-disconnect two-step close path (ADR-0011)
    - subscribe/resubscribe request handling that browser socket's subscribeWithRetry (ADR-0022) drives
    - hello/initial-state payload extended with pending ApprovalRequest/QuestionRequest per FR-P0-08
- **test seams**: `src/server/api/gateway_inbound_test.go`, `src/server/api/gateway_terminal_test.go`, `clients/ui/src/wire/codec.test.ts`
- **accepted ADR refs**: `adr-20260624-0025-transcript-rest-backfill-then-ws-tail`, `adr-20260624-0011-two-step-ws-close-on-daemon-disconnect`, `adr-20260624-0022-subscribe-retry-in-socket-layer`
- **implements contracts**: `contract-reconnect-authoritative-resubscribe`
- **depends on**: `component-approval-question-proto-wire`, `component-server-api-approval-gateway`

_Grounding rationale_: The three cited ADRs were read in full: ADR-0025 separates REST backfill from WS tail, ADR-0011 defines the two-step close, ADR-0022 puts subscribe retry in the socket layer. None of the three currently carries a notion of 'authoritative pending approval on resubscribe' — this is additive, matching FR-P1-11's scoping requirement.

### `component-protocol-schema-repo` — protocol/ schema source of truth

_Responsibility_: Hold the versioned, tool-consumable wire contracts — the message-type schemas (events/commands/capabilities/deep-links/notifications.schema.json) as the normative SoT plus openapi.yaml as the REST-binding declaration — that every generated SDK and the hand-written Go/TS wire types round-trip-validate against.

- **kind**: existing
- **owner**: contract layer maintainers (new cross-cutting ownership)
- **paths**: `protocol/README.md`
- **integration points**:
    - protocol/README.md already-declared planned file list
    - SDK generation pipeline (component-sdk-generation-pipeline) consumes these as generator input
    - src/server/api/wire.go and src/host/proto/* become schema-validated outputs once populated
- **test seams**: `new schema-validation test analogous to src/server/api/wire_fixtures_test.go, extended to check protocol/*.schema.json against Go/TS wire fixtures`, `the new compatibility CI profile group`
- **accepted ADR refs**: `adr-20260624-0021-frontend-wire-types-hand-written`, `adr-20260724-protocol-cross-language-sdks-supersedes-0021`
- **implements contracts**: `contract-approval-question-envelope`, `contract-sdk-generation-determinism`, `contract-deep-link-uri-shape`
- **depends on**: `component-approval-question-proto-wire`

_Grounding rationale_: protocol/README.md already exists (created by plan-20260723-repo-structure.md) and states verbatim: 'ここが唯一の正本. 生成コードは各消費者の木に置き、手編集しない.' This design fills that already-declared contract rather than re-deciding it.

### `component-behavior-contracts-repo` — contracts/ behavior contract documents

_Responsibility_: Hold the behavior contracts schemas cannot express (approval-contract.md, question-contract.md, reconnect-contract.md, command-acknowledgement.md, notification-policy.md, handoff-contract.md, compatibility-policy.md) that all clients and the simulator's recorded scenarios treat as the common specification.

- **kind**: existing
- **owner**: contract layer maintainers
- **paths**: `contracts/README.md`
- **integration points**:
    - contracts/README.md already-declared planned file list, including approval-contract.md and question-contract.md
    - referenced by the new ADR superseding ADR-0021
    - referenced by component-simulator's recorded scenarios
- **test seams**: `dev-docs conformance checks (docs-skill managed record validation)`, `cross-reference from src/server/api tests' doc comments back to these contracts`
- **implements contracts**: `contract-capability-negotiation-policy`, `contract-auth-trust-boundary-approval-answering`
- **depends on**: `component-approval-question-state-domain`, `component-protocol-schema-repo`

_Grounding rationale_: contracts/README.md already exists with the exact seven planned filenames the plan's Shared contracts section lists, and states 'クライアント実装の都合で契約を曲げない — 契約が先、実装が後'.

### `component-sdk-generation-pipeline` — Generated C#/Swift/Kotlin/TypeScript SDK pipeline

_Responsibility_: Generate typed models + serialization for four target languages from protocol/*.schema.json via pinned quicktype, deterministically, and own the hand-written per-language thin transport (REST calls per openapi.yaml, WS framing, reconnect) — with the Go-side output (if any) staying stdlib-only.

- **kind**: planned
- **owner**: contract layer maintainers, consumed by per-platform teams
- **paths**: `clients/sdk/csharp`, `clients/sdk/swift`, `clients/sdk/kotlin`, `clients/sdk/ts`, `protocol/README.md`, `plans/plan-20260723-repo-structure.md`
- **integration points**:
    - protocol/*.schema.json as quicktype input; openapi.yaml as the REST-binding reference for the hand-written transport
    - clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/ as first C# consumer (plan-20260723-windows-shell-design.md §3.1)
    - clients/ui/src/wire/* as existing hand-written TS precedent this may incrementally replace
- **test seams**: `per-language generated-client unit tests driven against component-simulator`, `compatibility CI profile group`
- **accepted ADR refs**: `adr-20260624-0021-frontend-wire-types-hand-written`, `adr-20260724-protocol-cross-language-sdks-supersedes-0021`, `adr-20260724-protocol-message-schema-sot-rest-binding`, `adr-20260724-sdk-codegen-quicktype-typegen`
- **implements contracts**: `contract-sdk-generation-determinism`, `contract-adr0021-supersede-stdlib-preservation`
- **depends on**: `component-protocol-schema-repo`

_Grounding rationale_: No generated-SDK directory exists yet; clients/windows-shell/AgentGrid.Shell.Core/GatewayClient/ is explicitly named in plan-20260723-windows-shell-design.md §3.1 as 'the generated C# client の薄い所有' — confirming this is a planned, plan-referenced consumer.

### `component-capability-negotiation` — Capability negotiation / compatibility policy

_Responsibility_: Define and wire-declare the bundled-vs-remote two-axis compatibility policy: bundled clients (same-build daemon) skip per-capability negotiation; remote/version-skewed clients negotiate via capabilities.schema.json and degrade gracefully.

- **kind**: planned
- **owner**: contract layer maintainers, jointly with plan-20260723-windows-shell-design.md DaemonSupervisor
- **paths**: `contracts/README.md`, `protocol/README.md`, `src/server/api/auth.go`
- **integration points**:
    - capabilities.schema.json (planned)
    - compatibility-policy.md (planned)
    - plan-20260723-windows-shell-design.md §3.6 DaemonSupervisor adopt/spawn/health state machine
- **test seams**: `contract test exercising a fixed-old-version client against a fixed-new-version daemon (and vice versa) via component-simulator`, `compatibility CI profile group`
- **accepted ADR refs**: `adr-20260724-capability-negotiation-bundled-remote-two-axis`
- **implements contracts**: `contract-capability-negotiation-policy`
- **depends on**: `component-protocol-schema-repo`, `component-behavior-contracts-repo`

_Grounding rationale_: The plan's own Strategic decisions #5 states verbatim: 'Version skew between shell and daemon still makes compatibility-policy.md a day-one need'.

### `component-simulator` — Simulator (fixtures + recorded stream + simulation server)

_Responsibility_: Let every generated SDK be driven end-to-end against protocol/'s REST+WS surface without a live agent, by deterministically replaying a version-controlled recorded event stream.

- **kind**: planned
- **owner**: contract layer maintainers
- **paths**: `protocol/README.md`, `plans/plan-20260723-repo-structure.md`
- **integration points**:
    - protocol/simulator/ subdirectory is a governed extension of plan-20260723-repo-structure.md's planned-file table, recorded as adr-20260724-simulator-under-protocol-directory
    - protocol/*.schema.json as the surface the sim server serves
    - src/platform/agent/fakecodex/ recorded rollout .jsonl files as a precedent for what a 'recorded session' looks like in this repo
    - component-sdk-generation-pipeline's per-language test suites as the sim server's consumers
- **test seams**: `sim-server-driven per-SDK e2e suites`, `Go-side fake-agent scenario (src/server/api/testsupport/fakeagents pattern) as source of a fresh recorded scenario`
- **accepted ADR refs**: `adr-20260705-wire-fixtures-pipeline`, `adr-20260724-simulator-under-protocol-directory`
- **implements contracts**: `contract-simulator-recorded-scenario-replay`
- **depends on**: `component-protocol-schema-repo`, `component-behavior-contracts-repo`

_Grounding rationale_: protocol/README.md was read directly and does not currently list simulator/; adr-20260724-simulator-under-protocol-directory records the extension so kind=planned does not contradict the scaffold decision. src/platform/agent/fakecodex/ already contains real recorded rollout .jsonl session files, showing recorded-session capture is an established pattern in this repo.

### `component-compatibility-ci-gate` — Compatibility CI gate (protocol drift / undocumented-SDK-behavior rejection)

_Responsibility_: Fail CI when a generated SDK invokes wire surface not declared in protocol/*, or when a new SDK target is added without passing the recorded-scenario suite — operationalizing plan's 'Protocol drift' and 'Excessive parallel scope' risk mitigations as an enforced gate.

- **kind**: existing
- **owner**: test-harness / CI maintainers
- **paths**: `.github/workflows/ci.yml`, `scripts/run-verification-profile.sh`, `test-harness/profiles.json`
- **integration points**:
    - ci.yml's existing schema-drift job — new 'compatibility' job following the same pattern
    - test-harness/profiles.json profile/group/command schema
    - AG_HARNESS_PROFILE_ARTIFACT convention for machine-readable evidence
- **test seams**: `test-harness/profiles.json (new 'compatibility' group entries)`, `scripts/run-verification-profile.sh existing pass/fail/skip semantics, unmodified`
- **accepted ADR refs**: `adr-20260705-test-tier-taxonomy`
- **implements contracts**: `contract-compatibility-ci-drift-gate`
- **depends on**: `component-protocol-schema-repo`, `component-sdk-generation-pipeline`, `component-simulator`

_Grounding rationale_: ci.yml and scripts/run-verification-profile.sh were read directly: the schema-drift job is structurally identical to what this gate needs, confirming this is additive reuse of an already-working mechanism.

### `component-auth-trust-boundary` — Auth / trust boundary + per-WS client-instance-id issuance

_Responsibility_: Record which auth mechanism gates a CmdApprovalRespond/CmdQuestionRespond in Phase 0/1 (existing bearer + single-use WS ticket, extended to mint a per-WS-connection ephemeral client-instance-id) versus what is explicitly deferred to multi-host-gateway.md's user-signed-op chain for Phase R.

- **kind**: existing
- **owner**: server/api gateway maintainers, jointly with multi-host-gateway.md
- **paths**: `src/server/api/auth.go`, `src/server/api/ticket.go`, `plans/multi-host-gateway.md`
- **integration points**:
    - TokenAuth constant-time bearer check (unchanged)
    - ticketStore single-use 30s-TTL WS ticket — extended to also mint (ticket, client-instance-id) pair; id is bound to the WS lifetime and cleared on disconnect
    - multi-host-gateway.md §6.2.3 Client→Host E2E chain and §6.4 revoke semantics as authoritative for cross-host answering
- **test seams**: `src/server/api/auth_test.go`, `src/server/api/ticket_test.go`
- **accepted ADR refs**: `adr-20260724-approval-answerer-identity-per-ws-instance`
- **implements contracts**: `contract-auth-trust-boundary-approval-answering`
- **depends on**: `component-approval-question-proto-wire`

_Grounding rationale_: auth.go and ticket.go were read directly: TokenAuth wraps any http.Handler; ticketStore is keyed by opaque token, not by request kind. Extending mint() to also return a client-instance-id is additive; the bearer scheme is not modified. The identity is local-scope only and does not preempt multi-host-gateway.md §6.2's per-op signed chain, addressing the shared-bearer identity gap flagged by issue-d2-answerer-identity-gap.

### `component-ts-wire-migration-precedent` — Existing hand-written TS wire layer (ADR-0021 migration precedent)

_Responsibility_: Serve as the existing, already-tested TS wire implementation that the plan explicitly says 'may migrate incrementally' to the generated TS SDK, and as the live proof of ADR-0021's fixture-based drift-detection discipline that the successor ADR must preserve.

- **kind**: existing
- **owner**: clients/ui frontend maintainers
- **paths**: `clients/ui/src/wire/client.ts`, `clients/ui/src/wire/server.ts`, `clients/ui/src/wire/codec.ts`, `docs/adr/adr-20260624-0021-frontend-wire-types-hand-written.md`
- **integration points**:
    - clients/ui Zustand store and socket layer consume src/wire/* directly today
    - eventual generated TS SDK is a drop-in replacement candidate for this directory (plan's Shared contracts closing line)
- **test seams**: `clients/ui/src/wire/codec.test.ts`, `clients/ui/src/wire/testdata (fixture round-trip)`
- **accepted ADR refs**: `adr-20260624-0021-frontend-wire-types-hand-written`, `adr-20260724-protocol-cross-language-sdks-supersedes-0021`
- **decision closure reason**: The existing TS wire layer is a migration source referenced by contract-adr0021-supersede-stdlib-preservation and contract-sdk-generation-determinism; its own implementation surface is unchanged by Phase 0/1 (incremental migration lives in a later chunk under the same contracts), so no separate implementation contract is required.

_Grounding rationale_: Directory listing confirms clients/ui/src/wire/{client,server,codec}.ts + testdata/ already exist exactly as ADR-0021 describes.

## Implementation contracts

### `contract-approval-question-envelope` — Wire envelope for ApprovalRequest/QuestionRequest — id semantics, expiry timing, conflict-resolution frame, question free-text answer, and coexistence with ADR-0023 discriminated-union broadcast shape.

- **dimension**: `integration_contract`
- **owner component**: `component-approval-question-proto-wire`
- **requirements**: `FR-P0-01`, `FR-P0-02`, `FR-P0-03`, `FR-P0-04`, `FR-P0-05`, `FR-P0-06`, `FR-P0-07`, `FR-P0-12`, `FR-P1-01`, `FR-P1-12`
- **units**: `u-p0-01-approval-question-types`, `u-p0-03-server-api-approval-endpoints`, `u-p1-01-protocol-events-commands-schemas`
- **ADRs**: `adr-20260724-approval-broadcast-coexists-view-update`, `adr-20260724-approval-expiry-deny-default-no-extension`, `adr-20260724-approval-single-writer-first-commit`

**Invariants**
- Exactly one authoritative resolution ever leaves the state domain per ApprovalRequest ID (invariant relied on by the single-writer contract and the conflict-error observable).
- The wire envelope carries resolving_client_instance_id when status=resolved and never carries it while status=pending; no observer can infer resolver identity from anything other than an authoritative EvtApprovalResolved.

**Decision rules**
- `decision-envelope-question-answer-shape` (total, determinate) — A QuestionRequest is emitted onto the wire in Phase 0/1. → Wire envelope carries a single free-text answer field on the shared kind-discriminated HumanInputRequest shape; structured per-question-type schemas are a future additive schema evolution governed by NFR-05, not a Phase 0/1 wire shape.
- `decision-envelope-broadcast-shape-coexists` (total, determinate) — The daemon encodes an EvtApprovalRequested/Resolved or EvtQuestionRequested/Resolved onto a subscriber's WebSocket. → The frame is emitted as an additional k=... case in the same discriminated-union that already carries tt/et/n/viewUpdate, on the same subscription surface, satisfying ADR-0023 and ADR-20260705-view-update-sessions-only without a new subscription negotiation.

**Observable effects**
- `observable-envelope-conflict-error` (scope=global) — The losing CmdApprovalRespond receives a resolved-by-other error frame whose payload echoes both the authoritative decision and the winning resolving_client_instance_id.
- `observable-envelope-question-answer-field` (scope=global) — A generated SDK deserializes a QuestionRequest carrying answer as a single free-text string field (kind-discriminated HumanInputRequest.free_text).
- `observable-envelope-broadcast-frame-shape` (scope=global) — An EvtApproval*/EvtQuestion* WS frame is a k=... discriminated-union member sharing the same subscription surface as v/tt/et/n.
- `observable-envelope-expiry-broadcast` (scope=global) — When an ApprovalRequest expires, every subscriber observes exactly one EvtApprovalResolved with resolution_reason=expired and decision equal to the value captured at creation time.

**Operational inputs**
- `input-envelope-expiry-clock` (kind=information, preservation=current_lookup) — producer: external `host runtime event-loop tick clock, injected into Reduce as a value per the no-wall-clock-inside-Reduce FC/IS invariant`; source: the runtime shell's tick/event value (mirrors DEvTick in host/state/reduce_tick.go), never read from time.Now() inside Reduce; unavailable → structurally impossible given Reduce's existing signature; reduce_fuzz_test.go guards against any clockless Reduce path

**Failure semantics**
- `failure-envelope-loser-rejected` — class: loser CmdApprovalRespond rejected against already-resolved state; source=unsupported_input; outcome: resolved-by-other error frame is returned; state is unchanged and no additional EvtApprovalResolved is broadcast; recovery=fail_fast (preserves)
- `failure-envelope-malformed-command` — class: malformed CmdApprovalRespond payload; source=unsupported_input; outcome: REST/WS layer rejects with 400 before reaching Reduce; ApprovalRequest state is unchanged; recovery=fail_fast (preserves)

### `contract-approval-question-lifecycle` — ApprovalRequest/QuestionRequest lifecycle in host/state: pending -> resolved | expired | cancelled, with cancel/teardown as a first-class terminal transition that reaps held driver JSON-RPC requests.

- **dimension**: `state_lifecycle`
- **owner component**: `component-approval-question-state-domain`
- **requirements**: `FR-P0-01`, `FR-P0-02`, `FR-P0-06`, `FR-P0-07`, `FR-P0-11`, `NFR-02`, `NFR-06`
- **units**: `u-p0-01-approval-question-types`, `u-p0-02-codex-driver-hold-open`, `u-p0-05-conflict-expiry-cancel-tests`
- **ADRs**: `adr-20260724-approval-lifecycle-teardown-cancel`, `adr-20260724-approval-expiry-deny-default-no-extension`, `adr-20260724-approval-question-state-domain-in-host-state`

**Invariants**
- For every ApprovalRequest r with status in {resolved, expired, cancelled}, r.status never changes again for the lifetime of the process.
- For every session s that reaches teardown, state.PendingApprovals[s] and PendingQuestions[s] are empty within the same Reduce cycle that processes the teardown DEv.

**Decision rules**
- `decision-lifecycle-single-forward-transition` (total, determinate) — A Cmd/DEv event references an ApprovalRequest/QuestionRequest ID r that already exists in state. → Reduce advances r's status only forward (pending->{resolved|expired|cancelled}); a re-triggering driver event for r produces no second pending entry and no backward status change.
- `decision-lifecycle-teardown-reaps` (total, determinate) — A frame/session teardown DEv arrives while r is still pending. → r transitions to cancelled; the held driver JSON-RPC id is drained with a connection-lost error; state.PendingApprovals/PendingQuestions no longer references r.
- `decision-lifecycle-expiry-uses-captured-policy` (total, determinate) — expires_at elapses without a client response. → r transitions to expired; decision = the policy captured on r at creation time (deny for destructive kinds by default) — not re-read from the driver's current policy.

**Observable effects**
- `observable-lifecycle-status-monotonic` (scope=single) — state.PendingApprovals[session] and PendingQuestions[session] read via the atomic State snapshot show each r's status only ever moving forward.
- `observable-lifecycle-teardown-reap` (scope=local) — After a frame teardown DEv is processed, state.PendingApprovals[session]/PendingQuestions[session] is empty for that session and no held JSON-RPC id remains associated with the reaped r.
- `observable-lifecycle-expiry-uses-captured-policy` (scope=local) — For r that expires, the emitted resolution decision equals the value captured on r at creation, regardless of any mid-flight driver-policy mutation.

**Operational inputs**
- `input-lifecycle-expiry-policy` (kind=versioned_state, preservation=immutable_value) — producer: component `component-approval-question-state-domain`; source: the driver subsystem's per-session policy (analog of host/runtime/subsystem/stream.Backend.autoApprove), read once at ApprovalRequest creation time and copied onto the ApprovalRequest as immutable default_decision + expires_at fields; unavailable → If the driver's per-session policy is unset at creation time, deny-by-default for destructive kinds (NFR-02) is used; ApprovalRequest is still created with a captured default_decision so expiry is well-defined.

**Failure semantics**
- `failure-lifecycle-teardown-drain-error` — class: held driver JSON-RPC request drained on teardown before an answer arrives; source=environmental; outcome: The driver receives a connection-lost error reply; the ApprovalRequest is marked cancelled; recovery=none (preserves)
- `failure-lifecycle-duplicate-driver-event` — class: driver emits approval-requested for an ID already in state; source=unsupported_input; outcome: The duplicate DEv is dropped by Reduce; no second ApprovalRequest is created; recovery=fail_fast (preserves)

### `contract-approval-resolution-single-writer` — The single-writer-loop guarantee that at most one commit resolves a given ApprovalRequest — first-writer-wins invariant.

- **dimension**: `concurrency`
- **owner component**: `component-approval-question-state-domain`
- **requirements**: `FR-P0-04`, `FR-P0-05`
- **units**: `u-p0-01-approval-question-types`, `u-p0-05-conflict-expiry-cancel-tests`
- **ADRs**: `adr-20260724-approval-single-writer-first-commit`, `adr-20260724-approval-question-state-domain-in-host-state`

**Invariants**
- For every ApprovalRequest ID r, |{Reduce calls that mutate r.status from pending to resolved}| == 1 across the lifetime of r.

**Decision rules**
- `decision-single-writer-first-commit-wins` (total, determinate) — Two or more CmdApprovalRespond events for the same ApprovalRequest ID are already enqueued to the Reduce loop before either is processed. → The first Reduce call processes the first event and commits the resolution; every subsequent Reduce call for the same ID observes status=resolved and returns the current state plus a rejection effect for the loser.

**Observable effects**
- `observable-single-writer-atomic-snapshot` (scope=single) — host/state's published State snapshot (atomic.Pointer[State]) never shows two different resolution decisions for the same ApprovalRequest ID across any two consecutive reads.

**Failure semantics**
- `failure-single-writer-loser-rejected` — class: second commit against already-resolved state; source=unsupported_input; outcome: Reduce returns the existing state unchanged plus an effect describing the resolved-by-other rejection; no second state mutation occurs.; recovery=fail_fast (preserves)

### `contract-reconnect-authoritative-resubscribe` — A reconnecting/resubscribing client receives the authoritative current set of pending ApprovalRequest/QuestionRequest for its session and converges with every still-connected subscriber on the same final state, additive to ADR-0025/0011/0022's existing reconnect stack.

- **dimension**: `failure_recovery`
- **owner component**: `component-reconnect-event-model-extension`
- **requirements**: `FR-P0-08`, `FR-P1-11`, `NFR-05`
- **units**: `u-p0-06-reconnect-authoritative-state`, `u-p1-01-protocol-events-commands-schemas`

**Invariants**
- For every session S and every pair (reconnecting client c_r, still-connected client c_s), c_r and c_s eventually observe the same set of pending ApprovalRequest/QuestionRequest for S, without permanent divergence.

**Decision rules**
- `decision-reconnect-snapshot-derives-from-state` (total, determinate) — A client resubscribes to session S after a disconnect. → The resubscribe response includes host/state's current authoritative pending ApprovalRequest/QuestionRequest set for S; nothing is inferred from an event log.

**Observable effects**
- `observable-reconnect-snapshot-matches-state` (scope=global) — After a resubscribe completes, the reconnecting client's local view of pending approvals/questions for S is bit-equal to host/state's authoritative set at the snapshot instant; still-connected subscribers' subsequent broadcasts converge them to the same final state.

**Operational inputs**
- `input-reconnect-pending-set` (kind=versioned_state, preservation=snapshot) — producer: component `component-approval-question-state-domain`; source: host/state's atomic.Pointer[State] snapshot, read at the moment the resubscribe request is processed; unavailable → if S no longer exists (evicted/torn down) the existing session-not-found RespErr is returned instead of a stale/empty pending set

**Semantic profiles**
- `profile-reconnect-scope-consistency` (kind=scope_consistency)

**Failure semantics**
- `failure-reconnect-session-gone` — class: session no longer exists at resubscribe time; source=environmental; outcome: existing session-not-found RespErr is returned; no stale/empty pending set; recovery=escalate (preserves)

### `contract-capability-negotiation-policy` — Bundled-vs-remote two-axis compatibility policy: bundled = build-identical, skip negotiation; remote = capability-by-capability degrade via capabilities.schema.json.

- **dimension**: `migration_compatibility`
- **owner component**: `component-capability-negotiation`
- **requirements**: `FR-P1-03`, `FR-P1-04`, `NFR-04`
- **units**: `u-p0-04-compatibility-capability-skeleton`, `u-p1-01-protocol-events-commands-schemas`
- **ADRs**: `adr-20260724-capability-negotiation-bundled-remote-two-axis`

**Invariants**
- A client never issues a command or expects an event tied to a capability the daemon's hello did not declare (fail-closed on absence).

**Decision rules**
- `decision-capability-bundled-match` (total, determinate) — The client's compiled protocol version equals the daemon's build/protocol version at handshake. → Full compatibility is assumed; no per-capability negotiation round-trip is performed.
- `decision-capability-remote-degrade` (total, determinate) — The client's compiled version differs from the daemon's version (skew detected at handshake). → The client compares its compiled capability set against the daemon's capabilities.schema.json hello response; any capability the daemon does not declare is disabled/hidden.
- `decision-capability-missing-version-field` (total, determinate) — A peer omits the version field entirely (pre-Phase-1 daemon or client). → Negotiation treats the peer as lowest-capability; the client degrades every Phase-1-declared capability to disabled; never infers support from absence of information.

**Observable effects**
- `observable-capability-bundled-no-extra-round-trip` (scope=single) — Handshake trace on the bundled axis contains exactly one request/response pair (the existing hello); no per-capability probe follows.
- `observable-capability-remote-degrade` (scope=single) — For any capability C declared by the client but absent from the daemon's capabilities.schema.json response, the client emits no command targeting C and shows C in a documented disabled/unavailable UI state.

**Operational inputs**
- `input-capability-version-pair` (kind=information, preservation=capability_bound) — producer: external `the connecting daemon's build/protocol-version metadata and the connecting client SDK's compiled protocol/ version constant`; source: a capabilities.schema.json-declared hello field exchanged at connection/handshake time; unavailable → peer omits version field -> negotiation treats it as unknown/lowest-capability and every Phase-1 capability is degraded to disabled

**Semantic profiles**
- `profile-capability-contract-evolution` (kind=contract_evolution)
- `profile-capability-cost-bundled-round-trip` (kind=cost_convergence)

**Failure semantics**
- `failure-capability-degraded` — class: capability declared by client but not by daemon; source=environmental; outcome: capability is disabled/hidden in the client UI; associated commands are not issued; recovery=degrade (degrades)

### `contract-sdk-generation-determinism` — Byte-identical generated output for identical protocol/ input across the four target languages, using quicktype pinned via the npm lockfile.

- **dimension**: `integration_contract`
- **owner component**: `component-sdk-generation-pipeline`
- **requirements**: `FR-P1-02`, `NFR-01`
- **units**: `u-p1-02-sdk-codegen-quicktype-pinned`
- **ADRs**: `adr-20260724-sdk-codegen-quicktype-typegen`, `adr-20260724-protocol-message-schema-sot-rest-binding`, `adr-20260724-protocol-cross-language-sdks-supersedes-0021`

**Invariants**
- For every commit SHA in protocol/, the SHA256 of each generated SDK's model tree is a deterministic function of the pinned generator version and the protocol/ commit.

**Decision rules**
- `decision-sdk-quicktype-pinned` (total, determinate) — SDK model generation is invoked for any of C#/Swift/Kotlin/TS. → quicktype is invoked at the npm-lockfile-pinned version with checked-in per-language emit options; no wall-clock, UUID, or unpinned transitive dep may appear in output.

**Observable effects**
- `observable-sdk-byte-identical-output` (scope=global) — Two independent generation runs against the same protocol/ git commit and the same pinned quicktype version produce byte-identical files for every generated-SDK target.

**Failure semantics**
- `failure-sdk-unpinned-drift` — class: unpinned transitive dependency version or embedded generation timestamp; source=internal_violation; outcome: compatibility CI fails: SHA256 diff between two runs is non-zero; recovery=fail_fast (preserves)

### `contract-simulator-recorded-scenario-replay` — Every generated SDK observes the identical typed event/command sequence when driven against the simulator's replay of one recorded scenario, from protocol/simulator/ (a governed extension of plan-20260723-repo-structure.md).

- **dimension**: `user_observability`
- **owner component**: `component-simulator`
- **requirements**: `FR-P1-07`, `FR-P1-08`
- **units**: `u-p1-03-simulator-fixtures-and-server`
- **ADRs**: `adr-20260724-simulator-under-protocol-directory`

**Invariants**
- SDK-observed event sequence equals fixture canonical sequence exactly, for every language and every concurrent replay session.

**Decision rules**
- `decision-simulator-deterministic-replay` (total, determinate) — Any SDK is driven against the simulation server. → The sim server replays the fixture's canonical event sequence bit-identically; SDK-observed sequence equals the fixture.

**Observable effects**
- `observable-simulator-sequence-identity` (scope=global) — For a given recorded scenario, every generated SDK's observed event/command sequence is identical to the fixture's canonical sequence, independent of language or concurrent replay.

**Operational inputs**
- `input-simulator-recorded-scenario-fixture` (kind=versioned_state, preservation=immutable_value) — producer: component `component-simulator`; source: version-controlled recorded event stream file under protocol/simulator/fixtures/; unavailable → an SDK test referencing a missing scenario fails closed (test error), never falls back to a live agent

**Semantic profiles**
- `profile-simulator-scope-consistency` (kind=scope_consistency)

**Failure semantics**
- `failure-simulator-missing-fixture` — class: fixture file missing or renamed; source=environmental; outcome: test errors out; no fallback to a live agent; recovery=fail_fast (preserves)

### `contract-compatibility-ci-drift-gate` — CI fails closed when a generated SDK uses wire surface undeclared in protocol/*, or when a new SDK target skips the shared recorded-scenario suite.

- **dimension**: `failure_recovery`
- **owner component**: `component-compatibility-ci-gate`
- **requirements**: `FR-P1-05`, `FR-P1-06`
- **units**: `u-p1-04-compatibility-ci-profile`

**Invariants**
- compatibility profile default policy is fail; a PR merges only when the scan explicitly returns pass.

**Decision rules**
- `decision-ci-declared-surface` (multi_source, determinate) — The static scan classifies every generated-SDK call site as touching only declared wire fields/commands. → compatibility profile passes; PR may merge.
- `decision-ci-undeclared-surface` (multi_source, determinate) — The scan classifies at least one call site as touching an undeclared wire field/command. → compatibility profile fails; artifact reports the SDK + undeclared surface; PR merge is blocked.
- `decision-ci-inconclusive-scan` (multi_source, inconclusive) — The scan cannot classify a call site (dynamic field name via reflection or string concatenation) as declared or undeclared. → compatibility profile fails by default; a documented allowlist escape hatch exists for confirmed-safe dynamic patterns via a reviewed entry in test-harness/profiles.json.
- `decision-ci-new-sdk-target` (multi_source, determinate) — A PR adds a new generated-SDK target directory under clients/sdk/. → compatibility profile fails unless the new SDK passes the full recorded-scenario suite already gating existing SDK targets.
- `decision-ci-conflicting-scan` (multi_source, conflicting) — Two independent scan passes classify the same call site differently (e.g. schema+SDK sources diverge between scan tools). → compatibility profile fails; artifact reports the divergent classification.
- `decision-ci-unknown-scan` (multi_source, unknown) — The scan cannot resolve a call site's schema mapping (e.g. missing schema for a wire kind referenced by the SDK). → compatibility profile fails; artifact reports the missing schema mapping.

**Observable effects**
- `observable-ci-pass-fail-signal` (scope=global) — For any PR touching a generated SDK, the compatibility CI job's pass/fail result reflects whether every wire field/command that SDK's code references is declared in protocol/* at that PR's HEAD.

**Operational inputs**
- `input-ci-schema-vs-sdk-usage-diff` (kind=information, preservation=snapshot) — producer: external `CI job's schema-vs-SDK-usage scan comparing checked-out protocol/*.schema.json + openapi.yaml against each generated SDK's call-site inventory`; source: git checkout at the PR's HEAD, read by scripts/run-verification-profile.sh pr compatibility; unavailable → if protocol/ is missing or unreadable for a PR that touches generated-SDK code, the gate fails closed

**Semantic profiles**
- `profile-ci-outcome-partition` (kind=outcome_partition)

**Failure semantics**
- `failure-ci-fail-closed` — class: scan classification is inconclusive or protocol/ unreadable; source=environmental; outcome: compatibility profile exits non-zero; artifact reports the reason; recovery=fail_fast (preserves)

### `contract-auth-trust-boundary-approval-answering` — Which auth mechanism gates CmdApprovalRespond/CmdQuestionRespond in Phase 0/1, and the explicit local/remote scope split with multi-host-gateway.md — plus the per-WS-connection ephemeral client-instance-id issuance that gives `decided_by` a named producer.

- **dimension**: `security_boundary`
- **owner component**: `component-auth-trust-boundary`
- **requirements**: `FR-P0-03`, `FR-P0-10`, `FR-P0-12`
- **units**: `u-p0-03-server-api-approval-endpoints`, `u-p0-03b-ticketstore-client-instance-id`
- **ADRs**: `adr-20260724-approval-answerer-identity-per-ws-instance`

**Invariants**
- For every EvtApprovalResolved with status=resolved, resolving_client_instance_id is non-empty and equals the id minted for the answerer's WS ticket.
- A client-instance-id is never reused across WS connections; a disconnect + reconnect produces a fresh id.
- Cross-host answering is out of scope for Phase 0/1 and is answered by multi-host-gateway.md's user-signed-op chain; this design does not preempt or duplicate that mechanism.

**Decision rules**
- `decision-auth-bearer-plus-ticket-admits` (total, determinate) — A CmdApprovalRespond/CmdQuestionRespond request presents a valid bearer token (REST) or a valid single-use WS ticket (WS upgrade). → The existing TokenAuth/ticketStore middleware admits the request; the WS-bound client-instance-id (from mint) is attached to the command payload before Reduce sees it.
- `decision-auth-reject-unauthenticated` (total, determinate) — The request presents no bearer, an expired bearer, or an already-consumed/expired WS ticket. → The request is rejected with 401 before the approval domain observes it; ApprovalRequest state is unchanged.

**Observable effects**
- `observable-auth-admitted-decided-by` (scope=global) — For every admitted CmdApprovalRespond, the resulting EvtApprovalResolved carries resolving_client_instance_id equal to the id minted for the answerer's WS ticket.
- `observable-auth-rejected-no-state-change` (scope=local) — An unauthenticated/malformed CmdApprovalRespond produces no state change and no broadcast; the ApprovalRequest remains pending.

**Operational inputs**
- `input-auth-bearer-or-ticket` (kind=authority, preservation=capability_bound) — producer: external `the answering client's held bearer token or single-use WS ticket`; source: Authorization header (REST) or /ws?ticket= query parameter consumed once via ticketStore.consume; unavailable → no token/ticket -> 401 before the approval domain is reached; ApprovalRequest remains pending
- `input-auth-client-instance-id` (kind=authority, preservation=capability_bound) — producer: component `component-auth-trust-boundary`; source: extended ticketStore.mint returns (ticket, client-instance-id); the id is bound to the WS connection's lifetime and threaded onto CmdApprovalRespond/CmdQuestionRespond by the WS handler, or supplied via a session header on REST calls; unavailable → on WS disconnect, the associated id is dropped; any queued REST CmdApprovalRespond bearing the dropped id is rejected without touching state; the ApprovalRequest remains pending for other clients

**Failure semantics**
- `failure-auth-401-preserves-state` — class: unauthenticated CmdApprovalRespond; source=unsupported_input; outcome: 401 before approval domain; ApprovalRequest unchanged; recovery=fail_fast (preserves)
- `failure-auth-instance-id-dropped` — class: REST call presents a client-instance-id whose WS has already closed; source=environmental; outcome: request rejected as unknown-identity; ApprovalRequest unchanged; recovery=fail_fast (preserves)

### `contract-adr0021-supersede-stdlib-preservation` — The new ADR supersedes ADR-0021 for cross-language generated clients while the Go-side stdlib-only wire/persistence rule survives unchanged.

- **dimension**: `migration_compatibility`
- **owner component**: `component-sdk-generation-pipeline`
- **requirements**: `FR-P1-10`
- **units**: `u-p1-02-sdk-codegen-quicktype-pinned`
- **ADRs**: `adr-20260724-protocol-cross-language-sdks-supersedes-0021`

**Invariants**
- For every Go artifact reachable from src/host/proto/... or a Phase 1 generated Go helper (if any), all imports are stdlib packages.

**Decision rules**
- `decision-adr0021-go-side-stdlib-only` (total, determinate) — The generation pipeline emits any Go-side artifact (generated helper or hand-written wire/persistence type). → The Go artifact's import graph contains no non-stdlib dependency; the existing stdlib-only build/lint gate rejects violations.

**Observable effects**
- `observable-adr0021-go-import-graph-stdlib-only` (scope=global) — The Go module's import graph for every wire/persistence type (hand-written or generated) contains no non-stdlib dependency, verified by the same lint gate that already checks this for hand-written code.

**Semantic profiles**
- `profile-adr0021-contract-evolution` (kind=contract_evolution)

**Failure semantics**
- `failure-adr0021-non-stdlib-import` — class: generation pipeline introduces a non-stdlib Go import; source=internal_violation; outcome: make lint / go build fails; PR blocked; recovery=fail_fast (preserves)

### `contract-deep-link-uri-shape` — agent-grid:// URI shape declared in deep-links.schema.json adopts the shapes recorded in plans/remote-control-mobile-session-deep-link.md (session/<id>, approval/<id>), so native shells never hand-parse URI strings.

- **dimension**: `integration_contract`
- **owner component**: `component-protocol-schema-repo`
- **requirements**: `FR-P1-09`
- **units**: `u-p1-01-protocol-events-commands-schemas`
- **ADRs**: `adr-20260724-deep-link-shape-adopts-remote-control-plan`

**Invariants**
- Native shells never hand-parse agent-grid:// URIs; they invoke the SDK helper.

**Decision rules**
- `decision-deep-link-adopts-remote-control-shape` (total, determinate) — deep-links.schema.json declares a URI parseable path. → Path shapes are exactly agent-grid://session/<id> and agent-grid://approval/<id>, adopting plans/remote-control-mobile-session-deep-link.md; every generated SDK exposes typed parse/construct helpers.

**Observable effects**
- `observable-deep-link-typed-helper` (scope=global) — Each generated SDK exposes a parse(agent-grid://...) -> {kind, id} function that never returns raw string parts and that every native shell uses.

**Failure semantics**
- `failure-deep-link-malformed` — class: malformed agent-grid:// URI; source=unsupported_input; outcome: SDK helper returns a typed error; caller decides handling; recovery=fail_fast (preserves)

## Ordered milestones (chunks)

### `chunk-p0-01-approval-question-skeleton`

- **components**: `component-approval-question-state-domain`, `component-approval-question-proto-wire`

#### unit `u-p0-01-approval-question-types`

- **objective**: Introduce ApprovalRequest and QuestionRequest as durable types under src/host/state/ (following the existing SubsystemApproval pattern in driver_iface.go), the pending maps on State, the reducer for CmdApprovalRespond/CmdQuestionRespond/expiry/cancel, and the paired Evt*/Cmd* sum-type members in src/host/proto/.
- **output**: Go source: src/host/state/{approval_request.go, question_request.go, reduce_approval.go, reduce_question.go}, edits to src/host/state/event.go and reduce_event.go dispatch, additions to src/host/proto/{event.go, response.go, request.go}; matching unit tests in src/host/state/ and src/host/proto/.
- **tool guidance**: Read src/host/state/driver_iface.go for SubsystemApproval/SubsystemEventKind naming, src/host/state/reduce_messages.go for the single-reply commit precedent, src/host/proto/event.go and response.go for the ServerEvent/Response sum-type extension pattern. Do NOT modify src/host/runtime/subsystem/stream/event.go in this unit — that is unit u-p0-02.
- **boundaries**: State types + reducer + proto wire only. Server API surface and codex driver hold-open live in later chunks. No wire fixtures are regenerated until the proto shapes are final.
- **files touched**: `src/host/state/approval_request.go`, `src/host/state/question_request.go`, `src/host/state/reduce_approval.go`, `src/host/state/reduce_question.go`, `src/host/state/reduce_event.go`, `src/host/state/event.go`, `src/host/state/reduce_event_test.go`, `src/host/proto/event.go`, `src/host/proto/response.go`, `src/host/proto/request.go`
- **acceptance**:
    - cd src && go test ./host/state -run 'TestApproval|TestQuestion' passes.
    - cd src && go test ./host/proto passes; new Evt*/Cmd*/Resp* types satisfy their marker interfaces.
    - make lint (or GOCACHE=/tmp/gocache-agent-grid GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make lint) passes.
- **contracts**: `contract-approval-question-lifecycle`, `contract-approval-question-envelope`, `contract-approval-resolution-single-writer`

### `chunk-p0-02-codex-driver-hold-open`

- **depends on**: `chunk-p0-01-approval-question-skeleton`
- **components**: `component-approval-question-state-domain`

#### unit `u-p0-02-codex-driver-hold-open`

- **objective**: Replace the synchronous auto-accept in src/host/runtime/subsystem/stream/event.go handleRequest for MethodItemCommandExecutionRequestApproval/MethodItemFileChangeRequestApproval with a hold-open path: emit SubsystemApprovalRequested carrying the held RequestID, store the id in the backend so a later resolve can reply, and add the sibling ItemRequestUserInput hold-open path.
- **output**: Go source: src/host/runtime/subsystem/stream/event.go, helpers.go (or a new held_requests.go), stream_test.go additions.
- **tool guidance**: Read src/host/runtime/subsystem/stream/event.go handleRequest verbatim before editing; the current path calls b.conn.Reply(id, result) and emits SubsystemApprovalResolved in the same function body. Preserve the existing threadID -> frameID lookup and autoApprove semantics for the driver-side default_decision captured onto ApprovalRequest at creation.
- **boundaries**: Backend-side hold + resolve path only. The gateway REST/WS surface and the ticket-store id extension are chunks p0-03 and p0-03b. Do not change host/state (already done in u-p0-01).
- **files touched**: `src/host/runtime/subsystem/stream/event.go`, `src/host/runtime/subsystem/stream/helpers.go`, `src/host/runtime/subsystem/stream/stream_test.go`
- **acceptance**:
    - cd src && go test ./host/runtime/subsystem/stream passes; new tests cover hold + external resolve + teardown drain.
    - The existing SubsystemApprovalResolved fake behavior test either updates or splits so a real hold-then-answer path is exercised (per project rule 'if FakeVsReal fails fix the fake, not the assertion').
    - make vet passes; make lint passes.
- **contracts**: `contract-approval-question-lifecycle`, `contract-approval-question-envelope`

### `chunk-p0-03-server-api-gateway-extension`

- **depends on**: `chunk-p0-01-approval-question-skeleton`
- **components**: `component-server-api-approval-gateway`, `component-auth-trust-boundary`

#### unit `u-p0-03-server-api-approval-endpoints`

- **objective**: Wire approval/question through server/api: encode new Evt*/Resp* in wire.go (satisfying ADR-0023 k=... discriminated union), add REST POST /api/sessions/{id}/approvals/{approvalId} (CmdApprovalRespond), the paired question endpoint, and thread the client-instance-id from u-p0-03b onto every Cmd*.
- **output**: Go source: src/server/api/{wire.go, mux.go, gateway.go, viewupdate.go}; scenario tests in src/server/api/mux_scenario_test.go and gateway_view_update_test.go.
- **tool guidance**: Read src/server/api/wire.go encodeServerEvent switch (existing v/tt/et/n/viewUpdate/notification cases) and src/server/api/mux.go route registration verbatim before editing. Follow the {k:...} discriminated-union convention; extend wire_fixtures_test.go rather than starting a new fixture suite.
- **boundaries**: REST + WS surface only. ticket.go extension for client-instance-id is a separate unit (u-p0-03b) so the two changes can be reviewed independently. Reconnect authoritative snapshot is chunk p0-06.
- **files touched**: `src/server/api/wire.go`, `src/server/api/mux.go`, `src/server/api/gateway.go`, `src/server/api/viewupdate.go`, `src/server/api/mux_scenario_test.go`, `src/server/api/gateway_view_update_test.go`, `src/server/api/wire_fixtures_test.go`
- **acceptance**:
    - cd src && go test ./server/api -run 'TestApproval|TestQuestion|TestWireFixturesApproval' passes.
    - wire_fixtures round-trip is stable; new EvtApproval*/EvtQuestion* frames validate against the existing k=... encoder shape.
    - make lint passes.
- **contracts**: `contract-approval-question-envelope`, `contract-approval-resolution-single-writer`, `contract-auth-trust-boundary-approval-answering`

#### unit `u-p0-03b-ticketstore-client-instance-id`

- **objective**: Extend src/server/api/ticket.go ticketStore.mint to also mint a per-WS-connection ephemeral client-instance-id, bind it to the ticket lifetime, drop it on WS close, and expose it to the WS handler for threading onto CmdApprovalRespond/CmdQuestionRespond. Add a session header (X-Client-Instance-ID) that REST callers may use to reference the same id.
- **output**: Go source: src/server/api/ticket.go, auth.go (if needed for header wiring), gateway.go for WS handler binding; unit tests in ticket_test.go, auth_test.go, and one scenario in gateway_inbound_test.go covering id not-reused-across-connections.
- **tool guidance**: Read src/server/api/{ticket.go, auth.go} verbatim first; the extension is additive (mint returns (token, id)); do not modify TokenAuth's constant-time compare.
- **boundaries**: Identity mint + threading only. Do not attempt to persist the id, share it across daemons, or extend to a passkey scheme (that is Phase R territory owned by multi-host-gateway.md §6.2).
- **files touched**: `src/server/api/ticket.go`, `src/server/api/auth.go`, `src/server/api/gateway.go`, `src/server/api/ticket_test.go`, `src/server/api/auth_test.go`, `src/server/api/gateway_inbound_test.go`
- **acceptance**:
    - cd src && go test ./server/api -run 'TestTicket|TestAuth|TestApprovalDecidedBy' passes.
    - TestTicketClientInstanceIDNotReused asserts id_1 != id_2 across reconnect.
    - TestApprovalDecidedByClientInstance asserts resolving_client_instance_id populates on EvtApprovalResolved.
- **contracts**: `contract-auth-trust-boundary-approval-answering`

### `chunk-p0-04-compatibility-capability-skeleton`

- **depends on**: `chunk-p0-03-server-api-gateway-extension`
- **components**: `component-capability-negotiation`, `component-behavior-contracts-repo`

#### unit `u-p0-04-compatibility-capability-skeleton`

- **objective**: Draft the compatibility-policy skeleton in contracts/compatibility-policy.md (bundled/remote two-axis) and the capabilities hello field in the server/api handshake, without landing the full generated SDK negotiation yet.
- **output**: Markdown: contracts/compatibility-policy.md skeleton. Go source: src/server/api/gateway.go hello extension (version + capabilities placeholder). Placeholder JSON Schema stub in protocol/capabilities.schema.json.
- **tool guidance**: Read plan-20260723-windows-shell-design.md §3.6 DaemonSupervisor to align the bundled-axis semantics; read protocol/README.md and contracts/README.md so the new files match the scaffold naming.
- **boundaries**: Skeleton and hello wire only. The full per-capability degrade behavior in generated clients lands in chunk p1-05 clients/ui hosted-mode prep and later phases.
- **files touched**: `contracts/compatibility-policy.md`, `protocol/capabilities.schema.json`, `src/server/api/gateway.go`, `src/server/api/gateway_hello_test.go`
- **acceptance**:
    - docs lint passes for the new contracts/compatibility-policy.md.
    - cd src && go test ./server/api -run TestCapabilityBundledSingleRoundTrip passes with the bundled-axis behavior.
    - protocol/capabilities.schema.json is a valid JSON Schema draft 2020-12 file.
- **contracts**: `contract-capability-negotiation-policy`

### `chunk-p0-05-conflict-expiry-cancel-tests`

- **depends on**: `chunk-p0-02-codex-driver-hold-open`, `chunk-p0-03-server-api-gateway-extension`
- **components**: `component-approval-question-state-domain`, `component-server-api-approval-gateway`

#### unit `u-p0-05-conflict-expiry-cancel-tests`

- **objective**: Add the T1 scenario tests exercising two-client conflict (first-writer-wins), expiry-with-captured-policy (TOCTOU-free), and frame/session teardown cancel (no leaked goroutine/map entry), tying the sandbox release path (d1e3a8c4) into the same test.
- **output**: Go tests: src/server/api/{mux_scenario_test.go additions, gateway_view_update_test.go additions}, src/host/state/reduce_teardown_test.go, src/host/state/reduce_expiry_test.go, src/host/runtime/subsystem/stream/stream_test.go additions.
- **tool guidance**: Reuse the existing fake-agent scenario harness (src/server/api/testsupport/fakeagents); pattern after mux_scenario_test.go for two-client races.
- **boundaries**: Tests only; production code changes only where a test seam (interface, injected clock) is required and could not be added in earlier chunks.
- **files touched**: `src/host/state/reduce_teardown_test.go`, `src/host/state/reduce_expiry_test.go`, `src/host/runtime/subsystem/stream/stream_test.go`, `src/server/api/mux_scenario_test.go`, `src/server/api/gateway_view_update_test.go`
- **acceptance**:
    - cd src && go test ./host/state -run 'TestApprovalLifecycleTeardownReap|TestApprovalExpiryTOCTOU|TestApprovalExpiryDecisionCaptureAtCreation|TestApprovalSingleWriterFirstCommit|FuzzApprovalLifecycleMonotonic|FuzzApprovalSingleWriter' passes.
    - cd src && go test ./server/api -run 'TestApprovalConflict|TestApprovalRespond401' passes.
    - make test-race passes for the concurrency-sensitive subtrees named in Makefile.
- **contracts**: `contract-approval-question-lifecycle`, `contract-approval-resolution-single-writer`, `contract-approval-question-envelope`, `contract-auth-trust-boundary-approval-answering`

### `chunk-p0-06-reconnect-authoritative-state`

- **depends on**: `chunk-p0-03-server-api-gateway-extension`, `chunk-p0-05-conflict-expiry-cancel-tests`
- **components**: `component-reconnect-event-model-extension`

#### unit `u-p0-06-reconnect-authoritative-state`

- **objective**: Extend the resubscribe/hello payload with the session's current authoritative pending ApprovalRequest/QuestionRequest set (FR-P0-08); prove convergence between the reconnecting client's snapshot and the still-connected client's broadcasts (scope=global).
- **output**: Go source: src/server/api/{gateway.go, transcript.go, viewupdate.go} additions; src/host/proto/response.go Resp* additions if not already present; scenario tests in src/server/api/gateway_inbound_test.go.
- **tool guidance**: Read ADR-0025, ADR-0011, ADR-0022 verbatim; the extension is additive. Follow the wire-fixtures pipeline (adr-20260705-wire-fixtures-pipeline) so the old-client-decodes-new-fields regression holds.
- **boundaries**: Additive hello-payload extension only. No full event-sourced replay log. No changes to the two-step close or subscribe-retry mechanisms themselves.
- **files touched**: `src/server/api/gateway.go`, `src/server/api/transcript.go`, `src/server/api/viewupdate.go`, `src/host/proto/response.go`, `src/server/api/gateway_inbound_test.go`, `clients/ui/src/wire/codec.test.ts`
- **acceptance**:
    - cd src && go test ./server/api -run 'TestReconnectAuthoritativeResubscribe|TestReconnectSnapshotBroadcastConvergence' passes.
    - cd clients/ui && npm run test:unit passes; wire round-trip for the new hello payload is stable.
    - wire_fixtures old-fixture-decodes-new-schema regression passes.
- **contracts**: `contract-reconnect-authoritative-resubscribe`

### `chunk-p0-07-terminology-docs`


#### unit `u-p0-07-terminology-docs-pass`

- **objective**: Docs-only pass separating runtime layers (host/*, server/*) from user-facing clients (clients/*) per FR-P0-09; sweep living docs (ARCHITECTURE.md, AGENTS.md, docs/design/design-host.md, plan headers) for any residual 'client/* = daemon' phrasing that survived the M2 rename.
- **output**: Markdown edits only; no code changes.
- **tool guidance**: grep for 'client/' (with slash) across docs/ and ARCHITECTURE.md/AGENTS.md; contrast against docs/design/design-host.md which is the post-rename living doc.
- **boundaries**: Terminology only. Do not restructure any content; do not touch code.
- **files touched**: `ARCHITECTURE.md`, `AGENTS.md`, `docs/design/design-host.md`
- **acceptance**:
    - docs lint passes; no doc references 'client/' as the daemon layer.
    - grep -rn 'client/' docs/design/ ARCHITECTURE.md AGENTS.md returns only intentional references to user-facing clients or historical ADRs.
- **decision closure**: Docs-only sweep; no observable behavior change; the invariance is trivial (docs pass or fail lint deterministically).

### `chunk-p1-01-protocol-schemas`

- **depends on**: `chunk-p0-01-approval-question-skeleton`, `chunk-p0-03-server-api-gateway-extension`, `chunk-p0-04-compatibility-capability-skeleton`, `chunk-p0-06-reconnect-authoritative-state`
- **components**: `component-protocol-schema-repo`, `component-behavior-contracts-repo`

#### unit `u-p1-01-protocol-events-commands-schemas`

- **objective**: Author the initial protocol/*.schema.json + openapi.yaml matching Phase 0's finalized wire (approval, question, resubscribe, capabilities, deep-links) and the compatibility-policy skeleton; land contracts/{approval-contract.md, question-contract.md, reconnect-contract.md, deep-links helpers} as behavior contracts.
- **output**: protocol/{openapi.yaml, events.schema.json, commands.schema.json, capabilities.schema.json, deep-links.schema.json, notifications.schema.json}; contracts/{approval-contract.md, question-contract.md, reconnect-contract.md, compatibility-policy.md (fleshed out), handoff-contract.md skeleton}.
- **tool guidance**: Adopt OpenAPI 3.1 + JSON Schema draft 2020-12; author schemas by extracting shapes from src/host/proto (Phase 0 output) rather than inventing new shapes.
- **boundaries**: Schemas + behavior docs only. SDK generation lands in u-p1-02. Simulator server lands in u-p1-03.
- **files touched**: `protocol/openapi.yaml`, `protocol/events.schema.json`, `protocol/commands.schema.json`, `protocol/capabilities.schema.json`, `protocol/deep-links.schema.json`, `protocol/notifications.schema.json`, `contracts/approval-contract.md`, `contracts/question-contract.md`, `contracts/reconnect-contract.md`, `contracts/compatibility-policy.md`, `contracts/handoff-contract.md`
- **acceptance**:
    - Each protocol/*.schema.json validates as a JSON Schema draft 2020-12 file.
    - openapi.yaml validates as OpenAPI 3.1 (via a pinned validator invoked from the compatibility profile).
    - docs lint passes for all new contracts/*.md.
    - Round-trip test: src/server/api/wire_fixtures_test.go validates existing fixtures against protocol/*.schema.json.
- **contracts**: `contract-approval-question-envelope`, `contract-deep-link-uri-shape`, `contract-capability-negotiation-policy`, `contract-reconnect-authoritative-resubscribe`

### `chunk-p1-02-sdk-codegen-quicktype`

- **depends on**: `chunk-p1-01-protocol-schemas`
- **components**: `component-sdk-generation-pipeline`

#### unit `u-p1-02-sdk-codegen-quicktype-pinned`

- **objective**: Stand up the quicktype model-generation pipeline pinned via the npm lockfile, producing typed models + serializers for C#/Swift/Kotlin/TypeScript under clients/sdk/{csharp,swift,kotlin,ts}, plus the hand-written thin transport skeleton per language (REST calls per openapi.yaml, WS framing). Each SDK carries only transport + typed messages + validation + version negotiation; no presentation behavior. Assert Go-side stdlib-only survives the pipeline.
- **output**: Generator config (scripts/generate-sdks.sh + checked-in per-language quicktype emit options; quicktype pinned in the npm lockfile), Makefile targets, clients/sdk/{csharp,swift,kotlin,ts} generated model trees + hand-written transport files; CI wiring in .github/workflows/ci.yml + test-harness/profiles.json.
- **tool guidance**: Pin quicktype by exact version in the npm lockfile and record it in the compatibility profile; forbid any generator invocation outside the pinned wrapper script. Do NOT run generation as part of `go test`; keep it under the compatibility profile only.
- **boundaries**: Generation and CI plumbing only. Existing clients/ui hand-written wire migration lives in u-p1-05.
- **files touched**: `clients/sdk/csharp/`, `clients/sdk/swift/`, `clients/sdk/kotlin/`, `clients/sdk/ts/`, `Makefile`, `.github/workflows/ci.yml`, `test-harness/profiles.json`, `scripts/run-verification-profile.sh`, `scripts/generate-sdks.sh`
- **acceptance**:
    - scripts/run-verification-profile.sh pr compatibility exits zero on a clean tree.
    - make lint passes; Go-side stdlib-only depguard rule still enforces (no third-party Go imports from generated helpers).
    - Two independent generation runs against the same protocol/ commit produce byte-identical output for all four SDK model trees.
    - Per-language serializer round-trip against the wire fixtures passes (Codable / kotlinx.serialization / System.Text.Json / TS JSON fidelity spot-check).
- **contracts**: `contract-sdk-generation-determinism`, `contract-adr0021-supersede-stdlib-preservation`
- **implementation decisions**: `quicktype-emit-options-choice` — closed at landing (alternative-quicktype-default-emit; see §Implementation decisions)

### `chunk-p1-03-simulator`

- **depends on**: `chunk-p1-01-protocol-schemas`, `chunk-p1-02-sdk-codegen-quicktype`
- **components**: `component-simulator`

#### unit `u-p1-03-simulator-fixtures-and-server`

- **objective**: Land protocol/simulator/ with (a) deterministic fixtures with fixed IDs/timestamps, (b) a recorded event stream captured from the fake-agent scenario harness, and (c) a simulation server that replays the stream on protocol/'s REST+WS surface. Drive each generated SDK against the sim server in CI.
- **output**: protocol/simulator/{fixtures/, recordings/, server/} + a small Go binary or Node script for the sim server (pick whichever aligns best with the pinned CI env; ADR ADR_SIMULATOR pins protocol/simulator/ as the location, not the language of the server).
- **tool guidance**: Capture the initial recording via src/platform/agent/fakecodex/ replay + src/server/api/testsupport/fakeagents scenario; the .jsonl format from fakecodex is the precedent.
- **boundaries**: Fixture + server only. Extending the fixture library to more scenarios happens per-PR as needs arise; the exit criterion is one working end-to-end scenario driving all four SDKs.
- **files touched**: `protocol/simulator/fixtures/`, `protocol/simulator/recordings/`, `protocol/simulator/server/`, `test-harness/profiles.json`
- **acceptance**:
    - The recorded scenario drives all four SDKs identically; each SDK's observed sequence equals the fixture (scripts/run-verification-profile.sh pr compatibility passes).
    - Parallel cross-SDK replay against a shared sim server instance stays isolated (no cross-SDK frame leakage).
- **contracts**: `contract-simulator-recorded-scenario-replay`
- **implementation decisions**: `sim-server-language-choice` — closed at landing (alternative-sim-server-go; see §Implementation decisions)

### `chunk-p1-04-compatibility-ci-gate`

- **depends on**: `chunk-p1-02-sdk-codegen-quicktype`, `chunk-p1-03-simulator`
- **components**: `component-compatibility-ci-gate`

#### unit `u-p1-04-compatibility-ci-profile`

- **objective**: Wire the compatibility CI gate as a new test-harness/profiles.json group + a new .github/workflows/ci.yml job following the existing schema-drift job pattern. Enforce fail-closed on undeclared SDK surface, inconclusive scan, and new-SDK-target skipping the shared recorded-scenario suite.
- **output**: test-harness/profiles.json additions; .github/workflows/ci.yml compatibility job; scan tooling under scripts/ or the sim-server tree.
- **tool guidance**: Reuse scripts/run-verification-profile.sh's existing pass/fail/skip semantics and AG_HARNESS_PROFILE_ARTIFACT convention; do not introduce a bespoke CI runner (NFR-03).
- **boundaries**: CI wiring only. The behavior contracts describing what counts as declared/undeclared already exist in protocol/*.schema.json from u-p1-01.
- **files touched**: `test-harness/profiles.json`, `.github/workflows/ci.yml`, `scripts/run-verification-profile.sh`
- **acceptance**:
    - scripts/run-verification-profile.sh pr compatibility fails on a synthetic PR that references an undeclared field.
    - The compatibility CI job appears in .github/workflows/ci.yml PR runs; wall-clock <= 3x the existing schema-drift job's.
- **contracts**: `contract-compatibility-ci-drift-gate`

### `chunk-p1-05-clients-ui-hosted-mode-prep`

- **depends on**: `chunk-p1-02-sdk-codegen-quicktype`
- **components**: `component-ts-wire-migration-precedent`

#### unit `u-p1-05-clients-ui-hosted-mode-prep`

- **objective**: Land the minimum clients/ui changes that let the eventual generated TS SDK migrate incrementally: mode flag, wire-adapter seam behind clients/ui/src/wire/, and one round-trip test proving the generated TS SDK can substitute the hand-written codec without regressing existing UX.
- **output**: TypeScript source: clients/ui/src/wire/adapter.ts (new), clients/ui/src/wire/codec.ts (modified to route through adapter), unit test in codec.test.ts.
- **tool guidance**: Do NOT delete clients/ui/src/wire/{client,server,codec}.ts in this unit; the migration is incremental and ADR-0021's fixture-drift-test discipline must keep proving the round-trip.
- **boundaries**: Adapter + smoke test only. Cutover to generated TS SDK is a Phase 2 task under plan-20260723-windows-shell-design.md's hosted-mode scope.
- **files touched**: `clients/ui/src/wire/adapter.ts`, `clients/ui/src/wire/codec.ts`, `clients/ui/src/wire/codec.test.ts`
- **acceptance**:
    - cd clients/ui && npm run test:unit passes; existing fixture round-trip is preserved.
    - The adapter seam compiles against both the hand-written codec and (a stub of) the generated TS SDK.
- **decision closure**: This unit is scaffolding for a Phase 2 cutover; its observable is only that the existing test suite still passes and the adapter compiles against both codec paths. Neither owner, wire contract, failure behavior, nor migration semantics changes; it is closed against contract-sdk-generation-determinism and contract-adr0021-supersede-stdlib-preservation (which the u-p1-02 unit already owns).

## Implementation decisions (closed at landing 2026-07-24)

### `quicktype-emit-options-choice` — decided: `alternative-quicktype-default-emit`

- **decision**: Default per-language emit idioms — TS `--just-types --prefer-unions --prefer-const-values`, C# System.Text.Json (`--features complete`), Kotlin kotlinx.serialization, Swift Codable public structs.
- **why not strict**: Fail-on-unknown-field decoders contradict NFR-05's additive-only schema evolution — an old client must tolerate fields added by a newer daemon, so unknown-field tolerance is the correct compatibility behavior, not a laxity.
- **evidence**: `clients/sdk/quicktype-emit.json` (checked-in emit options), `clients/sdk/package-lock.json` (quicktype pinned at 23.0.171).
- **preserved contracts** (invariance honored): `contract-sdk-generation-determinism`, `contract-adr0021-supersede-stdlib-preservation` — byte-identical generation at the pin and Go stdlib-only both hold; the wire-fixture round-trip gates the emit set.

### `sim-server-language-choice` — decided: `alternative-sim-server-go`

- **decision**: Sim server is a small stdlib-only Go binary under protocol/simulator/server/ built by the existing Makefile.
- **rationale**: The recording sources (fakecodex .jsonl replay, fakeagents scenarios) are Go-side, so the format's producer and replayer share a language; compat CI needs npm only for quicktype itself.
- **evidence**: `protocol/simulator/server/main.go`.
- **preserved contracts** (invariance honored): `contract-simulator-recorded-scenario-replay` — deterministic fixture replay is language-independent; each SDK's observed sequence equals the fixture.

## Constraints

- Go-side wire/persistence types remain stdlib-only (AGENTS.md 'Wire-format and persistence types must remain stdlib-only'; ADR-0021's Go-side rule survives its own supersession).
- depguard import boundaries (platform/* / host/* / orchestrator/* / server/*, ARCHITECTURE.md's Layer Structure) must not be crossed by new domain, reconnect extension, or SDK generation.
- File/function length heuristics (500 lines / 80 lines; host/state/reduce_*.go dispatch tables exempt) apply to reduce_approval.go / reduce_question.go.
- Existing ADR-0011/ADR-0022/ADR-0025/ADR-0023/ADR-20260705 reconnect + broadcast behavior must not regress; the Phase 1 event-model extension is additive per FR-P1-11 and FR-P1-12.
- The new compatibility CI gate must run inside the existing test-harness/profiles.json + scripts/run-verification-profile.sh mechanism (NFR-03).
